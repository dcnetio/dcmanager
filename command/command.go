package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bigkevmcd/go-configparser"
	"github.com/dcnetio/dc/blockchain"
	"github.com/dcnetio/dc/config"
	"github.com/dcnetio/dc/util"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	goversion "github.com/hashicorp/go-version"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-ps"
)

const dcStorageListenPort = 6667
const dcUpgradeListenPort = 6666

const nodeContainerName = "dcstorage"
const chainContainerName = "dcchain"
const upgradeContainerName = "dcupgrade"
const pccsContainerName = "dcpccs"
const nodeVolueName = "dcstorage"
const chainVolueName = "dcchain"
const upgradeVolueName = "upgradeVolueName"
const pccsVolueName = "dcpccs"
const daemonFilepath = "/opt/dcnetio/data/.dcupgradedaemon"

var serviceConfigFileContent = `[Unit]
After=network.target

[Service]
ExecStart=/opt/dcnetio/bin/dc upgrade daemon
Restart=always

[Install]
WantedBy=default.target`

// servicename
const serviceConfigFile = "/etc/systemd/system/dc.service"

// const serviceConfigFile = "./test/dc.service"
const startupContent = "/opt/dcnetio/bin/dc upgrade daemon"

func ShowHelp() {
	fmt.Println("dcmanager version ", config.Version)
	fmt.Println("usage: sudo dc command [options]")
	fmt.Println("command")
	fmt.Println("")
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
	fmt.Println(" log  {storage|chain|upgrade|pccs}       show running log with service_name")
	fmt.Println("                                         \"storage\":  show dcstorage container running log")
	fmt.Println("                                         \"chain\":  show dcchain container running log")
	fmt.Println("                                         \"upgrade\":  show dcupgrade container running log")
	fmt.Println("                                         \"pccs\":  show local pccs  running log")
	fmt.Println(" uniqueid                                show soft version and sgx enclaveid ")
	fmt.Println(" peerinfo  					          show local running peer info")
	fmt.Println(" checksum  filepath                      generate  sha256 checksum for file in the \"filepath\"")
	fmt.Println(" get cid [--name][--timeout][--secret]   get file from dc net with \"cid\" ")
	fmt.Println("                                         \"--name\": file to save name")
	fmt.Println("                                         \"--timeout\":  wait seconds for file to complete download")
	fmt.Println("                                         \"--secret\":  file decode secret with base32 encoded")
	fmt.Println(" rotate-keys                             generate new storage session keys")
}

var log = logging.Logger("dcmanager")

func StartCommandDeal() {
	if len(os.Args) < 3 {
		ShowHelp()
		return
	}
	switch os.Args[2] {
	case "storage":
		err := startDcStorageNode()
		if err == nil {
			showContainerLog(nodeContainerName, 0)
		} else {
			log.Error(err)
		}

	case "chain":
		err := startDcChain()
		if err == nil {
			showContainerLog(chainContainerName, 0)
		}
	case "pccs":
		err := runPccsInDocker()
		if err == nil {
			showContainerLog(pccsContainerName, 0)
		}

	case "all":
		startDcChain()
		err := startDcStorageNode()
		if err == nil {
			showContainerLog(nodeContainerName, 0)
		} else {
			log.Error(err)
		}

	default:
		ShowHelp()
	}

}

func StopCommandDeal() {
	if len(os.Args) < 2 {
		ShowHelp()
		return
	}
	switch os.Args[2] {
	case "storage":
		stopDcnodeInDocker()
	case "chain":
		stopDcchainInDocker()
	case "pccs":
		stopPccsInDocker()
	case "all":
		stopDcnodeInDocker()
		stopDcchainInDocker()
		stopPccsInDocker()
	default:
		ShowHelp()
	}
}

