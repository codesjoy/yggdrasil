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
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codesjoy/yggdrasil/v3/admin/governor"
	internallifecycle "github.com/codesjoy/yggdrasil/v3/app/internal/lifecycle"
	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	internalidentity "github.com/codesjoy/yggdrasil/v3/internal/identity"
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
	lifecycleStateStopping
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

func withLifecycleLogger(logger *slog.Logger) lifecycleOption {
	return internallifecycle.WithLogger(logger)
}

func withLifecycleIdentity(identity internalidentity.Identity) lifecycleOption {
	return internallifecycle.WithIdentity(identity)
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
	// ErrProcessDefaultsAlreadyInstalled is returned when another App already owns process defaults.
	ErrProcessDefaultsAlreadyInstalled = errors.New(
		"process defaults are already installed by another yggdrasil app",
	)
)

// App is the yggdrasil runtime composition root.
type App struct {
	name string

	mu                     sync.Mutex
	state                  lifecycleState
	lifecycleOptsInstalled bool
	stopping               atomic.Bool

	opts *options

	lifecycle *lifecycleRunner
	hub       *module.Hub

	reloadMu   sync.Mutex
	installMu  sync.Mutex
	installing atomic.Bool

	watchMu      sync.Mutex
	stopWatch    func()
	watchStarted bool
	configWake   chan struct{}
	configStop   chan struct{}
	configDone   chan struct{}

	configPending      config.Snapshot
	configPendingValid bool
	configGeneration   uint64
	configLastApplied  string

	stopOnce sync.Once
	stopDone chan struct{}
	stopErr  error

	waitDone chan struct{}
	waitErr  error

	runtimeMu                  sync.RWMutex
	planMu                     sync.RWMutex
	runtimeSnapshot            *Snapshot
	foundationSnapshot         *Snapshot
	preparedFoundationSnapshot *Snapshot
	tracerShutdown             func(context.Context) error
	meterShutdown              func(context.Context) error
	processDefaultsLease       *processDefaultsLease
	pendingRuntimeAdapters     *preparedRuntimeAdapters

	runtime            Runtime
	reloadCandidate    *yassembly.Result
	lastPlanResult     *yassembly.Result
	stablePlanResult   *yassembly.Result
	lastSpecDiff       *yassembly.SpecDiff
	lastPlanHash       string
	lastStablePlanHash string
	assemblyErrors     assemblyErrorState
	assemblySpec       *yassembly.Spec
	runtimeAssembly    *preparedAssembly
	identity           internalidentity.Identity
	identityResolved   bool

	explicitBundleInstalled bool
	installedRPCServices    map[string]struct{}
	installedHTTPRoutes     map[string]struct{}
	installedGovernorRoutes map[string]struct{}
	bundleDiagnostics       []BundleDiag
}

// New creates a new App.
func New(appName string, ops ...Option) (*App, error) {
	appName = strings.TrimSpace(appName)
	if appName == "" {
		return nil, errors.New("app name is required")
	}
	opts := &options{}
	if err := applyOptions(opts, ops...); err != nil {
		return nil, err
	}
	lifecycle, err := newLifecycleRunner()
	if err != nil {
		return nil, err
	}
	return &App{
		name:                    appName,
		state:                   lifecycleStateNew,
		opts:                    opts,
		lifecycle:               lifecycle,
		hub:                     module.NewHub(),
		stopDone:                make(chan struct{}),
		installedRPCServices:    map[string]struct{}{},
		installedHTTPRoutes:     map[string]struct{}{},
		installedGovernorRoutes: map[string]struct{}{},
	}, nil
}

func (a *App) buildLifecycleOptions() []lifecycleOption {
	out := a.opts.buildLifecycleOptions()
	out = append(out, withLifecycleBeforeStopHooks(a.beginStop))
	if a.identityResolved {
		out = append(out, withLifecycleIdentity(a.identity))
	}
	out = append(
		out,
		withLifecycleLogger(a.runtimeLogger()),
		withLifecycleCleanup("runtime_adapters", a.shutdownRuntimeAdapters),
		withLifecycleCleanup("config_watch", func(context.Context) error {
			a.stopConfigWatch()
			return nil
		}),
	)
	return out
}

func (a *App) prepareStartLocked(ctx context.Context) error {
	if a.stopping.Load() {
		return errRestartUnsupported
	}
	switch a.state {
	case lifecycleStateServing, lifecycleStateRunning:
		return errApplicationAlreadyRunning
	case lifecycleStateStopping, lifecycleStateStopped:
		return errRestartUnsupported
	}
	if err := a.initializeLocked(ctx); err != nil {
		return err
	}
	a.state = lifecycleStateServing
	a.installConfigWatchLocked()
	a.waitDone = make(chan struct{})
	a.waitErr = nil
	return nil
}

func (a *App) ensureClientReadyLocked(ctx context.Context) error {
	if a.stopping.Load() || a.state == lifecycleStateStopping || a.state == lifecycleStateStopped {
		return errRestartUnsupported
	}
	return a.initializeLocked(ctx)
}

