package util

//从ipfs网络中下载文件

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dariubs/percent"
	"github.com/dcnetio/dc/blockchain"
	sym "github.com/dcnetio/gothreads-lib/crypto/symmetric"
	"github.com/dcnetio/gothreads-lib/go-libp2p-pubsub-rpc/peer/mdns"
	ipfslite "github.com/dcnetio/ipfs-lite"
	gproto "github.com/gogo/protobuf/proto"
	"github.com/ipfs/boxo/ipld/merkledag"
	ufsio "github.com/ipfs/boxo/ipld/unixfs/io"
	pb "github.com/ipfs/boxo/ipld/unixfs/pb"
	"github.com/ipfs/boxo/mfs"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

const dcFileHead = "$$dcfile$$"
const (
	FileDealStatusSuccess = iota
	FileDealStatusToIpfs
	FileDealStatusTransmit
	FileDealStatusFail
	FileDealStatusErr
)

type FileTransmit interface {
	//FileDealStatus 0: Success 1: Converting to ipfs object 2: File transfer in progress 3: Transfer failed 4: Exception
	UpdateTransmitSize(status int, size uint64)
}

type TransmitObj struct {
	TotalSize  uint64
	LogFlag    bool
	preLogTime uint64
}

func (tObj *TransmitObj) UpdateTransmitSize(status int, size uint64) {
	if status == FileDealStatusSuccess {
		fmt.Print("Downloading... 100% complete\r\n")
		if tObj.LogFlag {
			log.Infof("Downloading... 100% complete")
		}
		return
	} else if status == FileDealStatusFail {
		fmt.Print("Downloading... fail to complete \r\n")
		if tObj.LogFlag {
			log.Infof("Downloading... fail to complete")
		}
		return
	}
	currTime := uint64(time.Now().Unix())
	if currTime-tObj.preLogTime < 2 { //Print every 2 seconds
		return
	}
	tObj.preLogTime = currTime
	if tObj.TotalSize > 0 { //Show download percentage and size
		downloadPercent := percent.PercentOf(int(size), int(tObj.TotalSize))
		fmt.Printf("Downloading... %.2f%% complete, downloaded/totalsize: %d/%d   \r\n", downloadPercent, size, tObj.TotalSize)
		if tObj.LogFlag {
			log.Infof("Downloading... %.2f%% complete, downloaded/totalsize: %d/%d   ", downloadPercent, size, tObj.TotalSize)
		}
	} else { //Show only download size
		fmt.Printf("Downloading... %d complete \r\n", size)
		if tObj.LogFlag {
			log.Infof("Downloading... %d complete ", size)
		}
	}
}

// DownloadFromIpfs pulls files or folders from the network to the local based on cid
func DownloadFromIpfs(fcid, secret, savePath string, addrInfos []peer.AddrInfo, timeout time.Duration, fileTransmit FileTransmit) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	if timeout == 0 {
		cancel()
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()
	ds := ipfslite.NewInMemoryDatastore()
	hostKey, _, err := newIPFSHostKey()
	if err != nil {
		return
	}
	port, err := GetAvailablePort()
	if err != nil {
		return
	}
	hostAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port))
	if err != nil {
		return err
	}
	h, dht, err := ipfslite.SetupLibp2p(
		ctx,
		hostKey,
		nil,
		[]multiaddr.Multiaddr{hostAddr},
		ds,
		dht.ModeAuto,
		ipfslite.Libp2pOptionsExtra...,
	)
	if err != nil {
		fmt.Println(err)
		log.Error(err)
		return
	}
	defer func() {
		h.Close()
		dht.Close()
	}()

	//Connect to the trusted storage node of the DC network and add it to the bootpeers
	trustPeers, err := blockchain.GetTrustStoragePeers()
	if err != nil {
		fmt.Println(err)
		log.Error(err)
		return err
	}
	bootPeers := trustPeers
	bootPeers = append(bootPeers, addrInfos...)
	lite, err := ipfslite.New(ctx, ds, nil, h, dht, nil)
	if err != nil {
		fmt.Println(err)
		log.Error(err)
		return
	}

	err = lite.Bootstrap(bootPeers)
	if err != nil {
		return err
	}
	//Enable mdns service for discovery within lan (local area network)
	err = mdns.Start(ctx, h)
	if err != nil {
		fmt.Println("mdns start error:", err)
	}
	c, _ := cid.Decode(fcid)
	ioReader, err := lite.GetFile(ctx, c)
	if err != nil {
		if errors.Is(err, ufsio.ErrIsDir) { //It's a folder, download folder
			err := pullAndDownloadFolder(ctx, lite, c, savePath, secret, fileTransmit)
			if err != nil {
				return err
			}
			return nil
		} else {
			return err
		}
	}
	defer ioReader.Close()
	err = downloadFile(ctx, ioReader, savePath, secret, fileTransmit)
	return

}

