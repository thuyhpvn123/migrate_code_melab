package config

import (
	"fmt"
	"github.com/spf13/viper"
)

type AppConfig struct {
	MetaNodeVersion string
	DnsLink_        string

	AdminAddress string
	PrivateKeyAdmin           string
	ParentAddress         string
	NodeConnectionAddress string
	StorageAddress        string

	CodeAddress string
	MiningCodeAddress string

	ParentConnectionAddress  string
	ParentConnectionType     string
	ConnectionAddress_       string
	ChainId                  uint64
	StorageConnectionAddress string
	RpcURL                   string
}

var Config *AppConfig

func LoadConfig(path string) (*AppConfig, error) {
	viper.SetConfigFile(path)

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config AppConfig
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config:   %w", err)
	}

	Config = &config
	return &config, nil
}

func (c *AppConfig) DnsLink() string {
	return c.DnsLink_
}
