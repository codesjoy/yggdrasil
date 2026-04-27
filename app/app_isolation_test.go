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
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/codesjoy/yggdrasil/v3/capabilities"
	"github.com/codesjoy/yggdrasil/v3/internal/instance"
	"github.com/codesjoy/yggdrasil/v3/internal/remotelog"
	"github.com/codesjoy/yggdrasil/v3/module"
	xotel "github.com/codesjoy/yggdrasil/v3/observability/otel"
)

func isolationConfig(namespace string) map[string]any {
	return map[string]any{
		"yggdrasil": map[string]any{
			"admin": map[string]any{
				"application": map[string]any{
					"namespace": namespace,
					"version":   "1.2.3",
					"region":    "r1",
					"zone":      "z1",
					"campus":    "c1",
					"metadata": map[string]any{
						"owner": namespace,
					},
				},
				"governor": map[string]any{
					"port": 0,
				},
			},
		},
	}
}

func otelRegistration(
	tracer trace.TracerProvider,
	meter metric.MeterProvider,
) CapabilityRegistration {
	return CapabilityRegistration{
		Name: "test.otel.providers",
		Capabilities: func() []module.Capability {
			return []module.Capability{
				{
					Spec: capabilities.TracerProviderSpec,
					Name: "test-tracer",
					Value: xotel.TracerProviderBuilder(func(string) trace.TracerProvider {
						return tracer
					}),
				},
				{
					Spec: capabilities.MeterProviderSpec,
					Name: "test-meter",
					Value: xotel.MeterProviderBuilder(func(string) metric.MeterProvider {
						return meter
					}),
				},
			}
		},
	}
}

func telemetryConfig(namespace string) map[string]any {
	cfg := isolationConfig(namespace)
	ygg := cfg["yggdrasil"].(map[string]any)
	ygg["observability"] = map[string]any{
		"telemetry": map[string]any{
			"tracer": "test-tracer",
			"meter":  "test-meter",
		},
	}
	return cfg
}

func TestAppLocalRuntimeDoesNotMutateProcessGlobals(t *testing.T) {
	oldLogger := slog.Default()
	oldTracer := otel.GetTracerProvider()
	oldMeter := otel.GetMeterProvider()
	oldPropagator := otel.GetTextMapPropagator()
	oldRemote := remotelog.Logger()
	oldInstance := instance.ProcessDefaultSnapshot()

	tracerA := tracenoop.NewTracerProvider()
	meterA := metricnoop.NewMeterProvider()
	tracerB := tracenoop.NewTracerProvider()
	meterB := metricnoop.NewMeterProvider()

	appA, _ := newTestAppWithConfig(
		t,
		"app-a",
		telemetryConfig("ns-a"),
		WithCapabilityRegistrations(otelRegistration(tracerA, meterA)),
	)
	appB, _ := newTestAppWithConfig(
		t,
		"app-b",
		telemetryConfig("ns-b"),
		WithCapabilityRegistrations(otelRegistration(tracerB, meterB)),
	)
	t.Cleanup(func() {
		_ = appA.Stop(context.Background())
		_ = appB.Stop(context.Background())
	})

	require.NoError(t, appA.Prepare(context.Background()))
	require.NoError(t, appB.Prepare(context.Background()))

	assert.Same(t, oldLogger, slog.Default())
	assert.Same(t, oldTracer, otel.GetTracerProvider())
	assert.Same(t, oldMeter, otel.GetMeterProvider())
	assert.Same(t, oldPropagator, otel.GetTextMapPropagator())
	assert.Same(t, oldRemote, remotelog.Logger())
	assert.Equal(t, oldInstance, instance.ProcessDefaultSnapshot())

	rtA := appA.Runtime()
	rtB := appB.Runtime()
	require.NotNil(t, rtA)
	require.NotNil(t, rtB)
	assert.Equal(t, "app-a", rtA.Identity().AppName)
	assert.Equal(t, "ns-a", rtA.Identity().Namespace)
	assert.Equal(t, "app-b", rtB.Identity().AppName)
	assert.Equal(t, "ns-b", rtB.Identity().Namespace)
	assert.Equal(t, tracerA, rtA.TracerProvider())
	assert.Equal(t, meterA, rtA.MeterProvider())
	assert.Equal(t, tracerB, rtB.TracerProvider())
	assert.Equal(t, meterB, rtB.MeterProvider())
	assert.NotSame(t, rtA.Logger(), rtB.Logger())
}

