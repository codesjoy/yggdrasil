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
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil/v3/admin/governor"
	internallifecycle "github.com/codesjoy/yggdrasil/v3/app/internal/lifecycle"
	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/internal/instance"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

type lifecycleState uint32

// InternalServer is managed by the App lifecycle alongside the main server.
type InternalServer = internallifecycle.InternalServer

type (
	lifecycleRunner = internallifecycle.Runner
	lifecycleOption = internallifecycle.Option
)

const (
	lifecycleStateNew lifecycleState = iota
	lifecycleStatePlanned
	lifecycleStateInfraInitialized
	lifecycleStateInitialized
	lifecycleStateBusinessInstalled
	lifecycleStateServing
	lifecycleStateRunning
	lifecycleStateStopped
)

func withLifecycleBeforeStartHooks(hooks ...func(context.Context) error) lifecycleOption {
	return internallifecycle.WithBeforeStartHooks(hooks...)
}

func withLifecycleBeforeStopHooks(hooks ...func(context.Context) error) lifecycleOption {
	return internallifecycle.WithBeforeStopHooks(hooks...)
}

func withLifecycleAfterStopHooks(hooks ...func(context.Context) error) lifecycleOption {
	return internallifecycle.WithAfterStopHooks(hooks...)
}

func withLifecycleCleanup(name string, fn func(context.Context) error) lifecycleOption {
	return internallifecycle.WithCleanup(name, fn)
}

func newLifecycleRunner(opts ...lifecycleOption) (*lifecycleRunner, error) {
	return internallifecycle.New(opts...)
}

func withLifecycleInternalServers(servers ...InternalServer) lifecycleOption {
	converted := make([]internallifecycle.InternalServer, 0, len(servers))
	converted = append(converted, servers...)
	return internallifecycle.WithInternalServers(converted...)
}

func withLifecycleRegistry(reg registry.Registry) lifecycleOption {
	return internallifecycle.WithRegistry(reg)
}

func withLifecycleShutdownTimeout(timeout time.Duration) lifecycleOption {
	return internallifecycle.WithShutdownTimeout(timeout)
}

func withLifecycleServer(srv server.Server) lifecycleOption {
	return internallifecycle.WithServer(srv)
}

func withLifecycleGovernor(srv *governor.Server) lifecycleOption {
	return internallifecycle.WithGovernor(srv)
}

var (
	errApplicationAlreadyRunning = errors.New("application is already running")
	errRestartUnsupported        = errors.New(
		"restarting yggdrasil in the same process is not supported",
	)
)

// App is the yggdrasil runtime composition root.
type App struct {
	name string

	mu    sync.Mutex
	state lifecycleState

	opts *options

	lifecycle *lifecycleRunner
	hub       *module.Hub

	reloadMu     sync.Mutex
	stopWatch    func()
	watchStarted bool

	waitDone chan struct{}
	waitErr  error

	runtimeMu                  sync.RWMutex
	runtimeSnapshot            *Snapshot
	foundationSnapshot         *Snapshot
	preparedFoundationSnapshot *Snapshot
	tracerShutdown             func(context.Context) error
	meterShutdown              func(context.Context) error

	runtime            Runtime
	lastPlanResult     *yassembly.Result
	lastSpecDiff       *yassembly.SpecDiff
	lastPlanHash       string
	lastStablePlanHash string
	assemblyErrors     assemblyErrorState
	assemblySpec       *yassembly.Spec
	runtimeAssembly    *preparedAssembly

	explicitBundleInstalled bool
	installedRPCServices    map[string]struct{}
	installedHTTPRoutes     map[string]struct{}
	bundleDiagnostics       []BundleDiag
}

// New creates a new App.
func New(appName string, ops ...Option) (*App, error) {
	opts := &options{}
	if err := applyOptions(opts, ops...); err != nil {
		return nil, err
	}
	lifecycle, err := newLifecycleRunner()
	if err != nil {
		return nil, err
	}
	return &App{
		name:                 appName,
		state:                lifecycleStateNew,
		opts:                 opts,
		lifecycle:            lifecycle,
		hub:                  module.NewHub(),
		installedRPCServices: map[string]struct{}{},
		installedHTTPRoutes:  map[string]struct{}{},
	}, nil
}

