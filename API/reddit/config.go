package main

import (
	"io/ioutil"
	"os"

	"github.com/naoina/toml"
)

//TomlConfig is needed for configure programm.
type TomlConfig struct {
	Title        string
	GrpcAddress string
	HttpAddress string

	Reddit      Reddit
	TelegramBot struct {
		Token       string
		ChatIdDebug int64
	}

	Log struct {
		Type        string
		NetworkType string
		Host        string
		Severity    string
		Facility    string
		Port        string
		FilePath    string
		FileName    string
		DebugMode   bool
	}
}

var (
	//Config struct
	Config *TomlConfig
)

func getConfig(configFileName string) (*TomlConfig, error) {
	var config TomlConfig
	f, err := os.Open(configFileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(buf, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