// ???????????????????????????
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
	fmt.Println("daemon status:", dcStatus)
	switch secondArgs {
	case "storage":
		nodeStatus, _ := checkDcnodeStatus()
		fmt.Println("dcstorage status:", nodeStatus)
	case "chain":
		chainStatus, _ := checkDcchainStatus()
		fmt.Println("dcchain status:", chainStatus)
	case "pccs":
		pccsStatus, _ := checkPccsStatus()
		fmt.Println("pccs status:", pccsStatus)
	case "all":
		nodeStatus, _ := checkDcnodeStatus()
		fmt.Println("dcstorage status:", nodeStatus)
		chainStatus, _ := checkDcchainStatus()
		fmt.Println("dcchain status:", chainStatus)
		pccsStatus, _ := checkPccsStatus()
		fmt.Println("pccs status:", pccsStatus)
	default:
		ShowHelp()
	}
}

// ???????????????????????????????????????
func LogCommandDeal() { //
	if len(os.Args) < 3 {
		ShowHelp()
		return
	}
	switch os.Args[2] {
	case "storage":
		showContainerLog(nodeContainerName, 100)
	case "chain":
		showContainerLog(chainContainerName, 100)
	case "upgrade":
		showContainerLog(upgradeContainerName, 100)
	case "pccs":
		showContainerLog(pccsContainerName, 100)
	default:
		ShowHelp()
	}
}

// ??????????????????
func UpgradeCommandDeal() {
	if len(os.Args) > 2 {
		if os.Args[2] == "daemon" { //????????????????????????????????????????????????dcstorage,???????????????????????????
			//fork new process to run in deamon mode
			if os.Getppid() != 1 {
				// ????????????????????????????????????????????????????????????
				cmd := exec.Command(os.Args[0], "upgrade", "daemon")
				cmd.Start() // ????????????????????????????????????????????????
				os.Exit(0)
			} else {
				daemonCommandDeal()
			}
		} else if os.Args[2] == "cancel" { //????????????????????????
			cancelDaemonCommandDeal()
		} else {
			ShowHelp()
		}
	}
}

// ????????????enclave???enclaveid
func UniqueIdCommandDeal() {
	if len(os.Args) < 3 {
		ShowHelp()
		return
	}
	fmtStr := "dcstorage version: %s,enclaveid: %s\ndcupgrade version: %s,enclaveid: %s\n"
	upgradeVersion := ""
	upgradeEnclaveId := ""
	//??????dcupgrade????????????enclaveid??????
	//??????dcupgrade???????????????
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
	//??????dcstorage????????????enclaveid??????
	//??????dcstorage???????????????
	nodeStatus, _ := checkDcnodeStatus()
	if nodeStatus {
		var err error
		storageVersion, storageEnclaveId, err = getVersionByHttpGet(dcStorageListenPort)
		if err != nil {
			log.Error(err)
		}
	}
	fmt.Println("dcmanager version ", config.Version)
	fmt.Printf(fmtStr, storageVersion, storageEnclaveId, upgradeVersion, upgradeEnclaveId)
}

// ?????????????????????????????????
func PeerInfoCommandDeal() {
	peerid, account, walletAddr, err := getPeerInfoByHttpGet()
	if err != nil {
		fmt.Println("get peerinfo failed,please make sure storage service is running")
		return
	}
	fmt.Printf("peer ID: %s\npeer Pubkey: %s\npeer Wallet Address: %s\n", peerid, account, walletAddr)

}

// ???????????????hash?????????
func ChecksumCommandDeal() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: sudo dc checksum <file>")
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

// ???dc??????????????????
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
	//??????cid????????????????????????????????????????????????
	fileSize, addrInfos, err := blockchain.GetPeerAddrsForCid(cid)
	if err != nil || len(addrInfos) == 0 {
		fmt.Fprintf(os.Stderr, "Failed to get file with cid:%s \n", cid)
		return
	}
	tObj := &util.TransmitObj{
		TotalSize: uint64(fileSize),
	}
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
		return "", err
	}
	if !status {
		return "", errors.New("dcchain is not running")
	}
	//make http request to dcchain
	chainRpcUrl := fmt.Sprintf("http://127.0.0.1:%d", config.RunningConfig.ChainRpcListenPort)
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

