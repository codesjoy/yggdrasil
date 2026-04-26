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

package stats

import (
	"strings"
	"sync"
)

// ProviderSettings contains provider-specific stats payloads.
type ProviderSettings struct {
	OTel map[string]any `mapstructure:"otel"`
}

// Settings contains resolved observability stats settings.
type Settings struct {
	Server    string           `mapstructure:"server"`
	Client    string           `mapstructure:"client"`
	Providers ProviderSettings `mapstructure:"providers"`
}

var (
	settingsMu sync.RWMutex
	settingsV  Settings
)

// Configure replaces the stats settings and clears memoized handler chains.
func Configure(next Settings) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	settingsV = next
	svrOnce = sync.Once{}
	cliOnce = sync.Once{}
	svrHandler = nil
	cliHandler = nil
}

// CurrentSettings returns the current stats settings.
func CurrentSettings() Settings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return settingsV
}

// ParseHandlerNames parses a comma-separated handler list, trimming spaces and
// deduplicating names while preserving order.
func ParseHandlerNames(raw string) []string {
	items := strings.Split(raw, ",")
	names := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}
