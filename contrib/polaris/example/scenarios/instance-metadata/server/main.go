package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	libraryv1 "github.com/codesjoy/yggdrasil/v2/example/protogen/library/v1"
	_ "github.com/codesjoy/yggdrasil/v2/interceptor/logging"
	_ "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc"

	_ "github.com/codesjoy/yggdrasil/contrib/polaris/v2"
)

type LibraryImpl struct {
	libraryv1.UnimplementedLibraryServiceServer
}

func (s *LibraryImpl) GetShelf(
	_ context.Context,
	request *libraryv1.GetShelfRequest,
) (*libraryv1.Shelf, error) {
	return &libraryv1.Shelf{Name: request.Name, Theme: "test"}, nil
}

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.polaris.example.instance_metadata.server"); err != nil {
		slog.Error("init failed", slog.Any("error", err))
		os.Exit(1)
	}

	svc := &LibraryImpl{}
	if err := yggdrasil.Serve(
		yggdrasil.WithServiceDesc(&libraryv1.LibraryServiceServiceDesc, svc),
	); err != nil {
		slog.Error("serve failed", slog.Any("error", err))
		os.Exit(1)
	}
}
