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

package lifecycle

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

// InternalServer is managed by the lifecycle runner alongside the main server.
type InternalServer interface {
	Serve() error
	Stop(context.Context) error
}

// Stage identifies one lifecycle hook stage.
type Stage uint32

// Lifecycle hook stages.
const (
	_ Stage = iota
	StageBeforeStart
	StageBeforeStop
	StageCleanup
	StageAfterStop
	StageMax
)

const defaultShutdownTimeout = 30 * time.Second

const (
	registryStateInit = iota
	registryStateRegistering
	registryStateDone
	registryStateCancel
)

type endpoint struct {
	address  string
	scheme   string
	metadata map[string]string
}

func (e endpoint) Scheme() string {
	return e.scheme
}

func (e endpoint) Address() string {
	return e.address
}

func (e endpoint) Metadata() map[string]string {
	return e.metadata
}

var errGovernorRequired = errors.New("application governor is required")

// Option mutates one runner before startup.
type Option func(*Runner) error

// WithHook registers hook functions for one stage.
func WithHook(stage Stage, hooks ...func(context.Context) error) Option {
	return func(runner *Runner) error {
		stageHooks, ok := runner.hooks[stage]
		if !ok {
			return fmt.Errorf("hook stage not found")
		}
		stageHooks.Register(hooks...)
		return nil
	}
}

// WithBeforeStartHooks registers before-start hooks.
func WithBeforeStartHooks(hooks ...func(context.Context) error) Option {
	return WithHook(StageBeforeStart, hooks...)
}

// WithBeforeStopHooks registers before-stop hooks.
func WithBeforeStopHooks(hooks ...func(context.Context) error) Option {
	return WithHook(StageBeforeStop, hooks...)
}

// WithAfterStopHooks registers after-stop hooks.
func WithAfterStopHooks(hooks ...func(context.Context) error) Option {
	return WithHook(StageAfterStop, hooks...)
}

// WithRegistry configures the service registry.
func WithRegistry(reg registry.Registry) Option {
	return func(runner *Runner) error {
		runner.registry = reg
		return nil
	}
}

// WithShutdownTimeout configures the shutdown timeout.
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(runner *Runner) error {
		runner.shutdownTimeout = timeout
		return nil
	}
}

// WithServer configures the main application server.
func WithServer(srv server.Server) Option {
	return func(runner *Runner) error {
		runner.server = srv
		return nil
	}
}

// WithGovernor configures the governor server.
func WithGovernor(srv *governor.Server) Option {
	return func(runner *Runner) error {
		runner.governor = srv
		return nil
	}
}

// WithInternalServers registers internal servers managed by the lifecycle.
func WithInternalServers(servers ...InternalServer) Option {
	return func(runner *Runner) error {
		runner.internalServers = append(runner.internalServers, servers...)
		return nil
	}
}

// WithCleanup registers one named cleanup hook.
func WithCleanup(name string, fn func(context.Context) error) Option {
	return WithHook(StageCleanup, func(ctx context.Context) error {
		err := fn(ctx)
		if err == nil || name == "" {
			return err
		}
		return fmt.Errorf("cleanup %s: %w", name, err)
	})
}

// Runner orchestrates startup, registration, and shutdown sequencing.
type Runner struct {
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

	hooks map[Stage]*defers.Defer
}

func (runner *Runner) setRunning(running bool) {
	runner.optionsMu.Lock()
	defer runner.optionsMu.Unlock()
	runner.running = running
}

// New creates one lifecycle runner.
func New(opts ...Option) (*Runner, error) {
	runner := &Runner{
		hooks: map[Stage]*defers.Defer{},
	}
	for stage := Stage(1); stage < StageMax; stage++ {
		runner.hooks[stage] = defers.NewDefer()
	}
	for _, opt := range opts {
		if err := opt(runner); err != nil {
			return nil, err
		}
	}
	return runner, nil
}

// Init applies lifecycle options before startup.
func (runner *Runner) Init(opts ...Option) error {
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

// Stop stops the lifecycle exactly once.
func (runner *Runner) Stop() error {
	var err error
	runner.stopOnce.Do(func() {
		runner.setRunning(false)

		ctx, cancel := context.WithTimeout(context.Background(), runner.stopTimeout())
		defer cancel()

		err = runner.runStopSequence(ctx)
	})
	return err
}

func (runner *Runner) runStopSequence(ctx context.Context) error {
	var err error
	err = errors.Join(err, runner.runHooks(ctx, StageBeforeStop))
	err = errors.Join(err, runner.deregister(ctx))
	err = errors.Join(err, runner.stopServers(ctx))
	err = errors.Join(err, runner.runHooks(ctx, StageCleanup))
	err = errors.Join(err, runner.runHooks(ctx, StageAfterStop))
	return err
}

// Run starts the configured lifecycle.
func (runner *Runner) Run() error {
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

func (runner *Runner) runHooks(ctx context.Context, stage Stage) error {
	hooks, ok := runner.hooks[stage]
	if !ok {
		return nil
	}
	return hooks.Done(ctx)
}

func (runner *Runner) newStopAsyncOnFailure() func() {
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

func (runner *Runner) startManagedServer(
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

func (runner *Runner) registerAfterStartup(
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

func (runner *Runner) startServers() error {
	if runner.governor == nil {
		return errGovernorRequired
	}
	if err := runner.runHooks(context.Background(), StageBeforeStart); err != nil {
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

func (runner *Runner) stopServers(ctx context.Context) error {
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

func (runner *Runner) stopTimeout() time.Duration {
	if runner.shutdownTimeout <= 0 {
		return defaultShutdownTimeout
	}
	return runner.shutdownTimeout
}

func (runner *Runner) signalShutdownTimeout() time.Duration {
	if runner.shutdownTimeout < defaultShutdownTimeout {
		return defaultShutdownTimeout
	}
	return runner.shutdownTimeout
}

func (runner *Runner) waitSignals() func() {
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
