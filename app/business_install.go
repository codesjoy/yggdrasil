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
	"net/http"
	"slices"

	internalinstall "github.com/codesjoy/yggdrasil/v3/app/internal/install"
	internallifecycle "github.com/codesjoy/yggdrasil/v3/app/internal/lifecycle"
	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

// Runtime returns the prepared runtime surface.
func (a *App) Runtime() Runtime {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state < lifecycleStateInitialized || a.state == lifecycleStateStopped {
		return nil
	}
	return a.runtime
}

// Snapshot returns a detached copy of the prepared runtime snapshot.
func (a *App) Snapshot() *Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state < lifecycleStateInitialized || a.state == lifecycleStateStopped {
		return nil
	}
	return a.currentRuntimeSnapshot().Copy()
}

// Prepare moves the app to RuntimeReady without listening or serving.
func (a *App) Prepare(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.initializeLocked(ctx)
}

// Compose executes one business composition callback against the prepared runtime.
// If Prepare has not been called yet, Compose performs a lazy prepare first.
func (a *App) Compose(ctx context.Context, fn ComposeFunc) (*BusinessBundle, error) {
	if err := validateComposeInput(ctx, fn); err != nil {
		return nil, a.recordComposeError(err)
	}
	rt, err := a.prepareComposeRuntime(ctx)
	if err != nil {
		return nil, a.recordComposeError(err)
	}
	bundle, err := fn(rt)
	if err != nil {
		err = wrapAssemblyStageError("compose", err)
		a.mu.Lock()
		a.recordAssemblyErrorLocked(assemblyStageCompose, err)
		a.mu.Unlock()
		return nil, a.failBeforeStart(
			err,
			"compose",
			yassembly.ErrPreparedRuntimeRollbackFailed,
		)
	}
	if bundle == nil {
		bundle = &BusinessBundle{}
	}
	a.mu.Lock()
	a.clearAssemblyErrorLocked(assemblyStageCompose)
	a.mu.Unlock()
	return bundle, nil
}

func validateComposeInput(ctx context.Context, fn ComposeFunc) error {
	if fn == nil {
		return yassembly.NewError(
			yassembly.ErrComposeFailed,
			"compose",
			"compose function is nil",
			nil,
			nil,
		)
	}
	if ctx == nil {
		return yassembly.NewError(
			yassembly.ErrComposeFailed,
			"compose",
			"compose context is nil",
			nil,
			nil,
		)
	}
	return nil
}

func (a *App) recordComposeError(err error) error {
	a.mu.Lock()
	a.recordAssemblyErrorLocked(assemblyStageCompose, err)
	a.mu.Unlock()
	return err
}

func (a *App) prepareComposeRuntime(ctx context.Context) (Runtime, error) {
	if err := a.Prepare(ctx); err != nil {
		return nil, err
	}
	a.mu.Lock()
	if a.state >= lifecycleStateServing || a.state == lifecycleStateStopped {
		err := yassembly.NewError(
			yassembly.ErrComposeFailed,
			"compose",
			"compose is only allowed before start",
			nil,
			nil,
		)
		a.mu.Unlock()
		return nil, err
	}
	a.mu.Unlock()
	rt := a.Runtime()
	if rt == nil {
		err := yassembly.NewError(
			yassembly.ErrRuntimeSurfaceUnavailable,
			"compose",
			"runtime surface is not available",
			nil,
			nil,
		)
		return nil, err
	}
	return rt, nil
}

// ComposeAndInstall composes a bundle and installs it in one step.
func (a *App) ComposeAndInstall(ctx context.Context, fn ComposeFunc) error {
	bundle, err := a.Compose(ctx, fn)
	if err != nil {
		return err
	}
	return a.InstallBusiness(bundle)
}

// InstallBusiness installs one composed business bundle.
func (a *App) InstallBusiness(bundle *BusinessBundle) error {
	if err := a.beginBusinessInstall(); err != nil {
		return err
	}
	defer a.endBusinessInstall()
	if err := a.validateBusinessInstall(); err != nil {
		return err
	}

	err := a.installBundle(bundle)
	if err == nil {
		a.markBusinessInstalled()
		return nil
	}
	return a.recordBusinessInstallFailure(err)
}

