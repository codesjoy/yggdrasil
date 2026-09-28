// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	serverName = "github.com/codesjoy.yggdrasil.example.20-capability-registration"
)

func main() {
	if err := run(); err != nil {
		slog.Error("capability registration client failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	app, err := yapp.New(
		"github.com/codesjoy.yggdrasil.example.20-capability-registration.client",
		yapp.WithConfigPath("config.yaml"),
		yapp.WithCapabilityRegistrations(grpcx.NewRegistration()),
	)
	if err != nil {
		return fmt.Errorf("create client app: %w", err)
	}
	defer func() {
		_ = app.Stop(context.Background())
	}()

	cli, err := app.NewClient(ctx, serverName)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
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
		return fmt.Errorf("call SayHello: %w", err)
	}

	fmt.Println(reply.GetMessage())
	return nil
}
