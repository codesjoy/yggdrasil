// Package capabilities exposes the built-in framework capability contracts.
package capabilities

import (
	"reflect"

	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/observability/logger"
	xotel "github.com/codesjoy/yggdrasil/v3/observability/otel"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

var (
	// LoggerHandlerSpec is the named logger handler builder capability.
	LoggerHandlerSpec = module.CapabilitySpec{
		Name:        "observability.logger.handler",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((logger.HandlerBuilder)(nil)),
	}
	// LoggerWriterSpec is the named logger writer builder capability.
	LoggerWriterSpec = module.CapabilitySpec{
		Name:        "observability.logger.writer",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((logger.WriterBuilder)(nil)),
	}
	// TracerProviderSpec is the named tracer provider builder capability.
	TracerProviderSpec = module.CapabilitySpec{
		Name:        "observability.otel.tracer_provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((xotel.TracerProviderBuilder)(nil)),
	}
	// MeterProviderSpec is the named meter provider builder capability.
	MeterProviderSpec = module.CapabilitySpec{
		Name:        "observability.otel.meter_provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((xotel.MeterProviderBuilder)(nil)),
	}
	// StatsHandlerSpec is the named stats handler builder capability.
	StatsHandlerSpec = module.CapabilitySpec{
		Name:        "observability.stats.handler",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((stats.HandlerBuilder)(nil)),
	}
	// SecurityProfileProviderSpec is the named security profile provider capability.
	SecurityProfileProviderSpec = module.CapabilitySpec{
		Name:        "security.profile.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*security.Provider)(nil)).Elem(),
	}
	// MarshalerSchemeSpec is the named marshaler builder capability.
	MarshalerSchemeSpec = module.CapabilitySpec{
		Name:        "marshaler.scheme",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((marshaler.MarshalerBuilder)(nil)),
	}
	// TransportServerProviderSpec is the named server transport provider capability.
	TransportServerProviderSpec = module.CapabilitySpec{
		Name:        "transport.server.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*remote.TransportServerProvider)(nil)).Elem(),
	}
	// TransportClientProviderSpec is the named client transport provider capability.
	TransportClientProviderSpec = module.CapabilitySpec{
		Name:        "transport.client.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*remote.TransportClientProvider)(nil)).Elem(),
	}
	// UnaryServerInterceptorSpec is the ordered unary server interceptor capability.
	UnaryServerInterceptorSpec = module.CapabilitySpec{
		Name:        "rpc.interceptor.unary_server",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*interceptor.UnaryServerInterceptorProvider)(nil)).Elem(),
	}
	// StreamServerInterceptorSpec is the ordered stream server interceptor capability.
	StreamServerInterceptorSpec = module.CapabilitySpec{
		Name:        "rpc.interceptor.stream_server",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*interceptor.StreamServerInterceptorProvider)(nil)).Elem(),
	}
	// UnaryClientInterceptorSpec is the ordered unary client interceptor capability.
	UnaryClientInterceptorSpec = module.CapabilitySpec{
		Name:        "rpc.interceptor.unary_client",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*interceptor.UnaryClientInterceptorProvider)(nil)).Elem(),
	}
	// StreamClientInterceptorSpec is the ordered stream client interceptor capability.
	StreamClientInterceptorSpec = module.CapabilitySpec{
		Name:        "rpc.interceptor.stream_client",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*interceptor.StreamClientInterceptorProvider)(nil)).Elem(),
	}
	// RESTMiddlewareSpec is the ordered REST middleware provider capability.
	RESTMiddlewareSpec = module.CapabilitySpec{
		Name:        "transport.rest.middleware",
		Cardinality: module.OrderedMany,
		Type:        reflect.TypeOf((*rest.Provider)(nil)).Elem(),
	}
	// RegistryProviderSpec is the named discovery registry provider capability.
	RegistryProviderSpec = module.CapabilitySpec{
		Name:        "discovery.registry.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*registry.Provider)(nil)).Elem(),
	}
	// ResolverProviderSpec is the named discovery resolver provider capability.
	ResolverProviderSpec = module.CapabilitySpec{
		Name:        "discovery.resolver.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*resolver.Provider)(nil)).Elem(),
	}
	// BalancerProviderSpec is the named client balancer provider capability.
	BalancerProviderSpec = module.CapabilitySpec{
		Name:        "transport.balancer.provider",
		Cardinality: module.NamedOne,
		Type:        reflect.TypeOf((*balancer.Provider)(nil)).Elem(),
	}
)

// ProvideNamed wraps a named capability value without additional validation.
func ProvideNamed[T any](spec module.CapabilitySpec, name string, value T) module.Capability {
	return module.Capability{
		Spec:  spec,
		Name:  name,
		Value: value,
	}
}

// ProvideOrdered wraps one ordered capability value without additional validation.
func ProvideOrdered[T any](spec module.CapabilitySpec, name string, value T) module.Capability {
	return module.Capability{
		Spec:  spec,
		Name:  name,
		Value: value,
	}
}
