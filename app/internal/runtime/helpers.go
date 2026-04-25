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

package runtime

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"reflect"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/observability/logger"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	statsotel "github.com/codesjoy/yggdrasil/v3/observability/stats/otel"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	"github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

// ResolveNamedCapabilityMap resolves a named capability map with contextual errors.
func ResolveNamedCapabilityMap[T any](
	names []string,
	spec module.CapabilitySpec,
	resolve func(name string) (T, error),
) (map[string]T, error) {
	out := make(map[string]T, len(names))
	for _, name := range names {
		item, err := resolve(name)
		if err != nil {
			return nil, fmt.Errorf("resolve capability %q name %q: %w", spec.Name, name, err)
		}
		out[name] = item
	}
	return out, nil
}

// ResolveNamedRuntimeCapabilities resolves named module capabilities from the hub.
func ResolveNamedRuntimeCapabilities[T any](
	hub *module.Hub,
	bindings map[string][]string,
	key string,
	spec module.CapabilitySpec,
) (map[string]T, error) {
	return ResolveNamedCapabilityMap(
		bindings[key],
		spec,
		func(name string) (T, error) {
			return module.ResolveNamed[T](hub, spec, name)
		},
	)
}

// ResolveOrderedRuntimeCapabilities resolves ordered module capabilities from the hub.
func ResolveOrderedRuntimeCapabilities[T any](
	hub *module.Hub,
	bindings map[string][]string,
	key string,
	spec module.CapabilitySpec,
) (map[string]T, error) {
	return ResolveOrderedCapabilityMap[T](hub, bindings[key], spec)
}

// CopyIntoMap copies all entries into the destination map.
func CopyIntoMap[K comparable, V any](dst map[K]V, src map[K]V) {
	for key, value := range src {
		dst[key] = value
	}
}

// CopyPreferredIntoMap copies entries, applying preferred overrides when present.
func CopyPreferredIntoMap[K comparable, V any](dst map[K]V, src map[K]V, preferred map[K]V) {
	for key, value := range src {
		if override, ok := preferred[key]; ok {
			dst[key] = override
			continue
		}
		dst[key] = value
	}
}

// ResolveOrderedCapabilityMap resolves ordered capabilities into a name-keyed map.
func ResolveOrderedCapabilityMap[T any](
	hub *module.Hub,
	names []string,
	spec module.CapabilitySpec,
) (map[string]T, error) {
	items, err := module.ResolveOrdered[T](hub, spec, names)
	if err != nil {
		return nil, err
	}
	out := make(map[string]T, len(items))
	for i, name := range names {
		out[name] = items[i]
	}
	return out, nil
}

// BindLoggerWriterBuilders applies builtin writer config bindings.
func BindLoggerWriterBuilders(
	resolved settings.Resolved,
	builders map[string]logger.WriterBuilder,
) map[string]logger.WriterBuilder {
	out := cloneMap(builders)
	if _, ok := out["console"]; ok {
		out["console"] = func(string) (io.Writer, error) {
			return os.Stdout, nil
		}
	}
	if _, ok := out["file"]; ok {
		out["file"] = func(name string) (io.Writer, error) {
			writer := &lumberjack.Logger{}
			spec := resolved.Logging.Writers[name]
			if err := config.NewSnapshot(spec.Config).Decode(writer); err != nil {
				return nil, err
			}
			return writer, nil
		}
	}
	return out
}

// BindLoggerHandlerBuilders applies builtin handler config bindings.
func BindLoggerHandlerBuilders(
	resolved settings.Resolved,
	builders map[string]logger.HandlerBuilder,
	writerBuilders map[string]logger.WriterBuilder,
) map[string]logger.HandlerBuilder {
	out := cloneMap(builders)
	buildWriter := func(name string) (io.Writer, error) {
		spec := resolved.Logging.Writers[name]
		writerType := spec.Type
		if writerType == "" && name == "default" {
			writerType = "console"
		}
		builder := writerBuilders[writerType]
		if builder == nil {
			return nil, fmt.Errorf("writer builder for type %s not found", writerType)
		}
		return builder(name)
	}
	if _, ok := out["json"]; ok {
		out["json"] = func(writer string, cfgMap map[string]any) (slog.Handler, error) {
			cfg := &logger.JSONHandlerConfig{}
			if err := config.NewSnapshot(cfgMap).Decode(cfg); err != nil {
				return nil, err
			}
			target, err := buildWriter(writer)
			if err != nil {
				return nil, err
			}
			cfg.Writer = target
			return logger.NewJSONHandler(cfg)
		}
	}
	if _, ok := out["text"]; ok {
		out["text"] = func(writer string, cfgMap map[string]any) (slog.Handler, error) {
			cfg := &logger.ConsoleHandlerConfig{}
			if err := config.NewSnapshot(cfgMap).Decode(cfg); err != nil {
				return nil, err
			}
			target, err := buildWriter(writer)
			if err != nil {
				return nil, err
			}
			cfg.Writer = target
			return logger.NewConsoleHandler(cfg)
		}
	}
	return out
}

