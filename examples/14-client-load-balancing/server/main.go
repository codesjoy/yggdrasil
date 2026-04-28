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
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
	"github.com/codesjoy/yggdrasil/v3/examples/14-client-load-balancing/server/business"
)

var (
	portFlag = flag.Int("port", 0, "Server port (default: use config file)")
	hostname = "lb-server"
)

func main() {
	flag.Parse()

	port := *portFlag
	if port == 0 {
		port = 55884
	}

	instanceID := fmt.Sprintf("%s-%d", hostname, port)
	slog.Info("Starting load balancing server", "instance", instanceID, "port", port)

	err := yggdrasil.Run(
		context.Background(),
		business.ServerAppName(port),
		business.Compose(instanceID),
		yggdrasil.WithConfigPath("config.yaml"),
		yggdrasil.WithConfigSource(
			"example:port",
			config.PriorityOverride,
			memory.NewSource("example:port", map[string]any{
				"yggdrasil": map[string]any{
					"transports": map[string]any{
						"grpc": map[string]any{
							"server": map[string]any{
								"address": fmt.Sprintf("127.0.0.1:%d", port),
							},
						},
					},
				},
			}),
		),
	)
	if err != nil {
		slog.Error("run app", slog.Any("error", err))
		os.Exit(1)
	}
}
