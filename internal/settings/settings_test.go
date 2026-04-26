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

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/internal/backoff"
	"github.com/codesjoy/yggdrasil/v3/observability/logger"
	xotel "github.com/codesjoy/yggdrasil/v3/observability/otel"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	"github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"
	grpcprotocol "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc"
	rpchttp "github.com/codesjoy/yggdrasil/v3/transport/protocol/rpchttp"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
)

func TestCatalogAccessorsAndDecodePayload(t *testing.T) {
	manager := config.NewManager()
	require.NoError(
		t,
		manager.LoadLayer("test", config.PriorityOverride, memory.NewSource("test", map[string]any{
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
				"observability": map[string]any{
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
		})),
	)

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
	require.NotNil(t, service.ServiceSettings.Resolver)
	require.Equal(t, "discovery", *service.ServiceSettings.Resolver)

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

	require.PanicsWithValue(t, nilCatalogManagerPanic, func() {
		_ = NewCatalog(nil)
	})

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
							"security_profile":         "base-creds",
							"authority":                "base-auth",
							"initial_window_size":      11,
							"initial_conn_window_size": 22,
							"write_buffer_size":        33,
							"read_buffer_size":         44,
							"max_header_list_size":     55,
						},
					},
					"server": map[string]any{
						"security_profile": "server-creds",
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
				"security": map[string]any{
					"profiles": map[string]any{
						"base-creds":   map[string]any{"type": "insecure"},
						"server-creds": map[string]any{"type": "local"},
						"svc-creds":    map[string]any{"type": "tls"},
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
									"security_profile":         "svc-creds",
									"authority":                "svc-auth",
									"keepalive_params":         map[string]any{"time": "1s"},
									"initial_window_size":      66,
									"initial_conn_window_size": 77,
									"write_buffer_size":        88,
									"read_buffer_size":         99,
									"max_header_list_size":     111,
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
	require.Equal(t, "svc-creds", svcGRPC.Transport.SecurityProfile)
	require.Equal(t, "svc-auth", svcGRPC.Transport.Authority)
	require.Equal(
		t,
		gkeepalive.ClientParameters{Time: time.Second},
		svcGRPC.Transport.KeepaliveParams,
	)
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

	require.Equal(t, map[string]SecurityProfileSpec{
		"base-creds":   {Type: "insecure", Config: map[string]any{}},
		"server-creds": {Type: "local", Config: map[string]any{}},
		"svc-creds":    {Type: "tls", Config: map[string]any{}},
	}, resolved.Transports.SecurityProfiles)

	cloned := resolved.Transports.SecurityProfiles
	cloned["base-creds"] = SecurityProfileSpec{Type: "changed"}
	require.Equal(t, "insecure", root.Yggdrasil.Transports.Security.Profiles["base-creds"].Type)
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
	require.NotNil(t, resolved.Transports.SecurityProfiles)
}

func TestCompile_ProducesOrderedExtensionsAndCapabilityBindings(t *testing.T) {
	root := decodeRoot(t, map[string]any{
		"yggdrasil": map[string]any{
			"observability": map[string]any{
				"logging": map[string]any{
					"handlers": map[string]any{
						"default": map[string]any{"type": "text", "writer": "default"},
						"json":    map[string]any{"type": "json", "writer": "default"},
					},
					"writers": map[string]any{
						"default": map[string]any{"type": "console"},
						"file":    map[string]any{"type": "file"},
					},
				},
				"telemetry": map[string]any{
					"tracer": "noop-tracer",
					"meter":  "noop-meter",
					"stats": map[string]any{
						"server": "otel",
						"client": "otel",
					},
				},
			},
			"transports": map[string]any{
				"security": map[string]any{
					"profiles": map[string]any{
						"tls":      map[string]any{"type": "tls"},
						"insecure": map[string]any{"type": "insecure"},
					},
				},
				"http": map[string]any{
					"rest": map[string]any{
						"marshaler": map[string]any{
							"support": []any{"jsonpb", "proto", "jsonpb"},
						},
					},
				},
			},
			"extensions": map[string]any{
				"interceptors": map[string]any{
					"unary_server":  []any{"a", "b", "a"},
					"stream_server": []any{"s"},
					"unary_client":  []any{"c"},
					"stream_client": []any{"d"},
				},
				"middleware": map[string]any{
					"rest_all": []any{"m1", "m1"},
					"rest_rpc": []any{"m2"},
					"rest_web": []any{"m3"},
				},
			},
		},
	})

	resolved, err := Compile(root)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, resolved.OrderedExtensions.UnaryServer)
	require.Equal(t, []string{"m1"}, resolved.OrderedExtensions.RestAll)
	require.NotEmpty(t, resolved.ModuleViews["telemetry"])
	require.Equal(
		t,
		[]string{"json", "text"},
		resolved.CapabilityBindings["observability.logger.handler"],
	)
	require.Equal(
		t,
		[]string{"console", "file"},
		resolved.CapabilityBindings["observability.logger.writer"],
	)
	require.Equal(
		t,
		[]string{"noop-tracer"},
		resolved.CapabilityBindings["observability.otel.tracer_provider"],
	)
	require.Equal(
		t,
		[]string{"noop-meter"},
		resolved.CapabilityBindings["observability.otel.meter_provider"],
	)
	require.Equal(t, []string{"otel"}, resolved.CapabilityBindings["observability.stats.handler"])
	require.Equal(
		t,
		[]string{"insecure", "tls"},
		resolved.CapabilityBindings["security.profile.provider"],
	)
	require.Equal(t, []string{"jsonpb", "proto"}, resolved.CapabilityBindings["marshaler.scheme"])
}

func TestCompile_PreservesTemplateStyleExtensionsWithoutNormalizingThem(t *testing.T) {
	var root Root
	require.NoError(t, config.NewSnapshot(map[string]any{
		"yggdrasil": map[string]any{
			"extensions": map[string]any{
				"interceptors": map[string]any{
					"unary_server": "default-observable@v1",
				},
				"middleware": map[string]any{
					"rest_all": map[string]any{
						"template": "default-rest-observable",
						"version":  "v1",
					},
				},
			},
		},
	}).Decode(&root))

	resolved, err := Compile(root)
	require.NoError(t, err)
	require.Empty(t, resolved.OrderedExtensions.UnaryServer)
	require.Empty(t, resolved.OrderedExtensions.RestAll)
	require.Equal(
		t,
		"default-observable@v1",
		resolved.Root.Yggdrasil.Extensions.Interceptors.UnaryServer,
	)
	value, ok := resolved.Root.Yggdrasil.Extensions.Middleware.RestAll.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "default-rest-observable", value["template"])
	require.Equal(t, "v1", value["version"])
}