func (a *App) beginBusinessInstall() error {
	if a == nil {
		return internalinstall.ValidationError("app is not ready", nil)
	}
	if a.installing.Load() {
		return yassembly.NewError(
			yassembly.ErrInstallRegistrationConflict,
			"install",
			"business bundle installation is already in progress",
			nil,
			nil,
		)
	}

	a.installMu.Lock()
	a.installing.Store(true)
	return nil
}

func (a *App) endBusinessInstall() {
	a.installing.Store(false)
	a.installMu.Unlock()
}

func (a *App) validateBusinessInstall() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.explicitBundleInstalled {
		err := yassembly.NewError(
			yassembly.ErrInstallRegistrationConflict,
			"install",
			"business bundle already installed",
			nil,
			nil,
		)
		a.recordAssemblyErrorLocked(assemblyStageInstall, err)
		return err
	}
	if a.state < lifecycleStateInitialized || a.state >= lifecycleStateServing ||
		a.state == lifecycleStateStopped {
		err := yassembly.NewError(
			yassembly.ErrRuntimeNotReady,
			"install",
			"runtime is not ready",
			nil,
			nil,
		)
		a.recordAssemblyErrorLocked(assemblyStageInstall, err)
		return err
	}
	return nil
}

func (a *App) markBusinessInstalled() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.explicitBundleInstalled = true
	a.clearAssemblyErrorLocked(assemblyStageInstall)
	if a.state < lifecycleStateBusinessInstalled {
		a.state = lifecycleStateBusinessInstalled
	}
}

func (a *App) recordBusinessInstallFailure(err error) error {
	err = internalinstall.WrapError(err)
	a.mu.Lock()
	a.recordAssemblyErrorLocked(assemblyStageInstall, err)
	a.mu.Unlock()
	return a.failBeforeStart(err, "install", yassembly.ErrPartialInstallRollbackFailed)
}

func (a *App) installBundle(bundle *BusinessBundle) error {
	if bundle == nil {
		return nil
	}
	staged := newStagedInstall(a)
	if err := stageBundleBindings(staged, bundle); err != nil {
		return err
	}
	extensions, err := installBundleExtensions(staged, bundle.Extensions)
	if err != nil {
		return err
	}
	staged.diagnostics = append(staged.diagnostics, bundle.Diagnostics...)
	if err = staged.commit(); err != nil {
		if rollbackErr := staged.rollbackExtensions(extensions); rollbackErr != nil {
			return internalinstall.ValidationError(
				"install commit and rollback failed",
				errors.Join(err, rollbackErr),
			)
		}
		return err
	}
	return nil
}

func stageBundleBindings(staged *stagedInstall, bundle *BusinessBundle) error {
	for _, item := range bundle.RPCBindings {
		if err := staged.installRPCBinding(item); err != nil {
			return err
		}
	}
	for _, item := range bundle.RESTBindings {
		if err := staged.installRESTBinding(item); err != nil {
			return err
		}
	}
	for _, item := range bundle.RawHTTP {
		if err := staged.installRawHTTPBinding(item); err != nil {
			return err
		}
	}
	for _, item := range bundle.GovernorHTTP {
		if err := staged.installGovernorHTTPBinding(item); err != nil {
			return err
		}
	}
	for _, item := range bundle.Tasks {
		if err := staged.addBackgroundTask(item); err != nil {
			return err
		}
	}
	for _, item := range bundle.Hooks {
		if err := staged.addBusinessHook(item); err != nil {
			return err
		}
	}
	return nil
}

func installBundleExtensions(
	staged *stagedInstall,
	items []BusinessInstallable,
) ([]BusinessInstallable, error) {
	extensions := make([]BusinessInstallable, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		extensions = append(extensions, item)
		ctx := &InstallContext{
			Runtime:   staged.app.runtime,
			installer: staged,
		}
		if err := item.Install(ctx); err != nil {
			installErr := internalinstall.ValidationError(
				fmt.Sprintf("install extension %q: %v", item.Kind(), err),
				err,
			)
			if rollbackErr := staged.rollbackExtensions(extensions); rollbackErr != nil {
				return nil, internalinstall.ValidationError(
					fmt.Sprintf("install extension %q rollback failed", item.Kind()),
					errors.Join(installErr, rollbackErr),
				)
			}
			return nil, installErr
		}
	}
	return extensions, nil
}

