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

	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/file"
	helloworldpb "github.com/codesjoy/yggdrasil/v3/examples/protogen/helloworld"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
)

type GreeterServer struct {
	helloworldpb.UnimplementedGreeterServiceServer
}

func (s *GreeterServer) SayHello(
	ctx context.Context,
	req *helloworldpb.SayHelloRequest,
) (*helloworldpb.SayHelloResponse, error) {
	md, ok := metadata.FromInContext(ctx)
	if !ok {
		slog.Warn("no metadata found")
	} else {
		slog.Info("received request metadata", "metadata", md)
	}

	userID := md["user-id"]
	traceID := md["trace-id"]
	authorization := md["authorization"]

	slog.Info("request info",
		"user-id", userID,
		"trace-id", traceID,
		"has-authorization", len(authorization) > 0,
	)

	_ = metadata.SetHeader(ctx, metadata.Pairs(
		"server-version", "1.0.0",
		"request-id", fmt.Sprintf("req-%d", time.Now().Unix()),
		"processed-by", "metadata-server",
	))

	_ = metadata.SetTrailer(ctx, metadata.Pairs(
		"server", "metadata-server",
		"processing-time", "10ms",
		"status", "success",
	))

	return &helloworldpb.SayHelloResponse{
		Message: fmt.Sprintf("Hello %s!", req.Name),
	}, nil
}

func (s *GreeterServer) SayHelloStream(
	stream helloworldpb.GreeterServiceSayHelloStreamServer,
) error {
	slog.Info("SayHelloStream started")

	md, ok := metadata.FromInContext(stream.Context())
	if ok {
		slog.Info("stream metadata", "metadata", md)
	}

	_ = metadata.SetHeader(stream.Context(), metadata.Pairs(
		"stream-type", "bidirectional",
	))

	for {
		req, err := stream.Recv()
		if err != nil {
			_ = metadata.SetTrailer(stream.Context(), metadata.Pairs(
				"stream-status", "closed",
				"messages-count", "0",
			))
			return err
		}

		resp := &helloworldpb.SayHelloStreamResponse{
			Message: fmt.Sprintf("Hello %s!", req.Name),
		}

		if err := stream.Send(resp); err != nil {
			_ = metadata.SetTrailer(stream.Context(), metadata.Pairs(
				"stream-status", "error",
				"error", err.Error(),
			))
			return err
		}
	}
}

func (s *GreeterServer) SayHelloClientStream(
	stream helloworldpb.GreeterServiceSayHelloClientStreamServer,
) error {
	slog.Info("SayHelloClientStream started")

	md, ok := metadata.FromInContext(stream.Context())
	if ok {
		slog.Info("stream metadata", "metadata", md)
	}

	var names []string
	for {
		req, err := stream.Recv()
		if err != nil {
			break
		}

		slog.Info("SayHelloClientStream received", "name", req.Name)
		names = append(names, req.Name)
	}

	message := fmt.Sprintf("Hello %v!", names)
	resp := &helloworldpb.SayHelloClientStreamResponse{
		Message: message,
	}

	_ = metadata.SetTrailer(stream.Context(), metadata.Pairs(
		"server", "metadata-server",
		"processed-count", fmt.Sprintf("%d", len(names)),
	))

	slog.Info("SayHelloClientStream sending response", "names", names)
	return stream.SendAndClose(resp)
}

func (s *GreeterServer) SayHelloServerStream(
	req *helloworldpb.SayHelloServerStreamRequest,
	stream helloworldpb.GreeterServiceSayHelloServerStreamServer,
) error {
	slog.Info("SayHelloServerStream started", "name", req.Name)

	md, ok := metadata.FromInContext(stream.Context())
	if ok {
		slog.Info("stream metadata", "metadata", md)
	}

	_ = metadata.SetHeader(stream.Context(), metadata.Pairs(
		"stream-type", "server-streaming",
		"target-name", req.Name,
	))

	for i := 0; i < 5; i++ {
		resp := &helloworldpb.SayHelloServerStreamResponse{
			Message: fmt.Sprintf("Hello %s! (message %d)", req.Name, i+1),
		}

		if err := stream.Send(resp); err != nil {
			_ = metadata.SetTrailer(stream.Context(), metadata.Pairs(
				"stream-status", "error",
				"error", err.Error(),
				"messages-sent", fmt.Sprintf("%d", i),
			))
			return err
		}

		time.Sleep(100 * time.Millisecond)
	}

	_ = metadata.SetTrailer(stream.Context(), metadata.Pairs(
		"server", "metadata-server",
		"messages-sent", "5",
		"status", "completed",
	))

	slog.Info("SayHelloServerStream completed")
	return nil
}

func main() {
	if err := config.Default().LoadLayer("example:file", config.PriorityFile, file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}

	err := yggdrasil.Run(
		context.Background(),
		func(yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
			return &yggdrasil.BusinessBundle{
				RPCBindings: []yggdrasil.RPCBinding{
					{
						ServiceName: helloworldpb.GreeterServiceServiceDesc.ServiceName,
						Desc:        &helloworldpb.GreeterServiceServiceDesc,
						Impl:        &GreeterServer{},
					},
				},
			}, nil
		},
		yggdrasil.WithAppName("github.com.codesjoy.yggdrasil.example.advanced.metadata"),
	)
	if err != nil {
		os.Exit(1)
	}
}
