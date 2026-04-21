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

package yggdrasil

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/codesjoy/yggdrasil/v2/application"
	"github.com/codesjoy/yggdrasil/v2/config"
	configbootstrap "github.com/codesjoy/yggdrasil/v2/config/bootstrap"
	"github.com/codesjoy/yggdrasil/v2/config/source"
)

const (
	bootstrapConfigFlagName    = "yggdrasil-config"
	defaultBootstrapConfigPath = "./config.yaml"
)

func initConfigChain(opts *options) error {
	if opts.configManager == nil {
		opts.configManager = config.Default()
	} else {
		config.SetDefault(opts.configManager)
	}
	if err := loadBootstrapConfigChain(opts); err != nil {
		return err
	}
	if err := loadProgrammaticConfigSources(opts); err != nil {
		return err
	}
	if err := refreshResolvedSettings(opts); err != nil {
		return err
	}
	opts.initBootstrapPath = opts.bootstrapPath
	opts.initConfigManager = opts.configManager
	registerConfigSourceCleanup(opts)
	return nil
}

func loadBootstrapConfigChain(opts *options) error {
	if opts.bootstrapConfigLoaded {
		return nil
	}
	path, explicit := resolveBootstrapConfigPath(opts.bootstrapPath)
	loaded, err := loadBootstrapConfigFile(opts, path, explicit)
	if err != nil {
		return err
	}
	if loaded {
		opts.bootstrapConfigLoaded = true
	}
	return nil
}

func resolveBootstrapConfigPath(configuredPath string) (string, bool) {
	args := os.Args[1:]
	if path, ok := parseNamedFlagArg(args, bootstrapConfigFlagName); ok {
		return path, true
	}
	if configuredPath = strings.TrimSpace(configuredPath); configuredPath != "" {
		return configuredPath, true
	}
	f := flag.CommandLine.Lookup(bootstrapConfigFlagName)
	if f != nil {
		if path := strings.TrimSpace(f.Value.String()); path != "" {
			return path, false
		}
	}
	return defaultBootstrapConfigPath, false
}

func parseNamedFlagArg(args []string, name string) (string, bool) {
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

func loadBootstrapConfigFile(opts *options, path string, explicit bool) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		if explicit {
			return false, fmt.Errorf(
				"bootstrap config path is empty; use --%s=/path/to/config.yaml",
				bootstrapConfigFlagName,
			)
		}
		path = defaultBootstrapConfigPath
	}

	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if explicit {
				return false, fmt.Errorf(
					"bootstrap config file %q not found; use --%s=/path/to/config.yaml",
					path,
					bootstrapConfigFlagName,
				)
			}
			slog.Warn(
				"bootstrap config file not found, fallback to default startup; use --yggdrasil-config to set config path",
				slog.String("path", path),
			)
			return false, nil
		}
		return false, fmt.Errorf("stat bootstrap config file %q: %w", path, err)
	}

	loader := configbootstrap.NewLoader(nil)
	sources, loaded, err := loader.LoadFile(opts.configManager, path, explicit)
	if err != nil {
		return false, err
	}
	for _, item := range sources {
		addManagedConfigSource(opts, item)
	}
	return loaded, nil
}

func loadProgrammaticConfigSources(opts *options) error {
	return loadLayersAndTrack(opts, opts.bootstrapSources, "option")
}

func loadSourcesAndTrack(opts *options, sources []source.Source, priority config.Priority, scope string) error {
	for i, item := range sources {
		if item == nil {
			continue
		}
		if err := opts.configManager.LoadLayer(fmt.Sprintf("%s:%d", scope, i), priority, item); err != nil {
			return fmt.Errorf("%s config source[%d]: %w", scope, i, err)
		}
		addManagedConfigSource(opts, item)
	}
	return nil
}

func loadLayersAndTrack(opts *options, layers []configLayerSource, scope string) error {
	for i, item := range layers {
		if item.Source == nil {
			continue
		}
		name := item.Name
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("%s:%d", scope, i)
		}
		if err := opts.configManager.LoadLayer(name, item.Priority, item.Source); err != nil {
			return fmt.Errorf("%s config source[%d]: %w", scope, i, err)
		}
		addManagedConfigSource(opts, item.Source)
	}
	return nil
}

func addManagedConfigSource(opts *options, item source.Source) {
	if item == nil || hasSource(opts.managedConfigSources, item) {
		return
	}
	opts.managedConfigSources = append(opts.managedConfigSources, item)
}

func registerConfigSourceCleanup(opts *options) {
	if opts.configSourceCleanupRegistered {
		return
	}
	opts.appOpts = append(
		opts.appOpts,
		application.WithCleanup("config_sources", func(context.Context) error {
			return closeManagedConfigSources(opts)
		}),
	)
	opts.configSourceCleanupRegistered = true
}

func closeManagedConfigSources(opts *options) error {
	if opts.managedConfigSourcesClosed {
		return nil
	}
	if err := closeConfigSourcesReverse(opts.managedConfigSources); err != nil {
		return err
	}
	opts.managedConfigSourcesClosed = true
	return nil
}

func closeConfigSourcesReverse(sources []source.Source) error {
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

func validateServeStageConfigOptions(opts *options) error {
	if opts.initConfigManager != nil && opts.configManager != opts.initConfigManager {
		return errors.New("WithConfigManager cannot be changed in Serve after Init")
	}
	if opts.initBootstrapPath != opts.bootstrapPath {
		return errors.New("WithBootstrapPath cannot be changed in Serve after Init")
	}
	return nil
}
