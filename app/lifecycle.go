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
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/codesjoy/yggdrasil/v3/admin/governor"
	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/internal/defers"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

// InternalServer is managed by the App lifecycle alongside the main server.
type InternalServer interface {
	Serve() error
	Stop(context.Context) error
}

type lifecycleStage uint32

const (
	_ lifecycleStage = iota
	lifecycleStageBeforeStart
	lifecycleStageBeforeStop
	lifecycleStageCleanup
	lifecycleStageAfterStop
	lifecycleStageMax
)

const defaultShutdownTimeout = 30 * time.Second

const (
	registryStateInit = iota
	registryStateRegistering
	registryStateDone
	registryStateCancel
)

type lifecycleEndpoint struct {
	address  string
	scheme   string
	metadata map[string]string
}

func (e lifecycleEndpoint) Scheme() string {
	return e.scheme
}

func (e lifecycleEndpoint) Address() string {
	return e.address
}

func (e lifecycleEndpoint) Metadata() map[string]string {
	return e.metadata
}

var errGovernorRequired = errors.New("application governor is required")

type lifecycleOption func(*lifecycleRunner) error

func withLifecycleHook(stage lifecycleStage, hooks ...func(context.Context) error) lifecycleOption {
	return func(runner *lifecycleRunner) error {
		stageHooks, ok := runner.hooks[stage]
		if !ok {
			return fmt.Errorf("hook stage not found")
		}
		stageHooks.Register(hooks...)
		return nil
	}
}

func withLifecycleBeforeStartHooks(hooks ...func(context.Context) error) lifecycleOption {
	return withLifecycleHook(lifecycleStageBeforeStart, hooks...)
}

func withLifecycleBeforeStopHooks(hooks ...func(context.Context) error) lifecycleOption {
	return withLifecycleHook(lifecycleStageBeforeStop, hooks...)
}

func withLifecycleAfterStopHooks(hooks ...func(context.Context) error) lifecycleOption {
	return withLifecycleHook(lifecycleStageAfterStop, hooks...)
}

func withLifecycleRegistry(reg registry.Registry) lifecycleOption {
	return func(runner *lifecycleRunner) error {
		runner.registry = reg
		return nil
	}
}

func withLifecycleShutdownTimeout(timeout time.Duration) lifecycleOption {
	return func(runner *lifecycleRunner) error {
		runner.shutdownTimeout = timeout
		return nil
	}
}

func withLifecycleServer(srv server.Server) lifecycleOption {
	return func(runner *lifecycleRunner) error {
		runner.server = srv
		return nil
	}
}

func withLifecycleGovernor(srv *governor.Server) lifecycleOption {
	return func(runner *lifecycleRunner) error {
		runner.governor = srv
		return nil
	}
}

func withLifecycleInternalServers(servers ...InternalServer) lifecycleOption {
	return func(runner *lifecycleRunner) error {
		runner.internalServers = append(runner.internalServers, servers...)
		return nil
	}
}

func withLifecycleCleanup(name string, fn func(context.Context) error) lifecycleOption {
	return withLifecycleHook(lifecycleStageCleanup, func(ctx context.Context) error {
		err := fn(ctx)
		if err == nil || name == "" {
			return err
		}
		return fmt.Errorf("cleanup %s: %w", name, err)
	})
}

type lifecycleRunner struct {
	runOnce  sync.Once
	stopOnce sync.Once

	mu sync.Mutex

	optionsMu sync.RWMutex
	running   bool

	server server.Server

	governor *governor.Server

	internalServers []InternalServer

	registryState int
	registry      registry.Registry

	shutdownTimeout time.Duration

	hooks map[lifecycleStage]*defers.Defer
}

func (runner *lifecycleRunner) setRunning(running bool) {
	runner.optionsMu.Lock()
	defer runner.optionsMu.Unlock()
	runner.running = running
}

func newLifecycleRunner(opts ...lifecycleOption) (*lifecycleRunner, error) {
	runner := &lifecycleRunner{
		hooks: map[lifecycleStage]*defers.Defer{},
	}
	for stage := lifecycleStage(1); stage < lifecycleStageMax; stage++ {
		runner.hooks[stage] = defers.NewDefer()
	}
	for _, opt := range opts {
		if err := opt(runner); err != nil {
			return nil, err
		}
	}
	return runner, nil
}

func (runner *lifecycleRunner) Init(opts ...lifecycleOption) error {
	runner.optionsMu.Lock()
	defer runner.optionsMu.Unlock()
	if runner.running {
		slog.Warn("the application has been started, and the settings are no longer applied")
		return nil
	}
	for _, opt := range opts {
		if err := opt(runner); err != nil {
			return err
		}
	}
	return nil
}

func (runner *lifecycleRunner) Stop() error {
	var err error
	runner.stopOnce.Do(func() {
		runner.setRunning(false)

		ctx, cancel := context.WithTimeout(context.Background(), runner.stopTimeout())
		defer cancel()

		err = runner.runStopSequence(ctx)
	})
	return err
}

func (runner *lifecycleRunner) runStopSequence(ctx context.Context) error {
	var err error
	err = errors.Join(err, runner.runHooks(ctx, lifecycleStageBeforeStop))
	err = errors.Join(err, runner.deregister(ctx))
	err = errors.Join(err, runner.stopServers(ctx))
	err = errors.Join(err, runner.runHooks(ctx, lifecycleStageCleanup))
	err = errors.Join(err, runner.runHooks(ctx, lifecycleStageAfterStop))
	return err
}