// DownloadFile download file
func downloadFile(ctx context.Context, ioReader ufsio.ReadSeekCloser, savePath string, secret string, fileTransmit FileTransmit) error {
	completeFlag := false
	//Determine whether the file exists
	_, err := os.Stat(savePath) //Determine whether the file exists
	if err == nil {             //The file exists, delete the original file first
		os.Remove(savePath)
	}
	if ioReader == nil {
		return fmt.Errorf("ioReader is nil")
	}
	//The file does not exist and needs to be downloaded
	var symKey *sym.Key
	if secret != "" {
		symKey, err = sym.FromString(secret)
		if err != nil {
			return err
		}
	}
	var wg sync.WaitGroup
	rp, wp := io.Pipe()
	wg.Add(1)
	go func() { //Read data
		defer wp.Close()
		var readSize uint64 = 0
		waitBuffer := []byte{} //The introduction of cache is mainly to prevent the read from returning before it is full, resulting in unsuccessful decryption.
		headBuf := make([]byte, 32)
		bufLen := 3 << 20
		if symKey != nil {
			bufLen = 3<<20 + 28
		}
		buf := make([]byte, bufLen)
		headDealFlag := false
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if !headDealFlag {
				n, rerr := ioReader.Read(headBuf)
				if n > 0 {
					waitBuffer = append(waitBuffer, headBuf[:n]...)
					if len(waitBuffer) < 32 {
						headBuf = make([]byte, 32-len(waitBuffer))
						continue
					} else { //Determine whether it is the file header of DC network storage
						headDealFlag = true
						if bytes.Equal([]byte(dcFileHead), waitBuffer[0:10]) { //It is a file stored on the DC network, and the 32-byte additional header needs to be removed (the combination of the DC file flag and the user pubkey hash value)
							waitBuffer = waitBuffer[32:]
						}
					}
				} else if rerr != nil {
					if rerr == io.EOF {
						completeFlag = true
					}
					if len(waitBuffer) > 0 { //Determine whether there is still data in the buffer that has not been completely written
						if symKey != nil { //Decryption is required
							content, err := symKey.Decrypt(waitBuffer)
							if err != nil {
								return
							}
							wp.Write(content)
						} else {
							wp.Write(waitBuffer)
						}
					}
					return
				}
				continue
			}

			n, rerr := ioReader.Read(buf)
			if n > 0 {
				waitBuffer = append(waitBuffer, buf[:n]...)
				if len(waitBuffer) < bufLen {
					continue
				}
				content := waitBuffer[:bufLen]
				waitBuffer = waitBuffer[bufLen:]
				if symKey != nil { //Decryption is required
					content, err = symKey.Decrypt(content)
					if err != nil {
						return
					}
				}
				readSize += (uint64)(n) //Cumulative read file size
				_, werr := wp.Write(content)
				if werr != nil {
					return

				}

			} else if rerr != nil { //
				if rerr == io.EOF {
					completeFlag = true
				}
				if len(waitBuffer) > 0 { //Determine whether there is still data in the buffer that has not been completely written
					if symKey != nil { //Decryption is required
						content, err := symKey.Decrypt(waitBuffer)
						if err != nil {
							return
						}
						wp.Write(content)
					} else {
						wp.Write(waitBuffer)
					}
				}

				return
			}

		}
	}()
	go func() { //Add data to local
		defer rp.Close()
		defer wg.Done()

		f, err := os.Create(savePath) //Create a file
		if err != nil {
			return
		}
		w := bufio.NewWriter(f) //Create a new Writer object
		defer w.Flush()
		var dealedSize uint64 = 0
		buf := make([]byte, 3<<20)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			n, rerr := rp.Read(buf)
			if n > 0 {
				content := buf[:n]
				//Write content to file
				wn, err := w.Write(content)
				if err != nil {
					return
				}
				dealedSize += (uint64)(wn) //Cumulative file size written
				if fileTransmit != nil {
					fileTransmit.UpdateTransmitSize(FileDealStatusTransmit, dealedSize)
				}
			} else {
				if rerr != nil { //
					if fileTransmit != nil {
						if completeFlag {
							fileTransmit.UpdateTransmitSize(FileDealStatusSuccess, dealedSize)
						} else {
							fileTransmit.UpdateTransmitSize(FileDealStatusFail, dealedSize)
						}
					}
					return
				}
			}
		}
	}()

	wg.Wait()
	if !completeFlag {
		return fmt.Errorf("download file fail")
	}
	return nil

}

