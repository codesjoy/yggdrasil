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
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/codesjoy/yggdrasil/v2/governor"
	"github.com/codesjoy/yggdrasil/v2/internal/constant"
	"github.com/codesjoy/yggdrasil/v2/internal/instance"
	"github.com/codesjoy/yggdrasil/v2/registry"
	"github.com/codesjoy/yggdrasil/v2/server"
	"golang.org/x/sync/errgroup"

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
	// stageAfterStop after app stop
	stageAfterStop
	// stageMax stage max
	stageMax
)

const defaultShutdownTimeout = time.Second * 30

const (
	registryStateInit = iota
	registryStateDone
	registryStateCancel
)

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
		app.runHooks(stageBeforeStop)
		defer func() {
			app.runHooks(stageAfterStop)
		}()
		app.deregister()
		err = app.stopServers()
	})
	if err != nil {
		return err
	}
	return nil
}

// Run runs the application
func (app *application) Run() error {
	var err error
	app.runOnce.Do(func() {
		app.optsMu.Lock()
		app.running = true
		app.optsMu.Unlock()
		app.waitSignals()
		if err = app.startServers(); err != nil {
			return
		}
		slog.Info("app shutdown")
	})

	return err
}

func (app *application) runHooks(k Stage) {
	hooks, ok := app.hooks[k]
	if ok {
		hooks.Done()
	}
}

func (app *application) register() {
	if app.registry == nil {
		return
	}
	app.mu.Lock()
	if app.registryState != registryStateInit {
		app.mu.Unlock()
		return
	}
	app.registryState = registryStateDone
	app.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := app.registry.Register(ctx, app); err != nil {
		slog.Error("fault to register application", slog.Any("error", err))
		go func() {
			if stopErr := app.Stop(); stopErr != nil {
				slog.Error("fault to stop after registration failure", slog.Any("error", stopErr))
			}
		}()
		return
	}
	slog.Info("application has been registered")
}

func (app *application) deregister() {
	if app.registry == nil {
		return
	}
	app.mu.Lock()
	if app.registryState != registryStateDone {
		app.mu.Unlock()
		return
	}
	app.registryState = registryStateCancel
	app.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.TODO(), defaultShutdownTimeout)
	defer cancel()
	if err := app.registry.Deregister(ctx, app); err != nil {
		slog.Error("fault to deregister application", slog.Any("error", err))
	}
}

func (app *application) startServers() error {
	app.runHooks(stageBeforeStart)
	eg := errgroup.Group{}
	svrStarCh := make(chan struct{}, 1)
	if app.server != nil {
		eg.Go(func() error {
			return app.server.Serve(svrStarCh)
		})
	} else {
		svrStarCh <- struct{}{}
	}
	eg.Go(func() error {
		return app.governor.Serve()
	})
	for _, item := range app.internalSvr {
		svr := item
		eg.Go(func() error {
			return svr.Serve()
		})
	}

	eg.Go(func() error {
		_, ok := <-svrStarCh
		if !ok {
			return nil
		}
		defer close(svrStarCh)
		app.register()
		return nil
	})
	return eg.Wait()
}

func (app *application) stopServers() error {
	slog.Info("stopping servers...")
	eg := errgroup.Group{}
	if app.server != nil {
		eg.Go(func() error {
			slog.Info("stopping main server")
			if err := app.server.Stop(); err != nil {
				slog.Error("failed to stop main server", slog.Any("error", err))
				return err
			}
			slog.Info("main server stopped")
			return nil
		})
	}

	for idx, svr := range app.internalSvr {
		slog.Info("stopping internal server", slog.Int("index", idx))
		if err := svr.Stop(); err != nil {
			slog.Error("failed to stop internal server", slog.Int("index", idx), slog.Any("error", err))
			return err
		}
		slog.Info("internal server stopped", slog.Int("index", idx))
	}

	eg.Go(func() error {
		slog.Info("stopping governor")
		if err := app.governor.Stop(); err != nil {
			slog.Error("failed to stop governor", slog.Any("error", err))
			return err
		}
		slog.Info("governor stopped")
		return nil
	})

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
			attr := maps.Clone(item.Metadata())
			attr[registry.MDServerKind] = string(item.Kind())
			endpoints = append(endpoints, endpoint{
				address: item.Address(),
				scheme:  item.Scheme(),
				Attr:    attr,
			})
		}
	}
	governorInfo := app.governor.Info()
	attr := maps.Clone(governorInfo.Attr)
	attr[registry.MDServerKind] = string(constant.ServerKindGovernor)
	endpoints = append(endpoints, endpoint{
		address: governorInfo.Address,
		scheme:  governorInfo.Scheme,
		Attr:    attr,
	})
	return endpoints
}

func (app *application) getShutdownTimeout() time.Duration {
	if app.shutdownTimeout < defaultShutdownTimeout {
		return defaultShutdownTimeout
	}
	return app.shutdownTimeout
}

func (app *application) waitSignals() {
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, shutdownSignals...)
	go func() {
		s := <-sig
		go func() {
			<-time.After(app.getShutdownTimeout())
			os.Exit(128 + int(s.(syscall.Signal)))
		}()
		go func() {
			if err := app.Stop(); err != nil {
				slog.Error("fault to stop", slog.Any("error", err))
				return
			}
		}()
	}()
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
