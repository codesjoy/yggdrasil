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

// Package application implements the application.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/codesjoy/yggdrasil/v2/governor"
	"github.com/codesjoy/yggdrasil/v2/internal/constant"
	"github.com/codesjoy/yggdrasil/v2/internal/instance"
	"github.com/codesjoy/yggdrasil/v2/registry"
	"github.com/codesjoy/yggdrasil/v2/server"

	"github.com/codesjoy/yggdrasil/v2/internal/defers"
)

// Application application
type Application interface {
	Init(opts ...Option) error
	Run() error
	Stop() error
}

// Stage application stage
type Stage uint32

const (
	_ Stage = iota
	// stageBeforeStart before app start
	stageBeforeStart
	// stageBeforeStop before app stop
	stageBeforeStop
	// stageCleanup cleanup resources
	stageCleanup
	// stageAfterStop after app stop
	stageAfterStop
	// stageMax stage max
	stageMax
)

const defaultShutdownTimeout = time.Second * 30

const (
	registryStateInit = iota
	registryStateRegistering
	registryStateDone
	registryStateCancel
)

var errGovernorRequired = errors.New("application governor is required")

type application struct {
	runOnce  sync.Once
	stopOnce sync.Once

	mu sync.Mutex

	optsMu  sync.RWMutex
	running bool

	server server.Server

	governor *governor.Server

	internalSvr []InternalServer

	registryState int
	registry      registry.Registry

	shutdownTimeout time.Duration

	hooks map[Stage]*defers.Defer
}

// New create a new application
func New(opts ...Option) (Application, error) {
	return newApplication(opts...)
}

func newApplication(opts ...Option) (*application, error) {
	app := &application{
		hooks: map[Stage]*defers.Defer{},
	}
	for i := Stage(1); i < stageMax; i++ {
		app.hooks[i] = defers.NewDefer()
	}
	for _, o := range opts {
		if err := o(app); err != nil {
			return nil, err
		}
	}

	return app, nil
}

