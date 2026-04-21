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
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	gkeepalive "google.golang.org/grpc/keepalive"

	"github.com/codesjoy/yggdrasil/v2/client"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/memory"
	"github.com/codesjoy/yggdrasil/v2/interceptor"
	"github.com/codesjoy/yggdrasil/v2/internal/backoff"
	"github.com/codesjoy/yggdrasil/v2/logger"
	xotel "github.com/codesjoy/yggdrasil/v2/otel"
	"github.com/codesjoy/yggdrasil/v2/registry"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/remote/credentials"
	"github.com/codesjoy/yggdrasil/v2/remote/marshaler"
	grpcprotocol "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc"
	protocolhttp "github.com/codesjoy/yggdrasil/v2/remote/protocol/http"
	"github.com/codesjoy/yggdrasil/v2/remote/rest"
	restmiddleware "github.com/codesjoy/yggdrasil/v2/remote/rest/middleware"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/server"
	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

func TestCatalogAccessorsAndDecodePayload(t *testing.T) {
	manager := config.NewManager()
	require.NoError(t, manager.LoadLayer("test", config.PriorityOverride, memory.NewSource("test", map[string]any{
		"yggdrasil": map[string]any{
			"server": map[string]any{
				"transports": []any{"grpc", "http"},
			},
			"transports": map[string]any{
				"grpc": map[string]any{
					"client": map[string]any{
						"network": "tcp4",
					},
				},
			},
			"clients": map[string]any{
				"services": map[string]any{
					"svc": map[string]any{
						"resolver": "discovery",
					},
				},
			},
			"logging": map[string]any{
				"handlers": map[string]any{
					"main": map[string]any{
						"type":   "json",
						"writer": "stdout",
					},
				},
				"writers": map[string]any{
					"stdout": map[string]any{
						"type": "console",
					},
				},
			},
			"discovery": map[string]any{
				"registry": map[string]any{
					"type": "memory",
				},
				"resolvers": map[string]any{
					"svc": map[string]any{
						"type": "static",
					},
				},
			},
		},
	})))

	catalog := NewCatalog(manager)

	root, err := catalog.Root().Current()
	require.NoError(t, err)
	require.Equal(t, "memory", root.Yggdrasil.Discovery.Registry.Type)

	svr, err := catalog.Server().Current()
	require.NoError(t, err)
	require.Equal(t, []string{"grpc", "http"}, svr.Transports)

	transports, err := catalog.Transports().Current()
	require.NoError(t, err)
	require.Equal(t, "tcp4", transports.GRPC.Client.Network)

	service, err := catalog.ClientService("svc").Current()
	require.NoError(t, err)
	require.NotNil(t, service.ServiceConfig.Resolver)
	require.Equal(t, "discovery", *service.ServiceConfig.Resolver)

	handler, err := catalog.LoggingHandler("main").Current()
	require.NoError(t, err)
	require.Equal(t, "json", handler.Type)
	require.Equal(t, "stdout", handler.Writer)

	writer, err := catalog.LoggingWriter("stdout").Current()
	require.NoError(t, err)
	require.Equal(t, "console", writer.Type)

	reg, err := catalog.Registry().Current()
	require.NoError(t, err)
	require.Equal(t, "memory", reg.Type)

	resolverSpec, err := catalog.Resolver("svc").Current()
	require.NoError(t, err)
	require.Equal(t, "static", resolverSpec.Type)

	defaultCatalog := NewCatalog(nil)
	_, err = defaultCatalog.Root().Current()
	require.NoError(t, err)

	var payload struct {
		Nested struct {
			Value string `mapstructure:"value"`
		} `mapstructure:"nested"`
	}
	require.NoError(t, DecodePayload(&payload, map[string]any{
		"nested": map[string]any{
			"value": "ok",
		},
	}))
	require.Equal(t, "ok", payload.Nested.Value)

	var badTarget struct{}
	require.Error(t, DecodePayload(badTarget, map[string]any{"x": 1}))
}

