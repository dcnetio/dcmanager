package util

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ChainSafe/go-schnorrkel"
	"github.com/cosmos/go-bip39"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	logging "github.com/ipfs/go-log/v2"
	"github.com/klauspost/cpuid"
	"github.com/libp2p/go-libp2p/core/crypto"
)

var log = logging.Logger("dcmanager")

func HttpGet(url string, args ...string) ([]byte, error) {
	client := http.Client{Timeout: 10 * time.Second}
	if len(args) > 0 {
		url += "?" + strings.Join(args, "&")
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		newStr := buf.String()
		return nil, fmt.Errorf("http get err status,statuscode: %d,errmsg: %v", resp.StatusCode, newStr)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("err occur,no data get")
	}
	return body, nil
}

func HttpGetWithoutCheckCert(url string, args ...string) ([]byte, error) {
	//	client := http.Client{Timeout: time.Second}
	if len(args) > 0 {
		url += "?" + strings.Join(args, "&")
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Second}
	//request with out check cert
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "dcmanager")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		newStr := buf.String()
		return nil, fmt.Errorf("http get err status,statuscode: %d,errmsg: %v", resp.StatusCode, newStr)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("err occur,no data get")
	}
	return body, nil
}

func HttpPost(url string, body []byte) ([]byte, error) {
	client := http.Client{Timeout: time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		newStr := buf.String()
		return nil, fmt.Errorf("http post err status,statuscode: %d,errmsg: %v", resp.StatusCode, newStr)
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("err occur,no data get")
	}
	return body, nil
}

// Obtain random asymmetric encryption and decryption private key
func GetRandomPrivKey() (crypto.PrivKey, error) {
	//Generate mnemonic words
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return nil, err
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return nil, err
	}
	seed, err := schnorrkel.SeedFromMnemonic(mnemonic, "")
	if err != nil {
		return nil, err
	}
	secret := ed25519.NewKeyFromSeed(seed[:32])
	priv, err := crypto.UnmarshalEd25519PrivateKey([]byte(secret))
	if err != nil {
		return nil, err
	}

	return priv, nil

}

// SetupDefaultLoggingConfig sets up a standard logging configuration.
func SetupDefaultLoggingConfig(file string) error {
	c := logging.Config{
		Format: logging.ColorizedOutput,
		Stderr: false,
		Level:  logging.LevelInfo,
	}
	if file != "" {
		if err := os.MkdirAll(filepath.Dir(file), 0777); err != nil {
			return err
		}
		c.File = file
	}
	os.Chmod(file, 0777)
	logging.SetupLogging(c)
	return nil
}

func Sha256sum(filepath string) (checksum string, err error) {
	var f *os.File
	f, err = os.Open(filepath)
	if err != nil {
		return
	}
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return
	}
	checksum = fmt.Sprintf("%x", h.Sum(nil))
	return
}

// create volume
func CreateVolume(ctx context.Context, volumeName string) (v *volume.Volume, err error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatalf("create docker client fail,err:%v", err)
	}
	volumeList, err := cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		log.Fatalf("list docker volume fail,err:%v", err)
	}
	for _, v = range volumeList.Volumes {
		if v.Name == volumeName {
			return
		}
	}
	newVolume, err := cli.VolumeCreate(ctx, volume.CreateOptions{
		Name: volumeName,
	})
	v = &newVolume
	if err != nil {
		log.Fatalf("create docker volume fail,err:%v", err)
	} else {
		fmt.Printf("create docker volume %s success\n", volumeName)
	}
	return
}

// start container removeOldFlag: true  if exist same name container with different image,remove the old container
func StartContainer(ctx context.Context, containerName string, removeOldFlag bool, config *container.Config, hostConfig *container.HostConfig) (err error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return
	}
	defer cli.Close()
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return
	}
	createdFlag := false
	containerId := ""
	for _, container := range containers {
		if config.Image == container.Image {
			for _, name := range container.Names {
				if name == "/"+containerName {
					createdFlag = true
					containerId = container.ID
					break
				}
			}
		}
	}
	if !createdFlag { //need to create
		fmt.Printf("creating %s container ...\n", containerName)
		log.Infof("creating %s container ...", containerName)
		resp, cerr := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
		if cerr != nil {

			conflictMsg := fmt.Sprintf("Conflict. The container name \"/%s\" is already in use by container", containerName)
			//remove container with same name
			if strings.Contains(cerr.Error(), conflictMsg) && removeOldFlag {

				fmt.Printf("container %s already exists, need remove \n", containerName)
				log.Infof("container %s already exists, remove  it", containerName)
				fmt.Printf("stopping %s container ...\n", containerName)
				log.Infof("stopping %s container ...", containerName)
				err = cli.ContainerStop(ctx, containerName, container.StopOptions{})
				if err != nil {
					return
				}
				fmt.Printf("removing %s container ...\n", containerName)
				log.Infof("removing %s container ...", containerName)
				if err = cli.ContainerRemove(ctx, containerName, types.ContainerRemoveOptions{Force: true}); err != nil {
					return
				}
				fmt.Printf("creating %s container ...\n", containerName)
				log.Infof("creating %s container ...", containerName)
				resp, err = cli.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
				if err != nil {
					return
				}
			} else {

				err = cerr
				return
			}
		}
		containerId = resp.ID
	}

	execResp, err := cli.ContainerInspect(ctx, containerId)
	if err != nil {
		fmt.Fprintf(os.Stderr, "inspect %s container fail,err: %v\r\n", containerName, err)
		log.Infof("inspect %s container fail,err: %v", containerName, err)
		return

	}
	if !execResp.State.Running { // The service is not started
		fmt.Printf("starting %s  ...\n", containerName)
		log.Infof("starting %s  ...\n", containerName)
		if err := cli.ContainerStart(ctx, containerId, types.ContainerStartOptions{}); err != nil {
			fmt.Fprintf(os.Stderr, "start %s fail,err: %v\r\n", containerName, err)
			log.Infof("start %s fail,err: %v", containerName, err)
			return err
		}
		fmt.Printf("start %s success\r\n", containerName)
		log.Infof("start %s success", containerName)
	} else {
		fmt.Printf("%s is running\r\n", containerName)
	}
	return
}