// Init init  application
func (app *application) Init(opts ...Option) error {
	app.optsMu.Lock()
	defer app.optsMu.Unlock()
	if app.running {
		slog.Warn("the application has been started, and the settings are no longer applied")
		return nil
	}
	for _, o := range opts {
		if err := o(app); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops the application
func (app *application) Stop() error {
	var err error
	app.stopOnce.Do(func() {
		app.optsMu.Lock()
		app.running = false
		app.optsMu.Unlock()

		timeout := app.shutdownTimeout
		if timeout <= 0 {
			timeout = defaultShutdownTimeout
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		err = errors.Join(err, app.runHooks(ctx, stageBeforeStop))
		err = errors.Join(err, app.deregister(ctx))
		err = errors.Join(err, app.stopServers(ctx))
		err = errors.Join(err, app.runHooks(ctx, stageCleanup))
		err = errors.Join(err, app.runHooks(ctx, stageAfterStop))
	})
	if err != nil {
		return err
	}
	return nil
}

// Run runs the application
func (app *application) Run() error {
	if app.governor == nil {
		return errGovernorRequired
	}

	var err error
	app.runOnce.Do(func() {
		cleanupSignals := app.waitSignals()
		defer cleanupSignals()

		app.optsMu.Lock()
		app.running = true
		app.optsMu.Unlock()
		if err = app.startServers(); err != nil {
			return
		}
		slog.Info("app shutdown")
	})

	return err
}

func (app *application) runHooks(ctx context.Context, k Stage) error {
	hooks, ok := app.hooks[k]
	if ok {
		return hooks.Done(ctx)
	}
	return nil
}

func (app *application) register() error {
	if app.registry == nil {
		return nil
	}
	app.mu.Lock()
	switch app.registryState {
	case registryStateDone, registryStateCancel, registryStateRegistering:
		app.mu.Unlock()
		return nil
	}
	app.registryState = registryStateRegistering
	app.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := app.registry.Register(ctx, app); err != nil {
		app.mu.Lock()
		if app.registryState == registryStateRegistering {
			app.registryState = registryStateInit
		}
		app.mu.Unlock()
		slog.Error("fault to register application", slog.Any("error", err))
		return err
	}

	app.mu.Lock()
	state := app.registryState
	if state == registryStateRegistering {
		app.registryState = registryStateDone
	}
	app.mu.Unlock()

	if state == registryStateCancel {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := app.registry.Deregister(ctx, app); err != nil {
			slog.Error(
				"fault to deregister application after concurrent stop",
				slog.Any("error", err),
			)
			return err
		}
		return nil
	}

	slog.Info("application has been registered")
	return nil
}

func (app *application) deregister(ctx context.Context) error {
	if app.registry == nil {
		return nil
	}
	app.mu.Lock()
	switch app.registryState {
	case registryStateRegistering:
		app.registryState = registryStateCancel
		app.mu.Unlock()
		return nil
	case registryStateDone:
		app.registryState = registryStateCancel
	default:
		app.mu.Unlock()
		return nil
	}
	app.mu.Unlock()
	if err := app.registry.Deregister(ctx, app); err != nil {
		slog.Error("fault to deregister application", slog.Any("error", err))
		return err
	}
	return nil
}

func (app *application) startServers() error {
	if app.governor == nil {
		return errGovernorRequired
	}
	if err := app.runHooks(context.Background(), stageBeforeStart); err != nil {
		return err
	}
	eg := errgroup.Group{}
	svrStarCh := make(chan struct{}, 1)
	var stopOnce sync.Once
	stopAsync := func() {
		stopOnce.Do(func() {
			go func() {
				if err := app.Stop(); err != nil {
					slog.Error(
						"fault to stop application after serve failure",
						slog.Any("error", err),
					)
				}
			}()
		})
	}

	if app.server != nil {
		eg.Go(func() error {
			if err := app.server.Serve(svrStarCh); err != nil {
				stopAsync()
				return fmt.Errorf("main server: %w", err)
			}
			return nil
		})
	} else {
		svrStarCh <- struct{}{}
	}
	eg.Go(func() error {
		if err := app.governor.Serve(); err != nil {
			stopAsync()
			return fmt.Errorf("governor: %w", err)
		}
		return nil
	})
	for _, item := range app.internalSvr {
		svr := item
		eg.Go(func() error {
			if err := svr.Serve(); err != nil {
				stopAsync()
				return fmt.Errorf("internal server: %w", err)
			}
			return nil
		})
	}

	eg.Go(func() error {
		// Wait for servers to start
		_, ok := <-svrStarCh
		if !ok {
			// Channel closed without value - startup failed
			return nil
		}
		// Servers started successfully, register the service
		if err := app.register(); err != nil {
			stopAsync()
			return fmt.Errorf("register application: %w", err)
		}
		return nil
	})
	return eg.Wait()
}

func (app *application) stopServers(ctx context.Context) error {
	slog.Info("stopping servers")
	eg := errgroup.Group{}
	if app.server != nil {
		eg.Go(func() error {
			slog.Info("stopping main server")
			if err := app.server.Stop(ctx); err != nil {
				slog.Error("failed to stop main server", slog.Any("error", err))
				return err
			}
			slog.Info("main server stopped")
			return nil
		})
	}

	for idx, svr := range app.internalSvr {
		idx := idx
		svr := svr
		eg.Go(func() error {
			slog.Info("stopping internal server", slog.Int("index", idx))
			if err := svr.Stop(ctx); err != nil {
				slog.Error(
					"failed to stop internal server",
					slog.Int("index", idx),
					slog.Any("error", err),
				)
				return err
			}
			slog.Info("internal server stopped", slog.Int("index", idx))
			return nil
		})
	}
	if app.governor != nil {
		eg.Go(func() error {
			slog.Info("stopping governor")
			if err := app.governor.Shutdown(ctx); err != nil {
				slog.Error("failed to stop governor", slog.Any("error", err))
				return err
			}
			slog.Info("governor stopped")
			return nil
		})
	}

	// 等待所有服务器停止或超时
	if err := eg.Wait(); err != nil {
		return fmt.Errorf("error stopping servers: %w", err)
	}
	slog.Info("all servers stopped successfully")
	return nil
}

// Region return the region of the application
func (app *application) Region() string {
	return instance.Region()
}

// Zone return the zone of the application
func (app *application) Zone() string {
	return instance.Zone()
}

// Campus return the campus of the application
func (app *application) Campus() string {
	return instance.Campus()
}

// Namespace return the namespace of the application
func (app *application) Namespace() string {
	return instance.Namespace()
}

// Name return the name of the application
func (app *application) Name() string {
	return instance.Name()
}

// Version return the version of the application
func (app *application) Version() string {
	return instance.Version()
}

// Metadata return the metadata of the application
func (app *application) Metadata() map[string]string {
	return instance.Metadata()
}

// Endpoints return the endpoints of the application
func (app *application) Endpoints() []registry.Endpoint {
	endpoints := make([]registry.Endpoint, 0)
	if app.server != nil {
		for _, item := range app.server.Endpoints() {
			attr := cloneEndpointMetadata(item.Metadata())
			attr[registry.MDServerKind] = string(item.Kind())
			endpoints = append(endpoints, endpoint{
				address: item.Address(),
				scheme:  item.Scheme(),
				Attr:    attr,
			})
		}
	}
	if app.governor != nil {
		governorInfo := app.governor.Info()
		attr := cloneEndpointMetadata(governorInfo.Attr)
		attr[registry.MDServerKind] = string(constant.ServerKindGovernor)
		endpoints = append(endpoints, endpoint{
			address: governorInfo.Address,
			scheme:  governorInfo.Scheme,
			Attr:    attr,
		})
	}
	return endpoints
}

func cloneEndpointMetadata(src map[string]string) map[string]string {
	if cloned := maps.Clone(src); cloned != nil {
		return cloned
	}
	return map[string]string{}
}

func (app *application) getShutdownTimeout() time.Duration {
	if app.shutdownTimeout < defaultShutdownTimeout {
		return defaultShutdownTimeout
	}
	return app.shutdownTimeout
}

func (app *application) waitSignals() func() {
	sig := make(chan os.Signal, 2)
	done := make(chan struct{})
	var cleanupOnce sync.Once
	signal.Notify(sig, shutdownSignals...)

	cleanup := func() {
		cleanupOnce.Do(func() {
			signal.Stop(sig)
			close(done)
		})
	}

	go func() {
		select {
		case <-done:
			return
		case s := <-sig:
			go func() {
				if err := app.Stop(); err != nil {
					slog.Error("fault to stop", slog.Any("error", err))
				}
			}()

			timer := time.NewTimer(app.getShutdownTimeout())
			defer timer.Stop()
			select {
			case <-done:
				return
			case <-timer.C:
				if sig, ok := s.(syscall.Signal); ok {
					os.Exit(128 + int(sig))
				}
				os.Exit(1)
			}
		}
	}()
	return cleanup
}

// endpoint application endpoint
type endpoint struct {
	address string
	scheme  string
	Attr    map[string]string
}

// Scheme returns the scheme of the endpoint
func (e endpoint) Scheme() string {
	return e.scheme
}

// Address returns the address of the endpoint
func (e endpoint) Address() string {
	return e.address
}

// Metadata returns the metadata of the endpoint
func (e endpoint) Metadata() map[string]string {
	return e.Attr
}