func TestValidateV3RootShape(t *testing.T) {
	require.NoError(t, ValidateV3RootShape(map[string]any{
		"yggdrasil": map[string]any{},
		"app":       map[string]any{"name": "demo"},
	}))
	require.Error(t, ValidateV3RootShape(map[string]any{
		"server": map[string]any{},
	}))
	require.Error(t, ValidateV3RootShape(map[string]any{
		"yggdrasil": map[string]any{},
		"server":    map[string]any{},
	}))
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
			Interceptors: server.InterceptorSettings{
				Unary:  []string{"missing-unary-server"},
				Stream: []string{"missing-stream-server"},
			},
		},
		Root: Root{
			Yggdrasil: Framework{
				Clients: Clients{
					Defaults: ClientDefaults{
						ServiceSettings: client.ServiceSettings{
							Interceptors: client.InterceptorSettings{
								Unary:  []string{"missing-default-unary"},
								Stream: []string{"missing-default-stream"},
							},
						},
					},
				},
			},
		},
		Clients: client.Settings{
			Services: map[string]client.ServiceSettings{
				"svc": {
					Interceptors: client.InterceptorSettings{
						Unary:  []string{"missing-service-unary"},
						Stream: []string{"missing-service-stream"},
					},
				},
			},
		},
		Transports: ResolvedTransports{
			GRPC: grpcprotocol.Settings{
				ClientServices: map[string]grpcprotocol.ClientConfig{
					"svc": {},
				},
			},
			SecurityProfiles: map[string]SecurityProfileSpec{
				"missing-security": {Type: "missing-security-provider"},
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
	securityType := unique + "-security"
	unaryServer := unique + "-unary-server"
	streamServer := unique + "-stream-server"
	unaryClientDefault := unique + "-unary-client-default"
	streamClientDefault := unique + "-stream-client-default"
	unaryClientService := unique + "-unary-client-service"
	streamClientService := unique + "-stream-client-service"
	restMW := unique + "-rest"
	restMarshal := unique + "-marshal"

	require.NoError(t, registry.ConfigureProviders([]registry.Provider{
		registry.NewProvider(registryName, func(map[string]any) (registry.Registry, error) {
			return &testRegistry{}, nil
		}),
	}))
	t.Cleanup(func() {
		require.NoError(t, registry.ConfigureProviders(nil))
	})
	require.NoError(t, resolver.ConfigureProviders([]resolver.Provider{
		resolver.NewProvider(resolverName, func(string) (resolver.Resolver, error) {
			return &testResolver{}, nil
		}),
	}))
	t.Cleanup(func() {
		require.NoError(t, resolver.ConfigureProviders(nil))
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
	require.NoError(
		t,
		interceptor.ConfigureUnaryServerProviders([]interceptor.UnaryServerInterceptorProvider{
			interceptor.NewUnaryServerInterceptorProvider(
				unaryServer,
				func() interceptor.UnaryServerInterceptor {
					return func(ctx context.Context, req any, info *interceptor.UnaryServerInfo, handler interceptor.UnaryHandler) (any, error) {
						return handler(ctx, req)
					}
				},
			),
		}),
	)
	require.NoError(
		t,
		interceptor.ConfigureStreamServerProviders([]interceptor.StreamServerInterceptorProvider{
			interceptor.NewStreamServerInterceptorProvider(
				streamServer,
				func() interceptor.StreamServerInterceptor {
					return func(srv interface{}, ss stream.ServerStream, info *interceptor.StreamServerInfo, handler stream.Handler) error {
						return handler(srv, ss)
					}
				},
			),
		}),
	)
	require.NoError(
		t,
		interceptor.ConfigureUnaryClientProviders([]interceptor.UnaryClientInterceptorProvider{
			interceptor.NewUnaryClientInterceptorProvider(
				unaryClientDefault,
				func(string) interceptor.UnaryClientInterceptor {
					return func(ctx context.Context, method string, req, reply any, invoker interceptor.UnaryInvoker) error {
						return invoker(ctx, method, req, reply)
					}
				},
			),
			interceptor.NewUnaryClientInterceptorProvider(
				unaryClientService,
				func(string) interceptor.UnaryClientInterceptor {
					return func(ctx context.Context, method string, req, reply any, invoker interceptor.UnaryInvoker) error {
						return invoker(ctx, method, req, reply)
					}
				},
			),
		}),
	)
	require.NoError(
		t,
		interceptor.ConfigureStreamClientProviders([]interceptor.StreamClientInterceptorProvider{
			interceptor.NewStreamClientInterceptorProvider(
				streamClientDefault,
				func(string) interceptor.StreamClientInterceptor {
					return func(ctx context.Context, desc *stream.Desc, method string, streamer interceptor.Streamer) (stream.ClientStream, error) {
						return streamer(ctx, desc, method)
					}
				},
			),
			interceptor.NewStreamClientInterceptorProvider(
				streamClientService,
				func(string) interceptor.StreamClientInterceptor {
					return func(ctx context.Context, desc *stream.Desc, method string, streamer interceptor.Streamer) (stream.ClientStream, error) {
						return streamer(ctx, desc, method)
					}
				},
			),
		}),
	)
	t.Cleanup(func() {
		require.NoError(t, interceptor.ConfigureUnaryServerProviders(nil))
		require.NoError(t, interceptor.ConfigureStreamServerProviders(nil))
		require.NoError(t, interceptor.ConfigureUnaryClientProviders(nil))
		require.NoError(t, interceptor.ConfigureStreamClientProviders(nil))
	})
	require.NoError(t, rest.ConfigureProviders([]rest.Provider{
		rest.BuiltinLoggingProvider(),
		rest.BuiltinMarshalerProvider(),
		rest.NewProvider(restMW, func() func(http.Handler) http.Handler {
			return func(next http.Handler) http.Handler { return next }
		}),
	}))
	t.Cleanup(func() {
		require.NoError(t, rest.ConfigureProviders([]rest.Provider{
			rest.BuiltinLoggingProvider(),
			rest.BuiltinMarshalerProvider(),
		}))
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
			Interceptors: server.InterceptorSettings{
				Unary:  []string{unaryServer},
				Stream: []string{streamServer},
			},
		},
		Root: Root{
			Yggdrasil: Framework{
				Clients: Clients{
					Defaults: ClientDefaults{
						ServiceSettings: client.ServiceSettings{
							Interceptors: client.InterceptorSettings{
								Unary:  []string{unaryClientDefault},
								Stream: []string{streamClientDefault},
							},
						},
					},
				},
			},
		},
		Clients: client.Settings{
			Services: map[string]client.ServiceSettings{
				"svc": {
					Interceptors: client.InterceptorSettings{
						Unary:  []string{unaryClientService},
						Stream: []string{streamClientService},
					},
				},
			},
		},
		Transports: ResolvedTransports{
			GRPC: grpcprotocol.Settings{
				ClientServices: map[string]grpcprotocol.ClientConfig{
					"svc": {},
				},
			},
			SecurityProfiles: map[string]SecurityProfileSpec{
				"custom": {Type: securityType},
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
	require.NoError(t, err)
}

func TestMergeClientServiceSettings(t *testing.T) {
	fastFail := false
	base := client.ServiceSettings{
		FastFail: true,
		Resolver: "base-resolver",
		Balancer: "base-balancer",
		Backoff: backoff.Config{
			BaseDelay: time.Second,
			MaxDelay:  2 * time.Second,
		},
		Remote: client.RemoteSettings{
			Endpoints: []resolver.BaseEndpoint{
				{Address: "base:1"},
			},
			Attributes: map[string]any{
				"base": "yes",
			},
		},
		Interceptors: client.InterceptorSettings{
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

	merged := mergeClientServiceSettings(base, overlay)

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
	merged = mergeClientServiceSettings(base, overlay)
	require.Equal(t, "yes", merged.Remote.Attributes["overlay"])
}

func TestMergeHTTPClientConfig(t *testing.T) {
	base := rpchttp.ClientConfig{Timeout: time.Second}
	overlay := HTTPClientTransport{
		Timeout: ptr(2 * time.Second),
		Marshaler: &rpchttp.MarshalerConfigSet{
			Inbound: &rpchttp.MarshalerConfig{Type: "jsonpb"},
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
	base := grpcprotocol.ClientConfig{
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
			SecurityProfile:       "base-creds",
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
			SecurityProfile:       ptr("overlay-creds"),
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
	require.Equal(t, "overlay-creds", merged.Transport.SecurityProfile)
	require.Equal(t, "overlay-auth", merged.Transport.Authority)
	require.Equal(
		t,
		gkeepalive.ClientParameters{Time: time.Second},
		merged.Transport.KeepaliveParams,
	)
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