func (a *App) installOptions() (*options, error) {
	if a == nil || a.opts == nil {
		return nil, internalinstall.ValidationError("app is not ready", nil)
	}
	return a.opts, nil
}

func (a *App) installServer(bindingType string) (server.Server, error) {
	if _, err := a.installOptions(); err != nil {
		return nil, err
	}
	if a.opts.server == nil {
		if err := a.initServer(); err != nil {
			return nil, internalinstall.ValidationError(err.Error(), err)
		}
	}
	if a.opts.server == nil {
		return nil, internalinstall.ValidationError(
			fmt.Sprintf("server is not available for %s bindings", bindingType),
			nil,
		)
	}
	return a.opts.server, nil
}

type stagedRPCBinding struct {
	desc *server.ServiceDesc
	name string
	impl any
}

type stagedRESTBinding struct {
	desc     *server.RestServiceDesc
	impl     any
	prefixes []string
	routes   []string
}

type stagedRawHTTPBinding struct {
	desc *server.RestRawHandlerDesc
}

type stagedGovernorHTTPBinding struct {
	method  string
	path    string
	handler http.HandlerFunc
}

type stagedInstall struct {
	app *App
	svr server.Server

	rpcBindings      []stagedRPCBinding
	restBindings     []stagedRESTBinding
	rawHTTPBindings  []stagedRawHTTPBinding
	governorBindings []stagedGovernorHTTPBinding
	tasks            []BackgroundTask
	hooks            []BusinessHook
	diagnostics      []BundleDiag

	rpcServices    map[string]struct{}
	httpRoutes     map[string]struct{}
	governorRoutes map[string]struct{}
}

func newStagedInstall(app *App) *stagedInstall {
	staged := &stagedInstall{
		app:            app,
		rpcServices:    map[string]struct{}{},
		httpRoutes:     map[string]struct{}{},
		governorRoutes: map[string]struct{}{},
	}
	if app != nil {
		app.mu.Lock()
		for key := range app.installedRPCServices {
			staged.rpcServices[key] = struct{}{}
		}
		for key := range app.installedHTTPRoutes {
			staged.httpRoutes[key] = struct{}{}
		}
		for key := range app.installedGovernorRoutes {
			staged.governorRoutes[key] = struct{}{}
		}
		if app.opts != nil {
			staged.svr = app.opts.server
		}
		app.mu.Unlock()
	}
	return staged
}

func (s *stagedInstall) installRPCBinding(binding RPCBinding) error {
	opts, err := s.app.installOptions()
	if err != nil {
		return err
	}
	desc, serviceName, err := internalinstall.ValidateRPCBinding(
		len(opts.resolvedSettings.Server.Transports) > 0,
		binding.ServiceName,
		binding.Desc,
		binding.Impl,
	)
	if err != nil {
		return err
	}
	if _, exists := s.rpcServices[desc.ServiceName]; exists {
		return internalinstall.ConflictError(
			fmt.Sprintf("rpc service %q already installed", serviceName),
			nil,
		)
	}
	if err := s.ensureServer("rpc"); err != nil {
		return err
	}
	s.rpcServices[desc.ServiceName] = struct{}{}
	s.rpcBindings = append(s.rpcBindings, stagedRPCBinding{
		desc: desc,
		name: serviceName,
		impl: binding.Impl,
	})
	return nil
}

func (s *stagedInstall) installRESTBinding(binding RESTBinding) error {
	opts, err := s.app.installOptions()
	if err != nil {
		return err
	}
	desc, _, err := internalinstall.ValidateRESTBinding(
		opts.resolvedSettings.Server.RestEnabled,
		binding.Name,
		binding.Desc,
		binding.Impl,
	)
	if err != nil {
		return err
	}
	prefix := internalinstall.BuildRESTRoutePrefix(binding.Prefixes)
	routes := make([]string, 0, len(desc.Methods))
	for _, method := range desc.Methods {
		route := prefix + method.Path
		key := internalinstall.RouteKey(method.Method, route)
		if _, exists := s.httpRoutes[key]; exists {
			return internalinstall.ConflictError(
				fmt.Sprintf("rest route %s %s already installed", method.Method, route),
				nil,
			)
		}
		s.httpRoutes[key] = struct{}{}
		routes = append(routes, key)
	}
	if err := s.ensureServer("rest"); err != nil {
		return err
	}
	s.restBindings = append(s.restBindings, stagedRESTBinding{
		desc:     desc,
		impl:     binding.Impl,
		prefixes: append([]string(nil), binding.Prefixes...),
		routes:   routes,
	})
	return nil
}

