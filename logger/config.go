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

package logger

import "sync"

// HandlerSpec describes a named logger handler.
type HandlerSpec struct {
	Type   string         `mapstructure:"type"`
	Writer string         `mapstructure:"writer"`
	Config map[string]any `mapstructure:"config"`
}

// WriterSpec describes a named logger writer.
type WriterSpec struct {
	Type   string         `mapstructure:"type"`
	Config map[string]any `mapstructure:"config"`
}

// Settings contains framework logging settings resolved by the assembly layer.
type Settings struct {
	Handlers     map[string]HandlerSpec    `mapstructure:"handlers"`
	Writers      map[string]WriterSpec     `mapstructure:"writers"`
	Interceptors map[string]map[string]any `mapstructure:"interceptors"`
	RemoteLevel  string                    `mapstructure:"remote_level"`
}

var (
	settingsMu sync.RWMutex
	settingsV  = Settings{
		Handlers:     map[string]HandlerSpec{},
		Writers:      map[string]WriterSpec{},
		Interceptors: map[string]map[string]any{},
	}
)

// Configure replaces the process logger settings snapshot.
func Configure(next Settings) {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	if next.Handlers == nil {
		next.Handlers = map[string]HandlerSpec{}
	}
	if next.Writers == nil {
		next.Writers = map[string]WriterSpec{}
	}
	if next.Interceptors == nil {
		next.Interceptors = map[string]map[string]any{}
	}
	settingsV = next
}

// CurrentSettings returns the current logger settings snapshot.
func CurrentSettings() Settings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return settingsV
}

// InterceptorConfig returns the named interceptor payload.
func InterceptorConfig(name string) map[string]any {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	cfg := settingsV.Interceptors[name]
	if cfg == nil {
		return nil
	}
	out := make(map[string]any, len(cfg))
	for k, v := range cfg {
		out[k] = v
	}
	return out
}
