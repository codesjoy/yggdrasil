package business

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client"
)

func TestComposeBuildsRuntimeBundle(t *testing.T) {
	t.Helper()

	manager := config.NewManager()
	if err := manager.LoadLayer(
		"test",
		config.PriorityFile,
		memory.NewSource("test", map[string]any{
			"app": map[string]any{
				"runtime_bundle": map[string]any{
					"shelf_theme": "Reference",
					"raw_message": "ok",
					"task_label":  "test-task",
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

	if len(bundle.RPCBindings) != 1 {
		t.Fatalf("RPCBindings = %d, want 1", len(bundle.RPCBindings))
	}
	if len(bundle.RESTBindings) != 1 {
		t.Fatalf("RESTBindings = %d, want 1", len(bundle.RESTBindings))
	}
	if len(bundle.RawHTTP) != 1 {
		t.Fatalf("RawHTTP = %d, want 1", len(bundle.RawHTTP))
	}
	if len(bundle.Tasks) != 1 {
		t.Fatalf("Tasks = %d, want 1", len(bundle.Tasks))
	}
	if len(bundle.Hooks) != 3 {
		t.Fatalf("Hooks = %d, want 3", len(bundle.Hooks))
	}
	if len(bundle.Diagnostics) != 3 {
		t.Fatalf("Diagnostics = %d, want 3", len(bundle.Diagnostics))
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

func (rt fakeRuntime) Lookup(target any) error {
	ptr, ok := target.(**config.Manager)
	if !ok {
		return errors.New("unsupported lookup target")
	}
	*ptr = rt.manager
	return nil
}

var _ yapp.Runtime = fakeRuntime{}
