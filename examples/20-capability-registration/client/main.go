package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	grpcx "github.com/codesjoy/yggdrasil/v3/examples/20-capability-registration/grpcx"
	helloworld "github.com/codesjoy/yggdrasil/v3/examples/protogen/helloworld"
)

const (
	serverName = "github.com.codesjoy.yggdrasil.example.20-capability-registration"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	app, err := yapp.New(
		"github.com.codesjoy.yggdrasil.example.20-capability-registration.client",
		yapp.WithConfigPath("config.yaml"),
		yapp.WithCapabilityRegistrations(grpcx.NewRegistration()),
	)
	if err != nil {
		os.Exit(1)
	}
	defer func() {
		_ = app.Stop(context.Background())
	}()

	cli, err := app.NewClient(ctx, serverName)
	if err != nil {
		slog.Error("create client", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		_ = cli.Close()
	}()

	client := helloworld.NewGreeterServiceClient(cli)
	reply, err := client.SayHello(
		context.Background(),
		&helloworld.SayHelloRequest{Name: "extension"},
	)
	if err != nil {
		os.Exit(1)
	}

	fmt.Println(reply.GetMessage())
}
