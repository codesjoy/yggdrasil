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
	"io"
	"log/slog"

	"go.opentelemetry.io/otel"

	"github.com/codesjoy/yggdrasil/v3/internal/instance"
	"github.com/codesjoy/yggdrasil/v3/internal/remotelog"
	xotel "github.com/codesjoy/yggdrasil/v3/otel"
	"github.com/codesjoy/yggdrasil/v3/server"
)

func (a *App) currentRuntimeSnapshot() *Snapshot {
	a.runtimeMu.RLock()
	defer a.runtimeMu.RUnlock()
	return a.runtimeSnapshot
}

func (a *App) setRuntimeSnapshot(snapshot *Snapshot) {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	a.runtimeSnapshot = snapshot
}

func (a *App) stageFoundationSnapshot(snapshot *Snapshot) {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	a.preparedFoundationSnapshot = snapshot
}

func (a *App) commitFoundationSnapshot(snapshot *Snapshot) {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	a.foundationSnapshot = snapshot
	if a.preparedFoundationSnapshot == snapshot {
		a.preparedFoundationSnapshot = nil
	}
}

func (a *App) rollbackFoundationSnapshot(snapshot *Snapshot) {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	if a.preparedFoundationSnapshot == snapshot {
		a.preparedFoundationSnapshot = nil
	}
}

func (a *App) foundationSnapshotForRuntime() *Snapshot {
	a.runtimeMu.RLock()
	defer a.runtimeMu.RUnlock()
	if a.preparedFoundationSnapshot != nil {
		return a.preparedFoundationSnapshot
	}
	if a.foundationSnapshot != nil {
		return a.foundationSnapshot
	}
	return a.runtimeSnapshot
}

func runtimeShutdown(v any) func(context.Context) error {
	switch item := v.(type) {
	case interface{ Shutdown(context.Context) error }:
		return item.Shutdown
	case io.Closer:
		return func(context.Context) error { return item.Close() }
	default:
		return nil
	}
}

func (a *App) applyRuntimeAdapters(snapshot *Snapshot) error {
	if snapshot == nil {
		return errors.New("runtime snapshot is nil")
	}

	handler, err := snapshot.BuildDefaultLoggerHandler()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(handler))
	remoteLoggerLvStr := snapshot.Resolved.Logging.RemoteLevel
	if remoteLoggerLvStr == "" {
		remoteLoggerLvStr = "error"
	}
	var remoteLoggerLv slog.Level
	if err = remoteLoggerLv.UnmarshalText([]byte(remoteLoggerLvStr)); err != nil {
		return err
	}
	remotelog.Init(remoteLoggerLv, handler)
	xotel.ConfigureDefaultPropagator()

	oldTracer := a.swapTracerShutdown(nil)
	if tp, ok := snapshot.BuildTracerProvider(instance.Name()); ok {
		otel.SetTracerProvider(tp)
		a.swapTracerShutdown(runtimeShutdown(tp))
	} else {
		a.swapTracerShutdown(nil)
	}
	if oldTracer != nil {
		if shutdownErr := oldTracer(context.Background()); shutdownErr != nil {
			slog.Warn("shutdown previous tracer provider failed", slog.Any("error", shutdownErr))
		}
	}

	oldMeter := a.swapMeterShutdown(nil)
	if mp, ok := snapshot.BuildMeterProvider(instance.Name()); ok {
		otel.SetMeterProvider(mp)
		a.swapMeterShutdown(runtimeShutdown(mp))
	} else {
		a.swapMeterShutdown(nil)
	}
	if oldMeter != nil {
		if shutdownErr := oldMeter(context.Background()); shutdownErr != nil {
			slog.Warn("shutdown previous meter provider failed", slog.Any("error", shutdownErr))
		}
	}

	return nil
}

func (a *App) swapTracerShutdown(next func(context.Context) error) func(context.Context) error {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	prev := a.tracerShutdown
	a.tracerShutdown = next
	return prev
}

func (a *App) swapMeterShutdown(next func(context.Context) error) func(context.Context) error {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	prev := a.meterShutdown
	a.meterShutdown = next
	return prev
}

func (a *App) shutdownRuntimeAdapters(ctx context.Context) error {
	a.runtimeMu.Lock()
	tracerShutdown := a.tracerShutdown
	meterShutdown := a.meterShutdown
	a.tracerShutdown = nil
	a.meterShutdown = nil
	a.runtimeMu.Unlock()

	var err error
	if tracerShutdown != nil {
		err = errors.Join(err, tracerShutdown(ctx))
	}
	if meterShutdown != nil {
		err = errors.Join(err, meterShutdown(ctx))
	}
	return err
}

func (a *App) initRegistry() {
	if a == nil || a.opts == nil {
		return
	}
	typeName := a.opts.resolvedSettings.Discovery.Registry.Type
	if typeName == "" {
		return
	}
	snapshot := a.currentRuntimeSnapshot()
	if snapshot == nil {
		return
	}
	r, err := snapshot.NewRegistry()
	if err != nil {
		slog.Warn(
			"fault to initialize registry",
			slog.String("type", typeName),
			slog.Any("error", err),
		)
		return
	}
	a.opts.registry = r
	if c, ok := r.(io.Closer); ok {
		a.opts.lifecycleOptions = append(
			a.opts.lifecycleOptions,
			withLifecycleCleanup("registry", func(context.Context) error {
				return c.Close()
			}),
		)
	}
}

func (a *App) initServer() error {
	if a == nil || a.opts == nil {
		return nil
	}
	if len(a.opts.rpcServices) == 0 && len(a.opts.restServices) == 0 &&
		len(a.opts.restHandlers) == 0 {
		return nil
	}
	svr, err := server.New(a.currentRuntimeSnapshot())
	if err != nil {
		return err
	}
	server.RegisterGovernorRoutes(a.opts.governor, svr)
	for k, v := range a.opts.rpcServices {
		svr.RegisterService(k, v)
	}
	for k, v := range a.opts.restServices {
		svr.RegisterRestService(k, v.impl, v.prefixes...)
	}
	if len(a.opts.restHandlers) > 0 {
		svr.RegisterRestRawHandlers(a.opts.restHandlers...)
	}
	a.opts.server = svr
	return nil
}
