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
	"github.com/codesjoy/yggdrasil/v2/config/source"
	filesource "github.com/codesjoy/yggdrasil/v2/config/source/file"
)

const (
	bootstrapConfigFlagName    = "yggdrasil-config"
	defaultBootstrapConfigPath = "./config.yaml"
	bootstrapSourceKey         = "yggdrasil.config.sources"
)

type bootstrapSourceSpec struct {
	Type    string         `mapstructure:"type"`
	Enabled *bool          `mapstructure:"enabled"`
	Config  map[string]any `mapstructure:"config"`
}

func initConfigChain(opts *options) error {
	if err := loadBootstrapConfigChain(opts); err != nil {
		return err
	}
	opts.initConfigSourceCount = len(opts.configSources)
	if err := loadProgrammaticConfigSources(opts); err != nil {
		return err
	}
	registerConfigSourceCleanup(opts)
	return nil
}

func loadBootstrapConfigChain(opts *options) error {
	if opts.bootstrapConfigLoaded {
		return nil
	}
	path, explicit := resolveBootstrapConfigPath()
	loaded, err := loadBootstrapConfigFile(opts, path, explicit)
	if err != nil {
		return err
	}
	if loaded {
		opts.bootstrapConfigLoaded = true
	}
	return nil
}

func resolveBootstrapConfigPath() (string, bool) {
	args := os.Args[1:]
	if path, ok := parseNamedFlagArg(args, bootstrapConfigFlagName); ok {
		return path, true
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

	bootstrapSource := filesource.NewSource(path, false)
	if err := loadSourcesAndTrack(opts, []source.Source{bootstrapSource}, "bootstrap file"); err != nil {
		return false, err
	}

	sources, err := buildBootstrapDeclaredSources()
	if err != nil {
		return false, err
	}
	if err := loadSourcesAndTrack(opts, sources, "bootstrap declared"); err != nil {
		return false, err
	}
	return true, nil
}

func buildBootstrapDeclaredSources() ([]source.Source, error) {
	specs := make([]bootstrapSourceSpec, 0)
	if err := config.Scan(bootstrapSourceKey, &specs); err != nil {
		return nil, err
	}
	sources := make([]source.Source, 0, len(specs))
	for i, item := range specs {
		if strings.TrimSpace(item.Type) == "" {
			return nil, fmt.Errorf("bootstrap source[%d] type is required", i)
		}
		if item.Enabled != nil && !*item.Enabled {
			continue
		}
		cfg := item.Config
		if cfg == nil {
			cfg = map[string]any{}
		}
		ss, err := source.New(item.Type, cfg)
		if err != nil {
			return nil, fmt.Errorf("bootstrap source[%d] type=%q: %w", i, item.Type, err)
		}
		sources = append(sources, ss)
	}
	return sources, nil
}

func loadProgrammaticConfigSources(opts *options) error {
	if opts.loadedConfigSourceCount >= opts.initConfigSourceCount {
		return nil
	}
	pending := opts.configSources[opts.loadedConfigSourceCount:opts.initConfigSourceCount]
	if err := loadSourcesAndTrack(opts, pending, "option"); err != nil {
		return err
	}
	opts.loadedConfigSourceCount = opts.initConfigSourceCount
	return nil
}

func loadSourcesAndTrack(opts *options, sources []source.Source, scope string) error {
	for i, item := range sources {
		if item == nil {
			continue
		}
		if err := config.LoadSource(item); err != nil {
			return fmt.Errorf("%s config source[%d]: %w", scope, i, err)
		}
		addManagedConfigSource(opts, item)
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
					"close config source type=%q name=%q: %w",
					item.Type(),
					item.Name(),
					err,
				),
			)
		}
	}
	return multiErr
}

func dropServeStageConfigSources(opts *options) {
	if len(opts.configSources) <= opts.initConfigSourceCount {
		return
	}
	ignored := len(opts.configSources) - opts.initConfigSourceCount
	slog.Warn(
		"WithConfigSource is ignored in Serve; use Init/Run instead",
		slog.Int("ignored", ignored),
	)
	opts.configSources = opts.configSources[:opts.initConfigSourceCount]
}
