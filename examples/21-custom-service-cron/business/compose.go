package business

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	"github.com/robfig/cron/v3"
)

const AppName = "github.com.codesjoy.yggdrasil.example.21-custom-service-cron"

type customCronConfig struct {
	Schedule string `mapstructure:"schedule"`
	JobLabel string `mapstructure:"job_label"`
}

// Compose installs a custom cron service through the BusinessInstallable escape hatch.
func Compose(rt yapp.Runtime) (*yapp.BusinessBundle, error) {
	cfg := customCronConfig{}
	if manager := rt.Config(); manager != nil {
		if err := manager.Section("app", "custom_cron").Decode(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.Schedule == "" {
		cfg.Schedule = "@every 5s"
	}
	if cfg.JobLabel == "" {
		cfg.JobLabel = "cron-heartbeat"
	}

	rt.Logger().Info(
		"compose custom cron integration",
		"schedule",
		cfg.Schedule,
		"job_label",
		cfg.JobLabel,
	)

	return &yapp.BusinessBundle{
		Extensions: []yapp.BusinessInstallable{
			cronIntegration{
				logger:   rt.Logger(),
				schedule: cfg.Schedule,
				jobLabel: cfg.JobLabel,
			},
		},
		Diagnostics: []yapp.BundleDiag{
			{
				Code:    "custom.cron.schedule",
				Message: cfg.Schedule,
			},
			{
				Code:    "custom.cron.job",
				Message: cfg.JobLabel,
			},
		},
	}, nil
}

type cronIntegration struct {
	logger   *slog.Logger
	schedule string
	jobLabel string
}

func (i cronIntegration) Kind() string {
	return "custom.cron"
}

func (i cronIntegration) Install(ctx *yapp.InstallContext) error {
	task, err := newCronTask(i.logger, i.schedule, i.jobLabel)
	if err != nil {
		return err
	}
	return ctx.AddTask(task)
}

type cronTask struct {
	logger *slog.Logger
	cron   *cron.Cron

	stopOnce sync.Once
	stopped  chan struct{}
}

func newCronTask(logger *slog.Logger, schedule string, jobLabel string) (*cronTask, error) {
	if logger == nil {
		logger = slog.Default()
	}
	c := cron.New(
		cron.WithChain(cron.Recover(cronSlogLogger{logger: logger})),
		cron.WithLogger(cronSlogLogger{logger: logger}),
	)
	if _, err := c.AddFunc(schedule, func() {
		logger.Info("cron job heartbeat", "job", jobLabel, "schedule", schedule)
	}); err != nil {
		return nil, fmt.Errorf("add cron job %q: %w", jobLabel, err)
	}
	return &cronTask{
		logger:  logger,
		cron:    c,
		stopped: make(chan struct{}),
	}, nil
}

func (t *cronTask) Serve() error {
	t.cron.Start()
	<-t.stopped
	return nil
}

func (t *cronTask) Stop(ctx context.Context) error {
	var cronCtx context.Context
	t.stopOnce.Do(func() {
		cronCtx = t.cron.Stop()
	})
	if cronCtx == nil {
		return nil
	}

	select {
	case <-cronCtx.Done():
		close(t.stopped)
		return nil
	case <-ctx.Done():
		close(t.stopped)
		return ctx.Err()
	}
}

type cronSlogLogger struct {
	logger *slog.Logger
}

func (l cronSlogLogger) Info(msg string, keysAndValues ...any) {
	l.logger.Info(msg, keysAndValues...)
}

func (l cronSlogLogger) Error(err error, msg string, keysAndValues ...any) {
	args := append([]any{slog.Any("error", err)}, keysAndValues...)
	l.logger.Error(msg, args...)
}