func (a *App) prepareStartLocked(ctx context.Context) error {
	switch a.state {
	case lifecycleStateServing, lifecycleStateRunning:
		return errApplicationAlreadyRunning
	case lifecycleStateStopped:
		return errRestartUnsupported
	}
	if err := a.initializeLocked(ctx); err != nil {
		return err
	}
	if err := a.lifecycle.Init(a.opts.buildLifecycleOptions()...); err != nil {
		return err
	}
	a.state = lifecycleStateServing
	a.installConfigWatchLocked()
	a.waitDone = make(chan struct{})
	a.waitErr = nil
	return nil
}

func (a *App) ensureClientReadyLocked(ctx context.Context) error {
	if a.state == lifecycleStateStopped {
		return errRestartUnsupported
	}
	return a.initializeLocked(ctx)
}

func (a *App) stopConfigWatchLocked() {
	if a.stopWatch == nil {
		return
	}
	a.stopWatch()
	a.stopWatch = nil
}

func (a *App) setStoppedLocked() {
	if a.state != lifecycleStateNew {
		a.state = lifecycleStateStopped
	}
}

func (a *App) finishRun(err error) {
	a.mu.Lock()
	a.waitErr = err
	if a.waitDone != nil {
		close(a.waitDone)
		a.waitDone = nil
	}
	a.setStoppedLocked()
	a.mu.Unlock()
}

func (a *App) stopResources() error {
	var err error
	err = errors.Join(err, a.lifecycle.Stop())
	err = errors.Join(err, a.shutdownRuntimeAdapters(context.Background()))
	if a.opts != nil {
		err = errors.Join(err, closeManagedConfigSources(a.opts))
	}
	return err
}

func (a *App) reloadAsync() {
	go func() {
		if err := a.Reload(context.Background()); err != nil {
			slog.Error("auto reload failed", slog.Any("error", err))
		}
	}()
}

// Start initializes and runs the application.
func (a *App) Start(ctx context.Context) (err error) {
	a.mu.Lock()
	if err = a.prepareStartLocked(ctx); err != nil {
		a.mu.Unlock()
		return err
	}
	done := a.waitDone
	a.state = lifecycleStateRunning
	a.mu.Unlock()

	go func(waitDone chan struct{}) {
		_ = waitDone
		runErr := a.lifecycle.Run()
		a.finishRun(runErr)
	}(done)
	return nil
}

// Wait blocks until the running application exits.
func (a *App) Wait() error {
	a.mu.Lock()
	done := a.waitDone
	err := a.waitErr
	a.mu.Unlock()

	if done == nil {
		return err
	}
	<-done

	a.mu.Lock()
	defer a.mu.Unlock()
	return a.waitErr
}

// Stop stops the application.
func (a *App) Stop(ctx context.Context) error {
	_ = ctx

	a.mu.Lock()
	a.stopConfigWatchLocked()
	a.setStoppedLocked()
	a.mu.Unlock()

	return a.stopResources()
}

// NewClient creates a client for target service.
func (a *App) NewClient(ctx context.Context, name string) (client.Client, error) {
	if ctx == nil {
		return nil, errors.New("client context is nil")
	}
	a.mu.Lock()
	if err := a.ensureClientReadyLocked(ctx); err != nil {
		a.mu.Unlock()
		return nil, err
	}
	a.mu.Unlock()

	cli, err := client.New(ctx, name, a.currentRuntimeSnapshot())
	if err != nil {
		return nil, err
	}
	return cli, nil
}