// ??????dcstorage???????????????
func checkDcnodeStatus() (status bool, err error) {
	status = false
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	defer cli.Close()
	//??????dcstorage??????????????????
	resp, err := cli.ContainerInspect(context.Background(), nodeContainerName)
	if err != nil {
		return
	} else if resp.State.Running {
		status = true
	}
	return
}

// ??????dcchain???????????????
func checkDcchainStatus() (status bool, err error) {
	status = false
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	defer cli.Close()
	//??????dcchain??????????????????
	resp, err := cli.ContainerInspect(context.Background(), chainContainerName)
	if err != nil {
		return
	} else if resp.State.Running {
		status = true
	}
	return
}

// ??????dcmanager???????????????
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

// ??????pccs???????????????
func checkPccsStatus() (status bool, err error) {
	status = false
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	defer cli.Close()
	//??????pccs??????????????????
	resp, err := cli.ContainerInspect(context.Background(), pccsContainerName)
	if err != nil {
		return
	} else if resp.State.Running { //??????????????????????????????????????????????????????
		_, err = util.HttpGetWithoutCheckCert("https://localhost:8081/sgx/certification/v3/rootcacrl")
		if err != nil { //????????????
			return
		}
		status = true
	}
	return
}

// ????????????????????????
func daemonCommandDeal() {
	_, err := os.Stat(daemonFilepath)
	if err != nil {
		if os.IsNotExist(err) {
			os.Create(daemonFilepath)
		} else {
			fmt.Fprintf(os.Stderr, "check dcmanager daemon status fail,err: %v\n", err)
			return
		}
	}
	//read content from .dcdaemon
	content, err := os.ReadFile(daemonFilepath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read .dcdaemon file error:%v\n", err)
		return
	}
	//check if the content is empty
	if len(content) == 0 {
		//write the current pid to .dcdaemon
		err = os.WriteFile(daemonFilepath, []byte(strconv.Itoa(os.Getpid())), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "write .dcdaemon file error:%v\n", err)
			return
		}
	} else {
		//check if the pid is running
		pid, err := strconv.Atoi(string(content))
		if err != nil {
			fmt.Fprintf(os.Stderr, "read .dcdaemon file error:%v\n", err)
			return
		}
		//check if the pid is running
		p, err := ps.FindProcess(pid)
		if err == nil && p != nil {
			fmt.Fprintf(os.Stderr, "dcmanager daemon already on running\n")
			return
		}
		//write the current pid to .dcdaemon
		err = os.WriteFile(daemonFilepath, []byte(strconv.Itoa(os.Getpid())), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "write .dcdaemon file error:%v\n", err)
			return
		}
	}
	flag := configServiceStartup()
	if !flag {
		fmt.Fprintf(os.Stderr, "set auto upgrade service to run with startup fail\n")
		return
	}
	//start upgrade
	ticker := time.NewTicker(time.Minute * 5)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-ticker.C:
			upgradeDeal()
		case <-quit:
			os.Remove(daemonFilepath)
			os.Exit(1)
		}
	}
}

