package main

import (
	"fmt"
	"os"

	"github.com/dcnetio/dc/command"
	"github.com/dcnetio/dc/config"
	"github.com/dcnetio/dc/util"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("dcmanager")

// logpath := "./log"
const logpath = "/opt/dcnetio/log"

func main() {
	util.SetupDefaultLoggingConfig(logpath)
	//Determine whether the configuration file exists
	_, err := os.Stat(config.Config_file_path)
	if err != nil {
		if os.IsNotExist(err) { //File does not exist, update default configuration to configuration file
			//Create a directory
			if err = config.SaveConfig(config.RunningConfig); err != nil {
				return
			}
		} else {
			return
		}
	}
	//Read configuration file
	if config.RunningConfig, err = config.ReadConfig(); err != nil {
		log.Fatalf("read config file fail,err: %v", err)
		return
	}
	//Determine whether the chain node name is empty. If it is empty, generate a random chain node name.
	if config.RunningConfig.ChainNodeName == "" {
		config.RunningConfig.ChainNodeName = "dcnet_" + util.RandStringBytes(12)
		if err = config.SaveConfig(config.RunningConfig); err != nil {
			return
		}
	}
	//Read command line parameters and parse the response
	if len(os.Args) == 1 { //show help
		command.ShowHelp()
		os.Exit(1)
	}
	//Determine whether the verification node has been configured to open. If it is not configured, it prompts for configuration.
	if config.RunningConfig.ValidatorFlag == "" && os.Args[1] != "config" {
		fmt.Println("please config chain first,use command:  dc config")
		os.Exit(0) //exit the program
	}

	switch os.Args[1] {
	case "config":
		command.ConfigCommandDeal()
	case "start":
		command.StartCommandDeal()
	case "stop":
		command.StopCommandDeal()
	case "status":
		command.StatusCommandDeal()
	case "log":
		command.LogCommandDeal()
	case "upgrade":
		command.UpgradeCommandDeal()
	case "uniqueid":
		command.UniqueIdCommandDeal()
	case "peerinfo":
		command.PeerInfoCommandDeal()
	case "memusage":
		command.MemoryUsageCommandDeal()
	case "checksum":
		command.ChecksumCommandDeal()
	case "get":
		command.GetFileFromIpfsCommandDeal()
	case "rotate-keys":
		command.RotateKeyCommandDeal()
	case "pccs_api_key":
		command.PccsApiKeyCommandDeal()
	case "blockgc": //Manually enable block recycling
		command.BlockGcCommandDeal()
	default:
		command.ShowHelp()
	}
	os.Exit(1)
}
