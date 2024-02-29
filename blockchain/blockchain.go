package blockchain

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dcnetio/dc/config"
	gsrpc "github.com/dcnetio/go-substrate-rpc-client"
	"github.com/dcnetio/go-substrate-rpc-client/types"
	"github.com/dcnetio/go-substrate-rpc-client/types/codec"
	"github.com/dcnetio/gothreads-lib/core/thread"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	mbase "github.com/multiformats/go-multibase"
)

var log = logging.Logger("dcmanager")

// Threaddb log information
type Loginfo struct {
	Logid []byte
	Size  uint64
}

type StoreunitInfo struct {
	//File size
	Size int64
	//type file type 1:file 2:threaddb
	Utype uint32
	//Backup node id list
	Peers map[string]struct{}
	//List of users who own the file
	Users map[string]struct{}
	Logs  map[string]uint64 //When utype is 2, store the log file information list
}

// File information storage structure on the blockchain
type BlockStoreunitInfo struct {
	//Backup node id list
	Peers []string
	//List of users who own the file
	Users []types.AccountID
	//File size
	Size int64
	//type file type 1:file 2:threaddb
	Utype uint32
	Logs  []Loginfo
}
type BlockPeerInfo struct { //Node information
	Req_account   types.AccountID
	Stash         types.AccountID
	Total_space   uint64
	Free_space    uint64
	Status        uint32
	Report_number uint32
	Staked_number uint32 //Staking time after node access request
	Reward_number uint32
	Ip_address    types.Bytes
}

// enclaveId 的数据结构
type EnclaveIdInfo struct {
	Blockheight uint32
	EnclaveId   []byte //Hexadecimal encoding of activities authorized by the enclavid technical committee
	Signature   []byte //Hexadecimal representation of signature sign(EnclaveId)
}

var gChainApi *gsrpc.SubstrateAPI
var gMeta *types.Metadata

// Get connected to the blockchain
func GetChainApi() (chainApi *gsrpc.SubstrateAPI, meta *types.Metadata, err error) {
	if gChainApi != nil && gMeta != nil {
		chainApi = gChainApi
		meta = gMeta
		return
	}
	//Connect to the blockchain
	chainApi, err = gsrpc.NewSubstrateAPI(config.RunningConfig.ChainWsUrl)
	if err != nil {
		log.Errorf("Cann't connect to blockchain,please check chainWsUrl in /opt/dcnetio/etc/manage_config.yaml is correct.err: %v", err)
		return
	}
	meta, err = chainApi.RPC.State.GetMetadataLatest()
	if err != nil {
		log.Errorf("Cann't get meta from blockchain,err: %v", err)
		return
	}
	gChainApi = chainApi
	gMeta = meta
	return
}

// Reset connection to blockchain
func ResetChainApi() {
	gChainApi = nil
	gMeta = nil
}

// Get the latest version of dc node program information from the blockchain
func GetConfigedDcStorageInfo() (programInfo *config.DcProgram, err error) {
	//Randomly select a blockchain proxy to connect to
	var chainApi *gsrpc.SubstrateAPI
	var meta *types.Metadata
	ctx := context.Background()
	//Connect to the blockchain
	chainApi, meta, err = GetChainApi()
	if err != nil {
		return nil, err
	}
	//Wait for blockchain synchronization to complete
	waitForChainSyncCompleted(ctx, chainApi, meta)
	//Get information corresponding to the program version on the current blockchain
	programInfo, err = getRecommendProgram(chainApi, meta)
	if err != nil {
		log.Errorf("Cann't get program info from blockchain,err: %v", err)
		return nil, err
	}
	return
}

// Get program version information on the current blockchain
func getRecommendProgram(chainApi *gsrpc.SubstrateAPI, meta *types.Metadata) (programInfo *config.DcProgram, err error) {
	key, err := types.CreateStorageKey(meta, "DcNode", "DcProgram")
	if err != nil {
		return
	}
	programInfo = &config.DcProgram{}
	ok, err := chainApi.RPC.State.GetStorageLatest(key, programInfo)
	if err != nil { //Blockchain error
		return
	}
	if !ok {
		err = fmt.Errorf("get Program fail")
		return
	}
	return
}

// Wait for blockchain synchronization to complete
func waitForChainSyncCompleted(ctx context.Context, chainApi *gsrpc.SubstrateAPI, meta *types.Metadata) (err error) {
	health, err := chainApi.RPC.System.Health()
	if err != nil || health.IsSyncing {
		for {
			if err == nil && !health.IsSyncing {
				break
			}
			ticker := time.NewTicker(time.Second)

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				health, err = chainApi.RPC.System.Health()
				if err == nil {
					continue
				}
			}
		}
	}
	return
}

// Get the storage location information of the specified cid from the blockchain
func GetPeerAddrsForCid(sCid string) (fileSize int64, peerAddrInfos []peer.AddrInfo, err error) {
	//Randomly select a blockchain proxy to connect to
	var chainApi *gsrpc.SubstrateAPI
	var meta *types.Metadata
	ctx := context.Background()
	//连接区块链
	chainApi, meta, err = GetChainApi()
	if err != nil {
		return 0, nil, err
	}
	//Wait for blockchain synchronization to complete
	fmt.Println("Wait for blockchain syncing complete")
	waitForChainSyncCompleted(ctx, chainApi, meta)
	fmt.Println("Blockchain syncing completed")
	//Prompt to start obtaining storage location information
	fmt.Println("Start to get storage location information")
	return getPeerAddrsForCid(sCid, chainApi, meta)
}

