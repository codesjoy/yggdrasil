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
	"strings"

	internalbootstrap "github.com/codesjoy/yggdrasil/v3/app/internal/bootstrap"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	internalidentity "github.com/codesjoy/yggdrasil/v3/internal/identity"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
)

const (
	configFlagName    = internalbootstrap.ConfigFlagName
	defaultConfigPath = internalbootstrap.DefaultConfigPath
)

func initConfigChain(opts *options) error {
	if opts.configManager == nil {
		if opts.processDefaults {
			opts.configManager = config.Default()
		} else {
			opts.configManager = config.NewManager()
		}
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
	sources, loaded, err := internalbootstrap.LoadConfigFile(opts.configManager, path, explicit)
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