func (s *stagedInstall) installRawHTTPBinding(binding RawHTTPBinding) error {
	opts, err := s.app.installOptions()
	if err != nil {
		return err
	}
	desc, err := internalinstall.ValidateRawHTTPBinding(
		opts.resolvedSettings.Server.RestEnabled,
		binding.Desc,
		binding.Method,
		binding.Path,
		binding.Handler,
	)
	if err != nil {
		return err
	}
	key := internalinstall.RouteKey(desc.Method, desc.Path)
	if _, exists := s.httpRoutes[key]; exists {
		return internalinstall.ConflictError(
			fmt.Sprintf("raw http route %s %s already installed", desc.Method, desc.Path),
			nil,
		)
	}
	if err := s.ensureServer("raw http"); err != nil {
		return err
	}
	s.httpRoutes[key] = struct{}{}
	s.rawHTTPBindings = append(s.rawHTTPBindings, stagedRawHTTPBinding{desc: desc})
	return nil
}

func (s *stagedInstall) installGovernorHTTPBinding(binding GovernorHTTPBinding) error {
	opts, err := s.app.installOptions()
	if err != nil {
		return err
	}
	method, path, err := internalinstall.ValidateGovernorHTTPBinding(
		binding.Method,
		binding.Path,
		binding.Handler,
	)
	if err != nil {
		return err
	}
	if opts.governor == nil {
		return internalinstall.ValidationError("governor server is not available", nil)
	}
	if opts.governor.HasRoute(path) {
		return internalinstall.ConflictError(
			fmt.Sprintf("governor route %s %s already installed", method, path),
			nil,
		)
	}
	key := internalinstall.RouteKey("", path)
	if _, exists := s.governorRoutes[key]; exists {
		return internalinstall.ConflictError(
			fmt.Sprintf("governor http route %s already installed", path),
			nil,
		)
	}
	s.governorRoutes[key] = struct{}{}
	s.governorBindings = append(s.governorBindings, stagedGovernorHTTPBinding{
		method:  method,
		path:    path,
		handler: binding.Handler,
	})
	return nil
}

func (s *stagedInstall) addBackgroundTask(task BackgroundTask) error {
	if task == nil {
		return internalinstall.ValidationError("background task is nil", nil)
	}
	s.tasks = append(s.tasks, task)
	return nil
}

func (s *stagedInstall) addBusinessHook(hook BusinessHook) error {
	if hook.Func == nil {
		return internalinstall.ValidationError("business hook func is nil", nil)
	}
	switch hook.Stage {
	case BusinessHookBeforeStart, BusinessHookBeforeStop, BusinessHookAfterStop:
	default:
		return internalinstall.ValidationError(
			fmt.Sprintf("unsupported business hook stage %q", hook.Stage),
			nil,
		)
	}
	s.hooks = append(s.hooks, hook)
	return nil
}

func (s *stagedInstall) ensureServer(bindingType string) error {
	if s.svr != nil {
		return nil
	}
	svr, err := s.app.installServer(bindingType)
	if err != nil {
		return err
	}
	s.svr = svr
	return nil
}

