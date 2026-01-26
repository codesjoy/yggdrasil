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
	"log/slog"
	"os"
	"sync"

	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	librarypb "github.com/codesjoy/yggdrasil/v2/example/protogen/library/v1"
	_ "github.com/codesjoy/yggdrasil/v2/interceptor/logging"
	"github.com/codesjoy/yggdrasil/v2/metadata"
)

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", "error", err)
		os.Exit(1)
	}

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.xds.example.traffic-splitting.client"); err != nil {
		slog.Error("failed to initialize yggdrasil", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting traffic splitting client...")

	cli, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.sample")
	if err != nil {
		slog.Error("failed to create client", "error", err)
		os.Exit(1)
	}
	defer cli.Close()

	client := librarypb.NewLibraryServiceClient(cli)
	ctx := metadata.WithStreamContext(context.Background())

	slog.Info("Starting traffic splitting test...")

	requestCount := 100
	v1Count := 0
	v2Count := 0
	var mu sync.Mutex

	for i := 0; i < requestCount; i++ {
		shelf, err := client.GetShelf(ctx, &librarypb.GetShelfRequest{
			Name: "shelves/1",
		})
		if err != nil {
			slog.Error("failed to call GetShelf", "error", err)
			continue
		}

		mu.Lock()
		switch shelf.Theme {
		case "Traffic Splitting v1":
			v1Count++
		case "Traffic Splitting v2":
			v2Count++
		}
		mu.Unlock()

		slog.Info("GetShelf response", "index", i, "name", shelf.Name, "theme", shelf.Theme)
	}

	slog.Info(
		"Traffic splitting test completed",
		"total_requests",
		requestCount,
		"v1_count",
		v1Count,
		"v2_count",
		v2Count,
		"v1_percentage",
		float64(v1Count)/float64(requestCount)*100,
		"v2_percentage",
		float64(v2Count)/float64(requestCount)*100,
	)
}
