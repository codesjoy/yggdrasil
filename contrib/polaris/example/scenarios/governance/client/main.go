package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	libraryv1 "github.com/codesjoy/yggdrasil/contrib/polaris/v2/example/protogen/library/v1"
	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	_ "github.com/codesjoy/yggdrasil/v2/interceptor/logging"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	_ "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc"
	"github.com/codesjoy/yggdrasil/v2/status"

	_ "github.com/codesjoy/yggdrasil/contrib/polaris/v2"
)

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}
	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.polaris.example.governance.client"); err != nil {
		slog.Error("init failed", slog.Any("error", err))
		os.Exit(1)
	}

	cli, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.contrib.polaris.example.governance.server")
	if err != nil {
		slog.Error("new client failed", slog.Any("error", err))
		os.Exit(1)
	}

	client := libraryv1.NewLibraryServiceClient(cli)
	ctx := metadata.WithStreamContext(context.Background())
	ctx = metadata.WithOutContext(ctx, metadata.Pairs(
		"env", "dev",
		"lane", "stable",
		"user_id", "123",
	))

	_, err = client.GetShelf(ctx, &libraryv1.GetShelfRequest{Name: "shelves/1"})
	if err != nil {
		slog.Error("GetShelf failed", slog.Any("error", err))
		os.Exit(1)
	}
	if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
		fmt.Println("trailer:", trailer)
	}
	if header, ok := metadata.FromHeaderCtx(ctx); ok {
		fmt.Println("header:", header)
	}

	_, err = client.MoveBook(context.Background(), &libraryv1.MoveBookRequest{Name: "shelves/1/books/1"})
	if err != nil {
		st := status.FromError(err)
		fmt.Println("reason:", st.ErrorInfo().Reason)
		fmt.Println("code:", st.Code())
		fmt.Println("httpCode:", st.HTTPCode())
	}
}