func TestAppIdentityAvailableAfterPrepare(t *testing.T) {
	app, _ := newTestAppWithConfig(t, "identity-app", isolationConfig("identity-ns"))
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	_, ok := app.Identity()
	require.False(t, ok)
	require.NoError(t, app.Prepare(context.Background()))

	identity, ok := app.Identity()
	require.True(t, ok)
	assert.Equal(t, "identity-app", identity.AppName)
	assert.Equal(t, "identity-ns", identity.Namespace)
	assert.Equal(t, "1.2.3", identity.Version)
	assert.Equal(t, "identity-ns", identity.Metadata["owner"])
	identity.Metadata["owner"] = "mutated"

	again, ok := app.Identity()
	require.True(t, ok)
	assert.Equal(t, "identity-ns", again.Metadata["owner"])
}

func TestProcessDefaultsLeaseConflictDoesNotStopLosingApp(t *testing.T) {
	appA, _ := newTestAppWithConfig(
		t,
		"process-a",
		isolationConfig("default"),
		WithProcessDefaults(true),
	)
	appB, _ := newTestAppWithConfig(
		t,
		"process-b",
		isolationConfig("default"),
		WithProcessDefaults(true),
	)
	t.Cleanup(func() {
		_ = appA.Stop(context.Background())
		_ = appB.Stop(context.Background())
	})

	require.NoError(t, appA.Prepare(context.Background()))
	err := appB.Prepare(context.Background())
	require.ErrorIs(t, err, ErrProcessDefaultsAlreadyInstalled)
	appB.mu.Lock()
	assert.Equal(t, lifecycleStateNew, appB.state)
	appB.mu.Unlock()

	require.NoError(t, appA.Stop(context.Background()))
	require.NoError(t, appB.Prepare(context.Background()))
}

func TestProcessDefaultsRestoreGlobalsAndReleaseGuardOnShutdownError(t *testing.T) {
	oldLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	oldTracer := tracenoop.NewTracerProvider()
	oldMeter := metricnoop.NewMeterProvider()
	oldPropagator := propagation.TraceContext{}
	oldRemote := slog.New(slog.NewTextHandler(io.Discard, nil))
	oldInstance := instance.Snapshot{
		AppName: "old-app",
		Config: instance.Config{
			Namespace: "old-ns",
			Metadata:  map[string]string{},
		},
	}

	prevLogger := slog.Default()
	prevTracer := otel.GetTracerProvider()
	prevMeter := otel.GetMeterProvider()
	prevPropagator := otel.GetTextMapPropagator()
	prevRemote := remotelog.Logger()
	prevInstance := instance.ProcessDefaultSnapshot()
	t.Cleanup(func() {
		slog.SetDefault(prevLogger)
		otel.SetTracerProvider(prevTracer)
		otel.SetMeterProvider(prevMeter)
		otel.SetTextMapPropagator(prevPropagator)
		remotelog.SetLogger(prevRemote)
		instance.RestoreProcessDefault(prevInstance)
	})

	slog.SetDefault(oldLogger)
	otel.SetTracerProvider(oldTracer)
	otel.SetMeterProvider(oldMeter)
	otel.SetTextMapPropagator(oldPropagator)
	remotelog.SetLogger(oldRemote)
	instance.RestoreProcessDefault(oldInstance)

	var tracerShutdowns int32
	tracerErr := errors.New("shutdown tracer")
	tracer := &failingShutdownTracerProvider{
		TracerProvider: tracenoop.NewTracerProvider(),
		shutdowns:      &tracerShutdowns,
		err:            tracerErr,
	}
	meter := metricnoop.NewMeterProvider()

	app, _ := newTestAppWithConfig(
		t,
		"process-restore",
		telemetryConfig("restore-ns"),
		WithProcessDefaults(true),
		WithCapabilityRegistrations(otelRegistration(tracer, meter)),
	)

	require.NoError(t, app.Prepare(context.Background()))
	require.NotSame(t, oldLogger, slog.Default())
	assert.Equal(t, "process-restore", instance.ProcessDefaultSnapshot().AppName)

	err := app.Stop(context.Background())
	require.ErrorIs(t, err, tracerErr)
	assert.Equal(t, int32(1), atomic.LoadInt32(&tracerShutdowns))
	assert.Same(t, oldLogger, slog.Default())
	assert.Equal(t, oldTracer, otel.GetTracerProvider())
	assert.Equal(t, oldMeter, otel.GetMeterProvider())
	assert.Equal(t, oldPropagator, otel.GetTextMapPropagator())
	assert.Same(t, oldRemote, remotelog.Logger())
	assert.Equal(t, oldInstance, instance.ProcessDefaultSnapshot())

	next, _ := newTestAppWithConfig(
		t,
		"process-next",
		isolationConfig("next-ns"),
		WithProcessDefaults(true),
	)
	t.Cleanup(func() { _ = next.Stop(context.Background()) })
	require.NoError(t, next.Prepare(context.Background()))
}

type failingShutdownTracerProvider struct {
	trace.TracerProvider
	shutdowns *int32
	err       error
}

func (p *failingShutdownTracerProvider) Shutdown(context.Context) error {
	atomic.AddInt32(p.shutdowns, 1)
	return p.err
}
