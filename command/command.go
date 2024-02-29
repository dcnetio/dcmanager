package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dcnetio/dc/blockchain"
	"github.com/dcnetio/dc/config"
	"github.com/dcnetio/dc/util"
	"github.com/dcnetio/go-substrate-rpc-client/types/codec"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	goversion "github.com/hashicorp/go-version"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-ps"
	mbase "github.com/multiformats/go-multibase"
)

const dcStorageListenPort = 6667
const dcUpgradeListenPort = 6666

const nodeContainerName = "dcstorage"
const chainContainerName = "dcchain"
const upgradeContainerName = "dcupgrade"
const teeReportServerContainerName = "teereportserver"
const pccsContainerName = "dcpccs"
const nodeVolume = "dcstorage"
const pccsVolume = "dcpccs"
const teeReportServerVolume = "teereportserver"
const daemonFilepath = "/opt/dcnetio/data/.dcupgradedaemon"
const runCmdStateFilepath = "/opt/dcnetio/data/.cmdstate" //Record the status of currently running commands

const serverhost = "127.0.0.1"

const chainDataDir = "/opt/dcnetio/chaindata"

func ShowHelp() {

	fmt.Println("dcmanager version ", config.GetVersion, "EPC Size: ", util.GetEpcSize())
	fmt.Println("usage:  dc command [options]")
	fmt.Println("commands:")
	fmt.Println(" config                                  start config chainmode")
	fmt.Println(" start {storage|chain|pccs|all}          start service with service_name")
	fmt.Println("                                         \"storage\": start dcstorage service")
	fmt.Println("                                         \"chain\": start dcchain service")
	fmt.Println("                                         \"pccs\": start local pccs service")
	fmt.Println("                                         \"all\": start dcstorage and dcchain service")
	fmt.Println(" stop {storage|chain|pccs|all}           stop service  with service_name")
	fmt.Println("                                         \"storage\": stop dcstorage service")
	fmt.Println("                                         \"chain\": stop dcchain service")
	fmt.Println("                                         \"pccs\": stop local pccs service")
	fmt.Println("                                         \"all\": stop dcstorage and dcchain service")
	fmt.Println(" status {storage|chain|pccs|all}         check dc daemon status and  service status")
	fmt.Println("                                         \"storage\": check dcstorage service status")
	fmt.Println("                                         \"chain\": check dcchain service status")
	fmt.Println("                                         \"pccs\": check local pccs service status")
	fmt.Println("                                         \"all\": check dcstorage and dcchain service status")
	fmt.Println(" log  {storage|chain|upgrade|pccs} [num] show running service logs last \"num\" lines")
	fmt.Println("                                         \"storage\":  show dcstorage container running log")
	fmt.Println("                                         \"chain\":  show dcchain container running log")
	fmt.Println("                                         \"upgrade\":  show dcupgrade container running log")
	fmt.Println("                                         \"pccs\":  show local pccs  running log")
	fmt.Println(" uniqueid                                show soft version and sgx enclaveid ")
	fmt.Println(" peerinfo                                show local running peer info")
	fmt.Println(" memusage                                show memory usage of local running peer")
	fmt.Println(" blockgc                                 send block gc command to dcsotrage")
	fmt.Println(" checksum  filepath                      generate  sha256 checksum for file in the \"filepath\"")
	fmt.Println(" get cid [--name][--timeout][--secret]   get file from dc net with \"cid\" ")
	fmt.Println("                                         \"--name\": file to save name")
	fmt.Println("                                         \"--timeout\":  wait seconds for file to complete download")
	fmt.Println("                                         \"--secret\":  file decode secret with base32 encoded")
	fmt.Println(" pccs_api_key [apikey]                   get or set pccs api key,if no apikey set,will show current apikey")
	fmt.Println(" rotate-keys                             generate new storage session keys")
}

var log = logging.Logger("dcmanager")

func ConfigCommandDeal() {
	//Determine whether it has been configured
	if config.RunningConfig.ValidatorFlag != "" {
		fmt.Print("chainmode is already configed,continue will clean the chain data,continue?(y/n): ")
		var input string
		for {
			input = ""
			fmt.Scanln(&input)
			input = strings.ToLower(input)
			if input != "y" && input != "n" {
				fmt.Print("please input y or n : ")
				continue
			} else {
				break
			}
		}
		if input == "y" {
			//remove dcchain docker
			err := removeDockerContainer(chainContainerName)
			if err != nil {
				log.Error(err)
				return
			}
			//Remove dcchain data directory
			err = os.RemoveAll(chainDataDir)
			if err != nil {
				log.Error(err)
				return
			}
		} else {
			return
		}

	}
	// Officially start configuring chainmode, prompting the user whether to use the current node as a verification node.
	fmt.Print("do you want to config this node as validator node?(y/n): ")
	var input string
	for {
		input = ""
		fmt.Scanln(&input)
		input = strings.ToLower(input)
		if input != "y" && input != "n" {
			fmt.Print("please input y or n : ")
			continue
		} else {
			break
		}
	}
	if input == "y" {
		config.RunningConfig.ValidatorFlag = "enable"
		config.RunningConfig.ChainSyncMode = "full"
	} else {
		config.RunningConfig.ValidatorFlag = "disable"
	}
	//Determine whether pccsapikey has been configured
	if config.RunningConfig.PccsKey == "" {
		fmt.Print("please input pccs api key: ")
		var input string
		for {
			input = ""
			fmt.Scanln(&input)
			if input == "" {
				fmt.Print("please input pccs api key: ")
				continue
			} else {
				break
			}
		}
		config.RunningConfig.PccsKey = input
	}
	//Save configuration
	if err := config.SaveConfig(config.RunningConfig); err != nil {
		fmt.Fprintf(os.Stdout, "save config fail,err: %v\n", err)
		return
	}
	// Prompt that the configuration is complete and automatically start dcchain
	fmt.Println("config chainmode success,starting dcchain service")
	err := startDcChain()
	if err == nil {
		showContainerLog(chainContainerName, 100)
	} else {
		log.Error(err)
	}
}

func StartCommandDeal() {
	if len(os.Args) < 3 {
		ShowHelp()
		return
	}
	switch os.Args[2] {
	case "storage":
		err := startDcStorageNode()
		if err == nil {
			setDcnodeCmdState("start")
			showContainerLog(nodeContainerName, 100)
		} else {
			log.Error(err)
		}

	case "chain":
		err := startDcChain()
		if err == nil {
			showContainerLog(chainContainerName, 100)
		}
	case "pccs":
		err := runPccsInDocker()
		if err == nil {
			showContainerLog(pccsContainerName, 100)
		}

	case "all":
		startDcChain()
		err := startDcStorageNode()
		if err == nil {
			setDcnodeCmdState("start")
			showContainerLog(nodeContainerName, 100)
		} else {
			log.Error(err)
		}

	default:
		ShowHelp()
	}

}