// stop container
func StopContainer(ctx context.Context, containerName string) (err error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return
	}
	defer cli.Close()
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return
	}
	containerId := ""
	for _, container := range containers {
		for _, name := range container.Names {
			if name == "/"+containerName {
				containerId = container.ID
				break
			}
		}
	}
	if containerId != "" {
		fmt.Printf("stopping %s  ...\r\n", containerName)
		log.Infof("stopping %s  ...", containerName)
		waitTimeout := 60 // 60s
		if err = cli.ContainerStop(ctx, containerId, container.StopOptions{Timeout: &waitTimeout}); err != nil {
			fmt.Fprintf(os.Stderr, "stop %s  fail,err: %v\r\n", containerName, err)
			log.Infof("stop %s  fail,err: %v", containerName, err)
			return
		}
	} else {
		fmt.Printf("%s  is not running\r\n", containerName)
		log.Infof("no need stop, %s  is not running", containerName)
	}
	return
}

func RemoveContainer(ctx context.Context, containerName string) (err error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return
	}
	defer cli.Close()
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return
	}
	containerId := ""
	for _, container := range containers {
		for _, name := range container.Names {
			if name == "/"+containerName {
				containerId = container.ID
				break
			}
		}
	}
	if containerId != "" {
		execResp, ierr := cli.ContainerInspect(ctx, containerId)
		if ierr != nil {
			fmt.Fprintf(os.Stderr, "inspect %s container fail,err: %v\r\n", containerName, ierr)
			log.Infof("inspect %s container fail,err: %v", containerName, ierr)
			return ierr

		}
		if execResp.State.Running { // The service is still started and needs to be stopped first.
			fmt.Printf("stopping %s  ...\r\n", containerName)
			log.Infof("stopping %s  ...", containerName)
			if err = cli.ContainerStop(ctx, containerId, container.StopOptions{}); err != nil {
				fmt.Fprintf(os.Stderr, "stop %s  fail,err: %v\r\n", containerName, err)
				log.Infof("stop %s  fail,err: %v", containerName, err)
				return
			}
		}
		fmt.Printf("removing container %s  ...\r\n", containerName)
		log.Infof("removing container %s  ...", containerName)
		if err = cli.ContainerRemove(ctx, containerId, types.ContainerRemoveOptions{Force: true}); err != nil {
			fmt.Fprintf(os.Stderr, "remove container %s  fail,err: %v\r\n", containerName, err)
			log.Infof("remove container %s  fail,err: %v", containerName, err)
			return err
		}
	}
	// Prompt removal successful
	fmt.Printf("remove container %s  success\r\n", containerName)
	return
}

// RemoveVolume removes a volume
func RemoveVolume(ctx context.Context, volumeName string) (err error) {
	// Determine whether the volume exists
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return
	}
	defer cli.Close()
	volumes, err := cli.VolumeList(context.Background(), volume.ListOptions{})
	if err != nil {
		return
	}
	volumeId := ""
	for _, volume := range volumes.Volumes {
		if volume.Name == volumeName {
			volumeId = volume.Name
			break
		}
	}
	if volumeId != "" {
		fmt.Printf("removing volume %s  ...\r\n", volumeName)
		log.Infof("removing volume %s  ...", volumeName)
		if err = cli.VolumeRemove(ctx, volumeId, true); err != nil {
			fmt.Fprintf(os.Stderr, "remove volume %s  fail,err: %v\r\n", volumeName, err)
			log.Infof("remove volume %s  fail,err: %v", volumeName, err)
			return err
		}
	}
	// Prompt removal successful
	fmt.Printf("remove volume %s  success\r\n", volumeName)
	return
}

// Generate random string
func RandStringBytes(n int) string {
	rn := rand.New(rand.NewSource(time.Now().UnixNano()))
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rn.Intn(len(letterBytes))]
	}
	return string(b)
}

// PrintMemUsage outputs the current, total and OS memory being used. As well as the number
// of garage collection cycles completed.
func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	log.Infof("Alloc = %v MiB", bToMb(m.Alloc))
	log.Infof("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	log.Infof("\tSys = %v MiB", bToMb(m.Sys))
	log.Infof("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

// if cpu support sgx, return true
func IsSgxSupport() bool {
	return cpuid.CPU.SGX.SGX1Supported || cpuid.CPU.SGX.SGX2Supported
}

// if cpu support sgx2, return true
func IsSgx2Support() bool {
	return cpuid.CPU.SGX.SGX2Supported
}

// Get the epc size of cpu
func GetEpcSize() uint64 {
	epcSize := uint64(0)
	for _, section := range cpuid.CPU.SGX.EPCSections {
		if epcSize < section.EPCSize {
			epcSize = section.EPCSize
		}
	}
	return epcSize/1024/1024/1024 + 1 //The unit is g
}
