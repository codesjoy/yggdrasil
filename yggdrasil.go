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
	"os"
	"os/signal"
	"strings"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/config"
	configchain "github.com/codesjoy/yggdrasil/v3/config/chain"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/module"
)

// ComposeFunc builds one business bundle from the prepared runtime.
type ComposeFunc = yapp.ComposeFunc

// Runtime is the business-safe runtime surface exposed during composition.
type Runtime = yapp.Runtime

// Identity contains the resolved runtime identity for one App.
type Identity = yapp.Identity

// RPCBinding declares one RPC service binding.
type RPCBinding = yapp.RPCBinding

// RESTBinding declares one REST service binding.
type RESTBinding = yapp.RESTBinding

// RawHTTPBinding declares one raw HTTP binding.
type RawHTTPBinding = yapp.RawHTTPBinding

// GovernorHTTPBinding declares one handler bound to the governor admin server.
type GovernorHTTPBinding = yapp.GovernorHTTPBinding

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

// BusinessInstallRollback optionally compensates an extension after a later
// installation step fails.
type BusinessInstallRollback = yapp.BusinessInstallRollback

// InstallContext is passed to BusinessInstallable implementations.
type InstallContext = yapp.InstallContext

// BusinessBundle is one prepared business installation bundle.
type BusinessBundle = yapp.BusinessBundle

// CapabilityRegistration declares one provider-only capability extension.
type CapabilityRegistration = yapp.CapabilityRegistration

// PlanOverride customizes declarative module and binding selection.
type PlanOverride = yassembly.Override

type configLayerSource struct {
	name     string
	priority config.Priority
	source   source.Source
}

type options struct {
	configPath              string
	mode                    string
	processDefaults         *bool
	signalHandling          *bool
	shutdownSignals         []os.Signal
	configSources           []configLayerSource
	configBuilders          map[string]configchain.ContextBuilder
	modules                 []module.Module
	planOverrides           []yassembly.Override
	capabilityRegistrations []yapp.CapabilityRegistration
}

// Option configures one root bootstrap app instance.
type Option func(*options) error

// ErrProcessDefaultsAlreadyInstalled is returned when another active App owns
// process-global compatibility defaults.
var ErrProcessDefaultsAlreadyInstalled = yapp.ErrProcessDefaultsAlreadyInstalled

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

// WithConfigSourceBuilder registers one declarative config source builder.
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

// WithMode overrides the mode resolved by New.
func WithMode(mode string) Option {
	return func(opts *options) error {
		opts.mode = mode
		return nil
	}
}

// WithSignalHandling controls whether Run installs default OS signal handling.
func WithSignalHandling(enabled bool) Option {
	return func(opts *options) error {
		opts.signalHandling = &enabled
		return nil
	}
}

// WithShutdownSignals customizes the OS signals handled by Run.
func WithShutdownSignals(signals ...os.Signal) Option {
	return func(opts *options) error {
		opts.shutdownSignals = append([]os.Signal(nil), signals...)
		return nil
	}
}

// WithProcessDefaults controls whether this App installs process-global
// compatibility defaults such as slog.Default, OTel globals, and legacy
// instance facade state.
func WithProcessDefaults(enabled bool) Option {
	return func(opts *options) error {
		opts.processDefaults = &enabled
		return nil
	}
}

// WithModules registers additional full lifecycle module candidates. Explicit
// configuration may select a candidate provider and pull the owning module into
// the plan. A module can also be enabled with WithPlanOverrides and
// assembly.EnableModule.
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

// WithPlanOverrides registers declarative planning overrides.
func WithPlanOverrides(overrides ...yassembly.Override) Option {
	return func(opts *options) error {
		for _, override := range overrides {
			if override != nil {
				opts.planOverrides = append(opts.planOverrides, override)
			}
		}
		return nil
	}
}

// WithCapabilityRegistrations registers provider-only capability extensions.
func WithCapabilityRegistrations(regs ...CapabilityRegistration) Option {
	return func(opts *options) error {
		opts.capabilityRegistrations = append(opts.capabilityRegistrations, regs...)
		return nil
	}
}

// App is the thin root bootstrap facade over app.App.
type App struct {
	inner *yapp.App
}