func StopCommandDeal() {
	if len(os.Args) < 3 {
		ShowHelp()
		return
	}
	switch os.Args[2] {
	case "storage":
		setDcnodeCmdState("stop")
		stopDcnodeInDocker()
	case "chain":
		stopDcchainInDocker()
	case "pccs":
		stopPccsInDocker()
	case "all":
		setDcnodeCmdState("stop")
		stopDcnodeInDocker()
		stopDcchainInDocker()
		stopPccsInDocker()
	default:
		ShowHelp()
	}
}

// Get the running status of the program
func StatusCommandDeal() {
	if len(os.Args) < 2 {
		ShowHelp()
		return
	}
	secondArgs := "all"
	if len(os.Args) > 2 {
		secondArgs = os.Args[2]
	}
	dcStatus, _ := checkDcDeamonStatusDc()
	fmt.Println("daemon status:", statusToString(dcStatus))
	switch secondArgs {
	case "storage":
		nodeStatus, _ := checkDcnodeStatus()
		fmt.Println("dcstorage status:", statusToString(nodeStatus))
	case "chain":
		chainStatus, _ := checkDcchainStatus()
		fmt.Println("dcchain status:", statusToString(chainStatus))
	case "pccs":
		pccsStatus, _ := checkPccsStatus()
		fmt.Println("pccs status:", statusToString(pccsStatus))
	case "all":
		nodeStatus, _ := checkDcnodeStatus()
		fmt.Println("dcstorage status:", statusToString(nodeStatus))
		chainStatus, _ := checkDcchainStatus()
		fmt.Println("dcchain status:", statusToString(chainStatus))
		pccsStatus, _ := checkPccsStatus()
		fmt.Println("pccs status:", statusToString(pccsStatus))
	default:
		ShowHelp()
	}
}

func statusToString(status bool) string {
	if status {
		return "running"
	} else {
		return "stopped"
	}
}

// Print the real-time running log of a specific program
func LogCommandDeal() { //
	if len(os.Args) < 3 {
		ShowHelp()
		return
	}
	tnum := 100
	if len(os.Args) > 3 {
		tnum, _ = strconv.Atoi(os.Args[3])
	}
	switch os.Args[2] {
	case "storage":
		showContainerLog(nodeContainerName, tnum)
	case "chain":
		showContainerLog(chainContainerName, tnum)
	case "upgrade":
		showContainerLog(upgradeContainerName, tnum)
	case "pccs":
		showContainerLog(pccsContainerName, tnum)
	default:
		ShowHelp()
	}
}

// Upgrade command processing
func UpgradeCommandDeal() {
	if len(os.Args) > 2 {
		if os.Args[2] == "daemon" { //Enter daemon mode, automatically download and update dcstorage, and set it to boot and restart.
			//Determine whether the daemon has been started
			flag, _ := checkDcDeamonStatusDc()
			if flag {
				log.Info("daemon is already running")
				return
			}
			//fork new process to run in deamon mode
			if os.Getppid() != 1 {
				// Convert the execution file path in the command line parameters into a usable path
				cmd := exec.Command(os.Args[0], "upgrade", "daemon")
				cmd.SysProcAttr = &syscall.SysProcAttr{
					Setpgid: true,
					Pgid:    0,
				}
				cmd.Start() // Start executing a new process without waiting for the new process to exit
				os.Exit(0)
			} else {
				daemonCommandDeal()
			}
		} else {
			ShowHelp()
		}
	}
}

// 获取指定enclave的enclaveid
func UniqueIdCommandDeal() {
	if len(os.Args) < 2 {
		ShowHelp()
		return
	}
	fmtStr := "dcstorage version: %s,enclaveid: %s\ndcupgrade version: %s,enclaveid: %s\n"
	fmtStrStorage := "dcstorage version: %s,enclaveid: %s\n"
	upgradeVersion := ""
	upgradeEnclaveId := ""
	//Get the version and enclaveid information of dcupgrade
	//Determine whether dcupgrade is running
	upgradeStatus, _ := checkDcDeamonStatusDc()
	if upgradeStatus {
		var err error
		upgradeVersion, upgradeEnclaveId, err = getVersionByHttpGet(dcUpgradeListenPort)
		if err != nil {
			log.Error(err)
		}
	}
	storageVersion := ""
	storageEnclaveId := ""
	//Get dcstorage version and enclaveid information
	//Determine whether dcstorage is running
	nodeStatus, _ := checkDcnodeStatus()
	if nodeStatus {
		var err error
		storageVersion, storageEnclaveId, err = getVersionByHttpGet(dcStorageListenPort)
		if err != nil {
			log.Error(err)
		}
	}
	fmt.Println("dcmanager version ", config.GetVersion)
	if upgradeVersion != "" {
		fmt.Printf(fmtStr, storageVersion, storageEnclaveId, upgradeVersion, upgradeEnclaveId)
	} else {
		fmt.Printf(fmtStrStorage, storageVersion, storageEnclaveId)
	}
}

// Get node information running locally
func PeerInfoCommandDeal() {
	peerid, pubkey, walletAddr, err := getPeerInfoByHttpGet()
	if err != nil {
		fmt.Println("get peerinfo failed,please make sure storage service is running")
		return
	}
	_, account, err := mbase.Decode(pubkey)
	if err != nil {
		fmt.Println("decode pubkey failed")
	}
	hexAccount := codec.HexEncodeToString(account)
	fmt.Printf("peer ID: %s\npeer Pubkey: %s\npeer Account: %s\npeer Wallet Address: %s\n", peerid, pubkey, hexAccount, walletAddr)
}

// Get the current memory usage of the node running locally
func MemoryUsageCommandDeal() {
	memUsageInfo, err := getMemoryUsageByHttpGet()
	if err != nil {
		fmt.Println("get memory usage failed,please make sure storage service is running")
		return
	}
	fmt.Printf("dcstorage memory usage: %s\n", memUsageInfo)
}

// Manually start the block recycling command of dcstorage
func BlockGcCommandDeal() {
	err := sendBlockGcCommand()
	if err != nil {
		fmt.Println("block gc command deal failed,please make sure storage service is running")
		return
	}
	fmt.Println("dcstorage gc command has been sent")
}