func TestCompile_AppliesDefaultsAndMergesOverrides(t *testing.T) {
	root := decodeRoot(t, map[string]any{
		"yggdrasil": map[string]any{
			"server": map[string]any{
				"transports": []any{"grpc"},
			},
			"transports": map[string]any{
				"grpc": map[string]any{
					"client": map[string]any{
						"network":             "tcp",
						"connect_timeout":     "1s",
						"wait_conn_timeout":   "2s",
						"max_send_msg_size":   128,
						"max_recv_msg_size":   256,
						"compressor":          "gzip",
						"back_off_max_delay":  "3s",
						"min_connect_timeout": "4s",
						"transport": map[string]any{
							"user_agent":               "ua-base",
							"creds_proto":              "base-creds",
							"authority":                "base-auth",
							"initial_window_size":      11,
							"initial_conn_window_size": 22,
							"write_buffer_size":        33,
							"read_buffer_size":         44,
							"max_header_list_size":     55,
						},
					},
					"server": map[string]any{
						"creds_proto": "server-creds",
					},
					"credentials": map[string]any{
						"base": map[string]any{"enabled": true},
					},
				},
				"http": map[string]any{
					"client": map[string]any{
						"timeout": "5s",
					},
					"rest": map[string]any{
						"middleware": map[string]any{
							"rpc": []any{"rpcmw"},
						},
					},
				},
			},
			"clients": map[string]any{
				"defaults": map[string]any{
					"fast_fail": true,
					"resolver":  "default-resolver",
					"balancer":  "roundrobin",
					"backoff": map[string]any{
						"baseDelay":  "1s",
						"multiplier": 2.0,
						"jitter":     0.1,
						"maxDelay":   "8s",
					},
					"remote": map[string]any{
						"attributes": map[string]any{
							"base": "yes",
						},
					},
					"interceptors": map[string]any{
						"unary":  []any{"u-base", "u-base"},
						"stream": []any{"s-base"},
					},
				},
				"services": map[string]any{
					"svc": map[string]any{
						"fast_fail": false,
						"resolver":  "svc-resolver",
						"balancer":  "svc-balancer",
						"backoff": map[string]any{
							"baseDelay":  "2s",
							"multiplier": 3.0,
							"jitter":     0.2,
							"maxDelay":   "10s",
						},
						"remote": map[string]any{
							"endpoints": []any{
								map[string]any{
									"address":  "127.0.0.1:8080",
									"protocol": "grpc",
								},
							},
							"attributes": map[string]any{
								"svc": "true",
							},
						},
						"interceptors": map[string]any{
							"unary":  []any{"u-svc", "", "u-base"},
							"stream": []any{"s-svc"},
						},
						"transports": map[string]any{
							"grpc": map[string]any{
								"connect_timeout": "9s",
								"transport": map[string]any{
									"user_agent":               "ua-svc",
									"creds_proto":              "svc-creds",
									"authority":                "svc-auth",
									"keepalive_params":         map[string]any{"time": "1s"},
									"initial_window_size":      66,
									"initial_conn_window_size": 77,
									"write_buffer_size":        88,
									"read_buffer_size":         99,
									"max_header_list_size":     111,
								},
								"credentials": map[string]any{
									"svc": map[string]any{"enabled": true},
								},
							},
							"http": map[string]any{
								"timeout": "12s",
								"marshaler": map[string]any{
									"inbound": map[string]any{"type": "jsonpb"},
								},
							},
						},
					},
				},
			},
		},
	})

	resolved, err := Compile(root)
	require.NoError(t, err)

	require.True(t, resolved.Server.RestEnabled)
	require.Equal(t, "text", resolved.Logging.Handlers["default"].Type)
	require.Equal(t, "default", resolved.Logging.Handlers["default"].Writer)
	require.Equal(t, logger.WriterSpec{Type: "console"}, resolved.Logging.Writers["default"])
	require.Equal(t, "error", resolved.Logging.RemoteLevel)
	require.NotNil(t, resolved.Discovery.Resolvers)
	require.NotNil(t, resolved.Balancers.Defaults)
	require.NotNil(t, resolved.Balancers.Services)
	require.NotNil(t, resolved.Transports.Rest)

	svcClient := resolved.Clients.Services["svc"]
	require.False(t, svcClient.FastFail)
	require.Equal(t, "svc-resolver", svcClient.Resolver)
	require.Equal(t, "svc-balancer", svcClient.Balancer)
	require.Equal(t, backoff.Config{
		BaseDelay:  2 * time.Second,
		Multiplier: 3,
		Jitter:     0.2,
		MaxDelay:   10 * time.Second,
	}, svcClient.Backoff)
	require.Len(t, svcClient.Remote.Endpoints, 1)
	require.Equal(t, "yes", svcClient.Remote.Attributes["base"])
	require.Equal(t, "true", svcClient.Remote.Attributes["svc"])
	require.Equal(t, []string{"u-base", "u-svc"}, svcClient.Interceptors.Unary)
	require.Equal(t, []string{"s-base", "s-svc"}, svcClient.Interceptors.Stream)

	svcGRPC := resolved.Transports.GRPC.ClientServices["svc"]
	require.Equal(t, 9*time.Second, svcGRPC.ConnectTimeout)
	require.Equal(t, "gzip", svcGRPC.Compressor)
	require.Equal(t, "ua-svc", svcGRPC.Transport.UserAgent)
	require.Equal(t, "svc-creds", svcGRPC.Transport.CredsProto)
	require.Equal(t, "svc-auth", svcGRPC.Transport.Authority)
	require.Equal(t, gkeepalive.ClientParameters{Time: time.Second}, svcGRPC.Transport.KeepaliveParams)
	require.EqualValues(t, 66, svcGRPC.Transport.InitialWindowSize)
	require.EqualValues(t, 77, svcGRPC.Transport.InitialConnWindowSize)
	require.Equal(t, 88, svcGRPC.Transport.WriteBufferSize)
	require.Equal(t, 99, svcGRPC.Transport.ReadBufferSize)
	require.NotNil(t, svcGRPC.Transport.MaxHeaderListSize)
	require.EqualValues(t, 111, *svcGRPC.Transport.MaxHeaderListSize)

	svcHTTP := resolved.Transports.HTTP.ClientServices["svc"]
	require.Equal(t, 12*time.Second, svcHTTP.Timeout)
	require.NotNil(t, svcHTTP.Marshaler)
	require.Equal(t, "jsonpb", svcHTTP.Marshaler.Inbound.Type)

	require.Equal(t, map[string]map[string]any{
		"base": {"enabled": true},
	}, resolved.Transports.GRPCCredentials)
	require.Equal(t, map[string]map[string]map[string]any{
		"svc": {
			"svc": {"enabled": true},
		},
	}, resolved.Transports.GRPCServiceCredentials)

	cloned := resolved.Transports.GRPCCredentials
	cloned["base"]["enabled"] = false
	require.Equal(t, true, root.Yggdrasil.Transports.GRPC.Credentials["base"]["enabled"])
}

