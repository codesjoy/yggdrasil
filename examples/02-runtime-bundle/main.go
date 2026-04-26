package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/examples/02-runtime-bundle/business"
)

const diagnosticsURL = "http://127.0.0.1:56022/diagnostics?pretty=true"

func main() {
	slog.Info("start runtime bundle example", "diagnostics", diagnosticsURL)
	if err := yggdrasil.Run(
		context.Background(),
		business.Compose,
		yggdrasil.WithConfigPath("config.yaml"),
	); err != nil {
		slog.Error("run app", slog.Any("error", err))
		os.Exit(1)
	}
}
