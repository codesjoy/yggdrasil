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
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/file"
	helloworldpb "github.com/codesjoy/yggdrasil/v3/example/protogen/helloworld"
	"github.com/codesjoy/yggdrasil/v3/metadata"
)

var (
	portFlag = flag.Int("port", 0, "Server port (default: use config file)")
	hostname = "lb-server"
)

type GreeterServer struct {
	helloworldpb.UnimplementedGreeterServiceServer
	instanceID string
}

func (s *GreeterServer) SayHello(
	ctx context.Context,
	req *helloworldpb.SayHelloRequest,
) (*helloworldpb.SayHelloResponse, error) {
	slog.Info("SayHello called", "name", req.Name, "instance", s.instanceID)

	_ = metadata.SetTrailer(ctx, metadata.Pairs(
		"server", s.instanceID,
		"instance-type", "load-balancing",
	))

	return &helloworldpb.SayHelloResponse{
		Message: fmt.Sprintf("Hello %s! from %s", req.Name, s.instanceID),
	}, nil
}

func (s *GreeterServer) SayHelloStream(
	stream helloworldpb.GreeterServiceSayHelloStreamServer,
) error {
	slog.Info("SayHelloStream started", "instance", s.instanceID)

	for {
		req, err := stream.Recv()
		if err != nil {
			slog.Info("SayHelloStream client closed stream", "instance", s.instanceID)
			return nil
		}
		if err != nil {
			slog.Error("SayHelloStream error", "error", err)
			return err
		}

		resp := &helloworldpb.SayHelloStreamResponse{
			Message: fmt.Sprintf("Hello %s! from %s", req.Name, s.instanceID),
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
	slog.Info("SayHelloClientStream started", "instance", s.instanceID)

	var names []string
	for {
		req, err := stream.Recv()
		if err != nil {
			slog.Info("SayHelloClientStream client closed stream", "instance", s.instanceID)
			break
		}
		if err != nil {
			slog.Error("SayHelloClientStream error", "error", err)
			return err
		}

		slog.Info("SayHelloClientStream received", "name", req.Name, "instance", s.instanceID)
		names = append(names, req.Name)
	}

	message := fmt.Sprintf("Hello %v! from %s", names, s.instanceID)
	resp := &helloworldpb.SayHelloClientStreamResponse{
		Message: message,
	}

	slog.Info("SayHelloClientStream sending response", "names", names, "instance", s.instanceID)
	return stream.SendAndClose(resp)
}

func (s *GreeterServer) SayHelloServerStream(
	req *helloworldpb.SayHelloServerStreamRequest,
	stream helloworldpb.GreeterServiceSayHelloServerStreamServer,
) error {
	slog.Info("SayHelloServerStream started", "name", req.Name, "instance", s.instanceID)

	for i := 0; i < 5; i++ {
		resp := &helloworldpb.SayHelloServerStreamResponse{
			Message: fmt.Sprintf("Hello %s! (message %d) from %s", req.Name, i+1, s.instanceID),
		}

		if err := stream.Send(resp); err != nil {
			slog.Error("SayHelloServerStream send error", "error", err)
			return err
		}
	}

	slog.Info("SayHelloServerStream completed", "instance", s.instanceID)
	return nil
}

func main() {
	flag.Parse()

	port := *portFlag
	if port == 0 {
		port = 55884
	}

	instanceID := fmt.Sprintf("%s-%d", hostname, port)

	slog.Info("Starting load balancing server", "instance", instanceID, "port", port)

	if err := config.Default().LoadLayer("example:file", config.PriorityFile, file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}

	ss := &GreeterServer{
		instanceID: instanceID,
	}
	app, err := yggdrasil.New(
		fmt.Sprintf("github.com.codesjoy.yggdrasil.example.advanced.load-balancing.%d", port),
		yggdrasil.WithRPCService(&helloworldpb.GreeterServiceServiceDesc, ss),
	)
	if err != nil {
		os.Exit(1)
	}
	if err := app.Start(context.Background()); err != nil {
		os.Exit(1)
	}
}
