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
	"io"
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

type GreeterServer struct {
	helloworldpb.UnimplementedGreeterServiceServer
}

func (s *GreeterServer) SayHello(
	ctx context.Context,
	req *helloworldpb.SayHelloRequest,
) (*helloworldpb.SayHelloResponse, error) {
	slog.Info("SayHello called", "name", req.Name)

	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "streaming-server"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "streaming-server"))

	return &helloworldpb.SayHelloResponse{
		Message: fmt.Sprintf("Hello %s!", req.Name),
	}, nil
}

func (s *GreeterServer) SayHelloStream(
	stream helloworldpb.GreeterServiceSayHelloStreamServer,
) error {
	slog.Info("SayHelloStream started")

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			slog.Info("SayHelloStream client closed stream")
			return nil
		}
		if err != nil {
			slog.Error("SayHelloStream error", "error", err)
			return err
		}

		slog.Info("SayHelloStream received", "name", req.Name)

		resp := &helloworldpb.SayHelloStreamResponse{
			Message: fmt.Sprintf("Hello %s!", req.Name),
		}

		if err := stream.Send(resp); err != nil {
			slog.Error("SayHelloStream send error", "error", err)
			return err
		}
	}
}

func (s *GreeterServer) SayHelloClientStream(
	stream helloworldpb.GreeterServiceSayHelloClientStreamServer,
) error {
	slog.Info("SayHelloClientStream started")

	var names []string
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			slog.Info("SayHelloClientStream client closed stream")
			break
		}
		if err != nil {
			slog.Error("SayHelloClientStream error", "error", err)
			return err
		}

		slog.Info("SayHelloClientStream received", "name", req.Name)
		names = append(names, req.Name)
	}

	message := fmt.Sprintf("Hello %v!", names)
	resp := &helloworldpb.SayHelloClientStreamResponse{
		Message: message,
	}

	slog.Info("SayHelloClientStream sending response", "names", names)
	return stream.SendAndClose(resp)
}

func (s *GreeterServer) SayHelloServerStream(
	req *helloworldpb.SayHelloServerStreamRequest,
	stream helloworldpb.GreeterServiceSayHelloServerStreamServer,
) error {
	slog.Info("SayHelloServerStream started", "name", req.Name)

	for i := 0; i < 5; i++ {
		resp := &helloworldpb.SayHelloServerStreamResponse{
			Message: fmt.Sprintf("Hello %s! (message %d)", req.Name, i+1),
		}

		if err := stream.Send(resp); err != nil {
			slog.Error("SayHelloServerStream send error", "error", err)
			return err
		}

		time.Sleep(500 * time.Millisecond)
	}

	slog.Info("SayHelloServerStream completed")
	return nil
}

func (s *GreeterServer) SayError(
	_ context.Context,
	req *helloworldpb.SayErrorRequest,
) (*helloworldpb.SayErrorResponse, error) {
	slog.Info("SayError called", "name", req.Name)

	return &helloworldpb.SayErrorResponse{
		Message: fmt.Sprintf("Error: %s", req.Name),
	}, nil
}

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.example.advanced.streaming"); err != nil {
		os.Exit(1)
	}

	ss := &GreeterServer{}
	if err := yggdrasil.Serve(
		yggdrasil.WithServiceDesc(&helloworldpb.GreeterServiceServiceDesc, ss),
	); err != nil {
		os.Exit(1)
	}
}
