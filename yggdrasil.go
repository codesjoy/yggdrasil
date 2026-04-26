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

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/module"
)

// ComposeFunc builds one business bundle from the prepared runtime.
type ComposeFunc = yapp.ComposeFunc

// Runtime is the business-safe runtime surface exposed during composition.
type Runtime = yapp.Runtime

// RPCBinding declares one RPC service binding.
type RPCBinding = yapp.RPCBinding

// RESTBinding declares one REST service binding.
type RESTBinding = yapp.RESTBinding

// RawHTTPBinding declares one raw HTTP binding.
type RawHTTPBinding = yapp.RawHTTPBinding

// BackgroundTask is one managed background task.
type BackgroundTask = yapp.BackgroundTask

// BusinessHookStage identifies one business hook stage.
type BusinessHookStage = yapp.BusinessHookStage

// Business lifecycle hook stages, re-exported from app.
const (
	BusinessHookBeforeStart = yapp.BusinessHookBeforeStart
	BusinessHookBeforeStop  = yapp.BusinessHookBeforeStop
	BusinessHookAfterStop   = yapp.BusinessHookAfterStop
)

// BusinessHook is one managed business hook.
type BusinessHook = yapp.BusinessHook

// BundleDiag is one bundle diagnostic item.
type BundleDiag = yapp.BundleDiag

// BusinessInstallable is one extension install item.
type BusinessInstallable = yapp.BusinessInstallable

// InstallContext is passed to BusinessInstallable implementations.
type InstallContext = yapp.InstallContext

// BusinessBundle is one prepared business installation bundle.
type BusinessBundle = yapp.BusinessBundle

// CapabilityRegistration declares one provider-only capability extension.
type CapabilityRegistration = yapp.CapabilityRegistration

type configLayerSource struct {
	name     string
	priority config.Priority
	source   source.Source
}

type options struct {
	appName                 string
	configPath              string
	mode                    string
	configSources           []configLayerSource
	modules                 []module.Module
	capabilityRegistrations []yapp.CapabilityRegistration
}

// Option configures one root bootstrap app instance.
type Option func(*options) error

// WithAppName overrides the app name resolved by New.
func WithAppName(name string) Option {
	return func(opts *options) error {
		opts.appName = name
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

// WithConfigSource registers one config source layer.
func WithConfigSource(name string, priority config.Priority, src source.Source) Option {
	return func(opts *options) error {
		if src == nil {
			return nil
		}
		opts.configSources = append(opts.configSources, configLayerSource{
			name:     name,
			priority: priority,
			source:   src,
		})
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

// WithModules registers additional full lifecycle modules.
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

// WithCapabilityRegistrations registers provider-only capability extensions.
func WithCapabilityRegistrations(regs ...CapabilityRegistration) Option {
	return func(opts *options) error {
		for _, reg := range regs {
			opts.capabilityRegistrations = append(opts.capabilityRegistrations, reg)
		}
		return nil
	}
}

// App is the thin root bootstrap facade over app.App.
type App struct {
	inner *yapp.App
}

// New creates one App ready for the bootstrap flow.
func New(opts ...Option) (*App, error) {
	appOpts, err := convertOptions(opts...)
	if err != nil {
		return nil, err
	}
	inner, err := yapp.New("", appOpts...)
	if err != nil {
		return nil, err
	}
	return &App{inner: inner}, nil
}

// Run executes the default business bootstrap flow.
func Run(ctx context.Context, fn ComposeFunc, opts ...Option) error {
	if ctx == nil {
		return errors.New("run context is nil")
	}
	app, err := New(opts...)
	if err != nil {
		return err
	}
	if err := app.ComposeAndInstall(ctx, fn); err != nil {
		_ = app.Stop(ctx)
		return err
	}
	if err := app.Start(ctx); err != nil {
		_ = app.Stop(ctx)
		return err
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = app.Stop(context.Background())
		case <-done:
		}
	}()
	return app.Wait()
}

// ComposeAndInstall composes a business bundle and installs it in one step.
func (a *App) ComposeAndInstall(ctx context.Context, fn ComposeFunc) error {
	if a == nil || a.inner == nil {
		return errors.New("app is not initialized")
	}
	return a.inner.ComposeAndInstall(ctx, fn)
}

// Start starts the underlying app lifecycle.
func (a *App) Start(ctx context.Context) error {
	if a == nil || a.inner == nil {
		return errors.New("app is not initialized")
	}
	return a.inner.Start(ctx)
}

// Wait blocks until the app exits.
func (a *App) Wait() error {
	if a == nil || a.inner == nil {
		return errors.New("app is not initialized")
	}
	return a.inner.Wait()
}

// Stop stops the app lifecycle.
func (a *App) Stop(ctx context.Context) error {
	if a == nil || a.inner == nil {
		return errors.New("app is not initialized")
	}
	return a.inner.Stop(ctx)
}

func convertOptions(opts ...Option) ([]yapp.Option, error) {
	rootOpts := options{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&rootOpts); err != nil {
			return nil, err
		}
	}
	appOpts := make([]yapp.Option, 0, 6+len(rootOpts.configSources))
	if rootOpts.appName != "" {
		appOpts = append(appOpts, yapp.WithAppName(rootOpts.appName))
	}
	if rootOpts.configPath != "" {
		appOpts = append(appOpts, yapp.WithConfigPath(rootOpts.configPath))
	}
	if rootOpts.mode != "" {
		appOpts = append(appOpts, yapp.WithMode(rootOpts.mode))
	}
	for _, item := range rootOpts.configSources {
		appOpts = append(appOpts, yapp.WithConfigSource(item.name, item.priority, item.source))
	}
	if len(rootOpts.modules) > 0 {
		appOpts = append(appOpts, yapp.WithModules(rootOpts.modules...))
	}
	if len(rootOpts.capabilityRegistrations) > 0 {
		appOpts = append(
			appOpts,
			yapp.WithCapabilityRegistrations(rootOpts.capabilityRegistrations...),
		)
	}
	return appOpts, nil
}