func (s *stagedInstall) commit() error {
	if s.app == nil || s.app.opts == nil {
		return internalinstall.ValidationError("app is not ready", nil)
	}
	s.app.mu.Lock()
	ready := s.app.state >= lifecycleStateInitialized &&
		s.app.state < lifecycleStateServing &&
		s.app.state != lifecycleStateStopped &&
		!s.app.stopping.Load()
	s.app.mu.Unlock()
	if !ready {
		return internalinstall.ValidationError("runtime is not ready", nil)
	}
	if err := s.app.installStagedLifecycle(s.tasks, s.hooks); err != nil {
		return internalinstall.ValidationError("install staged lifecycle", err)
	}

	s.app.mu.Lock()
	defer s.app.mu.Unlock()
	if s.svr != nil {
		s.app.opts.server = s.svr
	}
	for _, binding := range s.rpcBindings {
		s.svr.RegisterService(binding.desc, binding.impl)
		s.app.installedRPCServices[binding.desc.ServiceName] = struct{}{}
	}
	for _, binding := range s.restBindings {
		s.svr.RegisterRestService(binding.desc, binding.impl, binding.prefixes...)
		for _, route := range binding.routes {
			s.app.installedHTTPRoutes[route] = struct{}{}
		}
	}
	for _, binding := range s.rawHTTPBindings {
		s.svr.RegisterRestRawHandlers(binding.desc)
		s.app.installedHTTPRoutes[internalinstall.RouteKey(
			binding.desc.Method,
			binding.desc.Path,
		)] = struct{}{}
	}
	for _, binding := range s.governorBindings {
		s.app.opts.governor.Handle(binding.method, binding.path, binding.handler)
		s.app.installedGovernorRoutes[internalinstall.RouteKey(
			"",
			binding.path,
		)] = struct{}{}
	}
	for _, task := range s.tasks {
		s.app.opts.internalServers = append(s.app.opts.internalServers, task)
	}
	beforeStart, beforeStop, afterStop := splitBusinessHooks(s.hooks)
	s.app.opts.beforeStartHooks = append(s.app.opts.beforeStartHooks, beforeStart...)
	s.app.opts.beforeStopHooks = append(s.app.opts.beforeStopHooks, beforeStop...)
	s.app.opts.afterStopHooks = append(s.app.opts.afterStopHooks, afterStop...)
	s.app.bundleDiagnostics = append(s.app.bundleDiagnostics, s.diagnostics...)
	return nil
}

func (a *App) installStagedLifecycle(
	tasks []BackgroundTask,
	hooks []BusinessHook,
) error {
	opts := make([]lifecycleOption, 0, len(tasks)+len(hooks)+1)
	if len(tasks) > 0 {
		servers := make([]InternalServer, len(tasks))
		for index, task := range tasks {
			servers[index] = task
		}
		opts = append(opts, internallifecycle.WithInternalServers(servers...))
	}
	beforeStart, beforeStop, afterStop := splitBusinessHooks(hooks)
	if len(beforeStart) > 0 {
		opts = append(opts, internallifecycle.WithBeforeStartHooks(beforeStart...))
	}
	if len(beforeStop) > 0 {
		opts = append(opts, internallifecycle.WithBeforeStopHooks(beforeStop...))
	}
	if len(afterStop) > 0 {
		opts = append(opts, internallifecycle.WithAfterStopHooks(afterStop...))
	}
	if len(opts) == 0 {
		return nil
	}
	return a.lifecycle.Init(opts...)
}

func splitBusinessHooks(
	hooks []BusinessHook,
) (
	[]func(context.Context) error,
	[]func(context.Context) error,
	[]func(context.Context) error,
) {
	beforeStart := make([]func(context.Context) error, 0, len(hooks))
	beforeStop := make([]func(context.Context) error, 0, len(hooks))
	afterStop := make([]func(context.Context) error, 0, len(hooks))
	for _, hook := range hooks {
		switch hook.Stage {
		case BusinessHookBeforeStart:
			beforeStart = append(beforeStart, hook.Func)
		case BusinessHookBeforeStop:
			beforeStop = append(beforeStop, hook.Func)
		case BusinessHookAfterStop:
			afterStop = append(afterStop, hook.Func)
		}
	}
	return beforeStart, beforeStop, afterStop
}

func (s *stagedInstall) rollbackExtensions(extensions []BusinessInstallable) error {
	var err error
	for index := range slices.Backward(extensions) {
		item, ok := extensions[index].(BusinessInstallRollback)
		if !ok {
			continue
		}
		rollbackCtx := &InstallContext{
			Runtime:   s.app.runtime,
			installer: s,
		}
		if rollbackErr := item.RollbackInstall(rollbackCtx); rollbackErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf(
					"rollback extension %q: %w",
					extensions[index].Kind(),
					rollbackErr,
				),
			)
		}
	}
	return err
}
