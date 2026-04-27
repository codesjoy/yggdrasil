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
	"fmt"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/observability/logger"
	xotel "github.com/codesjoy/yggdrasil/v3/observability/otel"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

// Snapshot is the immutable App-scoped runtime assembly result.
type Snapshot struct {
	Identity Identity
	Resolved settings.Resolved

	Logger            *slog.Logger
	RemoteLogger      *slog.Logger
	TracerProvider    trace.TracerProvider
	MeterProvider     metric.MeterProvider
	TextMapPropagator propagation.TextMapPropagator

	LoggerHandlerBuilders map[string]logger.HandlerBuilder
	LoggerWriterBuilders  map[string]logger.WriterBuilder

	TracerProviderBuilders map[string]xotel.TracerProviderBuilder
	MeterProviderBuilders  map[string]xotel.MeterProviderBuilder

	StatsHandlerBuilders map[string]stats.HandlerBuilder
	ServerStats          stats.Handler
	ClientStats          stats.Handler

	SecurityProviders   map[string]security.Provider
	SecurityProfiles    map[string]security.Profile
	MarshalerBuilderMap map[string]marshaler.MarshalerBuilder

	TransportServerProviders map[string]remote.TransportServerProvider
	TransportClientProviders map[string]remote.TransportClientProvider

	UnaryServerInterceptorProviders  map[string]interceptor.UnaryServerInterceptorProvider
	StreamServerInterceptorProviders map[string]interceptor.StreamServerInterceptorProvider
	UnaryClientInterceptorProviders  map[string]interceptor.UnaryClientInterceptorProvider
	StreamClientInterceptorProviders map[string]interceptor.StreamClientInterceptorProvider

	RESTMiddlewareProviderMap map[string]rest.Provider

	RegistryProviders map[string]registry.Provider
	ResolverProviders map[string]resolver.Provider
	BalancerProviders map[string]balancer.Provider
}

