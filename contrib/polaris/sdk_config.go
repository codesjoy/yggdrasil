package polaris

import "github.com/codesjoy/yggdrasil/v2/config"

// SDKConfig is the config for the Polaris SDK.
type SDKConfig struct {
	Addresses     []string `mapstructure:"addresses"`
	ConfigAddress []string `mapstructure:"config_addresses"`
	Token         string   `mapstructure:"token"`
	ConfigFile    string   `mapstructure:"config_file"`
}

// LoadSDKConfig loads the SDK config for the given name.
func LoadSDKConfig(name string) SDKConfig {
	var cfg SDKConfig
	_ = config.Get(config.Join(config.KeyBase, "polaris", name)).Scan(&cfg)
	return cfg
}

func resolveSDKName(ownerName string, sdkName string) string {
	if sdkName != "" {
		return sdkName
	}
	return ownerName
}

func resolveSDKAddresses(ownerName string, sdkName string, explicit []string) []string {
	if len(explicit) > 0 {
		return explicit
	}
	return LoadSDKConfig(resolveSDKName(ownerName, sdkName)).Addresses
}

func resolveSDKConfigAddresses(ownerName string, sdkName string, explicit []string) []string {
	if len(explicit) > 0 {
		return explicit
	}
	cfg := LoadSDKConfig(resolveSDKName(ownerName, sdkName))
	return cfg.ConfigAddress
}
