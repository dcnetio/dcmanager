package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"

	yaml "gopkg.in/yaml.v2"
)

//
//go:embed  "version.json"
var configVersion []byte
var GetVersion = func() (verStr string) {
	var version struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(configVersion, &version); err != nil {
		fmt.Println("unmarshal version.json error:", err)
	}
	verStr = version.Version
	return
}()

const Config_file_path = "/opt/dcnetio/etc/manage_config.yaml"
const CommitBasePubkey = "bl3kr5jjklu2iijnmyhz7cy5lz3h5xhrlp7sim54bjhc4v3ztzfdq" //The pubkey used by the technical committee to release the upgraded version of dcstorage
// Node related program version information
type DcProgram struct {
	OriginUrl string   `yaml:"originUrl"` //Program download address
	MirrorUrl string   `yaml:"mirrorUrl"` //Program file download path mirror address, used as an alternative download address when nodes download program files.
	EnclaveId string   `yaml:"enclaveId"` //The tee enclaveid corresponding to the program
	Version   string   `yaml:"version"`   //Program version information
	MirrCids  []string `yaml:"mirrCids"`  //The cid list of the program file, the cid list of the docker image in the DC network
}

var RunningConfig = &DcManageConfig{
	ChainNodeName:        "",
	ValidatorFlag:        "",
	ChainSyncMode:        "full", //Blockchain synchronization mode supports full, fast, fast-unsafe, warp and defaults to fast
	ChainWsUrl:           "ws://127.0.0.1:9944",
	ChainRpcListenPort:   9944, //New version of chain node rpc listening port, default 9944
	PccsKey:              "",   //Subscription key for intel pccs service
	ChainImage:           "ghcr.io/dcnetio/dcchain:latest",
	NodeImage:            "ghcr.io/dcnetio/dcstorage:latest",
	UpgradeImage:         "ghcr.io/dcnetio/dcupgrade:latest",
	TeeReportServerImage: "ghcr.io/dcnetio/dcteereportserver:0.1.2",
	PccsImage:            "ghcr.io/dcnetio/pccs:latest",
	Registry:             "ghcr.io/dcnetio",
	ChainBootNode:        "",
	ChainExposeFlag:      "", //Whether to enable the RPC port of the chain node to be exposed to the public network. It is not enabled by default.
	NewVersion: DcProgram{
		OriginUrl: "",
		MirrorUrl: "",
		EnclaveId: "",
		Version:   "",
		MirrCids:  []string{},
	},
}

type DcManageConfig struct {
	ChainNodeName        string    `yaml:"chainNodeName"`
	ValidatorFlag        string    `yaml:"validatorFlag"`
	ChainSyncMode        string    `yaml:"chainSyncMode"`
	ChainWsUrl           string    `yaml:"chainWsUrl"`
	ChainRpcListenPort   int       `yaml:"chainRpcListenPort"`
	PccsKey              string    `yaml:"pccsKey"`
	ChainImage           string    `yaml:"chainImage"`
	NodeImage            string    `yaml:"nodeImage"`
	UpgradeImage         string    `yaml:"upgradeImage"`
	TeeReportServerImage string    `yaml:"teeReportServerImage"`
	PccsImage            string    `yaml:"pccsImage"`
	Registry             string    `yaml:"registry"`
	ChainBootNode        string    `yaml:"chainBootNode"`
	ChainExposeFlag      string    `yaml:"chainExposeFlag"`
	NewVersion           DcProgram `yaml:"newVersion"`
}

func ReadConfig() (*DcManageConfig, error) {
	yamlFile, err := os.ReadFile(Config_file_path)
	if err != nil {
		log.Fatalf("yamlFile.Get err #%v ", err)
		return nil, err
	}
	localconfig := &DcManageConfig{}
	err = yaml.Unmarshal(yamlFile, localconfig)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
		return nil, err
	}

	return localconfig, nil
}

func SaveConfig(config *DcManageConfig) (err error) {
	fileBytes, err := yaml.Marshal(config)
	if err != nil {
		log.Fatalf("Marshal: %v", err)
		return err
	}
	err = os.WriteFile(Config_file_path, fileBytes, os.ModePerm)
	if err != nil {
		log.Fatalf("yamlFile.Save err #%v ", err)
		return err
	}

	return nil
}