// BindStatsHandlerBuilders applies builtin stats config bindings.
func BindStatsHandlerBuilders(
	resolved settings.Resolved,
	builders map[string]stats.HandlerBuilder,
) map[string]stats.HandlerBuilder {
	out := cloneMap(builders)
	if _, ok := out["otel"]; ok {
		cfg := statsotel.Config{}
		_ = settings.DecodePayload(&cfg, resolved.Telemetry.Stats.Providers.OTel)
		out["otel"] = statsotel.BuiltinHandlerBuilderWithConfig(cfg)
	}
	return out
}

// CompileSecurityProfiles compiles configured security profiles.
func CompileSecurityProfiles(
	resolved settings.Resolved,
	providers map[string]security.Provider,
) (map[string]security.Profile, error) {
	out := make(map[string]security.Profile, len(resolved.Transports.SecurityProfiles))
	for name, spec := range resolved.Transports.SecurityProfiles {
		provider := providers[spec.Type]
		if provider == nil {
			return nil, fmt.Errorf("security provider for type %q not found", spec.Type)
		}
		profile, err := provider.Compile(name, spec.Config)
		if err != nil {
			return nil, fmt.Errorf("compile security profile %q: %w", name, err)
		}
		out[name] = profile
	}
	return out, nil
}

// LoggingInterceptorSource returns the preferred logging interceptor config source.
func LoggingInterceptorSource(resolved settings.Resolved) any {
	if cfg := resolved.Logging.Interceptors["logging"]; cfg != nil {
		return cfg
	}
	if cfg := resolved.Logging.Interceptors["logger"]; cfg != nil {
		return cfg
	}
	return nil
}

// NewMarshalerProvider builds the runtime marshaler REST provider.
func NewMarshalerProvider(
	resolved settings.Resolved,
	builders map[string]marshaler.MarshalerBuilder,
) rest.Provider {
	supported := []string{marshaler.SchemeJSONPb}
	var jsonpbCfg *marshaler.JSONPbConfig
	if resolved.Transports.Rest != nil {
		if len(resolved.Transports.Rest.Marshaler.Support) > 0 {
			supported = append(
				[]string(nil),
				resolved.Transports.Rest.Marshaler.Support...,
			)
		}
		jsonpbCfg = resolved.Transports.Rest.Marshaler.Config.JSONPB
	}
	registry := marshaler.BuildMarshalerRegistryWithBuilders(
		builders,
		jsonpbCfg,
		supported...,
	)
	return rest.NewProvider(
		"marshaler",
		func() func(http.Handler) http.Handler {
			return rest.NewMarshalerMiddleware(registry)
		},
	)
}

type namedProvider interface {
	Name() string
}

// MapNamedProviders converts named items into a lookup map.
func MapNamedProviders[T namedProvider](items []T) map[string]T {
	out := make(map[string]T, len(items))
	for _, item := range items {
		out[item.Name()] = item
	}
	return out
}

// MapUnaryServerProviders converts unary server providers into a lookup map.
func MapUnaryServerProviders(
	items []interceptor.UnaryServerInterceptorProvider,
) map[string]interceptor.UnaryServerInterceptorProvider {
	return MapNamedProviders(items)
}

// MapStreamServerProviders converts stream server providers into a lookup map.
func MapStreamServerProviders(
	items []interceptor.StreamServerInterceptorProvider,
) map[string]interceptor.StreamServerInterceptorProvider {
	return MapNamedProviders(items)
}

// MapUnaryClientProviders converts unary client providers into a lookup map.
func MapUnaryClientProviders(
	items []interceptor.UnaryClientInterceptorProvider,
) map[string]interceptor.UnaryClientInterceptorProvider {
	return MapNamedProviders(items)
}

// MapStreamClientProviders converts stream client providers into a lookup map.
func MapStreamClientProviders(
	items []interceptor.StreamClientInterceptorProvider,
) map[string]interceptor.StreamClientInterceptorProvider {
	return MapNamedProviders(items)
}

// ResolvedRequiresRestart reports whether changes in resolved settings require a restart.
func ResolvedRequiresRestart(current, next settings.Resolved) bool {
	return !reflect.DeepEqual(current.Server, next.Server) ||
		!reflect.DeepEqual(current.Clients, next.Clients) ||
		!reflect.DeepEqual(current.Discovery, next.Discovery) ||
		!reflect.DeepEqual(current.Balancers, next.Balancers) ||
		!reflect.DeepEqual(current.Transports, next.Transports) ||
		!reflect.DeepEqual(current.Extensions, next.Extensions) ||
		!reflect.DeepEqual(current.Telemetry.Stats, next.Telemetry.Stats)
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