// Generate hash check code of file
func ChecksumCommandDeal() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: dc checksum <file>")
		os.Exit(1)
	}
	for _, filename := range os.Args[2:] {
		checksum, err := util.Sha256sum(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "checksum: %v\n", err)
			continue
		}
		fmt.Printf("%s\t%s\n", checksum, filename)
	}
}

// Download files from dc network
func GetFileFromIpfsCommandDeal() {
	if len(os.Args) < 3 {
		ShowHelp()
		return
	}
	cid := os.Args[2]

	ipfsCmd := flag.NewFlagSet("ipfs", flag.ExitOnError)
	name := ipfsCmd.String("name", cid, "")
	timeout := ipfsCmd.Int("timeout", 600, "")
	secret := ipfsCmd.String("secret", "", "")
	if len(os.Args) > 3 {
		ipfsCmd.Parse(os.Args[3:])
	}
	tTimeout := time.Duration(*timeout) * time.Second
	//Query the node where the file exists from the blockchain based on cid
	fileSize, addrInfos, err := blockchain.GetPeerAddrsForCid(cid)
	if err != nil || len(addrInfos) == 0 {
		fmt.Fprintf(os.Stderr, "Failed to get file with cid:%s \n", cid)
		return
	}
	tObj := &util.TransmitObj{
		TotalSize: uint64(fileSize),
		LogFlag:   false,
	}
	fmt.Println("get storage location information success")
	util.DownloadFromIpfs(cid, *secret, *name, addrInfos, tTimeout, tObj)
}

type SessionKeyRes struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  string `json:"result"`
}

func RotateKeyCommandDeal() (sessionKey string, err error) {
	//check dcchain status
	status, err := checkDcchainStatus()
	if err != nil {
		fmt.Println("dcchain is not running")
		return "", err
	}
	if !status {
		fmt.Println("dcchain is not running")
		return "", errors.New("dcchain is not running")
	}
	if config.RunningConfig.ValidatorFlag != "enable" {
		fmt.Println("current node isn't configed as validator, please config node as validator by running command \" dc config\" first")
		return "", errors.New("only validator can rotate key")
	}
	//make http request to dcchain
	chainRpcUrl := fmt.Sprintf("http://%s:%d", serverhost, config.RunningConfig.ChainRpcListenPort)
	postData := `{"id":1, "jsonrpc":"2.0", "method": "author_rotateKeys", "params":[]}`
	res, err := util.HttpPost(chainRpcUrl, []byte(postData))
	if err != nil {
		fmt.Fprintf(os.Stderr, "request author_rotateKeys fail,  err: %v\n", err)
		return
	}
	var sessionKeyRes SessionKeyRes
	err = json.Unmarshal(res, &sessionKeyRes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse res author_rotateKeys failed,  err: %v\n", err)
		return
	}
	sessionKey = sessionKeyRes.Result
	fmt.Fprintf(os.Stdout, "session key: %s\n", sessionKey)
	return
}

// Setting or display of pccs apikey
func PccsApiKeyCommandDeal() {
	if len(os.Args) >= 3 { //Need to set pccsapikey
		apiKey := os.Args[2]
		config.RunningConfig.PccsKey = apiKey
		if err := config.SaveConfig(config.RunningConfig); err != nil {
			fmt.Fprintf(os.Stdout, "save config fail,err: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stdout, "set api key success\n")
	} else {
		fmt.Fprintf(os.Stdout, "pccs apikey: %s\n", config.RunningConfig.PccsKey)
	}
}

// Get the running status of dcstorage
func checkDcnodeStatus() (status bool, err error) {
	status = false
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	defer cli.Close()
	containerId, err := findContainerIdByName(nodeContainerName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find container:%s id error: %v\n", nodeContainerName, err)
		return
	}
	//Check whether the dcstorage container exists
	resp, err := cli.ContainerInspect(context.Background(), containerId)
	if err != nil {
		return
	} else if resp.State.Running {
		status = true
	}
	return
}

// Get the running status of dcchain
func checkDcchainStatus() (status bool, err error) {
	status = false
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	defer cli.Close()
	containerId, err := findContainerIdByName(chainContainerName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find container:%s id error: %v\n", chainContainerName, err)
		return
	}
	//Check whether the dcchain container exists
	resp, err := cli.ContainerInspect(context.Background(), containerId)
	if err != nil {
		return
	} else if resp.State.Running {
		status = true
	}
	return
}

// Get the running status of dcmanager
func checkDcDeamonStatusDc() (status bool, err error) {
	// Look for the dcmanager process.
	status = false
	//read content from .dcdaemon
	content, err := os.ReadFile(daemonFilepath)
	if err != nil {
		return
	}
	//get pid from .dcdaemon
	pid, err := strconv.Atoi(string(content))
	if err != nil {
		log.Errorf("get pid from %s  error:%v", daemonFilepath, err)
		return
	}
	//check if the pid is running
	p, err := ps.FindProcess(pid)
	if err != nil || p == nil {
		return
	}
	status = true
	return
}

// Get the command status of dcnode. The background service will automatically start dcnode only when dcnode is running.
func checkDcnodeCmdState() (status bool) {
	status = false
	//read content from .cmdstate
	content, err := os.ReadFile(runCmdStateFilepath)
	if err != nil {
		return
	}
	//get cmdstate from .cmdstate
	cmdstate := string(content)
	if cmdstate == "start" {
		status = true
	}
	return
}

// Set the command status of dcnode
func setDcnodeCmdState(state string) (err error) {
	err = os.WriteFile(runCmdStateFilepath, []byte(state), 0644)
	if err != nil {
		log.Errorf("write .cmdstate file error:%v\n", err)
		return
	}
	return
}

// Get the running status of pccs
func checkPccsStatus() (status bool, err error) {
	status = false
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	defer cli.Close()
	containerId, err := findContainerIdByName(pccsContainerName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find container:%s id error: %v\n", pccsContainerName, err)
		return
	}
	//Check whether the pccs container exists
	resp, err := cli.ContainerInspect(context.Background(), containerId)
	if err != nil {
		return
	} else if resp.State.Running { //The container exists and is running, check whether it can be accessed normally
		_, err = util.HttpGetWithoutCheckCert("https://localhost:8081/sgx/certification/v4/rootcacrl")
		if err != nil { //Access failed
			return
		}
		status = true
	}
	return
}

// Background upgrade tracking processing
func daemonCommandDeal() {
	_, err := os.Stat(daemonFilepath)
	if err != nil {
		if os.IsNotExist(err) {
			os.Create(daemonFilepath)
		} else {
			log.Errorf("check dcmanager daemon status fail,err: %v\n", err)
			return
		}
	}
	//read content from .dcdaemon
	content, err := os.ReadFile(daemonFilepath)
	if err != nil {
		log.Errorf("read .dcdaemon file error:%v\n", err)
		return
	}
	//check if the content is empty
	if len(content) == 0 {
		//write the current pid to .dcdaemon
		err = os.WriteFile(daemonFilepath, []byte(strconv.Itoa(os.Getpid())), 0644)
		if err != nil {
			log.Errorf("write .dcdaemon file error:%v\n", err)
			return
		}
	} else {
		//check if the pid is running
		pid, err := strconv.Atoi(string(content))
		if err != nil {
			log.Errorf("read .dcdaemon file error:%v\n", err)
			return
		}
		//check if the pid is running
		p, err := ps.FindProcess(pid)
		if err == nil && p != nil {
			log.Infof("dcmanager daemon already on running\n")
			return
		}
		//write the current pid to .dcdaemon
		err = os.WriteFile(daemonFilepath, []byte(strconv.Itoa(os.Getpid())), 0644)
		if err != nil {
			log.Errorf("write .dcdaemon file error:%v\n", err)
			return
		}
	}
	//start upgrade
	ticker := time.NewTicker(time.Minute * 5)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-ticker.C:
			if !checkDcnodeCmdState() { //The dcnode does not have a start command, which means it is shut down manually and no background upgrade service is performed.
				log.Info("dcnode is not start,skip upgrade")
				continue
			}
			err = upgradeDeal()
			if err != nil || !util.IsSgx2Support() { //If the upgrade fails or does not support sgx2.0, you can quickly enter the next upgrade check because the upgrade interruption time of nodes that do not support sgx2.0 is shorter.
				continue
			}
			//In order to prevent all nodes from being upgraded at the same time during the upgrade process, wait for a random period of time, and the maximum waiting time is 24 hours.
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			randnum := r.Int31n(int32(86400))
			log.Infof("sleep %d seconds\n", randnum)
			time.Sleep(time.Duration(randnum) * time.Second)
		case <-quit:
			os.Remove(daemonFilepath)
			os.Exit(1)
		}
	}
}