func (a *App) initializeLocked(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			a.recordAssemblyErrorLocked(assemblyStagePrepare, err)
			return
		}
		a.clearAssemblyErrorLocked(assemblyStagePrepare)
	}()
	switch a.state {
	case lifecycleStateInitialized,
		lifecycleStateBusinessInstalled,
		lifecycleStateServing,
		lifecycleStateRunning:
		return nil
	case lifecycleStateStopped:
		return errRestartUnsupported
	}
	cleanupNeeded := true
	defer func() {
		if !cleanupNeeded || err == nil {
			return
		}
		a.state = lifecycleStateStopped
		if cleanupErr := a.stopResources(); cleanupErr != nil {
			wrapped := yassembly.NewError(
				yassembly.ErrPreparedRuntimeRollbackFailed,
				"prepare",
				"prepare cleanup failed",
				cleanupErr,
				nil,
			)
			a.recordAssemblyErrorLocked(assemblyStagePrepare, wrapped)
			slog.Error("failed to cleanup prepare failure", slog.Any("error", wrapped))
			err = errors.Join(err, wrapped)
		}
	}()
	if err = initConfigChain(a.opts); err != nil {
		err = wrapAssemblyStageError("prepare", err)
		return err
	}
	if err = a.resolveIdentityLocked(); err != nil {
		err = wrapAssemblyStageError("prepare", err)
		return err
	}
	planResult, planErr := a.buildAssemblyResult(ctx)
	if planErr != nil {
		err = wrapAssemblyStageError("prepare", planErr)
		return err
	}
	a.lastPlanResult = planResult
	a.lastSpecDiff = nil
	a.lastPlanHash = planResult.Hash
	a.lastStablePlanHash = planResult.Hash
	a.assemblySpec = planResult.Spec
	a.state = lifecycleStatePlanned
	if err = a.initHub(ctx); err != nil {
		err = wrapAssemblyStageError("prepare", err)
		return err
	}
	a.state = lifecycleStateInfraInitialized
	if err = validateStartup(a.opts); err != nil {
		err = wrapAssemblyStageError("prepare", err)
		return err
	}
	initInstanceInfo(a.name, effectiveResolved(planResult, a.opts.resolvedSettings))
	if err = a.applyRuntimeAdapters(a.currentRuntimeSnapshot()); err != nil {
		err = wrapAssemblyStageError("prepare", err)
		return err
	}
	if err = initGovernor(a.opts); err != nil {
		err = wrapAssemblyStageError("prepare", err)
		return err
	}
	a.initRegistry()
	if err = a.initServer(); err != nil {
		err = wrapAssemblyStageError("prepare", err)
		return err
	}
	a.runtime = newRuntimeSurface(a)
	a.runtimeAssembly = &preparedAssembly{
		Spec:    a.assemblySpec,
		Modules: append([]module.Module(nil), planResult.Modules...),
		Runtime: a.runtime,
		Server:  a.opts.server,
		CloseFunc: func(ctx context.Context) error {
			_ = ctx
			return a.stopResources()
		},
	}
	a.installDiagnosticsRoutes()
	a.state = lifecycleStateInitialized
	cleanupNeeded = false
	return nil
}

func (a *App) installConfigWatchLocked() {
	if a.watchStarted || a.opts == nil || a.opts.configManager == nil {
		return
	}
	first := true
	a.stopWatch = a.opts.configManager.Watch(nil, func(config.Snapshot) {
		if first {
			first = false
			return
		}
		a.reloadAsync()
	})
	a.watchStarted = true
}

func (a *App) initHub(ctx context.Context) error {
	modules := a.plannedModules()
	if a.lastPlanResult != nil {
		modules = append([]module.Module(nil), a.lastPlanResult.Modules...)
	}
	if err := a.hub.Use(modules...); err != nil {
		return err
	}
	if err := a.hub.Seal(); err != nil {
		return err
	}
	a.hub.SetCapabilityBindings(
		selectedCapabilityBindings(a.lastPlanResult, a.opts.resolvedSettings),
	)
	if err := a.hub.Init(ctx, a.opts.configManager.Snapshot()); err != nil {
		return err
	}
	a.opts.lifecycleOptions = append(
		a.opts.lifecycleOptions,
		withLifecycleBeforeStartHooks(func(ctx context.Context) error {
			return a.hub.Start(ctx)
		}),
		withLifecycleCleanup("module_hub", func(ctx context.Context) error {
			return a.hub.Stop(ctx)
		}),
	)
	return nil
}

func initInstanceInfo(appName string, resolved settings.Resolved) {
	instance.InitInstanceInfo(appName, resolved.Admin.Application)
}

func initGovernor(opts *options) error {
	svr, err := governor.NewServerWithConfig(
		opts.resolvedSettings.Admin.Governor,
		opts.configManager,
	)
	if err != nil {
		return err
	}
	opts.governor = svr
	return nil
}

func applyOptions(opts *options, ops ...Option) error {
	for _, f := range ops {
		if err := f(opts); err != nil {
			return err
		}
	}
	return nil
}
