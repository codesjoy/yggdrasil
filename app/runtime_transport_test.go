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

package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
)

func TestRuntimeSnapshotTransportProvidersBuildClientsAndServersFromExplicitBuilders(t *testing.T) {
	app, _ := newInitializedAppWithConfig(t, "runtime-transport-providers", map[string]any{
		"yggdrasil": map[string]any{
			"server": map[string]any{
				"transports": []any{"grpc", "http"},
			},
			"clients": map[string]any{
				"services": map[string]any{
					"svc": map[string]any{
						"remote": map[string]any{
							"endpoints": []any{
								map[string]any{
									"address":  "127.0.0.1:18080",
									"protocol": "http",
								},
							},
						},
					},
				},
			},
			"transports": map[string]any{
				"grpc": map[string]any{
					"client": map[string]any{
						"transport": map[string]any{
							"security_profile": "insecure-default",
						},
					},
					"server": map[string]any{},
				},
				"security": map[string]any{
					"profiles": map[string]any{
						"insecure-default": map[string]any{"type": "insecure"},
					},
				},
				"http": map[string]any{
					"client": map[string]any{},
					"server": map[string]any{},
				},
			},
		},
	})
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	snapshot := app.currentRuntimeSnapshot()
	require.NotNil(t, snapshot)

	for _, protocol := range []string{"grpc", "http"} {
		serverProvider := snapshot.TransportServerProvider(protocol)
		require.NotNil(t, serverProvider)
		server, err := serverProvider.NewServer(func(remote.ServerStream) {})
		require.NoError(t, err)
		require.Equal(t, protocol, server.Info().Protocol)

		clientProvider := snapshot.TransportClientProvider(protocol)
		require.NotNil(t, clientProvider)
		client, err := clientProvider.NewClient(
			context.Background(),
			"svc",
			resolver.BaseEndpoint{Protocol: protocol, Address: "127.0.0.1:65535"},
			stats.NoOpHandler,
			nil,
		)
		require.NoError(t, err)
		require.Equal(t, protocol, client.Protocol())
		require.NoError(t, client.Close())
	}
}