// Start dcstorage, and when the d flag is true, start the background upgrade service
func startDcStorageNode() (err error) {
	//Determine whether pccs (docker) is already running. If it is not running, it needs to be run first.
	err = runPccsInDocker()
	if err != nil {
		return
	}
	if !util.IsSgx2Support() { //Need to start the local sgx service
		startTeeReportServerDocker()
	}
	//Determine whether dcstorage is already running. If it is not running, it needs to be run.
	err = startDcnodeInDocker()
	if err != nil {
		fmt.Println("start dcstorage fail,error: ", err.Error())
		return
	}
	return
}

func startDcChain() error {

	return startDcchainInDocker()
}

// start dcstorage in docker
func startDcnodeInDocker() (err error) {
	ctx := context.Background()
	_, err = util.CreateVolume(ctx, nodeVolume)
	if err != nil {
		return
	}
	logConfig := container.LogConfig{
		Type: "json-file",
		Config: map[string]string{
			"max-size": "100m",
			"max-file": "3",
		},
	}
	dataMount := mount.Mount{
		Type:   mount.TypeVolume,
		Source: nodeVolume,
		Target: "/opt/dcnetio/data",
	}
	disksMount := mount.Mount{
		Type:        mount.TypeBind,
		Source:      "/opt/dcnetio/disks",
		Target:      "/opt/dcnetio/disks",
		Consistency: mount.ConsistencyDefault,
		BindOptions: &mount.BindOptions{
			Propagation: mount.PropagationShared,
		},
	}
	etcMount := mount.Mount{
		Type:   mount.TypeBind,
		Source: "/opt/dcnetio/etc",
		Target: "/opt/dcnetio/etc",
	}
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: "always",
		},
		Mounts:      []mount.Mount{dataMount, disksMount, etcMount},
		NetworkMode: "host",
		Resources: container.Resources{
			Devices: []container.DeviceMapping{
				{
					PathOnHost:        "/dev/sgx/enclave",
					PathInContainer:   "/dev/sgx/enclave",
					CgroupPermissions: "rwm",
				},
				{
					PathOnHost:        "/dev/sgx/provision",
					PathInContainer:   "/dev/sgx/provision",
					CgroupPermissions: "rwm",
				},
			},
		},
		LogConfig: logConfig,
	}
	//Determine whether sgx is supported. In the early debugging stage, machines that do not support sgx can also run dcstorage.
	if !util.IsSgxSupport() {
		hostConfig = &container.HostConfig{
			RestartPolicy: container.RestartPolicy{
				Name: "always",
			},
			Mounts:      []mount.Mount{dataMount, disksMount, etcMount},
			NetworkMode: "host",
			LogConfig:   logConfig,
		}
	}
	containerConfig := &container.Config{ //Run the enclave in non-sgx2 simulation state, and use plug-in programs for node authentication (the machine must be in an environment supervised by the committee to maximize the performance of the machine that does not support sgx2, and it is only for the sgx1 device in the original debugging environment before going online. All subsequent devices must be support sgx2)
		Image:      config.RunningConfig.NodeImage,
		Entrypoint: []string{"dcstorage_native"},
	}
	//Determine whether sgx2 is supported
	if util.IsSgx2Support() {
		containerConfig = &container.Config{
			Image:      config.RunningConfig.NodeImage,
			Entrypoint: []string{"dcstorage"},
		}
	}

	//start container
	err = util.StartContainer(ctx, nodeContainerName, false, containerConfig, hostConfig)
	if err != nil {
		conflictMsg := fmt.Sprintf("Conflict. The container name \"/%s\" is already in use by container", nodeContainerName)
		if strings.Contains(err.Error(), conflictMsg) {
			err = fmt.Errorf("container %s already exist,please manual remove it first", nodeContainerName)
		}
	}
	return
}