func TestCompile_InitializesNilMaps(t *testing.T) {
	resolved, err := Compile(Root{})
	require.NoError(t, err)
	require.NotNil(t, resolved.Logging.Handlers)
	require.NotNil(t, resolved.Logging.Writers)
	require.NotNil(t, resolved.Logging.Interceptors)
	require.NotNil(t, resolved.Discovery.Resolvers)
	require.NotNil(t, resolved.Balancers.Defaults)
	require.NotNil(t, resolved.Balancers.Services)
	require.NotNil(t, resolved.Clients.Services)
	require.NotNil(t, resolved.Transports.GRPC.ClientServices)
	require.NotNil(t, resolved.Transports.HTTP.ClientServices)
	require.NotNil(t, resolved.Transports.GRPCCredentials)
	require.NotNil(t, resolved.Transports.GRPCServiceCredentials)
}

func TestValidate_DisabledReturnsNil(t *testing.T) {
	require.NoError(t, Validate(Resolved{}))
}

func TestValidate_EnableNonStrictWarnsButDoesNotFail(t *testing.T) {
	resolved := Resolved{
		Admin: Admin{
			Validation: Validation{
				Enable: true,
			},
		},
		Discovery: Discovery{
			Registry: registry.Spec{Type: "missing-registry"},
			Resolvers: map[string]resolver.Spec{
				"svc": {Type: "missing-resolver"},
			},
		},
		Telemetry: Telemetry{
			Tracer: "missing-tracer",
			Meter:  "missing-meter",
			Stats: stats.Settings{
				Server: "missing-server-handler",
				Client: "missing-client-handler",
			},
		},
		Server: server.Settings{
			Transports: []string{"missing-server"},
			Interceptors: server.InterceptorConfig{
				Unary:  []string{"missing-unary-server"},
				Stream: []string{"missing-stream-server"},
			},
		},
		Root: Root{
			Yggdrasil: Framework{
				Clients: Clients{
					Defaults: ClientDefaults{
						ServiceConfig: client.ServiceConfig{
							Interceptors: client.InterceptorConfig{
								Unary:  []string{"missing-default-unary"},
								Stream: []string{"missing-default-stream"},
							},
						},
					},
				},
			},
		},
		Clients: client.Settings{
			Services: map[string]client.ServiceConfig{
				"svc": {
					Interceptors: client.InterceptorConfig{
						Unary:  []string{"missing-service-unary"},
						Stream: []string{"missing-service-stream"},
					},
				},
			},
		},
		Transports: ResolvedTransports{
			GRPC: grpcprotocol.Settings{
				Server: grpcprotocol.ServerConfig{
					CredsProto: "missing-server-creds",
				},
				Client: grpcprotocol.Config{
					Transport: grpcprotocol.ClientTransportOptions{
						CredsProto: "missing-client-creds",
					},
				},
				ClientServices: map[string]grpcprotocol.Config{
					"svc": {
						Transport: grpcprotocol.ClientTransportOptions{
							CredsProto: "missing-service-creds",
						},
					},
				},
			},
			Rest: &rest.Config{
				Middleware: struct {
					RPC []string `mapstructure:"rpc"`
					Web []string `mapstructure:"web"`
					All []string `mapstructure:"all"`
				}{
					All: []string{"missing-rest-all"},
					RPC: []string{"missing-rest-rpc"},
					Web: []string{"missing-rest-web"},
				},
				Marshaler: struct {
					Support []string `mapstructure:"support"`
					Config  struct {
						JSONPB *marshaler.JSONPbConfig `mapstructure:"jsonpb"`
					} `mapstructure:"config"`
				}{
					Support: []string{"missing-marshaler"},
				},
			},
		},
	}

	require.NoError(t, Validate(resolved))
}