func (runner *lifecycleRunner) Run() error {
	if runner.governor == nil {
		return errGovernorRequired
	}

	var err error
	runner.runOnce.Do(func() {
		cleanupSignals := runner.waitSignals()
		defer cleanupSignals()

		runner.setRunning(true)

		if err = runner.startServers(); err != nil {
			return
		}
		slog.Info("app shutdown")
	})

	return err
}

func (runner *lifecycleRunner) runHooks(ctx context.Context, stage lifecycleStage) error {
	hooks, ok := runner.hooks[stage]
	if !ok {
		return nil
	}
	return hooks.Done(ctx)
}

func (runner *lifecycleRunner) newStopAsyncOnFailure() func() {
	var stopOnce sync.Once
	return func() {
		stopOnce.Do(func() {
			go func() {
				if err := runner.Stop(); err != nil {
					slog.Error(
						"fault to stop application after serve failure",
						slog.Any("error", err),
					)
				}
			}()
		})
	}
}

func (runner *lifecycleRunner) startManagedServer(
	group *errgroup.Group,
	stopAsync func(),
	name string,
	serve func() error,
) {
	group.Go(func() error {
		if err := serve(); err != nil {
			stopAsync()
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	})
}

func (runner *lifecycleRunner) registerAfterStartup(
	serverStartedCh <-chan struct{},
	stopAsync func(),
) error {
	if _, ok := <-serverStartedCh; !ok {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := runner.governor.WaitStarted(ctx); err != nil {
		stopAsync()
		return fmt.Errorf("wait governor startup: %w", err)
	}
	if err := runner.register(); err != nil {
		stopAsync()
		return fmt.Errorf("register application: %w", err)
	}
	return nil
}

func (runner *lifecycleRunner) startServers() error {
	if runner.governor == nil {
		return errGovernorRequired
	}
	if err := runner.runHooks(context.Background(), lifecycleStageBeforeStart); err != nil {
		return err
	}

	var group errgroup.Group
	serverStartedCh := make(chan struct{}, 1)
	stopAsync := runner.newStopAsyncOnFailure()

	if runner.server != nil {
		runner.startManagedServer(&group, stopAsync, "main server", func() error {
			return runner.server.Serve(serverStartedCh)
		})
	} else {
		serverStartedCh <- struct{}{}
	}

	runner.startManagedServer(&group, stopAsync, "governor", runner.governor.Serve)

	for _, item := range runner.internalServers {
		internalServer := item
		runner.startManagedServer(&group, stopAsync, "internal server", internalServer.Serve)
	}

	group.Go(func() error {
		return runner.registerAfterStartup(serverStartedCh, stopAsync)
	})

	return group.Wait()
}

func attrsToAny(attrs ...slog.Attr) []any {
	args := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		args = append(args, attr)
	}
	return args
}

func stopManagedComponent(
	ctx context.Context,
	name string,
	stop func(context.Context) error,
	attrs ...slog.Attr,
) error {
	slog.Info("stopping "+name, attrsToAny(attrs...)...)
	if err := stop(ctx); err != nil {
		errAttrs := make([]slog.Attr, len(attrs), len(attrs)+1)
		copy(errAttrs, attrs)
		errAttrs = append(errAttrs, slog.Any("error", err))
		slog.Error("failed to stop "+name, attrsToAny(errAttrs...)...)
		return err
	}
	slog.Info(name+" stopped", attrsToAny(attrs...)...)
	return nil
}

func (runner *lifecycleRunner) stopServers(ctx context.Context) error {
	slog.Info("stopping servers")

	var group errgroup.Group
	if runner.server != nil {
		group.Go(func() error {
			return stopManagedComponent(ctx, "main server", runner.server.Stop)
		})
	}

	for index, item := range runner.internalServers {
		index := index
		internalServer := item
		group.Go(func() error {
			return stopManagedComponent(
				ctx,
				"internal server",
				internalServer.Stop,
				slog.Int("index", index),
			)
		})
	}

	if runner.governor != nil {
		group.Go(func() error {
			return stopManagedComponent(ctx, "governor", runner.governor.Shutdown)
		})
	}

	if err := group.Wait(); err != nil {
		return fmt.Errorf("error stopping servers: %w", err)
	}
	slog.Info("all servers stopped successfully")
	return nil
}

func (runner *lifecycleRunner) stopTimeout() time.Duration {
	if runner.shutdownTimeout <= 0 {
		return defaultShutdownTimeout
	}
	return runner.shutdownTimeout
}

func (runner *lifecycleRunner) signalShutdownTimeout() time.Duration {
	if runner.shutdownTimeout < defaultShutdownTimeout {
		return defaultShutdownTimeout
	}
	return runner.shutdownTimeout
}

func (runner *lifecycleRunner) waitSignals() func() {
	signalsCh := make(chan os.Signal, 2)
	done := make(chan struct{})
	var cleanupOnce sync.Once
	signal.Notify(signalsCh, shutdownSignals...)

	cleanup := func() {
		cleanupOnce.Do(func() {
			signal.Stop(signalsCh)
			close(done)
		})
	}

	go func() {
		select {
		case <-done:
			return
		case sig := <-signalsCh:
			go func() {
				if err := runner.Stop(); err != nil {
					slog.Error("fault to stop", slog.Any("error", err))
				}
			}()

			timer := time.NewTimer(runner.signalShutdownTimeout())
			defer timer.Stop()

			select {
			case <-done:
				return
			case <-timer.C:
				if signalValue, ok := sig.(syscall.Signal); ok {
					os.Exit(128 + int(signalValue))
				}
				os.Exit(1)
			}
		}
	}()

	return cleanup
}
