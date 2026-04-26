package capabilities

import (
	"reflect"
	"testing"

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

func TestBuiltinSpecs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec module.CapabilitySpec
		want module.CapabilitySpec
	}{
		{
			name: "logger handler",
			spec: LoggerHandlerSpec,
			want: module.CapabilitySpec{
				Name:        "observability.logger.handler",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((logger.HandlerBuilder)(nil)),
			},
		},
		{
			name: "logger writer",
			spec: LoggerWriterSpec,
			want: module.CapabilitySpec{
				Name:        "observability.logger.writer",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((logger.WriterBuilder)(nil)),
			},
		},
		{
			name: "tracer provider",
			spec: TracerProviderSpec,
			want: module.CapabilitySpec{
				Name:        "observability.otel.tracer_provider",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((xotel.TracerProviderBuilder)(nil)),
			},
		},
		{
			name: "meter provider",
			spec: MeterProviderSpec,
			want: module.CapabilitySpec{
				Name:        "observability.otel.meter_provider",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((xotel.MeterProviderBuilder)(nil)),
			},
		},
		{
			name: "stats handler",
			spec: StatsHandlerSpec,
			want: module.CapabilitySpec{
				Name:        "observability.stats.handler",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((stats.HandlerBuilder)(nil)),
			},
		},
		{
			name: "security profile provider",
			spec: SecurityProfileProviderSpec,
			want: module.CapabilitySpec{
				Name:        "security.profile.provider",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((*security.Provider)(nil)).Elem(),
			},
		},
		{
			name: "marshaler scheme",
			spec: MarshalerSchemeSpec,
			want: module.CapabilitySpec{
				Name:        "marshaler.scheme",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((marshaler.MarshalerBuilder)(nil)),
			},
		},
		{
			name: "transport server provider",
			spec: TransportServerProviderSpec,
			want: module.CapabilitySpec{
				Name:        "transport.server.provider",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((*remote.TransportServerProvider)(nil)).Elem(),
			},
		},
		{
			name: "transport client provider",
			spec: TransportClientProviderSpec,
			want: module.CapabilitySpec{
				Name:        "transport.client.provider",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((*remote.TransportClientProvider)(nil)).Elem(),
			},
		},
		{
			name: "unary server interceptor",
			spec: UnaryServerInterceptorSpec,
			want: module.CapabilitySpec{
				Name:        "rpc.interceptor.unary_server",
				Cardinality: module.OrderedMany,
				Type: reflect.TypeOf((*interceptor.UnaryServerInterceptorProvider)(nil)).
					Elem(),
			},
		},
		{
			name: "stream server interceptor",
			spec: StreamServerInterceptorSpec,
			want: module.CapabilitySpec{
				Name:        "rpc.interceptor.stream_server",
				Cardinality: module.OrderedMany,
				Type: reflect.TypeOf((*interceptor.StreamServerInterceptorProvider)(nil)).
					Elem(),
			},
		},
		{
			name: "unary client interceptor",
			spec: UnaryClientInterceptorSpec,
			want: module.CapabilitySpec{
				Name:        "rpc.interceptor.unary_client",
				Cardinality: module.OrderedMany,
				Type: reflect.TypeOf((*interceptor.UnaryClientInterceptorProvider)(nil)).
					Elem(),
			},
		},
		{
			name: "stream client interceptor",
			spec: StreamClientInterceptorSpec,
			want: module.CapabilitySpec{
				Name:        "rpc.interceptor.stream_client",
				Cardinality: module.OrderedMany,
				Type: reflect.TypeOf((*interceptor.StreamClientInterceptorProvider)(nil)).
					Elem(),
			},
		},
		{
			name: "rest middleware",
			spec: RESTMiddlewareSpec,
			want: module.CapabilitySpec{
				Name:        "transport.rest.middleware",
				Cardinality: module.OrderedMany,
				Type:        reflect.TypeOf((*rest.Provider)(nil)).Elem(),
			},
		},
		{
			name: "registry provider",
			spec: RegistryProviderSpec,
			want: module.CapabilitySpec{
				Name:        "discovery.registry.provider",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((*registry.Provider)(nil)).Elem(),
			},
		},
		{
			name: "resolver provider",
			spec: ResolverProviderSpec,
			want: module.CapabilitySpec{
				Name:        "discovery.resolver.provider",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((*resolver.Provider)(nil)).Elem(),
			},
		},
		{
			name: "balancer provider",
			spec: BalancerProviderSpec,
			want: module.CapabilitySpec{
				Name:        "transport.balancer.provider",
				Cardinality: module.NamedOne,
				Type:        reflect.TypeOf((*balancer.Provider)(nil)).Elem(),
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.spec.Name != tc.want.Name {
				t.Fatalf("spec.Name = %q, want %q", tc.spec.Name, tc.want.Name)
			}
			if tc.spec.Cardinality != tc.want.Cardinality {
				t.Fatalf("spec.Cardinality = %v, want %v", tc.spec.Cardinality, tc.want.Cardinality)
			}
			if tc.spec.Type != tc.want.Type {
				t.Fatalf("spec.Type = %v, want %v", tc.spec.Type, tc.want.Type)
			}
		})
	}
}

func TestProvideHelpers(t *testing.T) {
	t.Parallel()

	named := ProvideNamed(TransportServerProviderSpec, "grpc", 123)
	if named.Spec != TransportServerProviderSpec {
		t.Fatalf("named.Spec = %#v", named.Spec)
	}
	if named.Name != "grpc" {
		t.Fatalf("named.Name = %q", named.Name)
	}
	if got, ok := named.Value.(int); !ok || got != 123 {
		t.Fatalf("named.Value = %#v", named.Value)
	}

	ordered := ProvideOrdered(RESTMiddlewareSpec, "logger", "value")
	if ordered.Spec != RESTMiddlewareSpec {
		t.Fatalf("ordered.Spec = %#v", ordered.Spec)
	}
	if ordered.Name != "logger" {
		t.Fatalf("ordered.Name = %q", ordered.Name)
	}
	if got, ok := ordered.Value.(string); !ok || got != "value" {
		t.Fatalf("ordered.Value = %#v", ordered.Value)
	}
}
