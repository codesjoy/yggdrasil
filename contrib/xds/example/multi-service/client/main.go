package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	_ "github.com/codesjoy/yggdrasil/contrib/xds/v2"
	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	helloworldv1 "github.com/codesjoy/yggdrasil/v2/example/protogen/helloworld"
	libraryv1 "github.com/codesjoy/yggdrasil/v2/example/protogen/library/v1"
	_ "github.com/codesjoy/yggdrasil/v2/interceptor/logging"
	"github.com/codesjoy/yggdrasil/v2/metadata"
)

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", "error", err)
		os.Exit(1)
	}

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.xds.example.multi-service.client"); err != nil {
		slog.Error("failed to initialize yggdrasil", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting multi-service client...")

	libraryClient, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.library")
	if err != nil {
		slog.Error("failed to create library client", "error", err)
		os.Exit(1)
	}
	defer libraryClient.Close()

	greeterClient, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.greeter")
	if err != nil {
		slog.Error("failed to create greeter client", "error", err)
		os.Exit(1)
	}
	defer greeterClient.Close()

	library := libraryv1.NewLibraryServiceClient(libraryClient)
	greeter := helloworldv1.NewGreeterServiceClient(greeterClient)

	slog.Info("Starting multi-service test loop...")

	for i := 1; i <= 20; i++ {
		time.Sleep(500 * time.Millisecond)

		if i%2 == 0 {
			ctx := metadata.WithStreamContext(context.Background())
			shelf, err := library.GetShelf(ctx, &libraryv1.GetShelfRequest{
				Name: "shelf-" + strconv.Itoa(i),
			})
			if err != nil {
				slog.Error("Library service call failed", "error", err)
				continue
			}
			slog.Info("Library service response", "name", shelf.Name, "theme", shelf.Theme)
		} else {
			ctx := metadata.WithStreamContext(context.Background())
			response, err := greeter.SayHello(ctx, &helloworldv1.SayHelloRequest{
				Name: "world",
			})
			if err != nil {
				slog.Error("Greeter service call failed", "error", err)
				continue
			}
			slog.Info("Greeter service response", "message", response.Message)
		}
	}

	slog.Info("Multi-service client completed successfully")
}
