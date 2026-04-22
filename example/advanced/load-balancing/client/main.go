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
	"sync"

	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/file"
	helloworldpb "github.com/codesjoy/yggdrasil/v3/example/protogen/helloworld"
	"github.com/codesjoy/yggdrasil/v3/metadata"
)

type LoadBalancerStats struct {
	mu                sync.Mutex
	requestCount      map[string]int
	errorCount        map[string]int
	responseInstances map[string]int
}

func (s *LoadBalancerStats) RecordRequest(instanceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.requestCount == nil {
		s.requestCount = make(map[string]int)
	}
	s.requestCount[instanceID]++
}

func (s *LoadBalancerStats) RecordError(instanceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.errorCount == nil {
		s.errorCount = make(map[string]int)
	}
	s.errorCount[instanceID]++
}

func (s *LoadBalancerStats) RecordResponse(instanceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.responseInstances == nil {
		s.responseInstances = make(map[string]int)
	}
	s.responseInstances[instanceID]++
}

func (s *LoadBalancerStats) Print() {
	s.mu.Lock()
	defer s.mu.Unlock()

	slog.Info("=== Load Balancer Statistics ===")
	slog.Info("Request distribution", "stats", s.requestCount)
	slog.Info("Error distribution", "stats", s.errorCount)
	slog.Info("Response distribution", "stats", s.responseInstances)
}

func main() {
	if err := config.Default().LoadLayer("example:file", config.PriorityFile, file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}
	app, err := yggdrasil.New("github.com.codesjoy.yggdrasil.example.advanced.load-balancing.client")
	if err != nil {
		os.Exit(1)
	}

	cli, err := app.NewClient("github.com.codesjoy.yggdrasil.example.advanced.load-balancing")
	if err != nil {
		slog.Error("failed to create client", slog.Any("error", err))
		os.Exit(1)
	}
	defer cli.Close()

	client := helloworldpb.NewGreeterServiceClient(cli)
	ctx := metadata.WithStreamContext(context.Background())

	stats := &LoadBalancerStats{}

	slog.Info("=== Testing Load Balancing ===")
	if err := testLoadBalancing(ctx, client, stats); err != nil {
		slog.Error("load balancing test failed", slog.Any("error", err))
	}

	slog.Info("=== Testing Stream Load Balancing ===")
	if err := testStreamLoadBalancing(ctx, client, stats); err != nil {
		slog.Error("stream load balancing test failed", slog.Any("error", err))
	}

	stats.Print()

	slog.Info("Load balancing tests completed successfully!")
}

func testLoadBalancing(
	ctx context.Context,
	client helloworldpb.GreeterServiceClient,
	stats *LoadBalancerStats,
) error {
	slog.Info("Testing unary RPC load balancing...")

	for i := 0; i < 10; i++ {
		resp, err := client.SayHello(ctx, &helloworldpb.SayHelloRequest{
			Name: fmt.Sprintf("User-%d", i),
		})
		if err != nil {
			slog.Error("request failed", "index", i, "error", err)
			stats.RecordError("unknown")
			continue
		}

		slog.Info("request succeeded", "index", i, "message", resp.Message)

		if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
			if instanceIDs, ok := trailer["server"]; ok && len(instanceIDs) > 0 {
				instanceID := instanceIDs[0]
				stats.RecordRequest(instanceID)
				stats.RecordResponse(instanceID)
				slog.Info("served by instance", "instance", instanceID)
			}
		}
	}

	return nil
}

func testStreamLoadBalancing(
	ctx context.Context,
	client helloworldpb.GreeterServiceClient,
	stats *LoadBalancerStats,
) error {
	slog.Info("Testing stream RPC load balancing...")

	stream, err := client.SayHelloStream(ctx)
	if err != nil {
		return err
	}

	names := []string{"Alice", "Bob", "Charlie", "David", "Eve"}

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
		}
		if err := stream.CloseSend(); err != nil {
			errChan <- err
			return
		}
	}()

	for i := 0; i < len(names); i++ {
		resp, err := stream.Recv()
		if err != nil {
			return err
		}

		slog.Info("Received message", "index", i, "message", resp.Message)

		if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
			if instanceIDs, ok := trailer["server"]; ok && len(instanceIDs) > 0 {
				instanceID := instanceIDs[0]
				stats.RecordRequest(instanceID)
				stats.RecordResponse(instanceID)
				slog.Info("served by instance", "instance", instanceID)
			}
		}
	}

	return <-errChan
}
