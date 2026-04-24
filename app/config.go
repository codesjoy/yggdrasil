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

package app

import (
	"context"
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

const (
	configFlagName    = "yggdrasil-config"
	defaultConfigPath = "./config.yaml"
)

func initConfigChain(opts *options) error {
	if opts.configManager == nil {
		opts.configManager = config.Default()
	} else {
		config.SetDefault(opts.configManager)
	}
	if err := loadConfigFileChain(opts); err != nil {
		return err
	}
	if err := loadConfigSources(opts); err != nil {
		return err
	}
	if err := refreshResolvedSettings(opts); err != nil {
		return err
	}
	registerConfigSourceCleanup(opts)
	return nil
}

func loadConfigFileChain(opts *options) error {
	if opts.configFileLoaded {
		return nil
	}
	path, explicit := resolveConfigPath(opts.configPath)
	loaded, err := loadConfigFile(opts, path, explicit)
	if err != nil {
		return err
	}
	if loaded {
		opts.configFileLoaded = true
	}
	return nil
}

func resolveConfigPath(configuredPath string) (string, bool) {
	if path, ok := parseNamedFlagArg(os.Args[1:], configFlagName); ok {
		return path, true
	}
	if path := strings.TrimSpace(configuredPath); path != "" {
		return path, true
	}
	if path, ok := lookupRegisteredFlagValue(configFlagName); ok {
		return path, false
	}
	return defaultConfigPath, false
}

func lookupRegisteredFlagValue(name string) (string, bool) {
	f := flag.CommandLine.Lookup(name)
	if f == nil {
		return "", false
	}
	if path := strings.TrimSpace(f.Value.String()); path != "" {
		return path, true
	}
	return "", false
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

func loadConfigFile(opts *options, path string, explicit bool) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		if explicit {
			return false, fmt.Errorf(
				"config path is empty; use --%s=/path/to/config.yaml",
				configFlagName,
			)
		}
		path = defaultConfigPath
	}

	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if explicit {
				return false, fmt.Errorf(
					"config file %q not found; use --%s=/path/to/config.yaml",
					path,
					configFlagName,
				)
			}
			slog.Warn(
				"config file not found, fallback to default startup; use --yggdrasil-config to set config path",
				slog.String("path", path),
			)
			return false, nil
		}
		return false, fmt.Errorf("stat config file %q: %w", path, err)
	}

	loader := configchain.NewLoader(nil)
	sources, loaded, err := loader.LoadFile(opts.configManager, path, explicit)
	if err != nil {
		return false, err
	}
	for _, item := range sources {
		addManagedConfigSource(opts, item)
	}
	return loaded, nil
}

func loadConfigSources(opts *options) error {
	return loadLayersAndTrack(opts, opts.configSources, "option")
}

func loadSourcesAndTrack(opts *options, sources []source.Source, priority config.Priority, scope string) error {
	for i, item := range sources {
		if err := loadConfigLayer(opts, fmt.Sprintf("%s:%d", scope, i), priority, item, scope, i); err != nil {
			return err
		}
	}
	return nil
}

func loadLayersAndTrack(opts *options, layers []configLayerSource, scope string) error {
	for i, item := range layers {
		if err := loadConfigLayer(opts, item.Name, item.Priority, item.Source, scope, i); err != nil {
			return err
		}
	}
	return nil
}

func loadConfigLayer(
	opts *options,
	name string,
	priority config.Priority,
	item source.Source,
	scope string,
	index int,
) error {
	if item == nil {
		return nil
	}
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("%s:%d", scope, index)
	}
	if err := opts.configManager.LoadLayer(name, priority, item); err != nil {
		return fmt.Errorf("%s config source[%d]: %w", scope, index, err)
	}
	addManagedConfigSource(opts, item)
	return nil
}

func addManagedConfigSource(opts *options, item source.Source) {
	if item == nil {
		return
	}
	for _, existing := range opts.managedConfigSources {
		if existing == item {
			return
		}
	}
	opts.managedConfigSources = append(opts.managedConfigSources, item)
}

func registerConfigSourceCleanup(opts *options) {
	if opts.configSourceCleanupRegistered {
		return
	}
	opts.lifecycleOptions = append(
		opts.lifecycleOptions,
		withLifecycleCleanup("config_sources", func(context.Context) error {
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

func refreshResolvedSettings(opts *options) error {
	if opts == nil {
		return errors.New("options is nil")
	}
	if opts.configManager == nil {
		opts.configManager = config.Default()
	}
	if err := settings.ValidateV3RootShape(opts.configManager.Snapshot().Map()); err != nil {
		return err
	}
	catalog := settings.NewCatalog(opts.configManager)
	root, err := catalog.Root().Current()
	if err != nil {
		return err
	}
	resolved, err := settings.Compile(root)
	if err != nil {
		return err
	}
	opts.resolvedSettings = resolved
	return nil
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

func validateStartup(opts *options) error {
	resolved, err := resolveStartupSettings(opts)
	if err != nil {
		return err
	}
	strict := resolved.Admin.Validation.Strict
	enable := strict || resolved.Admin.Validation.Enable
	if err := settings.Validate(resolved); err != nil {
		return err
	}
	if !enable || opts == nil {
		return nil
	}

	validator := startupValidator{strict: strict}

	return validator.err
}

func resolveStartupSettings(opts *options) (settings.Resolved, error) {
	resolved := settings.Resolved{}
	if opts != nil {
		resolved = opts.resolvedSettings
	}
	if opts == nil || needsDefaultStartupSettings(resolved) {
		root, err := settings.NewCatalog(config.Default()).Root().Current()
		if err != nil {
			return settings.Resolved{}, err
		}
		resolved, err = settings.Compile(root)
		if err != nil {
			return settings.Resolved{}, err
		}
	}
	if opts != nil {
		opts.resolvedSettings = resolved
	}
	return resolved, nil
}

func needsDefaultStartupSettings(resolved settings.Resolved) bool {
	return resolved.Logging.Handlers == nil &&
		resolved.Discovery.Registry.Type == "" &&
		len(resolved.Server.Transports) == 0 &&
		resolved.Transports.Rest == nil
}

func (a *App) resolveIdentityLocked() error {
	if a == nil || a.opts == nil {
		return errors.New("app options are not initialized")
	}
	if name := a.opts.appName; name != "" {
		a.name = name
		return nil
	}
	if resolvedName := strings.TrimSpace(a.opts.resolvedSettings.App.Name); resolvedName != "" {
		a.name = resolvedName
		return nil
	}
	if strings.TrimSpace(a.name) != "" {
		return nil
	}
	return errors.New("app name is required; use WithAppName or yggdrasil.app.name")
}
