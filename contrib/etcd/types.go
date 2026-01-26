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

package etcd

import (
	"time"

	"github.com/codesjoy/yggdrasil/v2/config/source"
)

const (
	// ConfigSourceModeBlob is the mode for blob config source.
	ConfigSourceModeBlob = "blob"
	// ConfigSourceModeKV is the mode for kv config source.
	ConfigSourceModeKV = "kv"
)

// ClientConfig defines the configuration for an etcd client.
type ClientConfig struct {
	Endpoints   []string      `mapstructure:"endpoints"`
	DialTimeout time.Duration `mapstructure:"dialTimeout"`
	Username    string        `mapstructure:"username"`
	Password    string        `mapstructure:"password"`
}

// ConfigSourceConfig defines the configuration for an etcd config source.
type ConfigSourceConfig struct {
	Client ClientConfig  `mapstructure:"client"`
	Key    string        `mapstructure:"key"`
	Prefix string        `mapstructure:"prefix"`
	Mode   string        `mapstructure:"mode"`
	Watch  *bool         `mapstructure:"watch"`
	Format source.Parser `mapstructure:"format"`
	Name   string        `mapstructure:"name"`
}

// RegistryConfig defines the configuration for an etcd registry.
type RegistryConfig struct {
	Client        ClientConfig  `mapstructure:"client"`
	Prefix        string        `mapstructure:"prefix"`
	TTL           time.Duration `mapstructure:"ttl"`
	KeepAlive     *bool         `mapstructure:"keepAlive"`
	RetryInterval time.Duration `mapstructure:"retryInterval"`
}

// ResolverConfig defines the configuration for an etcd resolver.
type ResolverConfig struct {
	Client    ClientConfig  `mapstructure:"client"`
	Prefix    string        `mapstructure:"prefix"`
	Namespace string        `mapstructure:"namespace"`
	Protocols []string      `mapstructure:"protocols"`
	Debounce  time.Duration `mapstructure:"debounce"`
}

type instanceRecord struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	Region    string            `json:"region"`
	Zone      string            `json:"zone"`
	Campus    string            `json:"campus"`
	Metadata  map[string]string `json:"metadata"`
	Endpoints []endpointRecord  `json:"endpoints"`
}

type endpointRecord struct {
	Scheme   string            `json:"scheme"`
	Address  string            `json:"address"`
	Metadata map[string]string `json:"metadata"`
}
