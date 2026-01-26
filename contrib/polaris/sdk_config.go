// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