func TestValidate_EnableNonStrictUsesDefaultRestMarshalerSupport(t *testing.T) {
	require.NoError(t, Validate(Resolved{
		Admin: Admin{
			Validation: Validation{
				Enable: true,
			},
		},
		Discovery: Discovery{
			Resolvers: map[string]resolver.Spec{
				"blank": {},
			},
		},
		Transports: ResolvedTransports{
			Rest: &rest.Config{},
		},
	}))
}

func TestValidate_StrictReturnsJoinedErrorsAndHandlesInvalidCredentialConfig(t *testing.T) {
	unique := fmt.Sprintf("settings-test-%d", time.Now().UnixNano())
	registryName := unique + "-registry"
	resolverName := unique + "-resolver"
	tracerName := unique + "-tracer"
	meterName := unique + "-meter"
	statsName := unique + "-stats"
	serverProto := unique + "-server"
	serverCreds := unique + "-server-creds"
	clientCreds := unique + "-client-creds"
	serviceCreds := unique + "-service-creds"
	unaryServer := unique + "-unary-server"
	streamServer := unique + "-stream-server"
	unaryClientDefault := unique + "-unary-client-default"
	streamClientDefault := unique + "-stream-client-default"
	unaryClientService := unique + "-unary-client-service"
	streamClientService := unique + "-stream-client-service"
	restMW := unique + "-rest"
	restMarshal := unique + "-marshal"

	registry.RegisterBuilder(registryName, func(map[string]any) (registry.Registry, error) {
		return &testRegistry{}, nil
	})
	resolver.RegisterBuilder(resolverName, func(string) (resolver.Resolver, error) {
		return &testResolver{}, nil
	})
	xotel.RegisterTracerProviderBuilder(tracerName, func(string) trace.TracerProvider {
		return tracenoop.NewTracerProvider()
	})
	xotel.RegisterMeterProviderBuilder(meterName, func(string) metric.MeterProvider {
		return metricnoop.NewMeterProvider()
	})
	stats.RegisterHandlerBuilder(statsName, func(bool) stats.Handler {
		return stats.NoOpHandler
	})
	remote.RegisterServerBuilder(serverProto, func(remote.MethodHandle) (remote.Server, error) {
		return nil, nil
	})
	credentials.RegisterBuilder(serverCreds, func(string, bool) credentials.TransportCredentials {
		return nil
	})
	credentials.RegisterBuilder(clientCreds, func(string, bool) credentials.TransportCredentials {
		return nil
	})
	credentials.RegisterBuilder(serviceCreds, func(string, bool) credentials.TransportCredentials {
		return nil
	})
	interceptor.RegisterUnaryServerIntBuilder(unaryServer, func() interceptor.UnaryServerInterceptor {
		return func(ctx context.Context, req any, info *interceptor.UnaryServerInfo, handler interceptor.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	})
	interceptor.RegisterStreamServerIntBuilder(streamServer, func() interceptor.StreamServerInterceptor {
		return func(srv interface{}, ss stream.ServerStream, info *interceptor.StreamServerInfo, handler stream.Handler) error {
			return handler(srv, ss)
		}
	})
	interceptor.RegisterUnaryClientIntBuilder(unaryClientDefault, func(string) interceptor.UnaryClientInterceptor {
		return func(ctx context.Context, method string, req, reply any, invoker interceptor.UnaryInvoker) error {
			return invoker(ctx, method, req, reply)
		}
	})
	interceptor.RegisterStreamClientIntBuilder(streamClientDefault, func(string) interceptor.StreamClientInterceptor {
		return func(ctx context.Context, desc *stream.Desc, method string, streamer interceptor.Streamer) (stream.ClientStream, error) {
			return streamer(ctx, desc, method)
		}
	})
	interceptor.RegisterUnaryClientIntBuilder(unaryClientService, func(string) interceptor.UnaryClientInterceptor {
		return func(ctx context.Context, method string, req, reply any, invoker interceptor.UnaryInvoker) error {
			return invoker(ctx, method, req, reply)
		}
	})
	interceptor.RegisterStreamClientIntBuilder(streamClientService, func(string) interceptor.StreamClientInterceptor {
		return func(ctx context.Context, desc *stream.Desc, method string, streamer interceptor.Streamer) (stream.ClientStream, error) {
			return streamer(ctx, desc, method)
		}
	})
	restmiddleware.RegisterBuilder(restMW, func() func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler { return next }
	})
	marshaler.RegisterMarshallerBuilder(restMarshal, func() (marshaler.Marshaler, error) {
		return nil, errors.New("unused")
	})

	resolved := Resolved{
		Admin: Admin{
			Validation: Validation{
				Strict: true,
			},
		},
		Discovery: Discovery{
			Registry: registry.Spec{Type: registryName},
			Resolvers: map[string]resolver.Spec{
				"svc": {Type: resolverName},
			},
		},
		Telemetry: Telemetry{
			Tracer: tracerName,
			Meter:  meterName,
			Stats: stats.Settings{
				Server: statsName,
				Client: statsName,
			},
		},
		Server: server.Settings{
			Transports: []string{serverProto},
			Interceptors: server.InterceptorConfig{
				Unary:  []string{unaryServer},
				Stream: []string{streamServer},
			},
		},
		Root: Root{
			Yggdrasil: Framework{
				Clients: Clients{
					Defaults: ClientDefaults{
						ServiceConfig: client.ServiceConfig{
							Interceptors: client.InterceptorConfig{
								Unary:  []string{unaryClientDefault},
								Stream: []string{streamClientDefault},
							},
						},
					},
				},
			},
		},
		Clients: client.Settings{
			Services: map[string]client.ServiceConfig{
				"svc": {
					Interceptors: client.InterceptorConfig{
						Unary:  []string{unaryClientService},
						Stream: []string{streamClientService},
					},
				},
			},
		},
		Transports: ResolvedTransports{
			GRPC: grpcprotocol.Settings{
				Server: grpcprotocol.ServerConfig{
					CredsProto: serverCreds,
				},
				Client: grpcprotocol.Config{
					Transport: grpcprotocol.ClientTransportOptions{
						CredsProto: clientCreds,
					},
				},
				ClientServices: map[string]grpcprotocol.Config{
					"svc": {
						Transport: grpcprotocol.ClientTransportOptions{
							CredsProto: serviceCreds,
						},
					},
				},
			},
			Rest: &rest.Config{
				Middleware: struct {
					RPC []string `mapstructure:"rpc"`
					Web []string `mapstructure:"web"`
					All []string `mapstructure:"all"`
				}{
					All: []string{restMW},
					RPC: []string{restMW},
					Web: []string{restMW},
				},
				Marshaler: struct {
					Support []string `mapstructure:"support"`
					Config  struct {
						JSONPB *marshaler.JSONPbConfig `mapstructure:"jsonpb"`
					} `mapstructure:"config"`
				}{
					Support: []string{restMarshal},
				},
			},
		},
	}

	err := Validate(resolved)
	require.Error(t, err)
	require.ErrorContains(t, err, "remote credentials config invalid")
	require.ErrorContains(t, err, serverCreds)
	require.ErrorContains(t, err, clientCreds)
	require.ErrorContains(t, err, serviceCreds)
}

