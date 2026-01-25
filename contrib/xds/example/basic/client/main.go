package main

import (
	"context"
	"log/slog"
	"os"

	_ "github.com/codesjoy/yggdrasil/contrib/xds/v2"
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

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.xds.example.basic.client"); err != nil {
		slog.Error("failed to initialize yggdrasil", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting xDS basic client...")

	cli, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.sample")
	if err != nil {
		slog.Error("failed to create client", "error", err)
		os.Exit(1)
	}
	defer cli.Close()

	client := librarypb.NewLibraryServiceClient(cli)
	ctx := metadata.WithStreamContext(context.Background())

	slog.Info("Calling GetShelf...")

	shelf, err := client.GetShelf(ctx, &librarypb.GetShelfRequest{
		Name: "shelves/1",
	})
	if err != nil {
		slog.Error("failed to call GetShelf", "error", err)
		os.Exit(1)
	}

	slog.Info("GetShelf response", "name", shelf.Name, "theme", shelf.Theme)

	if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
		slog.Info("Response trailer", "trailer", trailer)
	}
	if header, ok := metadata.FromHeaderCtx(ctx); ok {
		slog.Info("Response header", "header", header)
	}

	slog.Info("Calling CreateShelf...")

	newShelf, err := client.CreateShelf(ctx, &librarypb.CreateShelfRequest{
		Shelf: &librarypb.Shelf{
			Name:  "shelves/2",
			Theme: "History",
		},
	})
	if err != nil {
		slog.Error("failed to call CreateShelf", "error", err)
		os.Exit(1)
	}

	slog.Info("CreateShelf response", "name", newShelf.Name, "theme", newShelf.Theme)

	slog.Info("Calling ListShelves...")

	shelves, err := client.ListShelves(ctx, &librarypb.ListShelvesRequest{
		PageSize: 10,
	})
	if err != nil {
		slog.Error("failed to call ListShelves", "error", err)
		os.Exit(1)
	}

	slog.Info("ListShelves response", "count", len(shelves.Shelves))

	for i, shelf := range shelves.Shelves {
		slog.Info("Shelf", "index", i, "name", shelf.Name, "theme", shelf.Theme)
	}

	slog.Info("xDS basic client completed successfully")
}
