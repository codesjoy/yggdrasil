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
	"fmt"
	"log/slog"
	"os"
	"strings"

	internalbootstrap "github.com/codesjoy/yggdrasil/v3/app/internal/bootstrap"
	"github.com/codesjoy/yggdrasil/v3/config"
	configchain "github.com/codesjoy/yggdrasil/v3/config/chain"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	envsource "github.com/codesjoy/yggdrasil/v3/config/source/env"
	flagsource "github.com/codesjoy/yggdrasil/v3/config/source/flag"
	internalidentity "github.com/codesjoy/yggdrasil/v3/internal/identity"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
)

const (
	configFlagName        = internalbootstrap.ConfigFlagName
	configSourcesEnvName  = internalbootstrap.ConfigSourcesEnvName
	configSourcesFlagName = internalbootstrap.ConfigSourcesFlagName
	defaultConfigPath     = internalbootstrap.DefaultConfigPath
)

func initConfigChain(opts *options) error {
	if opts.configManager == nil {
		if opts.processDefaults {
			opts.configManager = config.Default()
		} else {
			opts.configManager = config.NewManager()
		}
	}
	if err := loadBootstrapConfigSources(opts); err != nil {
		return err
	}
	if err := loadConfigFileChain(opts); err != nil {
		return err
	}
	if err := loadConfigSources(opts); err != nil {
		return err
	}
	if !opts.configFileLoaded && len(opts.managedConfigSources) == 0 {
		if err := loadDefaultConfigSources(opts); err != nil {
			return err
		}
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
	path, explicit := internalbootstrap.ResolveConfigPath(opts.configPath)
	loaded, err := loadConfigFile(opts, path, explicit)
	if err != nil {
		return err
	}
	if loaded {
		opts.configFileLoaded = true
	}
	return nil
}

func loadConfigFile(opts *options, path string, explicit bool) (bool, error) {
	registry := configSourceRegistry(opts)
	sources, loaded, err := internalbootstrap.LoadConfigFile(
		opts.configManager,
		path,
		explicit,
		registry,
	)
	if err != nil {
		return false, err
	}
	for _, item := range sources {
		addManagedConfigSource(opts, item)
	}
	return loaded, nil
}

func loadBootstrapConfigSources(opts *options) error {
	if opts == nil {
		return errors.New("options is nil")
	}
	if value := strings.TrimSpace(os.Getenv(configSourcesEnvName)); value != "" {
		if err := loadConfigSourcesFromSpec(opts, value, "bootstrap-env"); err != nil {
			return fmt.Errorf("%s: %w", configSourcesEnvName, err)
		}
	}
	if value, ok := internalbootstrap.ParseNamedFlagArg(
		os.Args[1:],
		configSourcesFlagName,
	); ok && strings.TrimSpace(value) != "" {
		if err := loadConfigSourcesFromSpec(opts, value, "bootstrap-flag"); err != nil {
			return fmt.Errorf("--%s: %w", configSourcesFlagName, err)
		}
	}
	return nil
}

func loadConfigSourcesFromSpec(opts *options, value string, scope string) error {
	specs, err := configchain.ParseSourceSpecs(value)
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		return nil
	}
	registry := configSourceRegistry(opts)
	for i, spec := range specs {
		if spec.Enabled != nil && !*spec.Enabled {
			continue
		}
		ctx := configchain.BuildContext{Snapshot: opts.configManager.Snapshot()}
		src, priority, err := registry.BuildWithContext(ctx, spec)
		if err != nil {
			return fmt.Errorf("config source[%d]: %w", i, err)
		}
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			name = fmt.Sprintf(
				"config:%s:%s:%d",
				scope,
				strings.ToLower(strings.TrimSpace(spec.Kind)),
				i,
			)
		}
		if err := loadConfigLayer(opts, name, priority, src, scope, i); err != nil {
			return err
		}
	}
	return nil
}

func configSourceRegistry(opts *options) *configchain.Registry {
	registry := configchain.NewRegistry()
	if opts == nil {
		return registry
	}
	for _, mod := range opts.modules {
		provider, ok := mod.(module.ConfigSourceProvider)
		if !ok {
			continue
		}
		for kind, builder := range provider.ConfigSourceBuilders() {
			registry.RegisterContext(kind, builder)
		}
	}
	for kind, builder := range opts.configBuilders {
		registry.RegisterContext(kind, builder)
	}
	return registry
}

func loadConfigSources(opts *options) error {
	return loadLayersAndTrack(opts, opts.configSources, "option")
}

func loadDefaultConfigSources(opts *options) error {
	slog.Info(
		"loading default config sources",
		slog.String("env_prefix", "YGGDRASIL"),
	)
	envSrc := envsource.NewSource(
		[]string{"YGGDRASIL"},
		nil,
		envsource.WithName("default_env"),
		envsource.WithParseArray(","),
	)
	if err := loadConfigLayer(
		opts,
		"config:env:default",
		config.PriorityEnv,
		envSrc,
		"default",
		0,
	); err != nil {
		return err
	}
	flagSrc := flagsource.NewSourceWithOptions(
		nil,
		flagsource.WithIgnoredNames(configFlagName, configSourcesFlagName),
	)
	return loadConfigLayer(
		opts,
		"config:flag:default",
		config.PriorityFlag,
		flagSrc,
		"default",
		1,
	)
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
	if err := internalbootstrap.LoadConfigLayer(
		opts.configManager,
		name,
		priority,
		item,
		scope,
		index,
	); err != nil {
		return err
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
	if err := internalbootstrap.CloseConfigSourcesReverse(opts.managedConfigSources); err != nil {
		return err
	}
	opts.managedConfigSourcesClosed = true
	return nil
}

func refreshResolvedSettings(opts *options) error {
	if opts == nil {
		return errors.New("options is nil")
	}
	resolved, err := internalbootstrap.RefreshResolvedSettings(opts.configManager)
	if err != nil {
		return err
	}
	opts.resolvedSettings = resolved
	return nil
}

func validateStartup(opts *options) error {
	resolved := settings.Resolved{}
	var mgr *config.Manager
	if opts != nil {
		resolved = opts.resolvedSettings
		mgr = opts.configManager
	}
	resolved, err := internalbootstrap.ResolveStartupSettings(resolved, mgr)
	if err != nil {
		return err
	}
	if opts != nil {
		opts.resolvedSettings = resolved
	}
	return internalbootstrap.ValidateStartupResolved(resolved)
}

func (a *App) resolveIdentityLocked() error {
	if a == nil || a.opts == nil {
		return errors.New("app options are not initialized")
	}
	if name := a.opts.appName; name != "" {
		a.name = name
	} else if resolvedName := strings.TrimSpace(a.opts.resolvedSettings.App.Name); resolvedName != "" {
		a.name = resolvedName
	} else if strings.TrimSpace(a.name) == "" {
		return errors.New("app name is required; use WithAppName or yggdrasil.app.name")
	}
	a.identity = internalidentity.FromInstanceConfig(
		a.name,
		a.opts.resolvedSettings.Admin.Application,
	)
	a.identityResolved = true
	return nil
}
