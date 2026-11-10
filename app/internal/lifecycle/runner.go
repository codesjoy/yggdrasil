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
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil/v3/admin/governor"
	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/internal/defers"
	internalidentity "github.com/codesjoy/yggdrasil/v3/internal/identity"
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

var (
	errGovernorRequired = errors.New("application governor is required")
	errRunnerStopped    = errors.New("lifecycle runner is stopped")
)

type runnerState uint32

const (
	runnerStateIdle runnerState = iota
	runnerStateStarting
	runnerStateRunning
	runnerStateStopping
	runnerStateStopped
)

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

// WithLogger configures the lifecycle logger.
func WithLogger(logger *slog.Logger) Option {
	return func(runner *Runner) error {
		runner.logger = logger
		return nil
	}
}

// WithIdentity configures the App-local identity used by registration and
// lifecycle diagnostics.
func WithIdentity(identity internalidentity.Identity) Option {
	return func(runner *Runner) error {
		identity.Metadata = identity.MetadataCopy()
		runner.identity = identity
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
	stopOnce sync.Once

	mu    sync.Mutex
	state runnerState

	startDone   chan struct{}
	startErr    error
	startCancel context.CancelFunc
	stopDone    chan struct{}
	stopErr     error

	serveWG       sync.WaitGroup
	serveDone     chan struct{}
	serveDoneOnce sync.Once
	runErr        error

	optionsMu sync.RWMutex
	running   bool

	server server.Server

	governor *governor.Server

	internalServers []InternalServer

	registryState int
	registry      registry.Registry
	identity      internalidentity.Identity

	shutdownTimeout time.Duration
	logger          *slog.Logger

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
		hooks:     map[Stage]*defers.Defer{},
		stopDone:  make(chan struct{}),
		serveDone: make(chan struct{}),
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
		runner.loggerOrDefault().Warn(
			"the application has been started, and the settings are no longer applied",
		)
		return nil
	}
	for _, opt := range opts {
		if err := opt(runner); err != nil {
			return err
		}
	}
	return nil
}

func (runner *Runner) loggerOrDefault() *slog.Logger {
	if runner != nil && runner.logger != nil {
		return runner.logger
	}
	return slog.Default()
}

func (runner *Runner) withStopTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, runner.stopTimeout())
}

func (runner *Runner) runStopSequence(ctx context.Context) error {
	var err error
	err = errors.Join(err, runner.runHooks(ctx, StageBeforeStop))
	err = errors.Join(err, runner.deregister(ctx))
	err = errors.Join(err, runner.stopServers(ctx))
	err = errors.Join(err, runner.waitForServeLoops(ctx))
	err = errors.Join(err, runner.runHooks(ctx, StageCleanup))
	err = errors.Join(err, runner.runHooks(ctx, StageAfterStop))
	return err
}

