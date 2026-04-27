package business

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client"
)

func TestComposeBuildsCustomCronBundle(t *testing.T) {
	manager := config.NewManager()
	if err := manager.LoadLayer(
		"test",
		config.PriorityFile,
		memory.NewSource("test", map[string]any{
			"app": map[string]any{
				"custom_cron": map[string]any{
					"schedule":  "@every 1s",
					"job_label": "test-cron",
				},
			},
		}),
	); err != nil {
		t.Fatalf("load config: %v", err)
	}

	bundle, err := Compose(fakeRuntime{
		manager: manager,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("Compose returned error: %v", err)
	}
	if bundle == nil {
		t.Fatal("Compose returned nil bundle")
	}
	if len(bundle.Extensions) != 1 {
		t.Fatalf("Extensions = %d, want 1", len(bundle.Extensions))
	}
	if len(bundle.RPCBindings) != 0 {
		t.Fatalf("RPCBindings = %d, want 0", len(bundle.RPCBindings))
	}
	if len(bundle.RESTBindings) != 0 {
		t.Fatalf("RESTBindings = %d, want 0", len(bundle.RESTBindings))
	}
	if len(bundle.Diagnostics) != 2 {
		t.Fatalf("Diagnostics = %d, want 2", len(bundle.Diagnostics))
	}
}

func TestComposeUsesCustomCronDefaults(t *testing.T) {
	bundle, err := Compose(fakeRuntime{
		manager: config.NewManager(),
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("Compose returned error: %v", err)
	}
	integration, ok := bundle.Extensions[0].(cronIntegration)
	if !ok {
		t.Fatalf("extension type = %T, want cronIntegration", bundle.Extensions[0])
	}
	if integration.schedule != "@every 5s" {
		t.Fatalf("schedule = %q, want %q", integration.schedule, "@every 5s")
	}
	if integration.jobLabel != "cron-heartbeat" {
		t.Fatalf("jobLabel = %q, want %q", integration.jobLabel, "cron-heartbeat")
	}
}

func TestCronIntegrationRejectsInvalidSchedule(t *testing.T) {
	integration := cronIntegration{
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		schedule: "not a schedule",
		jobLabel: "bad-cron",
	}
	err := integration.Install(&yapp.InstallContext{})
	if err == nil {
		t.Fatal("Install returned nil, want error")
	}
}

func TestCronTaskServeStops(t *testing.T) {
	task, err := newCronTask(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		"@every 1s",
		"test-cron",
	)
	if err != nil {
		t.Fatalf("newCronTask returned error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- task.Serve()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := task.Stop(ctx); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not stop")
	}
}

type fakeRuntime struct {
	manager *config.Manager
	logger  *slog.Logger
}

func (rt fakeRuntime) NewClient(context.Context, string) (client.Client, error) {
	return nil, errors.New("not used in this test")
}

func (rt fakeRuntime) Config() *config.Manager {
	return rt.manager
}

func (rt fakeRuntime) Logger() *slog.Logger {
	return rt.logger
}

func (rt fakeRuntime) TracerProvider() trace.TracerProvider {
	return otel.GetTracerProvider()
}

func (rt fakeRuntime) MeterProvider() metric.MeterProvider {
	return otel.GetMeterProvider()
}

func (rt fakeRuntime) Identity() yapp.Identity {
	return yapp.Identity{AppName: "test"}
}

func (rt fakeRuntime) Lookup(any) error {
	return errors.New("unsupported lookup target")
}

var _ yapp.Runtime = fakeRuntime{}
