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

// Package app provides the stable advanced application control API for
// Yggdrasil.
package app

import (
	"context"
	"time"

	"github.com/codesjoy/yggdrasil/v3/admin/governor"
	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/config"
	configchain "github.com/codesjoy/yggdrasil/v3/config/chain"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

type configLayerSource struct {
	Name     string
	Priority config.Priority
	Source   source.Source
}

type options struct {
	appName          string
	mode             string
	planOverrides    []yassembly.Override
	server           server.Server
	governor         *governor.Server
	internalServers  []InternalServer
	registry         registry.Registry
	shutdownTimeout  time.Duration
	beforeStartHooks []func(context.Context) error
	beforeStopHooks  []func(context.Context) error
	afterStopHooks   []func(context.Context) error
	lifecycleOptions []lifecycleOption
	configManager    *config.Manager
	configPath       string
	configSources    []configLayerSource
	configBuilders   map[string]configchain.ContextBuilder

	managedConfigSources          []source.Source
	configSourceCleanupRegistered bool
	configFileLoaded              bool
	managedConfigSourcesClosed    bool
	processDefaults               bool
	resolvedSettings              settings.Resolved
	modules                       []module.Module
	capabilityRegistrations       []CapabilityRegistration
}

func (opts *options) buildLifecycleOptions() []lifecycleOption {
	out := []lifecycleOption{
		withLifecycleServer(opts.server),
		withLifecycleGovernor(opts.governor),
		withLifecycleRegistry(opts.registry),
		withLifecycleShutdownTimeout(opts.shutdownTimeout),
		withLifecycleBeforeStartHooks(opts.beforeStartHooks...),
		withLifecycleBeforeStopHooks(opts.beforeStopHooks...),
		withLifecycleAfterStopHooks(opts.afterStopHooks...),
		withLifecycleInternalServers(opts.internalServers...),
	}
	out = append(out, opts.lifecycleOptions...)
	return out
}

// Option define the framework options
type Option func(*options) error

// WithInternalServer registers internal servers managed by the App lifecycle.
func WithInternalServer(servers ...InternalServer) Option {
	return func(opts *options) error {
		opts.internalServers = append(opts.internalServers, servers...)
		return nil
	}
}

// WithBeforeStartHook register the before start hook.
func WithBeforeStartHook(fns ...func(context.Context) error) Option {
	return func(opts *options) error {
		opts.beforeStartHooks = append(opts.beforeStartHooks, fns...)
		return nil
	}
}

// WithBeforeStopHook register the before stop hook.
func WithBeforeStopHook(fns ...func(context.Context) error) Option {
	return func(opts *options) error {
		opts.beforeStopHooks = append(opts.beforeStopHooks, fns...)
		return nil
	}
}

// WithAfterStopHook register the after stop hook.
func WithAfterStopHook(fns ...func(context.Context) error) Option {
	return func(opts *options) error {
		opts.afterStopHooks = append(opts.afterStopHooks, fns...)
		return nil
	}
}

// WithCleanup register a cleanup function.
func WithCleanup(name string, fn func(context.Context) error) Option {
	return func(opts *options) error {
		opts.lifecycleOptions = append(opts.lifecycleOptions, withLifecycleCleanup(name, fn))
		return nil
	}
}

// WithConfigManager replaces the default framework config manager.
func WithConfigManager(manager *config.Manager) Option {
	return func(opts *options) error {
		opts.configManager = manager
		return nil
	}
}

// WithProcessDefaults controls whether this App installs process-global
// compatibility defaults such as slog.Default, OTel globals, and legacy
// instance facade state.
func WithProcessDefaults(enabled bool) Option {
	return func(opts *options) error {
		opts.processDefaults = enabled
		return nil
	}
}

// WithAppName overrides the app name resolved by New.
func WithAppName(name string) Option {
	return func(opts *options) error {
		opts.appName = name
		return nil
	}
}

// WithMode overrides the mode resolved by New.
func WithMode(mode string) Option {
	return func(opts *options) error {
		opts.mode = mode
		return nil
	}
}

// WithPlanOverrides registers assembly overrides for New/Prepare.
func WithPlanOverrides(overrides ...yassembly.Override) Option {
	return func(opts *options) error {
		for _, item := range overrides {
			if item == nil {
				continue
			}
			opts.planOverrides = append(opts.planOverrides, item)
		}
		return nil
	}
}

// WithConfigPath overrides the config file path.
func WithConfigPath(path string) Option {
	return func(opts *options) error {
		opts.configPath = path
		return nil
	}
}

// WithConfigSource registers an explicit configuration source loaded after the config file.
func WithConfigSource(name string, priority config.Priority, src source.Source) Option {
	return func(opts *options) error {
		if src == nil {
			return nil
		}
		for _, item := range opts.configSources {
			if item.Source == src {
				return nil
			}
		}
		opts.configSources = append(opts.configSources, configLayerSource{
			Name:     name,
			Priority: priority,
			Source:   src,
		})
		return nil
	}
}

// WithConfigSourceBuilder registers a declarative config source builder.
func WithConfigSourceBuilder(kind string, builder configchain.ContextBuilder) Option {
	return func(opts *options) error {
		if builder == nil {
			return nil
		}
		if opts.configBuilders == nil {
			opts.configBuilders = map[string]configchain.ContextBuilder{}
		}
		opts.configBuilders[kind] = builder
		return nil
	}
}

// WithModules registers additional modules into the app module hub.
func WithModules(mods ...module.Module) Option {
	return func(opts *options) error {
		for _, mod := range mods {
			if mod == nil {
				continue
			}
			opts.modules = append(opts.modules, mod)
		}
		return nil
	}
}