// ????????????????????????
func cancelDaemonCommandDeal() {
	//remove startup service config
	flag := removeServiceStartup()
	if !flag {
		fmt.Fprintf(os.Stderr, "cancel auto upgrade service to run with startup fail\n")
	}
	_, err := os.Stat(daemonFilepath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "dcmanager daemon is not running\n")
			return
		} else {
			fmt.Fprintf(os.Stderr, "check dcmanager daemon status fail,err: %v\n", err)
			return
		}
	}
	//read content from .dcdaemon
	content, err := os.ReadFile(daemonFilepath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read .dcdaemon file error:%v\n", err)
		return
	}
	//check if the content is empty
	if len(content) == 0 {
		fmt.Fprintf(os.Stderr, "dcmanager daemon is not running\n")
		return
	} else {
		//check if the pid is running
		pid, err := strconv.Atoi(string(content))
		if err != nil {
			fmt.Fprintf(os.Stderr, "read .dcdaemon file error:%v\n", err)
			return
		}
		//check if the pid is running
		p, err := ps.FindProcess(pid)
		if err != nil || p == nil {
			return
		}
		//check if the pid is running
		process, err := os.FindProcess(pid)
		if err == nil && process != nil {
			process.Kill()
		}
		os.Remove(daemonFilepath)
	}
}

func startDcStorageNode() (err error) {
	//??????pccs???docker??????????????????????????????????????????????????????
	err = runPccsInDocker()
	if err != nil {
		return
	}
	//??????dcstorage????????????????????????????????????????????????
	err = startDcnodeInDocker()
	//????????????????????????
	cmd := exec.Command(os.Args[0], "upgrade", "daemon")
	cmd.Start() // ????????????????????????????????????????????????
	return
}

func startDcChain() error {
	return startDcchainInDocker()
}

// start dcstorage in docker
func startDcnodeInDocker() (err error) {
	ctx := context.Background()
	_, err = util.CreateVolume(ctx, nodeVolueName)
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
		Source: nodeVolueName,
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
		Source: "/opt/dcnetio/etc/",
		Target: "/opt/dcnetio/etc/",
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
	containerConfig := &container.Config{
		Image: config.RunningConfig.NodeImage,
	}
	//start container
	err = util.StartContainer(ctx, nodeContainerName, false, containerConfig, hostConfig)
	return
}