func cloneMap[K comparable, V any](in map[K]V) map[K]V {
	if len(in) == 0 {
		return map[K]V{}
	}
	out := make(map[K]V, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

// Copy returns a detached snapshot copy.
func (s *Snapshot) Copy() *Snapshot {
	if s == nil {
		return nil
	}
	identity := s.Identity
	identity.Metadata = identity.metadataCopy()
	return &Snapshot{
		Identity:                        identity,
		Resolved:                        s.Resolved,
		Logger:                          s.Logger,
		RemoteLogger:                    s.RemoteLogger,
		TracerProvider:                  s.TracerProvider,
		MeterProvider:                   s.MeterProvider,
		TextMapPropagator:               s.TextMapPropagator,
		LoggerHandlerBuilders:           cloneMap(s.LoggerHandlerBuilders),
		LoggerWriterBuilders:            cloneMap(s.LoggerWriterBuilders),
		TracerProviderBuilders:          cloneMap(s.TracerProviderBuilders),
		MeterProviderBuilders:           cloneMap(s.MeterProviderBuilders),
		StatsHandlerBuilders:            cloneMap(s.StatsHandlerBuilders),
		ServerStats:                     s.ServerStats,
		ClientStats:                     s.ClientStats,
		SecurityProviders:               cloneMap(s.SecurityProviders),
		SecurityProfiles:                cloneMap(s.SecurityProfiles),
		MarshalerBuilderMap:             cloneMap(s.MarshalerBuilderMap),
		TransportServerProviders:        cloneMap(s.TransportServerProviders),
		TransportClientProviders:        cloneMap(s.TransportClientProviders),
		UnaryServerInterceptorProviders: cloneMap(s.UnaryServerInterceptorProviders),
		StreamServerInterceptorProviders: cloneMap(
			s.StreamServerInterceptorProviders,
		),
		UnaryClientInterceptorProviders: cloneMap(s.UnaryClientInterceptorProviders),
		StreamClientInterceptorProviders: cloneMap(
			s.StreamClientInterceptorProviders,
		),
		RESTMiddlewareProviderMap: cloneMap(s.RESTMiddlewareProviderMap),
		RegistryProviders:         cloneMap(s.RegistryProviders),
		ResolverProviders:         cloneMap(s.ResolverProviders),
		BalancerProviders:         cloneMap(s.BalancerProviders),
	}
}

// ClientSettings returns the resolved settings for one client service.
func (s *Snapshot) ClientSettings(serviceName string) client.ServiceSettings {
	if s == nil {
		return client.ServiceSettings{}
	}
	return s.Resolved.Clients.Services[serviceName]
}

// ClientStatsHandler returns the App-scoped client stats handler.
func (s *Snapshot) ClientStatsHandler() stats.Handler {
	if s == nil {
		return stats.NoOpHandler
	}
	return s.ClientStats
}

// ServerStatsHandler returns the App-scoped server stats handler.
func (s *Snapshot) ServerStatsHandler() stats.Handler {
	if s == nil {
		return stats.NoOpHandler
	}
	return s.ServerStats
}

// ServerSettings returns the resolved server settings.
func (s *Snapshot) ServerSettings() server.Settings {
	if s == nil {
		return server.Settings{}
	}
	return s.Resolved.Server
}

// RESTConfig returns the resolved REST config.
func (s *Snapshot) RESTConfig() *rest.Config {
	if s == nil {
		return nil
	}
	return s.Resolved.Transports.Rest
}

// RESTMiddlewareProviders returns the App-scoped REST middleware providers.
func (s *Snapshot) RESTMiddlewareProviders() map[string]rest.Provider {
	if s == nil {
		return map[string]rest.Provider{}
	}
	return cloneMap(s.RESTMiddlewareProviderMap)
}

// MarshalerBuilders returns the App-scoped marshaler builders.
func (s *Snapshot) MarshalerBuilders() map[string]marshaler.MarshalerBuilder {
	if s == nil {
		return map[string]marshaler.MarshalerBuilder{}
	}
	return cloneMap(s.MarshalerBuilderMap)
}

// TransportServerProvider returns one server transport provider by protocol.
func (s *Snapshot) TransportServerProvider(protocol string) remote.TransportServerProvider {
	if s == nil {
		return nil
	}
	return s.TransportServerProviders[protocol]
}

// TransportClientProvider returns one client transport provider by protocol.
func (s *Snapshot) TransportClientProvider(protocol string) remote.TransportClientProvider {
	if s == nil {
		return nil
	}
	return s.TransportClientProviders[protocol]
}

// BuildUnaryServerInterceptor builds one unary server interceptor chain from the explicit provider map.
func (s *Snapshot) BuildUnaryServerInterceptor(names []string) interceptor.UnaryServerInterceptor {
	return interceptor.ChainUnaryServerInterceptorsWithProviders(
		names,
		s.UnaryServerInterceptorProviders,
	)
}

// BuildStreamServerInterceptor builds one stream server interceptor chain from the explicit provider map.
func (s *Snapshot) BuildStreamServerInterceptor(
	names []string,
) interceptor.StreamServerInterceptor {
	return interceptor.ChainStreamServerInterceptorsWithProviders(
		names,
		s.StreamServerInterceptorProviders,
	)
}

// BuildUnaryClientInterceptor builds one unary client interceptor chain from the explicit provider map.
func (s *Snapshot) BuildUnaryClientInterceptor(
	serviceName string,
	names []string,
) interceptor.UnaryClientInterceptor {
	return interceptor.ChainUnaryClientInterceptorsWithProviders(
		serviceName,
		names,
		s.UnaryClientInterceptorProviders,
	)
}

// BuildStreamClientInterceptor builds one stream client interceptor chain from the explicit provider map.
func (s *Snapshot) BuildStreamClientInterceptor(
	serviceName string,
	names []string,
) interceptor.StreamClientInterceptor {
	return interceptor.ChainStreamClientInterceptorsWithProviders(
		serviceName,
		names,
		s.StreamClientInterceptorProviders,
	)
}

// BuildRESTMiddlewares builds one REST middleware chain from the explicit provider map.
func (s *Snapshot) BuildRESTMiddlewares(names ...string) chi.Middlewares {
	return rest.BuildWithProviders(s.RESTMiddlewareProviderMap, names...)
}

// NewRegistry builds the configured default registry.
func (s *Snapshot) NewRegistry() (registry.Registry, error) {
	if s == nil {
		return nil, fmt.Errorf("runtime snapshot is nil")
	}
	typeName := s.Resolved.Discovery.Registry.Type
	if typeName == "" {
		return nil, fmt.Errorf("not found registry type")
	}
	return s.NewRegistryByType(typeName, s.Resolved.Discovery.Registry.Config)
}

// NewRegistryByType builds one registry from the explicit provider map.
func (s *Snapshot) NewRegistryByType(
	typeName string,
	cfg map[string]any,
) (registry.Registry, error) {
	if s == nil {
		return nil, fmt.Errorf("runtime snapshot is nil")
	}
	provider := s.RegistryProviders[typeName]
	if provider == nil {
		return nil, fmt.Errorf("not found registry provider, type: %s", typeName)
	}
	return provider.New(cfg)
}

// NewResolver builds one resolver by configured name from the explicit provider map.
func (s *Snapshot) NewResolver(name string) (resolver.Resolver, error) {
	if s == nil {
		return nil, fmt.Errorf("runtime snapshot is nil")
	}
	spec := s.Resolved.Discovery.Resolvers[name]
	typeName := spec.Type
	if typeName == "" {
		if name == resolver.DefaultResolverName {
			return nil, nil
		}
		return nil, fmt.Errorf("not found resolver type, name: %s", name)
	}
	provider := s.ResolverProviders[typeName]
	if provider == nil {
		return nil, fmt.Errorf("not found resolver provider, type: %s", typeName)
	}
	return provider.New(name)
}

// NewBalancer builds one balancer using explicit providers and resolved settings.
func (s *Snapshot) NewBalancer(
	serviceName string,
	balancerName string,
	cli balancer.Client,
) (balancer.Balancer, error) {
	typeName, err := s.resolveBalancerType(balancerName)
	if err != nil {
		return nil, err
	}
	provider := s.BalancerProviders[typeName]
	if provider == nil {
		return nil, fmt.Errorf("not found balancer provider, type: %s", typeName)
	}
	return provider.New(serviceName, balancerName, cli)
}

func (s *Snapshot) resolveBalancerType(balancerName string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("runtime snapshot is nil")
	}
	typeName := s.Resolved.Balancers.Defaults[balancerName].Type
	if typeName == "" {
		if balancerName == balancer.DefaultBalancerName {
			return balancer.DefaultBalancerType, nil
		}
		return "", fmt.Errorf("not found balancer type, name: %s", balancerName)
	}
	return typeName, nil
}

// BuildDefaultLoggerHandler builds the process default logger handler from the explicit builder maps.
func (s *Snapshot) BuildDefaultLoggerHandler() (slog.Handler, error) {
	if s == nil {
		return nil, fmt.Errorf("runtime snapshot is nil")
	}
	spec := s.Resolved.Logging.Handlers["default"]
	typeName := spec.Type
	if typeName == "" {
		typeName = "text"
	}
	if typeName == "console" {
		typeName = "text"
	}
	writerName := spec.Writer
	if writerName == "" {
		writerName = "default"
	}
	handlerBuilder := s.LoggerHandlerBuilders[typeName]
	if handlerBuilder == nil {
		return nil, fmt.Errorf("handler builder for type %s not found", typeName)
	}
	return handlerBuilder(writerName, spec.Config)
}

// BuildTracerProvider builds the configured tracer provider.
func (s *Snapshot) BuildTracerProvider(instanceName string) (trace.TracerProvider, bool) {
	if s == nil {
		return nil, false
	}
	name := s.Resolved.Telemetry.Tracer
	if name == "" {
		return nil, false
	}
	builder, ok := s.TracerProviderBuilders[name]
	if !ok {
		return nil, false
	}
	return builder(instanceName), true
}

// BuildMeterProvider builds the configured meter provider.
func (s *Snapshot) BuildMeterProvider(instanceName string) (metric.MeterProvider, bool) {
	if s == nil {
		return nil, false
	}
	name := s.Resolved.Telemetry.Meter
	if name == "" {
		return nil, false
	}
	builder, ok := s.MeterProviderBuilders[name]
	if !ok {
		return nil, false
	}
	return builder(instanceName), true
}
