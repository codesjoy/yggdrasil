package main

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	librarypb "github.com/codesjoy/yggdrasil/v2/example/protogen/library/v1"
	_ "github.com/codesjoy/yggdrasil/v2/interceptor/logging"
	"github.com/codesjoy/yggdrasil/v2/metadata"
)

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", "error", err)
		os.Exit(1)
	}

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.xds.example.canary.client"); err != nil {
		slog.Error("failed to initialize yggdrasil", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting canary deployment client...")

	cli, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.sample")
	if err != nil {
		slog.Error("failed to create client", "error", err)
		os.Exit(1)
	}
	defer cli.Close()

	client := librarypb.NewLibraryServiceClient(cli)
	ctx := metadata.WithStreamContext(context.Background())

	slog.Info("Starting canary deployment test with progressive traffic increase...")

	stages := []struct {
		name        string
		percentage  int
		requests    int
		waitSeconds int
	}{
		{"Stage 1: 5% to stable", 5, 100, 5},
		{"Stage 2: 10% to stable", 10, 100, 5},
		{"Stage 3: 25% to stable", 25, 100, 5},
		{"Stage 4: 50% to stable", 50, 100, 5},
		{"Stage 5: 75% to stable", 75, 100, 5},
		{"Stage 6: 100% to stable", 100, 100, 0},
	}

	for _, stage := range stages {
		slog.Info("Starting stage", "stage_name", stage.name, "canary_percentage", stage.percentage)

		stableCount := 0
		canaryCount := 0
		var mu sync.Mutex

		for i := 0; i < stage.requests; i++ {
			shelf, err := client.GetShelf(ctx, &librarypb.GetShelfRequest{
				Name: "shelves/1",
			})
			if err != nil {
				slog.Error("failed to call GetShelf", "error", err)
				continue
			}

			mu.Lock()
			if shelf.Theme == "Stable Version" {
				stableCount++
			} else if shelf.Theme == "Canary Version" {
				canaryCount++
			}
			mu.Unlock()
		}

		actualCanaryPercentage := float64(canaryCount) / float64(stage.requests) * 100
		slog.Info("Stage completed",
			"stage_name", stage.name,
			"expected_canary_percentage", stage.percentage,
			"actual_canary_percentage", actualCanaryPercentage,
			"total_requests", stage.requests,
			"stable_count", stableCount,
			"canary_count", canaryCount)

		if stage.waitSeconds > 0 {
			slog.Info("Waiting before next stage...", "seconds", stage.waitSeconds)
			time.Sleep(time.Duration(stage.waitSeconds) * time.Second)
		}
	}

	slog.Info("Canary deployment test completed successfully")
}
