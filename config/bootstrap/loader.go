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

package bootstrap

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source"
	filesource "github.com/codesjoy/yggdrasil/v2/config/source/file"
)

// Loader loads bootstrap config files and their declared sources.
type Loader struct {
	registry *Registry
}

// NewLoader creates a bootstrap loader.
func NewLoader(registry *Registry) *Loader {
	if registry == nil {
		registry = NewRegistry()
	}
	return &Loader{registry: registry}
}

// LoadFile loads a bootstrap file and all declarative sources it defines.
func (l *Loader) LoadFile(manager *config.Manager, path string, explicit bool) ([]source.Source, bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false, errors.New("bootstrap config path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if explicit {
				return nil, false, fmt.Errorf("bootstrap config file %q not found", path)
			}
			slog.Warn("bootstrap config file not found, fallback to default startup", slog.String("path", path))
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("stat bootstrap config file %q: %w", path, err)
	}

	loaded := make([]source.Source, 0, 4)
	bootstrapSource := filesource.NewSource(path, false)
	if err := manager.LoadLayer("bootstrap:file:"+path, config.PriorityFile, bootstrapSource); err != nil {
		return nil, false, err
	}
	loaded = append(loaded, bootstrapSource)

	specs := make([]SourceSpec, 0)
	if err := manager.Section("yggdrasil", "bootstrap", "sources").Decode(&specs); err != nil {
		return nil, false, err
	}
	for i, spec := range specs {
		if spec.Enabled != nil && !*spec.Enabled {
			continue
		}
		src, priority, err := l.registry.Build(spec)
		if err != nil {
			return nil, false, fmt.Errorf("bootstrap source[%d]: %w", i, err)
		}
		name := spec.Name
		if name == "" {
			name = fmt.Sprintf("bootstrap:%s:%d", strings.ToLower(spec.Kind), i)
		}
		if err := manager.LoadLayer(name, priority, src); err != nil {
			return nil, false, fmt.Errorf("bootstrap source[%d]: %w", i, err)
		}
		loaded = append(loaded, src)
	}
	return loaded, true, nil
}
