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
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/codesjoy/yggdrasil/v3/internal/instance"
	"github.com/codesjoy/yggdrasil/v3/internal/remotelog"
)

var (
	processDefaultsMu    sync.Mutex
	processDefaultsOwner *App
)

type processDefaultsLease struct {
	mu sync.Mutex

	app *App

	oldLogger     *slog.Logger
	oldTracer     trace.TracerProvider
	oldMeter      metric.MeterProvider
	oldPropagator propagation.TextMapPropagator
	oldRemote     *slog.Logger
	oldInstance   instance.Snapshot

	installed bool
	released  bool
}

func acquireProcessDefaultsLease(app *App) (*processDefaultsLease, error) {
	processDefaultsMu.Lock()
	defer processDefaultsMu.Unlock()

	if processDefaultsOwner != nil && processDefaultsOwner != app {
		return nil, ErrProcessDefaultsAlreadyInstalled
	}
	if app != nil && app.processDefaultsLease != nil {
		return app.processDefaultsLease, nil
	}
	lease := &processDefaultsLease{
		app:           app,
		oldLogger:     slog.Default(),
		oldTracer:     otel.GetTracerProvider(),
		oldMeter:      otel.GetMeterProvider(),
		oldPropagator: otel.GetTextMapPropagator(),
		oldRemote:     remotelog.Logger(),
		oldInstance:   instance.ProcessDefaultSnapshot(),
	}
	processDefaultsOwner = app
	if app != nil {
		app.processDefaultsLease = lease
	}
	return lease, nil
}

func (lease *processDefaultsLease) install(snapshot *Snapshot) {
	if lease == nil || snapshot == nil {
		return
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.released {
		return
	}
	if snapshot.Logger != nil {
		slog.SetDefault(snapshot.Logger)
	}
	if snapshot.TracerProvider != nil {
		otel.SetTracerProvider(snapshot.TracerProvider)
	}
	if snapshot.MeterProvider != nil {
		otel.SetMeterProvider(snapshot.MeterProvider)
	}
	if snapshot.TextMapPropagator != nil {
		otel.SetTextMapPropagator(snapshot.TextMapPropagator)
	}
	if snapshot.RemoteLogger != nil {
		remotelog.SetLogger(snapshot.RemoteLogger)
	}
	if !snapshot.Identity.isZero() {
		identity := snapshot.Identity.internal()
		instance.InstallProcessDefault(identity.AppName, identity.InstanceConfig())
	}
	lease.installed = true
}

func (lease *processDefaultsLease) release(context.Context) error {
	if lease == nil {
		return nil
	}
	lease.mu.Lock()
	if lease.released {
		lease.mu.Unlock()
		return nil
	}
	lease.released = true
	installed := lease.installed
	oldLogger := lease.oldLogger
	oldTracer := lease.oldTracer
	oldMeter := lease.oldMeter
	oldPropagator := lease.oldPropagator
	oldRemote := lease.oldRemote
	oldInstance := lease.oldInstance
	app := lease.app
	lease.mu.Unlock()

	if installed {
		if oldLogger != nil {
			slog.SetDefault(oldLogger)
		}
		if oldTracer != nil {
			otel.SetTracerProvider(oldTracer)
		}
		if oldMeter != nil {
			otel.SetMeterProvider(oldMeter)
		}
		if oldPropagator != nil {
			otel.SetTextMapPropagator(oldPropagator)
		}
		if oldRemote != nil {
			remotelog.SetLogger(oldRemote)
		}
		instance.RestoreProcessDefault(oldInstance)
	}

	processDefaultsMu.Lock()
	if processDefaultsOwner == app {
		processDefaultsOwner = nil
	}
	processDefaultsMu.Unlock()
	return nil
}