// start dcchain in docker
func startDcchainInDocker() (err error) {
	ctx := context.Background()
	logConfig := container.LogConfig{
		Type: "json-file",
		Config: map[string]string{
			"max-size": "100m",
			"max-file": "3",
		},
	}

	_, err = os.Stat(chainDataDir)
	if err != nil {
		if os.IsNotExist(err) { //file does not exist
			//Create a directory
			err = os.MkdirAll(chainDataDir, os.ModePerm)
			if err != nil {
				return
			}
		} else {
			return
		}
	}
	dataMount := mount.Mount{
		Type:        mount.TypeBind,
		Source:      chainDataDir,
		Target:      chainDataDir,
		Consistency: mount.ConsistencyDefault,
		BindOptions: &mount.BindOptions{
			Propagation: mount.PropagationShared,
		},
	}
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: "always",
		},
		Mounts:      []mount.Mount{dataMount},
		NetworkMode: "host",
		LogConfig:   logConfig,
	}
	var entrypoint []string
	entrypoint = append(entrypoint, "dcchain")
	entrypoint = append(entrypoint, "--chain=mainnet")
	entrypoint = append(entrypoint, "--port")
	entrypoint = append(entrypoint, "60666")
	if len(config.RunningConfig.ChainBootNode) > 20 {
		entrypoint = append(entrypoint, "--bootnodes")
		entrypoint = append(entrypoint, config.RunningConfig.ChainBootNode)
	}
	if config.RunningConfig.ValidatorFlag == "enable" {
		entrypoint = append(entrypoint, "--state-pruning")
		entrypoint = append(entrypoint, "archive")
		entrypoint = append(entrypoint, "--blocks-pruning")
		entrypoint = append(entrypoint, "archive")
		entrypoint = append(entrypoint, "--validator")
		fmt.Println("start dcchain with validator mode")
	}
	if config.RunningConfig.ChainExposeFlag == "enable" {
		entrypoint = append(entrypoint, "--unsafe-rpc-external")
		entrypoint = append(entrypoint, "--rpc-cors")
		entrypoint = append(entrypoint, "all")
	}

	entrypoint = append(entrypoint, "-d")
	entrypoint = append(entrypoint, chainDataDir)
	entrypoint = append(entrypoint, "--sync")
	if config.RunningConfig.ChainSyncMode != "" {
		entrypoint = append(entrypoint, config.RunningConfig.ChainSyncMode)
	} else {
		entrypoint = append(entrypoint, "full")
	}
	entrypoint = append(entrypoint, "--pool-limit")
	entrypoint = append(entrypoint, "819200")
	entrypoint = append(entrypoint, "--pool-kbytes")
	entrypoint = append(entrypoint, "2048000")
	entrypoint = append(entrypoint, "--rpc-max-subscriptions-per-connection")
	entrypoint = append(entrypoint, "1024000")
	entrypoint = append(entrypoint, "--name")
	entrypoint = append(entrypoint, config.RunningConfig.ChainNodeName)

	containerConfig := &container.Config{
		Image:      config.RunningConfig.ChainImage,
		Entrypoint: entrypoint,
	}
	//start container
	err = util.StartContainer(ctx, chainContainerName, true, containerConfig, hostConfig)
	return

}

// start dcupgrade in docker
func startDcupgradeInDocker() (err error) {
	ctx := context.Background()
	logConfig := container.LogConfig{
		Type: "json-file",
		Config: map[string]string{
			"max-size": "10m",
			"max-file": "3",
		},
	}
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: "always",
		},
		NetworkMode: "host",
		Resources: container.Resources{
			Devices: []container.DeviceMapping{
				{
					PathOnHost:        "/dev/sgx/enclave",
					PathInContainer:   "/dev/sgx/enclave",
					CgroupPermissions: "rwm",
				},
				{
					PathOnHost:        "/dev/sgx/provision",
					PathInContainer:   "/dev/sgx/provision",
					CgroupPermissions: "rwm",
				},
			},
		},
		LogConfig: logConfig,
	}
	containerConfig := &container.Config{
		Image: config.RunningConfig.UpgradeImage,
	}
	//start container
	util.StartContainer(ctx, upgradeContainerName, true, containerConfig, hostConfig)
	return
}

// start teeReportServer in docker
func startTeeReportServerDocker() (err error) {
	ctx := context.Background()
	_, err = util.CreateVolume(ctx, teeReportServerVolume)
	if err != nil {
		return
	}
	logConfig := container.LogConfig{
		Type: "json-file",
		Config: map[string]string{
			"max-size": "10m",
			"max-file": "3",
		},
	}
	dataMount := mount.Mount{
		Type:   mount.TypeVolume,
		Source: teeReportServerVolume,
		Target: "/opt/dcnetio/dcteereportserver",
	}
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: "always",
		},
		NetworkMode: "host",
		Mounts:      []mount.Mount{dataMount},
		Resources: container.Resources{
			Devices: []container.DeviceMapping{
				{
					PathOnHost:        "/dev/sgx/enclave",
					PathInContainer:   "/dev/sgx/enclave",
					CgroupPermissions: "rwm",
				},
				{
					PathOnHost:        "/dev/sgx/provision",
					PathInContainer:   "/dev/sgx/provision",
					CgroupPermissions: "rwm",
				},
			},
		},
		LogConfig: logConfig,
	}
	containerConfig := &container.Config{
		Image:      config.RunningConfig.TeeReportServerImage,
		Entrypoint: []string{"dcteereportserver"},
	}
	//start container
	err = util.StartContainer(ctx, teeReportServerContainerName, true, containerConfig, hostConfig)
	return
}

// stopDcnodeInDocker stop dcstorage in docker
func stopDcnodeInDocker() (err error) {
	ctx := context.Background()
	err = util.StopContainer(ctx, nodeContainerName)
	return
}

// stop dcupgrade in docker
func stopUpgradeInDocker() {
	ctx := context.Background()
	util.StopContainer(ctx, upgradeContainerName)
}

// stop dcchain in docker
func stopDcchainInDocker() {
	ctx := context.Background()
	util.StopContainer(ctx, chainContainerName)
}

// stop dcpccs in docker
func stopPccsInDocker() {
	ctx := context.Background()
	util.StopContainer(ctx, pccsContainerName)

}

// remove container from docker by container name
func removeDockerContainer(containerName string) (err error) {
	ctx := context.Background()
	err = util.RemoveContainer(ctx, containerName)
	return
}

// Use the dcstorage and dcupdate programs to provide local random number query services and obtain their corresponding enclavid
func getVersionByHttpGet(localport int) (version string, enclaveId string, err error) {
	dcEnclaveIdUrl := fmt.Sprintf("http://%s:%d/version", serverhost, localport)
	respBody, err := util.HttpGet(dcEnclaveIdUrl)
	if err != nil {
		return
	}
	versionInfo := string(respBody)
	values := strings.Split(versionInfo, "@")
	if len(values) < 2 {
		err = fmt.Errorf("get invalid version info")
	} else {
		enclaveId = values[0]
		version = values[1]
	}
	return

}

