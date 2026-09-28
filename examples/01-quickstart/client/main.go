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
	"github.com/codesjoy/yggdrasil/v3/examples/01-quickstart/server/business"
	helloworldpb "github.com/codesjoy/yggdrasil/v3/examples/protogen/helloworld"
)

func main() {
	if err := run(); err != nil {
		slog.Error("quickstart client failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	app, err := yapp.New("client", yapp.WithConfigPath("config.yaml"))
	if err != nil {
		return fmt.Errorf("create client app: %w", err)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			slog.Error("stop client app", slog.Any("error", err))
		}
	}()

	cli, err := app.NewClient(ctx, business.AppName)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	defer func() { _ = cli.Close() }()

	client := helloworldpb.NewGreeterServiceClient(cli)
	resp, err := client.SayHello(ctx, &helloworldpb.SayHelloRequest{Name: "quickstart"})
	if err != nil {
		return fmt.Errorf("call SayHello: %w", err)
	}

	fmt.Println(resp.GetMessage())
	return nil
}