// start dcchain in docker
func startDcchainInDocker() (err error) {
	ctx := context.Background()
	_, err = util.CreateVolume(ctx, chainVolueName)
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
		Source: chainVolueName,
		Target: "/opt/dcnetio/data",
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
	entrypoint = append(entrypoint, "-d")
	entrypoint = append(entrypoint, "/opt/dcnetio/data")
	if config.RunningConfig.ValidatorFlag {
		entrypoint = append(entrypoint, "--validator")
	}
	entrypoint = append(entrypoint, "--name")
	entrypoint = append(entrypoint, config.RunningConfig.ChainNodeName)
	//??????

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
	dataMount := mount.Mount{
		Type:   mount.TypeVolume,
		Source: upgradeVolueName,
		Target: "/opt/dcnetio/data",
	}
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
		Mounts:      []mount.Mount{dataMount},
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

// stop dcstorage in docker
func stopDcnodeInDocker() (err error) {
	ctx := context.Background()
	//????????????????????????
	cancelDaemonCommandDeal()
	err = util.StopContainer(ctx, nodeContainerName)
	return
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

// ??????dcstorage??????dcupdate???????????????????????????????????????????????????????????????enclavid
func getVersionByHttpGet(localport int) (version string, enclaveId string, err error) {
	dcEnclaveIdUrl := fmt.Sprintf("http://127.0.0.1:%d/version", localport)
	respBody, err := util.HttpGet(dcEnclaveIdUrl)
	if err != nil {
		return
	}
	versionInfo := string(respBody)
	values := strings.Split(versionInfo, "@")
	if len(values) < 2 {
		fmt.Println("get invalid version info")
	} else {
		enclaveId = values[0]
		version = values[1]
	}
	return

}

// ??????dcstorage????????????????????????????????????????????????????????????
func getPeerInfoByHttpGet() (peerid, account, walletAddr string, err error) {
	dcPeerInfoUrl := fmt.Sprintf("http://127.0.0.1:%d/peerinfo", dcStorageListenPort)
	respBody, err := util.HttpGet(dcPeerInfoUrl)
	if err != nil {
		return
	}
	peerInfo := string(respBody)
	values := strings.Split(peerInfo, "@")
	if len(values) < 3 {
		fmt.Println("get invalid peer info")
	} else {
		peerid = values[0]
		account = values[1]
		walletAddr = values[2]
	}
	return

}

// ?????????????????????dcupdate???dcstorage??????????????????
func waitDcUpdateGetPeerSecret() (bool, error) {
	dcSecretFlagUrl := fmt.Sprintf("http://127.0.0.1:%d/secretflag", dcUpgradeListenPort)
	ticker := time.NewTicker(time.Second)
	count := 0
	for {
		<-ticker.C
		respBody, err := util.HttpGet(dcSecretFlagUrl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "waitDcUpdateGetPeerSecret requset fail,  err: %v\n", err)
			return false, err
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

// ??????????????????????????????dcstorage???dcupdate????????????
func waitNewDcGetPeerSecret() (bool, error) {
	dcSecretFlagUrl := fmt.Sprintf("http://127.0.0.1:%d/upgradeflag", dcUpgradeListenPort)
	ticker := time.NewTicker(time.Second)
	count := 0
	for {
		<-ticker.C
		respBody, err := util.HttpGet(dcSecretFlagUrl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "waitNewDcGetPeerSecret requset fail,  err: %v\n", err)
			return false, err
		}
		flag := string(respBody)
		if flag == "true" {
			return true, nil
		} else {
			count++
			if count > 600 { //??????10??????
				return false, fmt.Errorf("new version dcstorage get peer secret timeout")
			}
			continue
		}
	}

}

// dcstorage ??????????????????
func upgradeDeal() (err error) {
	//????????????dcstorage????????????????????????????????????????????????dcstorage
	startDcStorageNode()
	//?????????????????????dcstorage???version???enclaveid
	version, enclaveId, err := getVersionByHttpGet(dcStorageListenPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dcstorage enclaveid get fail,err: %v\n", err)
		log.Errorf("dcstorage enclaveid get fail,err: %v", err)
		return
	}
	//???????????????????????????????????????enclaveid
	programInfo, err := blockchain.GetConfigedDcStorageInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "get dcstorage version info from blockchain fail,err: %v\n", err)
		log.Errorf("get dcstorage version info from blockchain fail,err: %v", err)
		return
	}
	//?????????????????????dcstorage???enclaveid??????????????????????????????????????????enclaveid??????
	if enclaveId == programInfo.EnclaveId {
		fmt.Println("dcstorage is the latest version")
		return
	}

	//???????????????????????????enclaveid????????????
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if !blockchain.IfEnclaveIdValid(ctx, programInfo.EnclaveId) {
		return
	}

	//????????????????????????????????????????????????
	localVersion, err := goversion.NewVersion(version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid local version format,err: %v\n", err)
		log.Errorf("invalid local version format,err: %v", err)
		return
	}
	configedVersion, err := goversion.NewVersion(programInfo.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid new version format,err: %v\n", err)
		log.Errorf("invalid new version format,err: %v", err)
		return
	}
	if !localVersion.LessThan(configedVersion) { //??????????????????????????????
		fmt.Fprintf(os.Stdout, "unneed upgrade ,dcstorage localVersion: %s,   configedVersion: %s\n", localVersion, configedVersion)
		return
	}
	//??????????????????????????????registry??????????????????????????????
	tagUrl := programInfo.OriginUrl
	if config.RunningConfig.Registry == "cn" {
		tagUrl = programInfo.MirrorUrl
	}
	//??????????????????dcstorage??????image
	err = pullDcStorageNodeImage(tagUrl)
	if err != nil {
		if config.RunningConfig.Registry == "cn" {
			tagUrl = programInfo.OriginUrl
		} else {
			tagUrl = programInfo.MirrorUrl
		}
		err = pullDcStorageNodeImage(tagUrl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pullDcStorageNodeImage fail,err: %v\n", err)
			log.Errorf("pullDcStorageNodeImage fail,err: %v", err)
			return
		}
	}
	//????????????????????????
	err = startDcupgradeInDocker()
	if err != nil {
		return
	}
	//??????dcupdate????????????????????????
	_, err = waitDcUpdateGetPeerSecret()
	if err != nil {
		fmt.Fprintf(os.Stderr, "update fail,err: %v\n", err)
		log.Errorf("update fail,err: %v", err)
		return
	}
	//?????????????????????dcstorage
	err = stopDcnodeInDocker()
	if err != nil {
		return
	}
	//??????????????????dcstoragenode???docker??????
	err = removeDcStorageNodeInDocker()
	if err != nil {
		return
	}
	//?????????????????????dcstorage??????
	err = startDcStorageNode()
	if err != nil {
		return
	}
	fmt.Println("wait new version to get peer secret")
	log.Info("wait new version to get peer secret")
	//??????????????????dcstorage????????????????????????
	_, err = waitNewDcGetPeerSecret()
	if err != nil {
		fmt.Fprintf(os.Stderr, "update fail,err: %v\n", err)
		log.Errorf("update fail,err: %v", err)
		return
	}
	log.Infof("new version dcstorage  get peer sceret success")
	fmt.Println("new version dcstorage  get peer sceret success")
	//??????dcStoragenode???image???????????????
	config.RunningConfig.NodeImage = tagUrl
	//??????????????????
	if err = config.SaveConfig(config.RunningConfig); err != nil {
		fmt.Fprintf(os.Stderr, "save config fail,err: %v\n", err)
		log.Errorf("save config fail,err: %v", err)
		return
	}
	//????????????version??????????????????????????????????????????
	version, enclaveId, err = getVersionByHttpGet(dcStorageListenPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dcstorage enclaveid get fail,err: %v\n", err)
		log.Errorf("dcstorage enclaveid get fail,err: %v", err)
		return
	}
	if version != programInfo.Version {
		fmt.Fprintf(os.Stderr, "dcstorage version check fail,version: %s, configedVersion: %s\n", version, programInfo.Version)
		log.Errorf("dcstorage version check fail,version: %s, configedVersion: %s", version, programInfo.Version)
		return
	}
	if enclaveId != programInfo.EnclaveId {
		fmt.Fprintf(os.Stderr, "dcstorage enclaveid check fail,enclaveId: %s, configedEnclaveId: %s\n", enclaveId, programInfo.EnclaveId)
		log.Errorf("dcstorage enclaveid check fail,enclaveId: %s, configedEnclaveId: %s", enclaveId, programInfo.EnclaveId)
		//??????????????????dcstorage
		stopDcnodeInDocker()
		return
	}
	log.Infof("dcstorage upgrade success,version: %s,enclaveid: %s", version, enclaveId)
	fmt.Fprintf(os.Stdout, "dcstorage upgrade success,version: %s,enclaveid: %s\n", version, enclaveId)
	return
}

// ?????????docker image
func pullDcStorageNodeImage(image string) (err error) {
	//docker pull
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return
	}
	ctx := context.Background()
	//docker pull
	out, err := cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return
	}
	defer out.Close()
	//docker pull
	_, err = io.ReadAll(out)
	if err != nil {
		return
	}
	return
}