func (a *App) stopConfigWatchLocked() {
	a.stopConfigWatch()
}

func (a *App) stopConfigWatch() {
	a.watchMu.Lock()
	if !a.watchStarted && a.stopWatch == nil {
		a.watchMu.Unlock()
		return
	}
	stopWatch := a.stopWatch
	stop := a.configStop
	done := a.configDone
	a.stopWatch = nil
	a.watchStarted = false
	a.watchMu.Unlock()

	if stopWatch != nil {
		stopWatch()
	}
	if stop != nil {
		close(stop)
	}
	if done != nil {
		<-done
	}
}

func (a *App) beginStop(ctx context.Context) error {
	a.stopping.Store(true)
	a.stopConfigWatch()
	a.reloadMu.Lock()
	a.reloadMu.Unlock() //nolint:staticcheck // barrier for an in-flight reload.
	_ = ctx
	return nil
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

func (a *App) stopResources(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	a.stopOnce.Do(func() {
		var err error
		err = errors.Join(err, a.lifecycle.Stop(ctx))
		err = errors.Join(err, a.shutdownRuntimeAdapters(ctx))
		if a.opts != nil {
			err = errors.Join(err, closeManagedConfigSources(a.opts))
			err = errors.Join(err, closeManagedConfigManager(a.opts))
		}
		a.stopErr = err
		close(a.stopDone)
	})
	<-a.stopDone
	return a.stopErr
}

func (a *App) reloadAsync() {
	go func() {
		if err := a.Reload(context.Background()); err != nil {
			a.runtimeLogger().Error("auto reload failed", slog.Any("error", err))
		}
	}()
}

// Start initializes and runs the application.
func (a *App) Start(ctx context.Context) (err error) {
	a.installMu.Lock()
	a.mu.Lock()
	if err = a.prepareStartLocked(ctx); err != nil {
		a.mu.Unlock()
		a.installMu.Unlock()
		return err
	}
	a.mu.Unlock()
	a.installMu.Unlock()

	if err = a.lifecycle.Start(ctx); err != nil {
		if a.stopping.Load() && errors.Is(err, context.Canceled) {
			stopErr := a.stopResources(context.Background())
			a.finishRun(stopErr)
			return nil
		}
		cleanupErr := a.stopResources(context.Background())
		startErr := errors.Join(err, cleanupErr)
		a.finishRun(startErr)
		return startErr
	}
	a.mu.Lock()
	a.state = lifecycleStateRunning
	a.mu.Unlock()
	a.wakeConfigWatch()
	go func() {
		a.finishRun(a.lifecycle.Wait())
	}()
	return nil
}

// Wait blocks until the running application exits.
func (a *App) Wait() error {
	a.mu.Lock()
	done := a.waitDone
	err := a.waitErr
	stopping := a.stopping.Load()
	stopDone := a.stopDone
	a.mu.Unlock()

	if done == nil {
		if !stopping {
			return err
		}
		<-stopDone
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.stopErr
	}
	<-done

	a.mu.Lock()
	defer a.mu.Unlock()
	return a.waitErr
}

// Stop stops the application.
func (a *App) Stop(ctx context.Context) error {
	a.installMu.Lock()
	a.mu.Lock()
	a.stopping.Store(true)
	if a.state != lifecycleStateNew {
		a.state = lifecycleStateStopping
	}
	a.mu.Unlock()
	a.installMu.Unlock()

	err := errors.Join(a.beginStop(ctx), a.stopResources(ctx))
	a.mu.Lock()
	a.setStoppedLocked()
	a.mu.Unlock()
	return err
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
	case lifecycleStateStopping, lifecycleStateStopped:
		return errRestartUnsupported
	}
	if a.opts != nil && a.opts.processDefaults && a.processDefaultsLease == nil {
		if _, err = acquireProcessDefaultsLease(a); err != nil {
			return err
		}
	}
	cleanupNeeded := true
	defer func() {
		if !cleanupNeeded || err == nil {
			return
		}
		if installErr := a.installLifecycleOptions(); installErr != nil {
			err = errors.Join(err, installErr)
		}
		a.state = lifecycleStateStopped
		if cleanupErr := a.stopResources(context.Background()); cleanupErr != nil {
			wrapped := yassembly.NewError(
				yassembly.ErrPreparedRuntimeRollbackFailed,
				"prepare",
				"prepare cleanup failed",
				cleanupErr,
				nil,
			)
			a.recordAssemblyErrorLocked(assemblyStagePrepare, wrapped)
			a.runtimeLogger().Error(
				"failed to cleanup prepare failure",
				slog.Any("error", wrapped),
			)
			err = errors.Join(err, wrapped)
		}
	}()
	if err = initConfigChainWithLogger(a.opts, a.runtimeLogger()); err != nil {
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
	a.planMu.Lock()
	a.stablePlanResult = planResult
	a.planMu.Unlock()
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
	if err = validateStartupWithLogger(a.opts, a.runtimeLogger()); err != nil {
		err = wrapAssemblyStageError("prepare", err)
		return err
	}
	if err = a.applyRuntimeAdapters(a.currentRuntimeSnapshot()); err != nil {
		err = wrapAssemblyStageError("prepare", err)
		return err
	}
	if err = initGovernor(a.opts); err != nil {
		err = wrapAssemblyStageError("prepare", err)
		return err
	}
	if err = a.initRegistry(); err != nil {
		err = wrapAssemblyStageError("prepare", err)
		return err
	}
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
			return a.stopResources(ctx)
		},
	}
	a.installDiagnosticsRoutes()
	if err = a.installLifecycleOptions(); err != nil {
		err = wrapAssemblyStageError("prepare", err)
		return err
	}
	a.state = lifecycleStateInitialized
	cleanupNeeded = false
	return nil
}

