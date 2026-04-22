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

package rpchttp

import (
	"time"

	"github.com/codesjoy/yggdrasil/v3/remote/marshaler"
)

// MarshalerConfig http marshaler config
type MarshalerConfig struct {
	Type   string            `mapstructure:"type"`
	Config *JSONPbConfigOpts `mapstructure:"config"`
}

// JSONPbConfigOpts json pb config
type JSONPbConfigOpts = marshaler.JSONPbConfig

// ClientConfig http client config
type ClientConfig struct {
	Timeout   time.Duration       `mapstructure:"timeout"   default:"10s"`
	Marshaler *MarshalerConfigSet `mapstructure:"marshaler"`
}

// MarshalerConfigSet http marshaler config
type MarshalerConfigSet struct {
	Inbound  *MarshalerConfig `mapstructure:"inbound"`
	Outbound *MarshalerConfig `mapstructure:"outbound"`
}

// ServerConfig http server config
type ServerConfig struct {
	Network      string              `mapstructure:"network"        default:"tcp"`
	Address      string              `mapstructure:"address"        default:":0"`
	ReadTimeout  time.Duration       `mapstructure:"read_timeout"   default:"0s"`
	WriteTimeout time.Duration       `mapstructure:"write_timeout"  default:"0s"`
	IdleTimeout  time.Duration       `mapstructure:"idle_timeout"   default:"0s"`
	MaxBodyBytes int64               `mapstructure:"max_body_bytes" default:"4194304"`
	Marshaler    *MarshalerConfigSet `mapstructure:"marshaler"`
	Attr         map[string]string   `mapstructure:"attr"`
}
