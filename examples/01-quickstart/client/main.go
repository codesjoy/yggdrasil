package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	"github.com/codesjoy/yggdrasil/v3/examples/01-quickstart/server/business"
	helloworldpb "github.com/codesjoy/yggdrasil/v3/examples/protogen/helloworld"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	app, err := yapp.New("client", yapp.WithConfigPath("config.yaml"))
	if err != nil {
		slog.Error("create client app", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			slog.Error("stop client app", slog.Any("error", err))
		}
	}()

	cli, err := app.NewClient(ctx, business.AppName)
	if err != nil {
		slog.Error("create client", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() { _ = cli.Close() }()

	client := helloworldpb.NewGreeterServiceClient(cli)
	resp, err := client.SayHello(ctx, &helloworldpb.SayHelloRequest{Name: "quickstart"})
	if err != nil {
		slog.Error("call SayHello", slog.Any("error", err))
		os.Exit(1)
	}

	fmt.Println(resp.GetMessage())
}
