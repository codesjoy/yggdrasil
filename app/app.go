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
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"github.com/codesjoy/yggdrasil/v3/client"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/governor"
	"github.com/codesjoy/yggdrasil/v3/internal/instance"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/server"
)

type lifecycleState uint32

const (
	lifecycleStateNew lifecycleState = iota
	lifecycleStateInitialized
	lifecycleStateRunning
	lifecycleStateStopped
)

var (
	errApplicationAlreadyRunning = errors.New("application is already running")
	errRestartUnsupported        = errors.New("restarting yggdrasil in the same process is not supported")
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

	runtimeMu                  sync.RWMutex
	runtimeSnapshot            *Snapshot
	foundationSnapshot         *Snapshot
	preparedFoundationSnapshot *Snapshot
	tracerShutdown             func(context.Context) error
	meterShutdown              func(context.Context) error
}

// New creates a new App.
func New(appName string, ops ...Option) (*App, error) {
	opts := &options{
		rpcServices:  map[*server.ServiceDesc]interface{}{},
		restServices: map[*server.RestServiceDesc]restServiceRegistration{},
	}
	if err := applyOptions(opts, ops...); err != nil {
		return nil, err
	}
	lifecycle, err := newLifecycleRunner()
	if err != nil {
		return nil, err
	}
	return &App{
		name:      appName,
		state:     lifecycleStateNew,
		opts:      opts,
		lifecycle: lifecycle,
		hub:       module.NewHub(),
	}, nil
}

func (a *App) prepareStartLocked(ctx context.Context) error {
	switch a.state {
	case lifecycleStateRunning:
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
	a.state = lifecycleStateRunning
	a.installConfigWatchLocked()
	return nil
}

func (a *App) ensureClientReadyLocked(ctx context.Context) error {
	if a.state == lifecycleStateNew {
		if err := a.initializeLocked(ctx); err != nil {
			return err
		}
	}
	if a.state == lifecycleStateStopped {
		return errRestartUnsupported
	}
	return nil
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

func (a *App) finishRun() {
	a.mu.Lock()
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
	a.mu.Unlock()

	err = a.lifecycle.Run()
	a.finishRun()
	return err
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

// Reload triggers one staged reload cycle.
func (a *App) Reload(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	a.reloadMu.Lock()
	defer a.reloadMu.Unlock()

	a.mu.Lock()
	if a.state != lifecycleStateRunning {
		a.mu.Unlock()
		return nil
	}
	opts := a.opts
	hub := a.hub
	prevBindings := cloneCapabilityBindings(opts.resolvedSettings.CapabilityBindings)
	a.mu.Unlock()

	if err := refreshResolvedSettings(opts); err != nil {
		return err
	}
	if !capabilityBindingsEqual(prevBindings, opts.resolvedSettings.CapabilityBindings) {
		hub.MarkCapabilityBindingsChanged()
	}
	hub.SetCapabilityBindings(opts.resolvedSettings.CapabilityBindings)
	return hub.Reload(ctx, opts.configManager.Snapshot())
}

// NewClient creates a client for target service.
func (a *App) NewClient(name string) (client.Client, error) {
	a.mu.Lock()
	if err := a.ensureClientReadyLocked(context.Background()); err != nil {
		a.mu.Unlock()
		return nil, err
	}
	a.mu.Unlock()

	cli, err := client.New(context.Background(), name, a.currentRuntimeSnapshot())
	if err != nil {
		return nil, err
	}
	return cli, nil
}

func (a *App) initializeLocked(ctx context.Context) (err error) {
	switch a.state {
	case lifecycleStateInitialized, lifecycleStateRunning:
		return nil
	case lifecycleStateStopped:
		return errRestartUnsupported
	}
	if err = initConfigChain(a.opts); err != nil {
		return err
	}
	if err = a.initHub(ctx); err != nil {
		return err
	}
	if err = validateStartup(a.opts); err != nil {
		return err
	}
	initInstanceInfo(a.name, a.opts.resolvedSettings)
	if err = a.applyRuntimeAdapters(a.currentRuntimeSnapshot()); err != nil {
		return err
	}
	if err = initGovernor(a.opts); err != nil {
		return err
	}
	a.initRegistry()
	if err = a.initServer(); err != nil {
		return err
	}
	a.installDiagnosticsRoutes()
	a.state = lifecycleStateInitialized
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
	modules := make([]module.Module, 0, 4+len(a.opts.modules))
	modules = append(modules,
		foundationBuiltinCapabilityModule{},
		connectivityBuiltinCapabilityModule{},
		foundationRuntimeModule{app: a},
		connectivityRuntimeModule{app: a},
	)
	modules = append(modules, a.opts.modules...)
	if err := a.hub.Use(modules...); err != nil {
		return err
	}
	if err := a.hub.Seal(); err != nil {
		return err
	}
	a.hub.SetCapabilityBindings(a.opts.resolvedSettings.CapabilityBindings)
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

func (a *App) installDiagnosticsRoutes() {
	if a.opts == nil || a.opts.governor == nil {
		return
	}
	a.opts.governor.HandleFunc("/module-hub", func(w http.ResponseWriter, r *http.Request) {
		resp := a.hub.Diagnostics()
		w.WriteHeader(200)
		encoder := json.NewEncoder(w)
		if r.URL.Query().Get("pretty") == "true" {
			encoder.SetIndent("", "    ")
		}
		_ = encoder.Encode(resp)
	})
}

func initInstanceInfo(appName string, resolved settings.Resolved) {
	instance.InitInstanceInfo(appName, resolved.Admin.Application)
}

func initGovernor(opts *options) error {
	svr, err := governor.NewServerWithConfig(opts.resolvedSettings.Admin.Governor, opts.configManager)
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

func cloneCapabilityBindings(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(in))
	for key, items := range in {
		out[key] = append([]string(nil), items...)
	}
	return out
}

func capabilityBindingsEqual(left, right map[string][]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftItems := range left {
		rightItems, ok := right[key]
		if !ok {
			return false
		}
		if len(leftItems) != len(rightItems) {
			return false
		}
		for i := range leftItems {
			if leftItems[i] != rightItems[i] {
				return false
			}
		}
	}
	return true
}
