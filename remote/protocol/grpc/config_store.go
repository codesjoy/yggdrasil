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

package grpc

import "sync"

// ServerConfig is the exported transport server config shape.
type ServerConfig = serverOptions

// ClientTransportOptions is the exported client transport config shape.
type ClientTransportOptions = clientTransportOptions

// Settings contains resolved gRPC transport settings.
type Settings struct {
	Client         Config
	ClientServices map[string]Config
	Server         ServerConfig
}

var (
	settingsMu sync.RWMutex
	settingsV  = Settings{ClientServices: map[string]Config{}}
)

// Configure replaces the resolved gRPC transport settings.
func Configure(next Settings) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if next.ClientServices == nil {
		next.ClientServices = map[string]Config{}
	}
	settingsV = next
}

func currentSettings() Settings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return settingsV
}
