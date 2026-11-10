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

	internalinstall "github.com/codesjoy/yggdrasil/v3/app/internal/install"
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
	if fn == nil {
		err := yassembly.NewError(
			yassembly.ErrComposeFailed,
			"compose",
			"compose function is nil",
			nil,
			nil,
		)
		a.mu.Lock()
		a.recordAssemblyErrorLocked(assemblyStageCompose, err)
		a.mu.Unlock()
		return nil, err
	}
	if ctx == nil {
		err := yassembly.NewError(
			yassembly.ErrComposeFailed,
			"compose",
			"compose context is nil",
			nil,
			nil,
		)
		a.mu.Lock()
		a.recordAssemblyErrorLocked(assemblyStageCompose, err)
		a.mu.Unlock()
		return nil, err
	}
	if err := a.Prepare(ctx); err != nil {
		a.mu.Lock()
		a.recordAssemblyErrorLocked(assemblyStageCompose, err)
		a.mu.Unlock()
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
		a.recordAssemblyErrorLocked(assemblyStageCompose, err)
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
		a.mu.Lock()
		a.recordAssemblyErrorLocked(assemblyStageCompose, err)
		a.mu.Unlock()
		return nil, err
	}
	bundle, err := fn(rt)
	if err != nil {
		err = wrapAssemblyStageError("compose", err)
		a.mu.Lock()
		a.recordAssemblyErrorLocked(assemblyStageCompose, err)
		a.mu.Unlock()
		return nil, a.failBeforeStart(err, "compose", yassembly.ErrPreparedRuntimeRollbackFailed)
	}
	if bundle == nil {
		bundle = &BusinessBundle{}
	}
	a.mu.Lock()
	a.clearAssemblyErrorLocked(assemblyStageCompose)
	a.mu.Unlock()
	return bundle, nil
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
	a.mu.Lock()
	if a.explicitBundleInstalled {
		err := yassembly.NewError(
			yassembly.ErrInstallRegistrationConflict,
			"install",
			"business bundle already installed",
			nil,
			nil,
		)
		a.recordAssemblyErrorLocked(assemblyStageInstall, err)
		a.mu.Unlock()
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
		a.mu.Unlock()
		return err
	}
	err := a.installBundleLocked(bundle)
	if err == nil {
		a.explicitBundleInstalled = true
		a.clearAssemblyErrorLocked(assemblyStageInstall)
		if a.state < lifecycleStateBusinessInstalled {
			a.state = lifecycleStateBusinessInstalled
		}
	}
	a.mu.Unlock()
	if err != nil {
		err = internalinstall.WrapError(err)
		a.mu.Lock()
		a.recordAssemblyErrorLocked(assemblyStageInstall, err)
		a.mu.Unlock()
		return a.failBeforeStart(err, "install", yassembly.ErrPartialInstallRollbackFailed)
	}
	return nil
}

func (a *App) installBundleLocked(bundle *BusinessBundle) error {
	if bundle == nil {
		return nil
	}
	staged := newStagedInstall(a)
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
	extensions := make([]BusinessInstallable, 0, len(bundle.Extensions))
	for _, item := range bundle.Extensions {
		if item == nil {
			continue
		}
		extensions = append(extensions, item)
		ctx := &InstallContext{
			Runtime:   a.runtime,
			installer: staged,
		}
		if err := item.Install(ctx); err != nil {
			installErr := internalinstall.ValidationError(
				fmt.Sprintf("install extension %q: %v", item.Kind(), err),
				err,
			)
			if rollbackErr := staged.rollbackExtensions(extensions); rollbackErr != nil {
				return internalinstall.ValidationError(
					fmt.Sprintf("install extension %q rollback failed", item.Kind()),
					errors.Join(installErr, rollbackErr),
				)
			}
			return installErr
		}
	}
	staged.diagnostics = append(staged.diagnostics, bundle.Diagnostics...)
	return staged.commit()
}

