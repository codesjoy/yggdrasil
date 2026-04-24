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
	"fmt"
	"reflect"
	"strings"

	internalinstall "github.com/codesjoy/yggdrasil/v3/app/internal/install"
	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/server"
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
	if a.state < lifecycleStateInitialized || a.state >= lifecycleStateServing || a.state == lifecycleStateStopped {
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
	for _, item := range bundle.RPCBindings {
		if err := a.installRPCBinding(item); err != nil {
			return err
		}
	}
	for _, item := range bundle.RESTBindings {
		if err := a.installRESTBinding(item); err != nil {
			return err
		}
	}
	for _, item := range bundle.RawHTTP {
		if err := a.installRawHTTPBinding(item); err != nil {
			return err
		}
	}
	for _, item := range bundle.Tasks {
		if err := a.addBackgroundTask(item); err != nil {
			return err
		}
	}
	for _, item := range bundle.Hooks {
		if err := a.addBusinessHook(item); err != nil {
			return err
		}
	}
	for _, item := range bundle.Extensions {
		if item == nil {
			continue
		}
		ctx := &InstallContext{
			Runtime:   a.runtime,
			installer: a,
		}
		if err := item.Install(ctx); err != nil {
			return internalinstall.ValidationError(fmt.Sprintf("install extension %q: %v", item.Kind(), err), err)
		}
	}
	a.bundleDiagnostics = append(a.bundleDiagnostics, bundle.Diagnostics...)
	return nil
}

func (a *App) installRPCBinding(binding RPCBinding) error {
	opts, err := a.installOptions()
	if err != nil {
		return err
	}
	if len(opts.resolvedSettings.Server.Transports) == 0 {
		return internalinstall.ValidationError("rpc bindings require at least one configured server transport", nil)
	}
	desc, ok := binding.Desc.(*server.ServiceDesc)
	if !ok || desc == nil {
		return internalinstall.ValidationError("rpc binding desc must be *server.ServiceDesc", nil)
	}
	serviceName := strings.TrimSpace(binding.ServiceName)
	if serviceName == "" {
		serviceName = desc.ServiceName
	}
	if binding.Impl == nil {
		return internalinstall.ValidationError(fmt.Sprintf("rpc service %q implementation is nil", serviceName), nil)
	}
	if !internalinstall.ImplementsHandler(desc.HandlerType, binding.Impl) {
		return internalinstall.ValidationError(
			fmt.Sprintf("rpc service %q handler does not satisfy interface", serviceName),
			nil,
		)
	}
	if _, exists := a.installedRPCServices[desc.ServiceName]; exists {
		return internalinstall.ConflictError(fmt.Sprintf("rpc service %q already installed", serviceName), nil)
	}
	svr, err := a.installServer("rpc")
	if err != nil {
		return err
	}
	svr.RegisterService(desc, binding.Impl)
	a.installedRPCServices[desc.ServiceName] = struct{}{}
	return nil
}

func (a *App) installRESTBinding(binding RESTBinding) error {
	opts, err := a.installOptions()
	if err != nil {
		return err
	}
	if !opts.resolvedSettings.Server.RestEnabled {
		return internalinstall.ValidationError("rest bindings require yggdrasil.transports.http.rest", nil)
	}
	desc, ok := binding.Desc.(*server.RestServiceDesc)
	if !ok || desc == nil {
		return internalinstall.ValidationError("rest binding desc must be *server.RestServiceDesc", nil)
	}
	name := strings.TrimSpace(binding.Name)
	if name == "" {
		if handlerType := reflect.TypeOf(desc.HandlerType); handlerType != nil {
			name = handlerType.String()
		} else {
			name = "rest"
		}
	}
	if binding.Impl == nil {
		return internalinstall.ValidationError(fmt.Sprintf("rest binding %q implementation is nil", name), nil)
	}
	if !internalinstall.ImplementsHandler(desc.HandlerType, binding.Impl) {
		return internalinstall.ValidationError(
			fmt.Sprintf("rest binding %q handler does not satisfy interface", name),
			nil,
		)
	}
	prefix := internalinstall.BuildRESTRoutePrefix(binding.Prefixes)
	for _, method := range desc.Methods {
		key := internalinstall.RouteKey(method.Method, prefix+method.Path)
		if _, exists := a.installedHTTPRoutes[key]; exists {
			return internalinstall.ConflictError(
				fmt.Sprintf("rest route %s %s already installed", method.Method, prefix+method.Path),
				nil,
			)
		}
	}
	svr, err := a.installServer("rest")
	if err != nil {
		return err
	}
	svr.RegisterRestService(desc, binding.Impl, binding.Prefixes...)
	for _, method := range desc.Methods {
		a.installedHTTPRoutes[internalinstall.RouteKey(method.Method, prefix+method.Path)] = struct{}{}
	}
	return nil
}

func (a *App) installRawHTTPBinding(binding RawHTTPBinding) error {
	opts, err := a.installOptions()
	if err != nil {
		return err
	}
	if !opts.resolvedSettings.Server.RestEnabled {
		return internalinstall.ValidationError("raw http bindings require yggdrasil.transports.http.rest", nil)
	}
	desc, err := internalinstall.NormalizeRawHTTPBinding(binding.Desc, binding.Method, binding.Path, binding.Handler)
	if err != nil {
		return err
	}
	if desc.Handler == nil {
		return internalinstall.ValidationError("raw http binding handler is nil", nil)
	}
	key := internalinstall.RouteKey(desc.Method, desc.Path)
	if _, exists := a.installedHTTPRoutes[key]; exists {
		return internalinstall.ConflictError(
			fmt.Sprintf("raw http route %s %s already installed", desc.Method, desc.Path),
			nil,
		)
	}
	svr, err := a.installServer("raw http")
	if err != nil {
		return err
	}
	svr.RegisterRestRawHandlers(desc)
	a.installedHTTPRoutes[key] = struct{}{}
	return nil
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
	if task == nil {
		return internalinstall.ValidationError("background task is nil", nil)
	}
	a.opts.internalServers = append(a.opts.internalServers, task)
	return nil
}

func (a *App) addBusinessHook(hook BusinessHook) error {
	if hook.Func == nil {
		return internalinstall.ValidationError("business hook func is nil", nil)
	}
	switch hook.Stage {
	case BusinessHookBeforeStart:
		a.opts.beforeStartHooks = append(a.opts.beforeStartHooks, hook.Func)
	case BusinessHookBeforeStop:
		a.opts.beforeStopHooks = append(a.opts.beforeStopHooks, hook.Func)
	case BusinessHookAfterStop:
		a.opts.afterStopHooks = append(a.opts.afterStopHooks, hook.Func)
	default:
		return internalinstall.ValidationError(fmt.Sprintf("unsupported business hook stage %q", hook.Stage), nil)
	}
	return nil
}