// ??????dcstoragenode???docker??????
func removeDcStorageNodeInDocker() (err error) {
	log.Infof("begin to remove old version dcstorage docker container")
	fmt.Println("begin to remove old version dcstorage docker container")
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return
	}
	//??????dcstorage???docker??????id
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
			//??????????????????dcstorage??????
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

// ???????????????????????????????????????????????????
func GetPidWithListenPort(listenPort int) (pid int64, err error) {
	cmd := fmt.Sprintf("lsof -i:%d| awk '/LISTEN/ && !/awk/ {print $2}'", listenPort)
	//???????????????????????????
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

// ???????????????????????????????????????
func ifStartupConfiged() bool {
	//??????????????????????????????????????????????????????????????????????????????
	_, err := os.Stat(serviceConfigFile)
	if err != nil { //?????????????????????????????????????????????????????????
		return false
	}
	p, err := configparser.NewConfigParserFromFile(serviceConfigFile)
	if err != nil {
		return false
	}
	v, err := p.Get("Service", "ExecStart")
	if err != nil {
		return false
	}
	//??????????????????
	return v == startupContent
}

// ????????????????????????????????????
func ifServiceStartupConfiged() bool {
	return ifStartupConfiged()
}

// ???????????????????????????
func configServiceStartup() bool {
	if !ifStartupConfiged() { //??????????????????????????????????????????
		serviceFile, err := os.OpenFile(serviceConfigFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			log.Fatal(err)
			return false
		}
		defer serviceFile.Close()
		if _, err = serviceFile.WriteString(serviceConfigFileContent); err != nil {
			log.Fatal(err)
			return false
		}
		//reload??????
		cmd := exec.Command("systemctl", "daemon-reload")
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
			return false
		}
		//??????????????????
		cmd = exec.Command("systemctl", "enable", "dc.service")
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
			return false
		}
	}
	return ifServiceStartupConfiged()
}