func (runner *Runner) waitForServeLoops(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		runner.serveWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Start starts the configured lifecycle and waits for startup to complete.
func (runner *Runner) Start(ctx context.Context) error {
	if runner.governor == nil {
		return errGovernorRequired
	}
	if ctx == nil {
		ctx = context.Background()
	}

	runner.mu.Lock()
	switch runner.state {
	case runnerStateIdle:
		runner.state = runnerStateStarting
		runner.startDone = make(chan struct{})
		runner.startErr = nil
		runner.runErr = nil
		startCtx, cancel := context.WithCancel(ctx)
		runner.startCancel = cancel
		runner.mu.Unlock()
		return runner.startAndMonitor(ctx, startCtx)
	case runnerStateStarting:
		startDone := runner.startDone
		runner.mu.Unlock()
		<-startDone
		runner.mu.Lock()
		defer runner.mu.Unlock()
		return runner.startErr
	case runnerStateRunning:
		runner.mu.Unlock()
		return nil
	case runnerStateStopping, runnerStateStopped:
		stopDone := runner.stopDone
		runner.mu.Unlock()
		<-stopDone
		return errRunnerStopped
	default:
		runner.mu.Unlock()
		return errRunnerStopped
	}
}

func (runner *Runner) startAndMonitor(ctx, startCtx context.Context) error {
	err := runner.startSequence(startCtx)

	runner.mu.Lock()
	startCancel := runner.startCancel
	runner.startCancel = nil
	stopRequested := runner.state == runnerStateStopping
	if err == nil && stopRequested {
		err = context.Canceled
	}
	runner.startErr = err
	if err == nil {
		runner.state = runnerStateRunning
		runner.setRunning(true)
	} else {
		runner.state = runnerStateStopping
		runner.setRunning(false)
	}
	close(runner.startDone)
	runner.mu.Unlock()

	if startCancel != nil {
		startCancel()
	}
	if err != nil {
		return errors.Join(err, runner.Stop(context.WithoutCancel(ctx)))
	}
	shutdownCtx := context.WithoutCancel(ctx)
	go func() {
		<-runner.serveDone
		if stopErr := runner.Stop(shutdownCtx); stopErr != nil {
			runner.loggerOrDefault().Error(
				"failed to stop after server exit",
				slog.Any("error", stopErr),
			)
		}
	}()
	return nil
}

// Wait blocks until shutdown has fully completed.
func (runner *Runner) Wait() error {
	runner.mu.Lock()
	if runner.state == runnerStateIdle {
		runner.mu.Unlock()
		_ = runner.Stop(context.Background())
		runner.mu.Lock()
	}
	stopDone := runner.stopDone
	runner.mu.Unlock()

	<-stopDone
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return errors.Join(runner.runErr, runner.stopErr)
}

// Stop stops the lifecycle exactly once and caches the first result.
func (runner *Runner) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	runner.mu.Lock()
	var startDone <-chan struct{}
	var cancel context.CancelFunc
	switch runner.state {
	case runnerStateIdle:
		runner.state = runnerStateStopping
	case runnerStateStarting:
		runner.state = runnerStateStopping
		startDone = runner.startDone
		cancel = runner.startCancel
	case runnerStateRunning:
		runner.state = runnerStateStopping
	case runnerStateStopping, runnerStateStopped:
	}
	runner.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if startDone != nil {
		select {
		case <-startDone:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	runner.stopOnce.Do(func() {
		runner.setRunning(false)
		stopCtx, cancelStop := runner.withStopTimeout(ctx)
		defer cancelStop()

		err := runner.runStopSequence(stopCtx)
		runner.mu.Lock()
		runner.stopErr = err
		runner.state = runnerStateStopped
		close(runner.stopDone)
		runner.mu.Unlock()
	})

	<-runner.stopDone
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return runner.stopErr
}

// Run starts the configured lifecycle and blocks until shutdown completes.
func (runner *Runner) Run(ctx context.Context) error {
	if err := runner.Start(ctx); err != nil {
		return err
	}
	return runner.Wait()
}

func (runner *Runner) startSequence(ctx context.Context) error {
	if runner.governor == nil {
		return errGovernorRequired
	}
	if err := runner.runHooks(ctx, StageBeforeStart); err != nil {
		return err
	}

	mainStarted := make(chan struct{}, 1)
	mainResult := make(chan error, 1)
	if runner.server != nil {
		runner.launchServe("main server", func() error {
			return runner.server.Serve(mainStarted)
		}, mainResult)
	} else {
		mainStarted <- struct{}{}
		mainResult <- nil
	}
	runner.launchServe("governor", runner.governor.Serve, nil)
	for index, item := range runner.internalServers {
		internalServer := item
		runner.launchServe(fmt.Sprintf("internal server[%d]", index), internalServer.Serve, nil)
	}

	if err := runner.waitForMainServer(ctx, mainStarted, mainResult); err != nil {
		return err
	}
	if err := runner.waitForGovernor(ctx); err != nil {
		return err
	}
	if err := runner.registerWithContext(ctx); err != nil {
		return fmt.Errorf("register application: %w", err)
	}
	return nil
}

func (runner *Runner) runHooks(ctx context.Context, stage Stage) error {
	hooks, ok := runner.hooks[stage]
	if !ok {
		return nil
	}
	return hooks.Done(ctx)
}

func (runner *Runner) launchServe(
	name string,
	serve func() error,
	result chan<- error,
) {
	runner.serveWG.Go(func() {
		err := serve()
		if result != nil {
			result <- err
		}
		if err != nil {
			runner.noteServeExit(fmt.Errorf("%s: %w", name, err))
		}
	})
}

func (runner *Runner) noteServeExit(err error) {
	runner.mu.Lock()
	switch runner.state {
	case runnerStateStarting, runnerStateRunning:
		runner.runErr = errors.Join(runner.runErr, err)
	}
	runner.mu.Unlock()
	runner.serveDoneOnce.Do(func() {
		close(runner.serveDone)
	})
}

func (runner *Runner) waitForMainServer(
	ctx context.Context,
	started <-chan struct{},
	result <-chan error,
) error {
	select {
	case _, ok := <-started:
		if ok {
			return nil
		}
		select {
		case err := <-result:
			if err == nil {
				return errors.New("main server exited during startup")
			}
			return fmt.Errorf("main server startup failed: %w", err)
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (runner *Runner) waitForGovernor(ctx context.Context) error {
	if err := runner.governor.WaitStarted(ctx); err != nil {
		return fmt.Errorf("wait governor startup: %w", err)
	}
	return nil
}

func attrsToAny(attrs ...slog.Attr) []any {
	args := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		args = append(args, attr)
	}
	return args
}

func (runner *Runner) stopManagedComponent(
	ctx context.Context,
	name string,
	stop func(context.Context) error,
	attrs ...slog.Attr,
) error {
	runner.loggerOrDefault().Info("stopping "+name, attrsToAny(attrs...)...)
	if err := stop(ctx); err != nil {
		errAttrs := make([]slog.Attr, len(attrs), len(attrs)+1)
		copy(errAttrs, attrs)
		errAttrs = append(errAttrs, slog.Any("error", err))
		runner.loggerOrDefault().Error("failed to stop "+name, attrsToAny(errAttrs...)...)
		return err
	}
	runner.loggerOrDefault().Info(name+" stopped", attrsToAny(attrs...)...)
	return nil
}

func (runner *Runner) stopServers(ctx context.Context) error {
	runner.loggerOrDefault().Info("stopping servers")

	type stopTarget struct {
		name  string
		stop  func(context.Context) error
		attrs []slog.Attr
	}
	targets := make([]stopTarget, 0, len(runner.internalServers)+2)
	if runner.server != nil {
		targets = append(targets, stopTarget{
			name: "main server",
			stop: runner.server.Stop,
		})
	}

	for index, item := range runner.internalServers {
		targets = append(targets, stopTarget{
			name:  "internal server",
			stop:  item.Stop,
			attrs: []slog.Attr{slog.Int("index", index)},
		})
	}

	if runner.governor != nil {
		targets = append(targets, stopTarget{
			name: "governor",
			stop: runner.governor.Shutdown,
		})
	}

	errs := make([]error, len(targets))
	var wg sync.WaitGroup
	for index := range targets {
		target := targets[index]
		wg.Go(func() {
			errs[index] = runner.stopManagedComponent(
				ctx,
				target.name,
				target.stop,
				target.attrs...,
			)
		})
	}
	wg.Wait()

	var err error
	for _, stopErr := range errs {
		err = errors.Join(err, stopErr)
	}
	if err != nil {
		return fmt.Errorf("error stopping servers: %w", err)
	}
	runner.loggerOrDefault().Info("all servers stopped successfully")
	return nil
}

func (runner *Runner) stopTimeout() time.Duration {
	if runner.shutdownTimeout <= 0 {
		return defaultShutdownTimeout
	}
	return runner.shutdownTimeout
}
