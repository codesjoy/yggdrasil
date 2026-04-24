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
	"log/slog"
	"reflect"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/codesjoy/yggdrasil/v3/client"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/server"
)

// Runtime is the business-safe runtime surface exposed after Prepare succeeds.
type Runtime interface {
	NewClient(ctx context.Context, service string) (client.Client, error)
	Config() *config.Manager
	Logger() *slog.Logger
	TracerProvider() trace.TracerProvider
	MeterProvider() metric.MeterProvider
	Lookup(target any) error
}

// ComposeFunc builds one business bundle from the prepared runtime.
type ComposeFunc func(Runtime) (*BusinessBundle, error)

// RPCBinding declares one RPC service binding to install.
type RPCBinding struct {
	ServiceName string
	Desc        any
	Impl        any
}

// RESTBinding declares one REST service binding to install.
type RESTBinding struct {
	Name     string
	Desc     any
	Impl     any
	Prefixes []string
}

// RawHTTPBinding declares one raw REST handler binding to install.
// Method/Path/Handler is the canonical public shape; Desc is kept for compatibility.
type RawHTTPBinding struct {
	Method  string
	Path    string
	Handler any

	// Desc is the legacy raw handler descriptor input kept for compatibility.
	Desc *server.RestRawHandlerDesc
}

// BackgroundTask reuses the managed internal server lifecycle contract.
type BackgroundTask interface {
	Serve() error
	Stop(context.Context) error
}

// BusinessHookStage identifies the lifecycle stage of one business hook.
type BusinessHookStage string

const (
	BusinessHookBeforeStart BusinessHookStage = "before_start"
	BusinessHookBeforeStop  BusinessHookStage = "before_stop"
	BusinessHookAfterStop   BusinessHookStage = "after_stop"
)

// BusinessHook is a managed lifecycle hook installed with one bundle.
type BusinessHook struct {
	Name  string
	Stage BusinessHookStage
	Func  func(context.Context) error
}

// BundleDiag is one bundle-owned diagnostic item.
type BundleDiag struct {
	Code    string
	Message string
}

// BusinessInstallable is the extension escape hatch for non-standard installs.
type BusinessInstallable interface {
	Kind() string
	Install(ctx *InstallContext) error
}

// BusinessBundle is the prepared business graph installation payload.
type BusinessBundle struct {
	RPCBindings  []RPCBinding
	RESTBindings []RESTBinding
	RawHTTP      []RawHTTPBinding
	Tasks        []BackgroundTask
	Hooks        []BusinessHook
	Extensions   []BusinessInstallable
	Diagnostics  []BundleDiag
}

type bundleInstaller interface {
	installRPCBinding(RPCBinding) error
	installRESTBinding(RESTBinding) error
	installRawHTTPBinding(RawHTTPBinding) error
	addBackgroundTask(BackgroundTask) error
	addBusinessHook(BusinessHook) error
}

// InstallContext is passed to bundle extensions during installation.
type InstallContext struct {
	Runtime Runtime

	installer bundleInstaller
}

// RegisterRPC installs one RPC binding through the app installer.
func (ctx *InstallContext) RegisterRPC(binding RPCBinding) error {
	installer, err := ctx.installerOrError()
	if err != nil {
		return err
	}
	return installer.installRPCBinding(binding)
}

// RegisterREST installs one REST binding through the app installer.
func (ctx *InstallContext) RegisterREST(binding RESTBinding) error {
	installer, err := ctx.installerOrError()
	if err != nil {
		return err
	}
	return installer.installRESTBinding(binding)
}

// RegisterRawHTTP installs one raw HTTP binding through the app installer.
func (ctx *InstallContext) RegisterRawHTTP(binding RawHTTPBinding) error {
	installer, err := ctx.installerOrError()
	if err != nil {
		return err
	}
	return installer.installRawHTTPBinding(binding)
}

// AddTask registers one managed background task.
func (ctx *InstallContext) AddTask(task BackgroundTask) error {
	installer, err := ctx.installerOrError()
	if err != nil {
		return err
	}
	return installer.addBackgroundTask(task)
}

// AddHook registers one managed lifecycle hook.
func (ctx *InstallContext) AddHook(hook BusinessHook) error {
	installer, err := ctx.installerOrError()
	if err != nil {
		return err
	}
	return installer.addBusinessHook(hook)
}

func (ctx *InstallContext) installerOrError() (bundleInstaller, error) {
	if ctx == nil || ctx.installer == nil {
		return nil, errors.New("install context is not ready")
	}
	return ctx.installer, nil
}

type runtimeSurface struct {
	app     *App
	lookups map[reflect.Type]any
}

func newRuntimeSurface(app *App) Runtime {
	rt := &runtimeSurface{
		app:     app,
		lookups: map[reflect.Type]any{},
	}
	if app != nil && app.opts != nil && app.opts.configManager != nil {
		rt.lookups[reflect.TypeOf((*config.Manager)(nil))] = app.opts.configManager
		rt.lookups[reflect.TypeOf(settings.Catalog{})] = settings.NewCatalog(app.opts.configManager)
	}
	rt.lookups[reflect.TypeOf((*slog.Logger)(nil))] = slog.Default()
	rt.lookups[reflect.TypeOf((*trace.TracerProvider)(nil)).Elem()] = otel.GetTracerProvider()
	rt.lookups[reflect.TypeOf((*metric.MeterProvider)(nil)).Elem()] = otel.GetMeterProvider()
	return rt
}

func (r *runtimeSurface) NewClient(ctx context.Context, service string) (client.Client, error) {
	if r == nil || r.app == nil {
		return nil, errors.New("runtime is not ready")
	}
	return r.app.NewClient(ctx, service)
}

func (r *runtimeSurface) Config() *config.Manager {
	if r == nil || r.app == nil || r.app.opts == nil {
		return nil
	}
	return r.app.opts.configManager
}

func (r *runtimeSurface) Logger() *slog.Logger {
	return slog.Default()
}

func (r *runtimeSurface) TracerProvider() trace.TracerProvider {
	return otel.GetTracerProvider()
}

func (r *runtimeSurface) MeterProvider() metric.MeterProvider {
	return otel.GetMeterProvider()
}

func (r *runtimeSurface) Lookup(target any) error {
	if target == nil {
		return errors.New("lookup target is nil")
	}
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("lookup target must be a non-nil pointer")
	}
	typ := rv.Elem().Type()
	value, ok := r.lookups[typ]
	if !ok {
		return errors.New("runtime capability not found")
	}
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return errors.New("runtime capability is invalid")
	}
	if !v.Type().AssignableTo(typ) {
		if v.Type().Implements(typ) {
			rv.Elem().Set(v)
			return nil
		}
		return errors.New("runtime capability type mismatch")
	}
	rv.Elem().Set(v)
	return nil
}
