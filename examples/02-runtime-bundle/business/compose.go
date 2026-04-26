package business

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	"github.com/codesjoy/yggdrasil/v3/config"
	libraryv1 "github.com/codesjoy/yggdrasil/v3/examples/protogen/library/v1"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
)

const AppName = "github.com.codesjoy.yggdrasil.example.02-runtime-bundle"

type runtimeBundleConfig struct {
	ShelfTheme string `mapstructure:"shelf_theme"`
	RawMessage string `mapstructure:"raw_message"`
	TaskLabel  string `mapstructure:"task_label"`
}

// Compose builds a bundle that exercises the main business installation surface.
func Compose(rt yapp.Runtime) (*yapp.BusinessBundle, error) {
	cfg := runtimeBundleConfig{}
	if manager := rt.Config(); manager != nil {
		if err := manager.Section("app", "runtime_bundle").Decode(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.ShelfTheme == "" {
		cfg.ShelfTheme = "Reference"
	}
	if cfg.RawMessage == "" {
		cfg.RawMessage = "runtime bundle ready"
	}
	if cfg.TaskLabel == "" {
		cfg.TaskLabel = "bundle-heartbeat"
	}

	var manager *config.Manager
	managerAvailable := rt.Lookup(&manager) == nil && manager != nil

	rt.Logger().Info(
		"compose runtime bundle",
		"shelf_theme",
		cfg.ShelfTheme,
		"config_lookup",
		managerAvailable,
		"tracer_provider",
		rt.TracerProvider() != nil,
		"meter_provider",
		rt.MeterProvider() != nil,
	)

	lib := &libraryService{
		logger: rt.Logger(),
		theme:  cfg.ShelfTheme,
	}

	return &yapp.BusinessBundle{
		RPCBindings: []yapp.RPCBinding{{
			ServiceName: libraryv1.LibraryServiceServiceDesc.ServiceName,
			Desc:        &libraryv1.LibraryServiceServiceDesc,
			Impl:        lib,
		}},
		RESTBindings: []yapp.RESTBinding{{
			Name: "runtime-bundle-rest",
			Desc: &libraryv1.LibraryServiceRestServiceDesc,
			Impl: lib,
		}},
		RawHTTP: []yapp.RawHTTPBinding{{
			Method:  http.MethodGet,
			Path:    "/healthz",
			Handler: rawHealthHandler(cfg.RawMessage),
		}},
		Tasks: []yapp.BackgroundTask{
			newPeriodicLogTask(rt.Logger(), cfg.TaskLabel, 2*time.Second),
		},
		Hooks: []yapp.BusinessHook{
			{
				Name:  "bundle.before_start",
				Stage: yapp.BusinessHookBeforeStart,
				Func: func(context.Context) error {
					rt.Logger().Info("before start hook", "task", cfg.TaskLabel)
					return nil
				},
			},
			{
				Name:  "bundle.before_stop",
				Stage: yapp.BusinessHookBeforeStop,
				Func: func(context.Context) error {
					rt.Logger().Info("before stop hook", "task", cfg.TaskLabel)
					return nil
				},
			},
			{
				Name:  "bundle.after_stop",
				Stage: yapp.BusinessHookAfterStop,
				Func: func(context.Context) error {
					rt.Logger().Info("after stop hook", "task", cfg.TaskLabel)
					return nil
				},
			},
		},
		Diagnostics: []yapp.BundleDiag{
			{
				Code:    "runtime.bundle.config",
				Message: "shelf_theme=" + cfg.ShelfTheme,
			},
			{
				Code:    "runtime.bundle.lookup",
				Message: fmt.Sprintf("config_manager_available=%t", managerAvailable),
			},
			{
				Code:    "runtime.bundle.raw_http",
				Message: "GET /healthz",
			},
		},
	}, nil
}

type libraryService struct {
	libraryv1.UnimplementedLibraryServiceServer
	logger *slog.Logger
	theme  string
}

func (s *libraryService) GetShelf(
	ctx context.Context,
	req *libraryv1.GetShelfRequest,
) (*libraryv1.Shelf, error) {
	s.logger.Info("GetShelf", "name", req.GetName())
	_ = metadata.SetHeader(ctx, metadata.Pairs("example", "runtime-bundle"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "get_shelf"))
	return &libraryv1.Shelf{
		Name:  req.GetName(),
		Theme: s.theme,
	}, nil
}

func (s *libraryService) ListShelves(
	ctx context.Context,
	_ *libraryv1.ListShelvesRequest,
) (*libraryv1.ListShelvesResponse, error) {
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "list_shelves"))
	return &libraryv1.ListShelvesResponse{
		Shelves: []*libraryv1.Shelf{
			{Name: "runtime-bundle", Theme: s.theme},
			{Name: "reference", Theme: "Docs"},
		},
	}, nil
}

func (s *libraryService) GetBook(
	ctx context.Context,
	req *libraryv1.GetBookRequest,
) (*libraryv1.Book, error) {
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "get_book"))
	return &libraryv1.Book{
		Name:   req.GetName(),
		Author: "Yggdrasil",
		Title:  "Runtime Bundle",
		Read:   true,
	}, nil
}

func rawHealthHandler(message string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(message))
	}
}

type periodicLogTask struct {
	logger   *slog.Logger
	label    string
	interval time.Duration
	stopOnce sync.Once
	stopCh   chan struct{}
}

func newPeriodicLogTask(
	logger *slog.Logger,
	label string,
	interval time.Duration,
) *periodicLogTask {
	return &periodicLogTask{
		logger:   logger,
		label:    label,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (t *periodicLogTask) Serve() error {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.logger.Info("background task heartbeat", "task", t.label)
		case <-t.stopCh:
			return nil
		}
	}
}

func (t *periodicLogTask) Stop(context.Context) error {
	t.stopOnce.Do(func() {
		close(t.stopCh)
	})
	return nil
}
