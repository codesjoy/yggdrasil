package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/file"
	"github.com/codesjoy/yggdrasil/v3/examples/03-diagnostics-reload/business"
)

const diagnosticsURL = "http://127.0.0.1:56032/diagnostics?pretty=true"

func main() {
	slog.Info(
		"start diagnostics reload example",
		"diagnostics",
		diagnosticsURL,
		"watch_file",
		"reload.yaml",
	)

	if err := yggdrasil.Run(
		context.Background(),
		business.AppName,
		business.Compose,
		yggdrasil.WithConfigPath("config.yaml"),
		yggdrasil.WithConfigSource(
			"example:reload",
			config.PriorityOverride,
			file.NewSource("reload.yaml", true),
		),
	); err != nil {
		slog.Error("run app", slog.Any("error", err))
		os.Exit(1)
	}
}