func (a *App) installRPCBinding(binding RPCBinding) error {
	staged := newStagedInstall(a)
	if err := staged.installRPCBinding(binding); err != nil {
		return err
	}
	return staged.commit()
}

func (a *App) installRESTBinding(binding RESTBinding) error {
	staged := newStagedInstall(a)
	if err := staged.installRESTBinding(binding); err != nil {
		return err
	}
	return staged.commit()
}

func (a *App) installRawHTTPBinding(binding RawHTTPBinding) error {
	staged := newStagedInstall(a)
	if err := staged.installRawHTTPBinding(binding); err != nil {
		return err
	}
	return staged.commit()
}

func (a *App) installGovernorHTTPBinding(binding GovernorHTTPBinding) error {
	staged := newStagedInstall(a)
	if err := staged.installGovernorHTTPBinding(binding); err != nil {
		return err
	}
	return staged.commit()
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

func (a *App) addBackgroundTask(task BackgroundTask) error {
	staged := newStagedInstall(a)
	if err := staged.addBackgroundTask(task); err != nil {
		return err
	}
	return staged.commit()
}

func (a *App) addBusinessHook(hook BusinessHook) error {
	staged := newStagedInstall(a)
	if err := staged.addBusinessHook(hook); err != nil {
		return err
	}
	return staged.commit()
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
	if _, err := s.ensureServer("rpc"); err != nil {
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
	if _, err := s.ensureServer("rest"); err != nil {
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
	if _, err := s.ensureServer("raw http"); err != nil {
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

func (s *stagedInstall) ensureServer(bindingType string) (server.Server, error) {
	if s.svr != nil {
		return s.svr, nil
	}
	svr, err := s.app.installServer(bindingType)
	if err != nil {
		return nil, err
	}
	s.svr = svr
	return svr, nil
}

func (s *stagedInstall) commit() error {
	if s.app == nil || s.app.opts == nil {
		return internalinstall.ValidationError("app is not ready", nil)
	}
	if err := s.app.installStagedLifecycle(s.tasks, s.hooks); err != nil {
		return internalinstall.ValidationError("install staged lifecycle", err)
	}
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
	for _, hook := range s.hooks {
		switch hook.Stage {
		case BusinessHookBeforeStart:
			s.app.opts.beforeStartHooks = append(s.app.opts.beforeStartHooks, hook.Func)
		case BusinessHookBeforeStop:
			s.app.opts.beforeStopHooks = append(s.app.opts.beforeStopHooks, hook.Func)
		case BusinessHookAfterStop:
			s.app.opts.afterStopHooks = append(s.app.opts.afterStopHooks, hook.Func)
		}
	}
	s.app.bundleDiagnostics = append(s.app.bundleDiagnostics, s.diagnostics...)
	return nil
}

func (a *App) installStagedLifecycle(
	tasks []BackgroundTask,
	hooks []BusinessHook,
) error {
	opts := make([]lifecycleOption, 0, len(tasks)+len(hooks)+1)
	if len(tasks) > 0 {
		servers := make([]InternalServer, 0, len(tasks))
		for _, task := range tasks {
			servers = append(servers, task)
		}
		opts = append(opts, withLifecycleInternalServers(servers...))
	}
	var beforeStart []func(context.Context) error
	var beforeStop []func(context.Context) error
	var afterStop []func(context.Context) error
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
	if len(beforeStart) > 0 {
		opts = append(opts, withLifecycleBeforeStartHooks(beforeStart...))
	}
	if len(beforeStop) > 0 {
		opts = append(opts, withLifecycleBeforeStopHooks(beforeStop...))
	}
	if len(afterStop) > 0 {
		opts = append(opts, withLifecycleAfterStopHooks(afterStop...))
	}
	if len(opts) == 0 {
		return nil
	}
	return a.lifecycle.Init(opts...)
}

func (s *stagedInstall) rollbackExtensions(extensions []BusinessInstallable) error {
	var err error
	for index := len(extensions) - 1; index >= 0; index-- {
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