func TestMergeClientServiceConfig(t *testing.T) {
	fastFail := false
	base := client.ServiceConfig{
		FastFail: true,
		Resolver: "base-resolver",
		Balancer: "base-balancer",
		Backoff: backoff.Config{
			BaseDelay: time.Second,
			MaxDelay:  2 * time.Second,
		},
		Remote: client.RemoteConfig{
			Endpoints: []resolver.BaseEndpoint{
				{Address: "base:1"},
			},
			Attributes: map[string]any{
				"base": "yes",
			},
		},
		Interceptors: client.InterceptorConfig{
			Unary:  []string{"a", "a"},
			Stream: []string{"b"},
		},
	}
	overlay := clientServiceConfigOverlay{
		FastFail: &fastFail,
		Resolver: ptr("overlay-resolver"),
		Balancer: ptr("overlay-balancer"),
		Backoff: &backoffConfigOverlay{
			BaseDelay: ptr(3 * time.Second),
			MaxDelay:  ptr(4 * time.Second),
		},
		Remote: &remoteConfigOverlay{
			Endpoints: ptr([]resolver.BaseEndpoint{
				{Address: "overlay:1"},
			}),
			Attributes: ptr(map[string]any{
				"overlay": "yes",
			}),
		},
		Interceptors: &interceptorConfigOverlay{
			Unary:  ptr([]string{"c", "", "a"}),
			Stream: ptr([]string{"d"}),
		},
	}

	merged := mergeClientServiceConfig(base, overlay)

	require.False(t, merged.FastFail)
	require.Equal(t, "overlay-resolver", merged.Resolver)
	require.Equal(t, "overlay-balancer", merged.Balancer)
	require.Equal(t, backoff.Config{
		BaseDelay: 3 * time.Second,
		MaxDelay:  4 * time.Second,
	}, merged.Backoff)
	require.Equal(t, []resolver.BaseEndpoint{{Address: "overlay:1"}}, merged.Remote.Endpoints)
	require.Equal(t, map[string]any{
		"base":    "yes",
		"overlay": "yes",
	}, merged.Remote.Attributes)
	require.Equal(t, []string{"a", "c"}, merged.Interceptors.Unary)
	require.Equal(t, []string{"b", "d"}, merged.Interceptors.Stream)

	base.Remote.Attributes = nil
	merged = mergeClientServiceConfig(base, overlay)
	require.Equal(t, "yes", merged.Remote.Attributes["overlay"])
}