// Use the dcstorage program to provide local random number query service and obtain node information
func getPeerInfoByHttpGet() (peerid, account, walletAddr string, err error) {
	dcPeerInfoUrl := fmt.Sprintf("http://%s:%d/peerinfo", serverhost, dcStorageListenPort)
	respBody, err := util.HttpGet(dcPeerInfoUrl)
	if err != nil {
		return
	}
	peerInfo := string(respBody)
	values := strings.Split(peerInfo, "@")
	if len(values) < 3 {
		log.Errorf("get invalid peer info")
	} else {
		peerid = values[0]
		account = values[1]
		walletAddr = values[2]
	}
	return

}

// Use the dcstorage program to provide query services and obtain memory usage information of node programs.
func getMemoryUsageByHttpGet() (memusage string, err error) {
	dcPeerInfoUrl := fmt.Sprintf("http://%s:%d/mem", serverhost, dcStorageListenPort)
	respBody, err := util.HttpGet(dcPeerInfoUrl)
	if err != nil {
		return
	}
	memusage = string(respBody)
	return

}

// Send gc command to dcstorage
func sendBlockGcCommand() (err error) {
	dcPeerInfoUrl := fmt.Sprintf("http://%s:%d/blockgc", serverhost, dcStorageListenPort)
	_, err = util.HttpGet(dcPeerInfoUrl)
	return
}

// During the upgrade process, wait for dcupdate to obtain the node key from dcstorage
func waitDcUpdateGetPeerSecret() (bool, error) {
	dcSecretFlagUrl := fmt.Sprintf("http://%s:%d/secretflag", serverhost, dcUpgradeListenPort)
	ticker := time.NewTicker(time.Second)
	count := 0
	for {
		<-ticker.C
		respBody, err := util.HttpGet(dcSecretFlagUrl)
		if err != nil {
			count++
			if count > 60 {
				log.Errorf("waitDcUpdateGetPeerSecret requset fail,  err: %v\n", err)
				return false, err
			}
			continue
		}
		flag := string(respBody)
		if flag == "true" {
			return true, nil
		} else {
			count++
			if count > 60 {
				return false, fmt.Errorf("dcupdate get peer secret timeout")
			}
			continue
		}
	}

}

// During the upgrade process, wait for the new version of dcstorage to retrieve the key from dcupdate.
func waitNewDcGetPeerSecret() (bool, error) {
	dcSecretFlagUrl := fmt.Sprintf("http://%s:%d/upgradeflag", serverhost, dcUpgradeListenPort)
	ticker := time.NewTicker(time.Second)
	count := 0
	for {
		<-ticker.C
		respBody, err := util.HttpGet(dcSecretFlagUrl)
		if err != nil {
			if count%30 == 0 {
				log.Infof("waitNewDcGetPeerSecret requset fail,  err: %v\n", err)
			}
			continue
		}
		flag := string(respBody)
		if flag == "true" {
			return true, nil
		} else {
			count++
			if count > 600 { //Wait 10 minutes
				return false, fmt.Errorf("new version dcstorage get peer secret timeout")
			}
			continue
		}
	}

}

var verionLogFlag = true //Upgrade log printing flag to avoid repeated printing
var waitEnclaveIdFlag = true
var versionGetErrCount = 0 //Number of failed attempts to obtain version information

