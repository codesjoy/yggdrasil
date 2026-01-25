package main

import (
	"context"
	"log/slog"
	"os"
	"sync"

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

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.xds.example.load-balancing.client"); err != nil {
		slog.Error("failed to initialize yggdrasil", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting load balancing client...")

	cli, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.sample")
	if err != nil {
		slog.Error("failed to create client", "error", err)
		os.Exit(1)
	}
	defer cli.Close()

	client := librarypb.NewLibraryServiceClient(cli)
	ctx := metadata.WithStreamContext(context.Background())

	slog.Info("Starting load balancing test...")

	requestCount := 30
	serverCounts := make(map[string]int)
	var mu sync.Mutex

	for i := 0; i < requestCount; i++ {
		shelf, err := client.GetShelf(ctx, &librarypb.GetShelfRequest{
			Name: "shelves/1",
		})
		if err != nil {
			slog.Error("failed to call GetShelf", "error", err)
			continue
		}

		serverID := "unknown"
		if header, ok := metadata.FromHeaderCtx(ctx); ok {
			if v, ok := header["server"]; ok && len(v) > 0 {
				serverID = v[0]
			}
		}

		mu.Lock()
		serverCounts[serverID]++
		mu.Unlock()

		slog.Info("GetShelf response", "index", i, "name", shelf.Name, "theme", shelf.Theme, "server", serverID)
	}

	slog.Info("Load balancing test completed", "total_requests", requestCount)
	slog.Info("Traffic Distribution:")
	for serverID, count := range serverCounts {
		slog.Info("Server", "server_id", serverID, "requests", count, "percentage", float64(count)/float64(requestCount)*100)
	}
}