func (a *App) installLifecycleOptions() error {
	if a.lifecycleOptsInstalled {
		return nil
	}
	if err := a.lifecycle.Init(a.buildLifecycleOptions()...); err != nil {
		return err
	}
	a.lifecycleOptsInstalled = true
	return nil
}

func (a *App) installConfigWatchLocked() {
	a.watchMu.Lock()
	if a.watchStarted || a.opts == nil || a.opts.configManager == nil {
		a.watchMu.Unlock()
		return
	}
	a.configWake = make(chan struct{}, 1)
	a.configStop = make(chan struct{})
	a.configDone = make(chan struct{})
	a.configLastApplied = string(a.opts.configManager.Snapshot().Bytes())
	a.configPendingValid = false
	a.watchStarted = true
	a.watchMu.Unlock()

	stopWatch := a.opts.configManager.WatchWithError(
		nil,
		a.queueConfigSnapshot,
		a.handleConfigWatchError,
	)
	a.watchMu.Lock()
	a.stopWatch = stopWatch
	go a.runConfigWatcher(a.configStop, a.configWake, a.configDone)
	a.watchMu.Unlock()
}

func (a *App) queueConfigSnapshot(snapshot config.Snapshot) {
	a.watchMu.Lock()
	a.configPending = snapshot
	a.configPendingValid = true
	a.configGeneration++
	wake := a.configWake
	a.watchMu.Unlock()
	signalChannel(wake)
}

func (a *App) handleConfigWatchError(err error) {
	if err == nil {
		return
	}
	_ = a.recordReloadError(wrapAssemblyStageError(
		"reload",
		fmt.Errorf("config watch: %w", err),
	))
}

func (a *App) wakeConfigWatch() {
	a.watchMu.Lock()
	wake := a.configWake
	a.watchMu.Unlock()
	signalChannel(wake)
}

func signalChannel(channel chan struct{}) {
	if channel == nil {
		return
	}
	select {
	case channel <- struct{}{}:
	default:
	}
}

func (a *App) runConfigWatcher(
	stop <-chan struct{},
	wake <-chan struct{},
	done chan<- struct{},
) {
	defer close(done)
	for {
		select {
		case <-stop:
			return
		case <-wake:
			a.processPendingConfig()
		}
	}
}

func (a *App) processPendingConfig() {
	a.mu.Lock()
	running := a.state == lifecycleStateRunning && !a.stopping.Load()
	manager := a.opts.configManager
	a.mu.Unlock()
	if !running || manager == nil {
		return
	}

	a.watchMu.Lock()
	if !a.configPendingValid {
		a.watchMu.Unlock()
		return
	}
	generation := a.configGeneration
	applied := a.configLastApplied
	a.watchMu.Unlock()

	snapshot := manager.Snapshot()
	if string(snapshot.Bytes()) == applied {
		a.watchMu.Lock()
		if a.configGeneration == generation {
			a.configPendingValid = false
		}
		a.watchMu.Unlock()
		return
	}

	err := a.Reload(context.Background())
	a.mu.Lock()
	stillRunning := a.state == lifecycleStateRunning && !a.stopping.Load()
	a.mu.Unlock()

	a.watchMu.Lock()
	if err == nil && stillRunning {
		a.configLastApplied = string(snapshot.Bytes())
	}
	next := a.configGeneration != generation
	if !next {
		a.configPendingValid = false
	}
	wake := a.configWake
	a.watchMu.Unlock()
	if next {
		signalChannel(wake)
	}
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

func initGovernor(opts *options) error {
	svr, err := governor.NewServerWithConfig(
		opts.resolvedSettings.Admin.Governor,
		opts.configManager,
	)
	if err != nil {
		if errors.Is(err, governor.ErrInsecureExposure) {
			return yassembly.NewError(
				yassembly.ErrInsecureAdminExposure,
				"prepare",
				"governor non-loopback exposure requires authentication",
				err,
				nil,
			)
		}
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