// dcstorage 程序升级处理
func upgradeDeal() (err error) {
	//Determine whether the current dcstorage is running, if not, start dcstorage
	status, err := checkDcnodeStatus()
	if err != nil || !status {
		err = startDcStorageNode()
		if err != nil {
			log.Errorf("start dcstorage fail,err: %v", err)
			return
		}
	} else {
		//Check if pccs is running, if not, start pccs
		status, err := checkPccsStatus()
		if err != nil || !status {
			runPccsInDocker()
		}
	}
	//Get the version and enclaveid of the currently running dcstorage
	version, enclaveId, err := getVersionByHttpGet(dcStorageListenPort)
	if err != nil {
		if waitEnclaveIdFlag {
			log.Info("wait dcstorage to start")
			waitEnclaveIdFlag = false
		}
		if versionGetErrCount > 150 { //If the version information has not been obtained after more than 10 minutes, it is considered that dcstorage has not successfully obtained the secret. Try to start dcupgrade for upgrade assistance.
			log.Errorf("get dcstorage version info fail,err: %v", err)
			versionGetErrCount = 0
			//Start dcupgrade
			startDcupgradeInDocker()
		} else {
			versionGetErrCount++
		}
		return
	}
	versionGetErrCount = 0
	if !waitEnclaveIdFlag {
		log.Infof("dcstorage is running,version: %s,enclaveid: %s", version, enclaveId)
	}
	waitEnclaveIdFlag = true
	//Get the latest configured node enclaveid on the blockchain
	programInfo, err := blockchain.GetConfigedDcStorageInfo()
	if err != nil {
		log.Errorf("get dcstorage version info from blockchain fail,err: %v", err)
		return
	}
	bcVersion, err := goversion.NewVersion(programInfo.Version)
	if err != nil {
		log.Errorf("invalid new version format on blockchain,err: %v", err)
		return
	}
	//Verify the enclaveid configured on the obtained chain
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if config.RunningConfig.NewVersion.Version != "" && config.RunningConfig.NewVersion.EnclaveId != "" { //If there is a new version number in the configuration file, use the version number in the configuration file (manual upgrade)
		configNewVersion, err := goversion.NewVersion(config.RunningConfig.NewVersion.Version)
		if err == nil && bcVersion.LessThan(configNewVersion) { //If the version number on the blockchain is smaller than the version number in the configuration file, use the version number in the configuration file (manual upgrade)
			if blockchain.IfEnclaveIdValid(ctx, config.RunningConfig.NewVersion.EnclaveId) {
				programInfo = &config.RunningConfig.NewVersion
			}

		}
	}
	//Determine whether the enclaveid of the currently running dcstorage is consistent with the latest configured node enclaveid on the blockchain
	if enclaveId == programInfo.EnclaveId {
		if verionLogFlag {
			log.Infof("dcstorage is the latest version")
			verionLogFlag = false
		}
		return
	}
	verionLogFlag = true
	if !blockchain.IfEnclaveIdValid(ctx, programInfo.EnclaveId) {
		return
	}
	//Compare the old and new version numbers to determine whether an upgrade is needed
	localVersion, err := goversion.NewVersion(version)
	if err != nil {
		log.Errorf("invalid local version format,err: %v", err)
		return
	}
	configedVersion, err := goversion.NewVersion(programInfo.Version)
	if err != nil {
		log.Errorf("invalid new version format,err: %v", err)
		return
	}
	if !localVersion.LessThan(configedVersion) { //The local version is updated, not updated
		log.Infof("unneed upgrade ,dcstorage localVersion: %s,   configedVersion: %s\n", localVersion, configedVersion)
		return
	}
	tagUrl := programInfo.OriginUrl
	imageLoadSuccess := false
	//Obtain the image of the upgrade assistant program. If it exists in the DC network, use the image in the DC network. Otherwise, use the image corresponding to the registry in the configuration file.
	for _, mCid := range programInfo.MirrCids {
		//Get the backup node address where the mcid file is located
		fileSize, addrInfos, err := blockchain.GetPeerAddrsForCid(mCid)
		if err != nil || len(addrInfos) == 0 {
			continue
		}
		tObj := &util.TransmitObj{
			TotalSize: uint64(fileSize),
			LogFlag:   true,
		}
		savePath := fmt.Sprintf("/tmp/%s.tar", mCid)
		err = util.DownloadFromIpfs(mCid, "", savePath, addrInfos, time.Hour, tObj)
		if err == nil {
			//Talk about image import obtained from DC network
			err = loadDcStorageImage(context.Background(), savePath)
			if err == nil {
				imageLoadSuccess = true
				break
			}
		}
	}
	if !imageLoadSuccess {
		// Obtain the corresponding image according to the corresponding registry address in the configuration file
		if config.RunningConfig.Registry == "cn" {
			tagUrl = programInfo.MirrorUrl
		}
		// Pull the new version of dcstorage program image
		err = pullDcStorageNodeImage(tagUrl)
		if err != nil {
			if config.RunningConfig.Registry == "cn" {
				tagUrl = programInfo.OriginUrl
			} else {
				tagUrl = programInfo.MirrorUrl
			}
			err = pullDcStorageNodeImage(tagUrl)
			if err != nil {
				log.Errorf("pullDcStorageNodeImage fail,err: %v", err)
				return
			}
		}
	}
	//First close the upgrade assistant program. Because whether the upgrade is successful or not, the internal flag of dcupgrade will only be reset when restarting, so it must be closed first.
	stopUpgradeInDocker()
	if util.IsSgx2Support() { //Sgx2 environment, you need to introduce an upgrade assistant program to transfer the node key
		// Run the upgrade assistant
		err = startDcupgradeInDocker()
		if err != nil {
			log.Errorf("startDcupgradeInDocker fail,err: %v", err)
			return
		}
		// Wait for dcupdate to successfully obtain the node key
		_, err = waitDcUpdateGetPeerSecret()
		if err != nil {
			log.Errorf("update fail,err: %v", err)
			return
		}
	}
	//Close the currently running dcstorage
	err = stopDcnodeInDocker()
	if err != nil {
		log.Errorf("stopDcnodeInDocker fail,err: %v", err)
		time.Sleep(10 * time.Second) //If you do not exit directly, the loop may fail due to apparmor, and you will never be able to upgrade.
	}
	//Delete the docker container of the old version of dcstoragenode
	err = removeDcStorageNodeInDocker()
	if err != nil {
		log.Errorf("removeDcStorageNodeInDocker fail,err: %v", err)
		return
	}
	//Update the image of dc storagenode to ensure that when starting, the new version of dcstorage is started.
	config.RunningConfig.NodeImage = tagUrl
	//Run the downloaded dcstorage program
	err = startDcStorageNode()
	if err != nil {
		log.Errorf("upgrade-startDcStorageNode fail,err: %v", err)
		return
	}
	log.Info("wait new version to get peer secret")
	if util.IsSgx2Support() {
		// Wait for the new version of dcstorage to successfully obtain the node key
		_, err = waitNewDcGetPeerSecret()
		if err != nil {
			log.Errorf("update fail,err: %v", err)
			return
		}
		stopUpgradeInDocker()
		log.Infof("new version dcstorage  get peer sceret success")
	}
	//Save the configuration file to ensure that the next time you start it, the new version of dcstorage will be started.
	if err = config.SaveConfig(config.RunningConfig); err != nil {
		log.Errorf("save config fail,err: %v", err)
		return
	}
	//Wait for dcstorage to restart successfully after obtaining the secret. Wait up to 10 minutes.
	log.Info("wait new version dcstorage to start with secret, max wait 10 minutes...")
	count := 0
	for {
		//Determine whether the new version of the program is running normally by checking the version
		version, enclaveId, err = getVersionByHttpGet(dcStorageListenPort)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Second)
		count++
		if count > 60 {
			log.Errorf("new version dcstorage start fail,err : %v", err)
			break
		}
	}
	if version != programInfo.Version {
		log.Errorf("dcstorage version check fail,version: %s, configedVersion: %s", version, programInfo.Version)
		return
	}
	if enclaveId != programInfo.EnclaveId && util.IsSgx2Support() {
		log.Errorf("dcstorage enclaveid check fail,enclaveId: %s, configedEnclaveId: %s", enclaveId, programInfo.EnclaveId)
		//Stop new version of dcstorage
		stopDcnodeInDocker()
		return
	}
	log.Infof("dcstorage upgrade success,version: %s,enclaveid: %s", version, enclaveId)
	//dc自身重启,因为启动docker容器的时候，内存会有泄漏，无法回收,所以重启后台升级服务
	cmd := exec.Command(os.Args[0], "upgrade", "daemon")
	cmd.Start() // 开始执行新进程，不等待新进程退出
	os.Exit(0)
	return
}

// Pull new docker image
func pullDcStorageNodeImage(image string) (err error) {
	//docker pull
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return
	}
	log.Info("begin to pull new version dcstorage docker image: ", image)
	ctx := context.Background()
	//docker pull
	out, err := cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		log.Errorf("pullDcStorageNodeImage-ImagePull fail,err: %v", err)
		return
	}
	defer out.Close()
	//docker pull
	size, err := io.Copy(io.Discard, out)
	if err != nil {
		log.Errorf("pullDcStorageNodeImage-ImagePull fail,err: %v", err)
		return
	}
	log.Infof("pull new version dcstorage docker image success, image size: %d", size)
	return
}

// loadDcStorageImage loads dcstorage object
func loadDcStorageImage(ctx context.Context, imagePath string) (err error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return
	}
	// Open the file input.txt
	imageReader, err := os.Open(imagePath)
	if err != nil {
		log.Error(err)
	}
	// close file
	defer imageReader.Close()
	_, err = cli.ImageLoad(ctx, imageReader, true)
	return

}

