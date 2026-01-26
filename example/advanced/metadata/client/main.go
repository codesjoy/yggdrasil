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

	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	helloworldpb "github.com/codesjoy/yggdrasil/v2/example/protogen/helloworld"
	_ "github.com/codesjoy/yggdrasil/v2/interceptor/logging"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	_ "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc"
)

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}
	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.example.advanced.metadata.client"); err != nil {
		os.Exit(1)
	}

	cli, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.advanced.metadata")
	if err != nil {
		slog.Error("failed to create client", slog.Any("error", err))
		os.Exit(1)
	}
	defer cli.Close()

	client := helloworldpb.NewGreeterServiceClient(cli)

	slog.Info("=== Testing Request Metadata ===")
	if err := testRequestMetadata(client); err != nil {
		slog.Error("request metadata test failed", slog.Any("error", err))
	}

	slog.Info("=== Testing Response Header ===")
	if err := testResponseHeader(client); err != nil {
		slog.Error("response header test failed", slog.Any("error", err))
	}

	slog.Info("=== Testing Response Trailer ===")
	if err := testResponseTrailer(client); err != nil {
		slog.Error("response trailer test failed", slog.Any("error", err))
	}

	slog.Info("=== Testing Stream Metadata ===")
	if err := testStreamMetadata(client); err != nil {
		slog.Error("stream metadata test failed", slog.Any("error", err))
	}

	slog.Info("All metadata tests completed successfully!")
}

func testRequestMetadata(client helloworldpb.GreeterServiceClient) error {
	slog.Info("Calling SayHello with metadata...")

	ctx := metadata.WithOutContext(context.Background(),
		metadata.Pairs(
			"user-id", "12345",
			"trace-id", "trace-abc-123",
			"authorization", "Bearer secret-token",
			"client-version", "1.0.0",
		),
	)

	resp, err := client.SayHello(ctx, &helloworldpb.SayHelloRequest{
		Name: "World",
	})
	if err != nil {
		return err
	}

	slog.Info("SayHello response", "message", resp.Message)

	if header, ok := metadata.FromHeaderCtx(ctx); ok {
		slog.Info("Response header", "header", header)
	}

	if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
		slog.Info("Response trailer", "trailer", trailer)
	}

	return nil
}

func testResponseHeader(client helloworldpb.GreeterServiceClient) error {
	slog.Info("Calling SayHello to check response header...")

	ctx := metadata.WithOutContext(context.Background(),
		metadata.Pairs(
			"user-id", "67890",
			"trace-id", "trace-xyz-789",
		),
	)

	resp, err := client.SayHello(ctx, &helloworldpb.SayHelloRequest{
		Name: "HeaderTest",
	})
	if err != nil {
		return err
	}

	slog.Info("SayHello response", "message", resp.Message)

	if header, ok := metadata.FromHeaderCtx(ctx); ok {
		slog.Info("Response header", "header", header)

		if reqID, ok := header["request-id"]; ok {
			slog.Info("Request ID from header", "request-id", reqID)
		}

		if version, ok := header["server-version"]; ok {
			slog.Info("Server version from header", "server-version", version)
		}
	}

	return nil
}

func testResponseTrailer(client helloworldpb.GreeterServiceClient) error {
	slog.Info("Calling SayHelloClientStream to check response trailer...")

	ctx := metadata.WithOutContext(context.Background(),
		metadata.Pairs(
			"user-id", "99999",
			"trace-id", "trace-trailer-999",
		),
	)

	stream, err := client.SayHelloClientStream(ctx)
	if err != nil {
		return err
	}

	names := []string{"Alice", "Bob", "Charlie"}

	for _, name := range names {
		req := &helloworldpb.SayHelloClientStreamRequest{
			Name: name,
		}
		if err := stream.Send(req); err != nil {
			return err
		}
		slog.Info("Sent message", "name", name)
		time.Sleep(100 * time.Millisecond)
	}

	err = stream.CloseSend()
	if err != nil {
		return err
	}

	if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
		slog.Info("Response trailer", "trailer", trailer)

		if processedCount, ok := trailer["processed-count"]; ok {
			slog.Info("Processed count from trailer", "processed-count", processedCount)
		}

		if serverName, ok := trailer["server-name"]; ok {
			slog.Info("Server name from trailer", "server-name", serverName)
		}
	}

	return nil
}

func testStreamMetadata(client helloworldpb.GreeterServiceClient) error {
	slog.Info("Calling SayHelloStream with metadata...")

	ctx := metadata.WithOutContext(context.Background(),
		metadata.Pairs(
			"user-id", "88888",
			"trace-id", "trace-stream-888",
			"stream-id", "stream-xyz",
		),
	)

	stream, err := client.SayHelloStream(ctx)
	if err != nil {
		return err
	}

	for i := 0; i < 3; i++ {
		req := &helloworldpb.SayHelloStreamRequest{
			Name: fmt.Sprintf("StreamUser-%d", i),
		}

		if err := stream.Send(req); err != nil {
			return err
		}

		resp, err := stream.Recv()
		if err != nil {
			return err
		}

		slog.Info("Stream response", "message", resp.Message)
		time.Sleep(100 * time.Millisecond)
	}

	err = stream.CloseSend()
	if err != nil {
		return err
	}

	if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
		slog.Info("Stream trailer", "trailer", trailer)
	}

	return nil
}
