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

// Package otel provides a tracing and metrics provider for OpenTelemetry.
package otel

import (
	"github.com/codesjoy/yggdrasil/pkg/config"
)

// Config is the configuration for the OpenTelemetry provider.
type Config struct {
	ReceivedEvent bool `default:"true"`
	SentEvent     bool `default:"true"`
	EnableMetrics bool `default:"true"`
}

func getCfg() *Config {
	globalCfg := &Config{}
	key := config.Join(config.KeyBase, "stats", "config", "otel")
	_ = config.Get(key).Scan(globalCfg)
	return globalCfg
}