//Reading file method

// Delete the docker container of dcstoragenode
func removeDcStorageNodeInDocker() (err error) {
	log.Infof("begin to remove old version dcstorage docker container")
	fmt.Println("begin to remove old version dcstorage docker container")
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return
	}
	//Get the docker container id of dcstorage
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return
	}
	for _, container := range containers {
		if container.Image == config.RunningConfig.NodeImage {
			log.Infof("begin to remove old version dcstorage docker container,container id: %s", container.ID)
			fmt.Printf("begin to remove old version dcstorage docker container,container id: %s\n", container.ID)
			err = cli.ContainerRemove(context.Background(), container.ID, types.ContainerRemoveOptions{Force: true})
			if err != nil {
				continue
			}
			log.Infof("remove old version dcstorage docker container success")
			return
		} else {
			//Remove old version of dcstorage container
			if strings.Contains(container.Image, "dcstorage") {
				log.Infof("begin to remove old version dcstorage docker container,container id: %s", container.ID)
				err = cli.ContainerRemove(context.Background(), container.ID, types.ContainerRemoveOptions{Force: true})
				if err != nil {
					continue
				}
				log.Infof("remove old version dcstorage docker container success")
				return
			}
			for _, name := range container.Names {
				if name == nodeContainerName {
					log.Infof("begin to remove old version dcstorage docker container,container id: %s", container.ID)
					err = cli.ContainerRemove(context.Background(), container.ID, types.ContainerRemoveOptions{Force: true})
					if err != nil {
						continue
					}
					log.Infof("remove old version dcstorage docker container success")
					return
				}
			}
		}

	}
	log.Infof("no old version dcstorage docker container")
	fmt.Println("no old version dcstorage docker container")
	return
}

// Determine whether the program is running by listening to the port
func GetPidWithListenPort(listenPort int) (pid int64, err error) {
	cmd := fmt.Sprintf("lsof -i:%d| awk '/LISTEN/ && !/awk/ {print $2}'", listenPort)
	//Check if the process is running
	out, err := exec.Command(cmd).Output()
	if err != nil {
		return
	}
	if out == nil {
		err = fmt.Errorf("no process on running")
		return
	}
	pid, err = strconv.ParseInt(string(out), 10, 32)
	return
}

// Use docker to start pccs
func runPccsInDocker() (err error) {
	listenPort := 8081
	//Check whether the port is already occupied
	pid, err := GetPidWithListenPort(listenPort)
	if err == nil && pid > 0 { //The port has been enabled, request data for testing
		_, err = util.HttpGetWithoutCheckCert("https://localhost:8081/sgx/certification/v4/rootcacrl")
		if err != nil {
			log.Errorf("Can't start pccs for 8081 port is occupied")
		}
		return
	}
	apiKey := config.RunningConfig.PccsKey
	if len(apiKey) < 32 { //
		err = fmt.Errorf("pccs api key is invalid.goto https://api.portal.trustedservices.intel.com/provisioning-certification and click on 'Subscribe' to get pccs apikey,and then run ' dc pccs_api_key \"apikey\"' to config")
		fmt.Println("start pccs fail,err: ", err.Error())
		return
	}
	pcsUrl := "PCSURL=https://api.trustedservices.intel.com/sgx/certification/v4/"
	apiKeyStr := fmt.Sprintf("APIKEY=%s", apiKey)
	userPassStr := "USERPASS=$Dcnetio_user0$" //default password
	adminPassStr := "ADMINPASS=$Dcnetio_admin0$"
	ctx := context.Background()
	dataMount := mount.Mount{
		Type:   mount.TypeVolume,
		Source: pccsVolume,
		Target: "/opt/intel/pccs",
	}
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: "always",
		},
		NetworkMode: "host",
		Mounts:      []mount.Mount{dataMount},
	}
	cConfig := &container.Config{
		Image: config.RunningConfig.PccsImage,
		Env:   []string{pcsUrl, apiKeyStr, userPassStr, adminPassStr},
	}
	err = util.StartContainer(ctx, pccsContainerName, true, cConfig, hostConfig)
	//check if pccs is running
	if err == nil {
		startFlag := false
		log.Infof("wait for the successful startup of pccs.")
		fmt.Println("wait for the successful startup of pccs.")
		//wait for pccs to start
		for i := 0; i < 10; i++ {
			_, gerr := util.HttpGetWithoutCheckCert("https://localhost:8081/sgx/certification/v4/rootcacrl")
			if gerr == nil {
				startFlag = true
				break
			}
			time.Sleep(2 * time.Second)
		}
		if !startFlag {
			err = fmt.Errorf("pccs start fail")
		}
	}
	return
}

// show Container log
func showContainerLog(containerName string, tnum int) {
	containerId, err := findContainerIdByName(containerName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find container id error: %v\n", err)
		return
	}
	err = showLogsForContainer(containerId, tnum)
	if err != nil {
		fmt.Fprintf(os.Stderr, "show logs error: %v\n", err)
		return
	}
}

// Print the log of the specified container id in docker
func showLogsForContainer(containerId string, tnum int) error {
	cli, _ := client.NewClientWithOpts(client.FromEnv)
	defer cli.Close()
	if tnum == 0 { //Print the latest log of the container running
		execResp, err := cli.ContainerInspect(context.Background(), containerId)
		if err != nil { //Container does not exist
			return err
		}
		reader, err := cli.ContainerLogs(context.Background(), containerId, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true, Since: execResp.State.StartedAt})
		if err != nil {
			return err
		}
		defer reader.Close()
		_, err = io.Copy(os.Stdout, reader)
		if err != nil && err != io.EOF {
			return err
		}

	} else {
		reader, err := cli.ContainerLogs(context.Background(), containerId, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true, Tail: fmt.Sprintf("%d", tnum)})
		if err != nil {
			return err
		}
		defer reader.Close()
		_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, reader)
		//	_, err = io.Copy(os.Stdout, reader)
		if err != nil && err != io.EOF {
			return err
		}
	}
	handleInterruptSignal()
	return nil
}

// find container id by Name
func findContainerIdByName(containerName string) (containerId string, err error) {
	cli, _ := client.NewClientWithOpts(client.FromEnv)
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return
	}
	for _, container := range containers {
		for _, name := range container.Names {
			if name == "/"+containerName {
				containerId = container.ID
				break
			}
		}
	}
	return
}

// handle interrupt signal
func handleInterruptSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Info("Interrupt signal received, shutting down...")
	os.Exit(0)
}
