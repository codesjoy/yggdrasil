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

package yggdrasil

import (
	"context"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/server"
)

// App is the framework application composition root.
type App = yapp.App

// Option configures one App instance.
type Option = yapp.Option

// InternalServer is managed by the App lifecycle alongside the main server.
type InternalServer = yapp.InternalServer

// New creates one App instance.
func New(appName string, opts ...Option) (*App, error) {
	return yapp.New(appName, opts...)
}

// WithRPCServices registers a batch of RPC services.
func WithRPCServices(desc map[*server.ServiceDesc]interface{}) Option {
	return yapp.WithRPCServices(desc)
}

// WithRPCService registers one RPC service.
func WithRPCService(desc *server.ServiceDesc, impl interface{}) Option {
	return yapp.WithRPCService(desc, impl)
}

// WithRESTService registers one REST service.
func WithRESTService(desc *server.RestServiceDesc, impl interface{}, prefix ...string) Option {
	return yapp.WithRESTService(desc, impl, prefix...)
}

// WithRESTHandlers registers raw REST handlers.
func WithRESTHandlers(desc ...*server.RestRawHandlerDesc) Option {
	return yapp.WithRESTHandlers(desc...)
}

// WithBeforeStartHook registers before-start hooks.
func WithBeforeStartHook(fns ...func(context.Context) error) Option {
	return yapp.WithBeforeStartHook(fns...)
}

// WithBeforeStopHook registers before-stop hooks.
func WithBeforeStopHook(fns ...func(context.Context) error) Option {
	return yapp.WithBeforeStopHook(fns...)
}

// WithAfterStopHook registers after-stop hooks.
func WithAfterStopHook(fns ...func(context.Context) error) Option {
	return yapp.WithAfterStopHook(fns...)
}

// WithInternalServer registers internal servers managed by the app lifecycle.
func WithInternalServer(svr ...InternalServer) Option {
	return yapp.WithInternalServer(svr...)
}

// WithCleanup registers additional cleanup hooks.
func WithCleanup(name string, fn func(context.Context) error) Option {
	return yapp.WithCleanup(name, fn)
}

// WithConfigManager injects one config manager instance.
func WithConfigManager(manager *config.Manager) Option {
	return yapp.WithConfigManager(manager)
}

// WithBootstrapPath overrides the bootstrap config path.
func WithBootstrapPath(path string) Option {
	return yapp.WithBootstrapPath(path)
}

// WithBootstrapSource registers one bootstrap source layer.
func WithBootstrapSource(name string, priority config.Priority, src source.Source) Option {
	return yapp.WithBootstrapSource(name, priority, src)
}

// WithModules registers extra modules into the app hub.
func WithModules(mods ...module.Module) Option {
	return yapp.WithModules(mods...)
}