// pullAndDownloadFolder Pull all files in the folder from the network to the local based on cid
func pullAndDownloadFolder(ctx context.Context, p *ipfslite.Peer, c cid.Cid, savePath string, secret string, fileTransmit FileTransmit) error {
	v := new(merkledag.ProgressTracker)
	pCtx := v.DeriveContext(ctx)
	fetchResChan := make(chan struct{})
	top := merkledag.NodeWithData(folderPBData([]byte(c.String())))
	top.SetLinks([]*ipld.Link{
		{
			Name: "root",
			Cid:  c,
		},
	})
	rt, err := mfs.NewRoot(pCtx, p.DAGService, top, nil)
	if err != nil {
		return err
	}
	// get this dir
	topi, err := rt.GetDirectory().Child("root")
	if err != nil {
		return err
	}

	//make dir
	err = os.MkdirAll(savePath, os.ModePerm)
	if err != nil {
		return err
	}
	// get all files
	go func() {
		defer close(fetchResChan)
		err = downloadFolderFromIpfs(pCtx, p, topi.(*mfs.Directory), savePath, secret, fileTransmit)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-fetchResChan:
		return nil
	}
}

// downloadFolderFromIpfs Pull all files in the folder from the network to the local based on cid
func downloadFolderFromIpfs(ctx context.Context, p *ipfslite.Peer, dir *mfs.Directory, savePath string, secret string, fileTransmit FileTransmit) error {
	err := dir.ForEachEntry(ctx, func(nl mfs.NodeListing) error {
		if nl.Type == int(mfs.TFile) {
			// get file
			fid, err := cid.Decode(nl.Hash)
			if err != nil {
				return err
			}
			ioReader, err := p.GetFile(ctx, fid)
			if err != nil {
				return err
			}
			defer ioReader.Close()
			err = downloadFile(ctx, ioReader, filepath.Join(savePath, nl.Name), secret, fileTransmit)
			if err != nil {
				return err
			}
		} else {
			subDir, err := dir.Child(nl.Name)
			if err != nil {
				return err
			}
			//mkdir
			dirPath := filepath.Join(savePath, nl.Name)
			err = os.MkdirAll(dirPath, os.ModePerm)
			if err != nil {
				return err
			}
			err = downloadFolderFromIpfs(ctx, p, subDir.(*mfs.Directory), dirPath, secret, fileTransmit)
			if err != nil {
				return err
			}

		}
		return nil
	})
	return err
}

// FolderPBData returns Bytes that represent a Directory.
func folderPBData(pathData []byte) []byte {
	pbfile := new(pb.Data)
	typ := pb.Data_Directory
	pbfile.Type = &typ
	pbfile.Data = pathData

	data, err := gproto.Marshal(pbfile)
	if err != nil {
		//this really shouldnt happen, i promise
		panic(err)
	}
	return data
}

// 获取可用端口
func GetAvailablePort() (int, error) {
	address, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("0.0.0.0:%d", 0))
	if err != nil {
		return 0, err
	}
	listener, err := net.ListenTCP("tcp", address)
	if err != nil {
		address, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:0", "0.0.0.0"))
		if err != nil {
			return 0, err
		}

		listener, err = net.ListenTCP("tcp", address)
		if err != nil {
			return 0, err
		}
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func newIPFSHostKey() (crypto.PrivKey, []byte, error) {
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		return nil, nil, err
	}
	key, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	return priv, key, nil
}