// Object status query (including file and database status)
func getPeerAddrsForCid(sCid string, chainApi *gsrpc.SubstrateAPI, meta *types.Metadata) (fileSize int64, peerAddrInfos []peer.AddrInfo, err error) {
	if chainApi == nil {
		return 0, nil, fmt.Errorf("chain proxy not init")
	}
	if len(sCid) == 0 {
		return 0, nil, fmt.Errorf("invalid key")
	}
	fileIdBytes, _ := codec.Encode([]byte(sCid))
	key, err := types.CreateStorageKey(meta, "DcNode", "Files", fileIdBytes)
	if err != nil {
		return
	}
	var blockStroreunitInfo BlockStoreunitInfo
	ok, err := chainApi.RPC.State.GetStorageLatest(key, &blockStroreunitInfo)
	if err != nil { //Blockchain error
		return
	}
	if !ok {
		err = fmt.Errorf("get object fail")
		return
	}
	for _, pid := range blockStroreunitInfo.Peers {
		addrInfo, err := GetPeerAddrInfo(pid, chainApi, meta)
		if err != nil {
			continue
		}
		peerAddrInfos = append(peerAddrInfos, addrInfo)
	}
	fileSize = blockStroreunitInfo.Size
	return
}

// Get node address information
func GetPeerAddrInfo(peerid string, chainApi *gsrpc.SubstrateAPI, meta *types.Metadata) (addrInfo peer.AddrInfo, err error) {
	peerIdBytes, _ := codec.Encode([]byte(peerid))
	// //Get node information based on pubkey
	key, err := types.CreateStorageKey(meta, "DcNode", "Peers", peerIdBytes)
	if err != nil {
		return
	}
	var blockPeerInfo BlockPeerInfo
	ok, err := chainApi.RPC.State.GetStorageLatest(key, &blockPeerInfo)
	if err != nil {
		return
	}
	if !ok {
		return
	}
	pAddrInfo, err := peer.AddrInfoFromString(string(blockPeerInfo.Ip_address))
	if err != nil {
		return
	}
	addrInfo = *pAddrInfo
	return

}

// Get a list of trusted storage nodes
func GetTrustStoragePeers() (peerAddrInfos []peer.AddrInfo, err error) {
	var chainApi *gsrpc.SubstrateAPI
	var meta *types.Metadata
	//Connect to the blockchain
	chainApi, meta, err = GetChainApi()
	if err != nil {
		return
	}
	key, err := types.CreateStorageKey(meta, "DcNode", "TrustedStorageNodes")
	if err != nil {
		return
	}
	var trustPeers []string
	ok, err := chainApi.RPC.State.GetStorageLatest(key, &trustPeers)
	if err != nil {
		return
	}
	if !ok {
		return
	}
	var addrInfo peer.AddrInfo
	for _, pidInfo := range trustPeers {
		if strings.Contains(pidInfo, "/ip") { //Carrying address information by default
			pAddrInfo, err1 := peer.AddrInfoFromString(string(pidInfo))
			if err1 != nil {
				err = err1
				return
			}
			addrInfo = *pAddrInfo
		} else {
			addrInfo, err = GetPeerAddrInfo(pidInfo, chainApi, meta)
			if err != nil {
				continue
			}
		}
		peerAddrInfos = append(peerAddrInfos, addrInfo)
	}
	return
}

// Determine whether the encalve ID is valid
func IfEnclaveIdValid(ctx context.Context, enclaveId string) (validFlag bool) {
	var chainApi *gsrpc.SubstrateAPI
	var meta *types.Metadata
	//连接区块链
	chainApi, meta, err := GetChainApi()
	if err != nil {
		return false
	}
	key, err := types.CreateStorageKey(meta, "DcNode", "EnclaveIds")
	if err != nil {
		return false
	}
	var enclaveIdInfos []EnclaveIdInfo //Signature of each enclaveid
	ok, err := chainApi.RPC.State.GetStorageLatest(key, &enclaveIdInfos)
	if err != nil { //Blockchain error
		fmt.Fprintln(os.Stderr, err.Error())
		return false
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "get EnclaveIds object fail")
		return false
	}
	//Generate the pubkey of the technical committee
	_, commitPubkeyBytes, err := mbase.Decode(config.CommitBasePubkey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		return
	}
	cpubkey, err := crypto.UnmarshalEd25519PublicKey(commitPubkeyBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "client request secret fail, err: %v\n", err)
		return
	}
	commitPubkey := thread.NewLibp2pPubKey(cpubkey)
	for _, enclaveIdInfo := range enclaveIdInfos {
		if enclaveId != string(enclaveIdInfo.EnclaveId) {
			continue
		}
		_, signature, err := mbase.Decode(string(enclaveIdInfo.Signature))
		if err != nil {
			continue
		}
		ok, err := commitPubkey.Verify(enclaveIdInfo.EnclaveId, signature)
		if err != nil || !ok {
			continue
		}
		return true
	}
	return false
}

// Get the number of online nodes
func GetOnchainPeerNumber(ctx context.Context) (num uint32, err error) {
	chainApi, meta, err := GetChainApi()
	if err != nil {
		return
	}
	//Get node information based on pubkey
	key, err := types.CreateStorageKey(meta, "DcNode", "OnchainPeerNumber")
	if err != nil {
		return
	}
	_, err = chainApi.RPC.State.GetStorageLatest(key, &num)
	return
}
