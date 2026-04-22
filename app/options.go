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

// Package app implements the internal runtime composition root used by the
// public yggdrasil package.
package app

import (
	"context"
	"time"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/governor"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/registry"
	"github.com/codesjoy/yggdrasil/v3/server"
)

type restServiceRegistration struct {
	impl     interface{}
	prefixes []string
}

type configLayerSource struct {
	Name     string
	Priority config.Priority
	Source   source.Source
}

type options struct {
	rpcServices       map[*server.ServiceDesc]interface{}
	restServices      map[*server.RestServiceDesc]restServiceRegistration
	restHandlers      []*server.RestRawHandlerDesc
	server            server.Server
	governor          *governor.Server
	internalServers   []InternalServer
	registry          registry.Registry
	shutdownTimeout   time.Duration
	beforeStartHooks  []func(context.Context) error
	beforeStopHooks   []func(context.Context) error
	afterStopHooks    []func(context.Context) error
	lifecycleOptions  []lifecycleOption
	configManager     *config.Manager
	bootstrapPath     string
	bootstrapSources  []configLayerSource
	initBootstrapPath string
	initConfigManager *config.Manager

	managedConfigSources          []source.Source
	configSourceCleanupRegistered bool
	bootstrapConfigLoaded         bool
	managedConfigSourcesClosed    bool
	resolvedSettings              settings.Resolved
	modules                       []module.Module
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

// WithRPCServices registers a batch of RPC services.
func WithRPCServices(desc map[*server.ServiceDesc]interface{}) Option {
	return func(opts *options) error {
		for k, v := range desc {
			opts.rpcServices[k] = v
		}
		return nil
	}
}

// WithRPCService registers one RPC service.
func WithRPCService(desc *server.ServiceDesc, impl interface{}) Option {
	return func(opts *options) error {
		opts.rpcServices[desc] = impl
		return nil
	}
}

// WithRESTService registers one REST service.
func WithRESTService(desc *server.RestServiceDesc, impl interface{}, prefix ...string) Option {
	return func(opts *options) error {
		opts.restServices[desc] = restServiceRegistration{
			impl:     impl,
			prefixes: prefix,
		}
		return nil
	}
}

// WithRESTHandlers registers raw REST handlers.
func WithRESTHandlers(desc ...*server.RestRawHandlerDesc) Option {
	return func(opts *options) error {
		opts.restHandlers = append(opts.restHandlers, desc...)
		return nil
	}
}

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

// WithBootstrapPath overrides the bootstrap config file path.
func WithBootstrapPath(path string) Option {
	return func(opts *options) error {
		opts.bootstrapPath = path
		return nil
	}
}

// WithBootstrapSource registers an explicit configuration source loaded after the bootstrap file.
func WithBootstrapSource(name string, priority config.Priority, src source.Source) Option {
	return func(opts *options) error {
		if src == nil {
			return nil
		}
		for _, item := range opts.bootstrapSources {
			if item.Source == src {
				return nil
			}
		}
		opts.bootstrapSources = append(opts.bootstrapSources, configLayerSource{
			Name:     name,
			Priority: priority,
			Source:   src,
		})
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

func hasSource(sources []source.Source, target source.Source) bool {
	for _, item := range sources {
		if item == target {
			return true
		}
	}
	return false
}
