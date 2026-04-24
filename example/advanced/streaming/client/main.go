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
	"io"
	"log/slog"
	"os"
	"time"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/file"
	helloworldpb "github.com/codesjoy/yggdrasil/v3/example/protogen/helloworld"
	"github.com/codesjoy/yggdrasil/v3/metadata"
)

func main() {
	if err := config.Default().LoadLayer("example:file", config.PriorityFile, file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}
	app, err := yapp.New("github.com.codesjoy.yggdrasil.example.advanced.streaming.client")
	if err != nil {
		os.Exit(1)
	}

	cli, err := app.NewClient(context.Background(), "github.com.codesjoy.yggdrasil.example.advanced.streaming")
	if err != nil {
		slog.Error("failed to create client", slog.Any("error", err))
		os.Exit(1)
	}
	defer cli.Close()

	client := helloworldpb.NewGreeterServiceClient(cli)
	ctx := metadata.WithStreamContext(context.Background())

	slog.Info("=== Testing Unary RPC ===")
	if err := testUnaryRPC(ctx, client); err != nil {
		slog.Error("unary rpc failed", slog.Any("error", err))
	}

	slog.Info("=== Testing Bidirectional Streaming ===")
	if err := testBidirectionalStreaming(ctx, client); err != nil {
		slog.Error("bidirectional streaming failed", slog.Any("error", err))
	}

	slog.Info("=== Testing Client Streaming ===")
	if err := testClientStreaming(ctx, client); err != nil {
		slog.Error("client streaming failed", slog.Any("error", err))
	}

	slog.Info("=== Testing Server Streaming ===")
	if err := testServerStreaming(ctx, client); err != nil {
		slog.Error("server streaming failed", slog.Any("error", err))
	}

	slog.Info("All streaming tests completed successfully!")
}

func testUnaryRPC(ctx context.Context, client helloworldpb.GreeterServiceClient) error {
	slog.Info("Calling SayHello...")

	resp, err := client.SayHello(ctx, &helloworldpb.SayHelloRequest{
		Name: "World",
	})
	if err != nil {
		return err
	}

	slog.Info("SayHello response", "message", resp.Message)

	if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
		slog.Info("Response trailer", "trailer", trailer)
	}
	if header, ok := metadata.FromHeaderCtx(ctx); ok {
		slog.Info("Response header", "header", header)
	}

	return nil
}

func testBidirectionalStreaming(
	ctx context.Context,
	client helloworldpb.GreeterServiceClient,
) error {
	slog.Info("Calling SayHelloStream...")

	stream, err := client.SayHelloStream(ctx)
	if err != nil {
		return err
	}

	names := []string{"Alice", "Bob", "Charlie"}

	errChan := make(chan error, 1)

	go func() {
		defer close(errChan)
		for i, name := range names {
			req := &helloworldpb.SayHelloStreamRequest{
				Name: name,
			}
			if err := stream.Send(req); err != nil {
				errChan <- err
				return
			}
			slog.Info("Sent message", "index", i, "name", name)
			time.Sleep(200 * time.Millisecond)
		}
		if err := stream.CloseSend(); err != nil {
			errChan <- err
			return
		}
	}()

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		slog.Info("Received message", "message", resp.Message)
	}

	return <-errChan
}

func testClientStreaming(ctx context.Context, client helloworldpb.GreeterServiceClient) error {
	slog.Info("Calling SayHelloClientStream...")

	stream, err := client.SayHelloClientStream(ctx)
	if err != nil {
		return err
	}

	names := []string{"David", "Eve", "Frank"}

	for i, name := range names {
		req := &helloworldpb.SayHelloClientStreamRequest{
			Name: name,
		}
		if err := stream.Send(req); err != nil {
			return err
		}
		slog.Info("Sent message", "index", i, "name", name)
		time.Sleep(200 * time.Millisecond)
	}

	err = stream.CloseSend()
	if err != nil {
		return err
	}

	slog.Info("SayHelloClientStream completed successfully")

	return nil
}

func testServerStreaming(ctx context.Context, client helloworldpb.GreeterServiceClient) error {
	slog.Info("Calling SayHelloServerStream...")

	stream, err := client.SayHelloServerStream(ctx, &helloworldpb.SayHelloServerStreamRequest{
		Name: "Grace",
	})
	if err != nil {
		return err
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		slog.Info("Received message", "message", resp.Message)
	}

	return nil
}
