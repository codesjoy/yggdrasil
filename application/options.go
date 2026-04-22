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

package application

import (
	"context"
	"fmt"
	"time"

	"github.com/codesjoy/yggdrasil/v3/governor"
	"github.com/codesjoy/yggdrasil/v3/registry"
	"github.com/codesjoy/yggdrasil/v3/server"
)

// Option application option
type Option func(*application) error

// WithHook register a hook
func WithHook(stage Stage, fns ...func(context.Context) error) Option {
	return func(app *application) error {
		hooks, ok := app.hooks[stage]
		if ok {
			hooks.Register(fns...)
			return nil
		}
		return fmt.Errorf("hook stage not found")
	}
}

// WithBeforeStopHook register the before stop hook
func WithBeforeStopHook(fns ...func(context.Context) error) Option {
	return WithHook(stageBeforeStop, fns...)
}

// WithBeforeStartHook register the before start hook
func WithBeforeStartHook(fns ...func(context.Context) error) Option {
	return WithHook(stageBeforeStart, fns...)
}

// WithAfterStopHook register the after stop hook
func WithAfterStopHook(fns ...func(context.Context) error) Option {
	return WithHook(stageAfterStop, fns...)
}

// WithRegistry register a registry
func WithRegistry(registry registry.Registry) Option {
	return func(application *application) error {
		application.registry = registry
		return nil
	}
}

// WithShutdownTimeout set the shutdown timeout
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(application *application) error {
		application.shutdownTimeout = timeout
		return nil
	}
}

// WithServer register a server
func WithServer(server server.Server) Option {
	return func(application *application) error {
		application.server = server
		return nil
	}
}

// WithGovernor register a governor
func WithGovernor(svr *governor.Server) Option {
	return func(application *application) error {
		application.governor = svr
		return nil
	}
}

// WithInternalServer register an internal server
func WithInternalServer(svr ...InternalServer) Option {
	return func(application *application) error {
		application.internalSvr = append(application.internalSvr, svr...)
		return nil
	}
}

// WithCleanup register a cleanup
func WithCleanup(name string, fn func(context.Context) error) Option {
	return WithHook(stageCleanup, func(ctx context.Context) error {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		if name == "" {
			return err
		}
		return fmt.Errorf("cleanup %s: %w", name, err)
	})
}