// ??????????????????
func removeServiceStartup() bool {
	if !ifServiceStartupConfiged() { //?????????????????????
		return true
	}
	//stop dc service with cmd
	cmd := exec.Command("service", "dc", "stop")
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stop dc service fail,err: %v\r", err)
	}
	//????????????????????????
	err = os.Remove(serviceConfigFile)
	if err != nil {
		log.Fatal(err)
		return false
	}
	//reload??????
	cmd = exec.Command("systemctl", "daemon-reload")
	cmd.Run()
	//??????????????????
	cmd = exec.Command("systemctl", "disable", "dc.service")
	cmd.Run()
	return true
}

// ??????docker??????pccs
func runPccsInDocker() (err error) {
	listenPort := 8081
	//?????????????????????????????????
	pid, err := GetPidWithListenPort(listenPort)
	if err == nil && pid > 0 { //?????????????????????????????????????????????
		_, err = util.HttpGetWithoutCheckCert("https://localhost:8081/sgx/certification/v3/rootcacrl")
		if err != nil {
			log.Errorf("Can't start pccs for 8081 port is occupied")
		}
		return
	}
	apiKey := config.RunningConfig.PccsKey
	if len(apiKey) < 32 { //
		return fmt.Errorf("%s is invalid pccs subscription key.For how to subscribe to Intel Provisioning Certificate Service and receive an API key, goto https://api.portal.trustedservices.intel.com/provisioning-certification and click on 'Subscribe'", apiKey)
	}
	apiKeyStr := fmt.Sprintf("APIKEY=%s", apiKey)
	ctx := context.Background()
	dataMount := mount.Mount{
		Type:   mount.TypeVolume,
		Source: pccsVolueName,
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
		Env:   []string{apiKeyStr},
	}
	err = util.StartContainer(ctx, pccsContainerName, true, cConfig, hostConfig)
	//check if pccs is running
	if err == nil {
		startFlag := false
		//wait for pccs to start
		for i := 0; i < 10; i++ {
			_, gerr := util.HttpGetWithoutCheckCert("https://localhost:8081/sgx/certification/v3/rootcacrl")
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

// ??????docker???????????????ID?????????
func showLogsForContainer(containerId string, tnum int) error {
	cli, _ := client.NewClientWithOpts(client.FromEnv)
	defer cli.Close()
	if tnum == 0 { //???????????????????????????????????????
		execResp, err := cli.ContainerInspect(context.Background(), containerId)
		if err != nil { //???????????????
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
		_, err = io.Copy(os.Stdout, reader)
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