func TestMergeHTTPClientConfig(t *testing.T) {
	base := protocolhttp.ClientConfig{Timeout: time.Second}
	overlay := HTTPClientTransport{
		Timeout: ptr(2 * time.Second),
		Marshaler: &protocolhttp.MarshalerConfigSet{
			Inbound: &protocolhttp.MarshalerConfig{Type: "jsonpb"},
		},
	}

	merged := mergeHTTPClientConfig(base, overlay)
	require.Equal(t, 2*time.Second, merged.Timeout)
	require.Equal(t, "jsonpb", merged.Marshaler.Inbound.Type)

	require.Equal(t, base, mergeHTTPClientConfig(base, HTTPClientTransport{}))
}

func TestMergeGRPCClientConfigAndTransport(t *testing.T) {
	baseHeader := uint32(10)
	overlayHeader := uint32(20)
	base := grpcprotocol.Config{
		WaitConnTimeout:   time.Second,
		ConnectTimeout:    2 * time.Second,
		MaxSendMsgSize:    10,
		MaxRecvMsgSize:    20,
		Compressor:        "gzip",
		BackOffMaxDelay:   3 * time.Second,
		MinConnectTimeout: 4 * time.Second,
		Network:           "tcp",
		Transport: grpcprotocol.ClientTransportOptions{
			UserAgent:             "base-ua",
			CredsProto:            "base-creds",
			Authority:             "base-auth",
			InitialWindowSize:     1,
			InitialConnWindowSize: 2,
			WriteBufferSize:       3,
			ReadBufferSize:        4,
			MaxHeaderListSize:     &baseHeader,
		},
	}
	overlay := grpcClientConfigOverlay{
		ConnectTimeout:    ptr(5 * time.Second),
		MaxSendMsgSize:    ptr(30),
		MaxRecvMsgSize:    ptr(40),
		Compressor:        ptr("snappy"),
		BackOffMaxDelay:   ptr(6 * time.Second),
		MinConnectTimeout: ptr(7 * time.Second),
		Network:           ptr("unix"),
		Transport: grpcClientTransportOptionsOverlay{
			UserAgent:             ptr("overlay-ua"),
			CredsProto:            ptr("overlay-creds"),
			Authority:             ptr("overlay-auth"),
			KeepaliveParams:       ptr(gkeepalive.ClientParameters{Time: time.Second}),
			InitialWindowSize:     ptr(int32(11)),
			InitialConnWindowSize: ptr(int32(12)),
			WriteBufferSize:       ptr(13),
			ReadBufferSize:        ptr(14),
			MaxHeaderListSize:     &overlayHeader,
		},
	}

	merged := mergeGRPCClientConfig(base, overlay)
	require.Equal(t, time.Second, merged.WaitConnTimeout)
	require.Equal(t, 5*time.Second, merged.ConnectTimeout)
	require.Equal(t, 30, merged.MaxSendMsgSize)
	require.Equal(t, 40, merged.MaxRecvMsgSize)
	require.Equal(t, "snappy", merged.Compressor)
	require.Equal(t, 6*time.Second, merged.BackOffMaxDelay)
	require.Equal(t, 7*time.Second, merged.MinConnectTimeout)
	require.Equal(t, "unix", merged.Network)
	require.Equal(t, "overlay-ua", merged.Transport.UserAgent)
	require.Equal(t, "overlay-creds", merged.Transport.CredsProto)
	require.Equal(t, "overlay-auth", merged.Transport.Authority)
	require.Equal(t, gkeepalive.ClientParameters{Time: time.Second}, merged.Transport.KeepaliveParams)
	require.EqualValues(t, 11, merged.Transport.InitialWindowSize)
	require.EqualValues(t, 12, merged.Transport.InitialConnWindowSize)
	require.Equal(t, 13, merged.Transport.WriteBufferSize)
	require.Equal(t, 14, merged.Transport.ReadBufferSize)
	require.EqualValues(t, overlayHeader, *merged.Transport.MaxHeaderListSize)
}

func TestCloneNestedMapAndDedupStrings(t *testing.T) {
	original := map[string]map[string]any{
		"a": {"k": "v"},
	}

	cloned := cloneNestedMap(original)
	cloned["a"]["k"] = "changed"
	require.Equal(t, "v", original["a"]["k"])
	require.Empty(t, cloneNestedMap(nil))

	require.Equal(t, []string{"a", "b"}, dedupStrings([]string{"", "a", "b", "a"}))
}

func decodeRoot(t *testing.T, payload map[string]any) Root {
	t.Helper()
	var root Root
	require.NoError(t, config.NewSnapshot(payload).Decode(&root))
	return root
}

type testRegistry struct{}

func (*testRegistry) Register(context.Context, registry.Instance) error   { return nil }
func (*testRegistry) Deregister(context.Context, registry.Instance) error { return nil }
func (*testRegistry) Type() string                                        { return "test" }

type testResolver struct{}

func (*testResolver) AddWatch(string, resolver.Client) error { return nil }
func (*testResolver) DelWatch(string, resolver.Client) error { return nil }
func (*testResolver) Type() string                           { return "test" }

func ptr[T any](v T) *T {
	return &v
}
