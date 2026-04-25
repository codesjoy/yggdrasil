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

package settings

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCompile_ServiceOverridesHonorExplicitZeroAndEmptyValues(t *testing.T) {
	root := decodeRoot(t, map[string]any{
		"yggdrasil": map[string]any{
			"transports": map[string]any{
				"grpc": map[string]any{
					"client": map[string]any{
						"connect_timeout": "3s",
						"transport": map[string]any{
							"user_agent": "base-ua",
						},
					},
				},
				"http": map[string]any{
					"client": map[string]any{
						"timeout": "10s",
					},
				},
			},
			"clients": map[string]any{
				"defaults": map[string]any{
					"resolver": "base-resolver",
					"balancer": "base-balancer",
					"backoff": map[string]any{
						"baseDelay": "1s",
						"maxDelay":  "8s",
					},
					"remote": map[string]any{
						"endpoints": []any{
							map[string]any{
								"address":  "base:1",
								"protocol": "grpc",
							},
						},
						"attributes": map[string]any{
							"base": "yes",
						},
					},
					"interceptors": map[string]any{
						"unary": []any{"base-unary"},
					},
				},
				"services": map[string]any{
					"svc": map[string]any{
						"resolver": "",
						"balancer": "",
						"backoff": map[string]any{
							"baseDelay": "0s",
							"maxDelay":  "0s",
						},
						"remote": map[string]any{
							"endpoints":  []any{},
							"attributes": map[string]any{},
						},
						"interceptors": map[string]any{
							"unary": []any{},
						},
						"transports": map[string]any{
							"grpc": map[string]any{
								"connect_timeout": "0s",
								"transport": map[string]any{
									"user_agent": "",
								},
							},
							"http": map[string]any{
								"timeout": "0s",
							},
						},
					},
				},
			},
		},
	})

	resolved, err := Compile(root)
	require.NoError(t, err)

	svc := resolved.Clients.Services["svc"]
	require.Equal(t, "", svc.Resolver)
	require.Equal(t, "", svc.Balancer)
	require.Equal(t, time.Duration(0), svc.Backoff.BaseDelay)
	require.Equal(t, time.Duration(0), svc.Backoff.MaxDelay)
	require.Empty(t, svc.Remote.Endpoints)
	require.Empty(t, svc.Remote.Attributes)
	require.Empty(t, svc.Interceptors.Unary)

	require.Equal(
		t,
		time.Duration(0),
		resolved.Transports.GRPC.ClientServices["svc"].ConnectTimeout,
	)
	require.Equal(t, "", resolved.Transports.GRPC.ClientServices["svc"].Transport.UserAgent)
	require.Equal(t, time.Duration(0), resolved.Transports.HTTP.ClientServices["svc"].Timeout)
}
