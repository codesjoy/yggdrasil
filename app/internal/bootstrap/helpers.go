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
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/config"
	configchain "github.com/codesjoy/yggdrasil/v3/config/chain"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
)

// Config flag and default path constants.
const (
	ConfigFlagName        = "yggdrasil-config"
	ConfigSourcesEnvName  = "YGGDRASIL_CONFIG_SOURCES"
	ConfigSourcesFlagName = "yggdrasil-config-sources"
	DefaultConfigPath     = "./config.yaml"
)

// ResolveConfigPath returns the config path and whether it was set explicitly.
func ResolveConfigPath(configuredPath string) (string, bool) {
	if path, ok := ParseNamedFlagArg(os.Args[1:], ConfigFlagName); ok {
		return path, true
	}
	if path := strings.TrimSpace(configuredPath); path != "" {
		return path, true
	}
	if path, ok := LookupRegisteredFlagValue(ConfigFlagName); ok {
		return path, false
	}
	return DefaultConfigPath, false
}

// LookupRegisteredFlagValue returns the value of an already registered flag.
func LookupRegisteredFlagValue(name string) (string, bool) {
	f := flag.CommandLine.Lookup(name)
	if f == nil {
		return "", false
	}
	if path := strings.TrimSpace(f.Value.String()); path != "" {
		return path, true
	}
	return "", false
}

// ParseNamedFlagArg extracts one named flag value from raw CLI args.
func ParseNamedFlagArg(args []string, name string) (string, bool) {
	longFlag := "--" + name
	shortFlag := "-" + name
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		switch {
		case arg == longFlag || arg == shortFlag:
			if i+1 < len(args) {
				return strings.TrimSpace(args[i+1]), true
			}
			return "", true
		case strings.HasPrefix(arg, longFlag+"="):
			return strings.TrimSpace(strings.TrimPrefix(arg, longFlag+"=")), true
		case strings.HasPrefix(arg, shortFlag+"="):
			return strings.TrimSpace(strings.TrimPrefix(arg, shortFlag+"=")), true
		}
	}
	return "", false
}

// LoadConfigFile loads the file-backed config chain and returns owned sources.
func LoadConfigFile(
	manager *config.Manager,
	path string,
	explicit bool,
	registry *configchain.Registry,
) ([]source.Source, bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		if explicit {
			return nil, false, fmt.Errorf(
				"config path is empty; use --%s=/path/to/config.yaml",
				ConfigFlagName,
			)
		}
		path = DefaultConfigPath
	}
	if manager == nil {
		return nil, false, errors.New("config manager is nil")
	}

	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if explicit {
				return nil, false, fmt.Errorf(
					"config file %q not found; use --%s=/path/to/config.yaml",
					path,
					ConfigFlagName,
				)
			}
			slog.Warn(
				"config file not found, fallback to default startup; use --yggdrasil-config to set config path",
				slog.String("path", path),
			)
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("stat config file %q: %w", path, err)
	}
	loader := configchain.NewLoader(registry)
	sources, loaded, err := loader.LoadFile(manager, path, explicit)
	if err != nil {
		return nil, false, err
	}
	return sources, loaded, nil
}

// LoadConfigLayer loads one explicit config layer into the manager.
func LoadConfigLayer(
	manager *config.Manager,
	name string,
	priority config.Priority,
	item source.Source,
	scope string,
	index int,
) error {
	if item == nil {
		return nil
	}
	if manager == nil {
		return errors.New("config manager is nil")
	}
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("%s:%d", scope, index)
	}
	if err := manager.LoadLayer(name, priority, item); err != nil {
		return fmt.Errorf("%s config source[%d]: %w", scope, index, err)
	}
	return nil
}

// CloseConfigSourcesReverse closes config sources in reverse registration order.
func CloseConfigSourcesReverse(sources []source.Source) error {
	var multiErr error
	for i := len(sources) - 1; i >= 0; i-- {
		item := sources[i]
		if item == nil {
			continue
		}
		if err := item.Close(); err != nil {
			multiErr = errors.Join(
				multiErr,
				fmt.Errorf(
					"close config source kind=%q name=%q: %w",
					item.Kind(),
					item.Name(),
					err,
				),
			)
		}
	}
	return multiErr
}

// RefreshResolvedSettings compiles the current manager snapshot into resolved settings.
func RefreshResolvedSettings(manager *config.Manager) (settings.Resolved, error) {
	if manager == nil {
		return settings.Resolved{}, errors.New("config manager is nil")
	}
	if err := settings.ValidateV3RootShape(manager.Snapshot().Map()); err != nil {
		return settings.Resolved{}, err
	}
	catalog := settings.NewCatalog(manager)
	root, err := catalog.Root().Current()
	if err != nil {
		return settings.Resolved{}, err
	}
	return settings.Compile(root)
}

// ResolveStartupSettings returns the resolved startup settings using the provided manager when needed.
func ResolveStartupSettings(
	resolved settings.Resolved,
	mgr *config.Manager,
) (settings.Resolved, error) {
	if !NeedsDefaultStartupSettings(resolved) {
		return resolved, nil
	}
	if mgr == nil {
		return settings.Resolved{}, errors.New("config manager is nil")
	}
	root, err := settings.NewCatalog(mgr).Root().Current()
	if err != nil {
		return settings.Resolved{}, err
	}
	return settings.Compile(root)
}

// NeedsDefaultStartupSettings reports whether startup defaults still need to be resolved from config.
func NeedsDefaultStartupSettings(resolved settings.Resolved) bool {
	return resolved.Logging.Handlers == nil &&
		resolved.Discovery.Registry.Type == "" &&
		len(resolved.Server.Transports) == 0 &&
		resolved.Transports.Rest == nil
}

type startupValidator struct {
	strict bool
	err    error
}

func (v *startupValidator) add(msg string, err error, attrs ...slog.Attr) {
	if err == nil {
		return
	}
	if v.strict {
		v.err = errors.Join(v.err, fmt.Errorf("%s: %w", msg, err))
		return
	}
	attrs = append(attrs, slog.Any("error", err))
	args := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		args = append(args, attr)
	}
	slog.Warn(msg, args...)
}

// ValidateStartupResolved validates already resolved startup settings.
func ValidateStartupResolved(resolved settings.Resolved) error {
	strict := resolved.Admin.Validation.Strict
	enable := strict || resolved.Admin.Validation.Enable
	if err := settings.Validate(resolved); err != nil {
		return err
	}
	if !enable {
		return nil
	}

	validator := startupValidator{strict: strict}
	return validator.err
}