// New creates one App ready for the bootstrap flow.
func New(appName string, opts ...Option) (*App, error) {
	appName = strings.TrimSpace(appName)
	if appName == "" {
		return nil, errors.New("app name is required")
	}
	appOpts, err := convertOptions(opts...)
	if err != nil {
		return nil, err
	}
	inner, err := yapp.New(appName, appOpts...)
	if err != nil {
		return nil, err
	}
	return &App{inner: inner}, nil
}

// Run executes the default business bootstrap flow.
func Run(ctx context.Context, appName string, fn ComposeFunc, opts ...Option) error {
	if ctx == nil {
		return errors.New("run context is nil")
	}
	appName = strings.TrimSpace(appName)
	if appName == "" {
		return errors.New("app name is required")
	}
	rootOpts, err := collectOptions(opts...)
	if err != nil {
		return err
	}
	if rootOpts.processDefaults == nil {
		enabled := true
		rootOpts.processDefaults = &enabled
	}
	runCtx, stopSignals := rootOpts.runContext(ctx)
	defer stopSignals()

	app, err := newFromOptions(appName, rootOpts)
	if err != nil {
		return err
	}
	if err := app.ComposeAndInstall(runCtx, fn); err != nil {
		stopErr := app.Stop(context.Background())
		return errors.Join(err, stopErr)
	}
	if err := app.Start(runCtx); err != nil {
		stopErr := app.Stop(context.Background())
		return errors.Join(err, stopErr)
	}
	done := make(chan struct{})
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		select {
		case <-runCtx.Done():
			_ = app.Stop(context.WithoutCancel(runCtx))
		case <-done:
		}
	}()
	waitErr := app.Wait()
	close(done)
	<-watchDone
	stopErr := app.Stop(context.Background())
	return errors.Join(waitErr, stopErr)
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

// Identity returns the resolved App identity.
func (a *App) Identity() (Identity, bool) {
	if a == nil || a.inner == nil {
		return Identity{}, false
	}
	return a.inner.Identity()
}

func convertOptions(opts ...Option) ([]yapp.Option, error) {
	rootOpts, err := collectOptions(opts...)
	if err != nil {
		return nil, err
	}
	return rootOpts.appOptions(), nil
}

func collectOptions(opts ...Option) (options, error) {
	rootOpts := options{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&rootOpts); err != nil {
			return options{}, err
		}
	}
	return rootOpts, nil
}

func newFromOptions(appName string, rootOpts options) (*App, error) {
	appOpts := rootOpts.appOptions()
	inner, err := yapp.New(appName, appOpts...)
	if err != nil {
		return nil, err
	}
	return &App{inner: inner}, nil
}

func (rootOpts options) appOptions() []yapp.Option {
	appOpts := make([]yapp.Option, 0, 6+len(rootOpts.configSources))
	if rootOpts.configPath != "" {
		appOpts = append(appOpts, yapp.WithConfigPath(rootOpts.configPath))
	}
	if rootOpts.mode != "" {
		appOpts = append(appOpts, yapp.WithMode(rootOpts.mode))
	}
	if rootOpts.processDefaults != nil {
		appOpts = append(appOpts, yapp.WithProcessDefaults(*rootOpts.processDefaults))
	}
	for _, item := range rootOpts.configSources {
		appOpts = append(appOpts, yapp.WithConfigSource(item.name, item.priority, item.source))
	}
	for kind, builder := range rootOpts.configBuilders {
		appOpts = append(appOpts, yapp.WithConfigSourceBuilder(kind, builder))
	}
	if len(rootOpts.modules) > 0 {
		appOpts = append(appOpts, yapp.WithModules(rootOpts.modules...))
	}
	if len(rootOpts.planOverrides) > 0 {
		appOpts = append(appOpts, yapp.WithPlanOverrides(rootOpts.planOverrides...))
	}
	if len(rootOpts.capabilityRegistrations) > 0 {
		appOpts = append(
			appOpts,
			yapp.WithCapabilityRegistrations(rootOpts.capabilityRegistrations...),
		)
	}
	return appOpts
}

func (rootOpts options) runContext(parent context.Context) (context.Context, context.CancelFunc) {
	enabled := true
	if rootOpts.signalHandling != nil {
		enabled = *rootOpts.signalHandling
	}
	if !enabled {
		return parent, func() {}
	}
	signals := rootOpts.shutdownSignals
	if len(signals) == 0 {
		signals = defaultShutdownSignals
	}
	return signal.NotifyContext(parent, signals...)
}
