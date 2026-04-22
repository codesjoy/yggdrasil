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
	"io"
	"log/slog"
	"net/http"
	"os"
	"reflect"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/interceptor"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/logger"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/remote/credentials"
	"github.com/codesjoy/yggdrasil/v3/remote/credentials/insecure"
	"github.com/codesjoy/yggdrasil/v3/remote/credentials/local"
	ytls "github.com/codesjoy/yggdrasil/v3/remote/credentials/tls"
	"github.com/codesjoy/yggdrasil/v3/remote/marshaler"
	restmiddleware "github.com/codesjoy/yggdrasil/v3/server/rest/middleware"
	"github.com/codesjoy/yggdrasil/v3/stats"
	statsotel "github.com/codesjoy/yggdrasil/v3/stats/otel"
)

func resolveNamedCapabilityMap[T any](
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

func copyIntoMap[K comparable, V any](dst map[K]V, src map[K]V) {
	for key, value := range src {
		dst[key] = value
	}
}

func copyPreferredIntoMap[K comparable, V any](dst map[K]V, src map[K]V, preferred map[K]V) {
	for key, value := range src {
		if override, ok := preferred[key]; ok {
			dst[key] = override
			continue
		}
		dst[key] = value
	}
}

func resolveOrderedCapabilityMap[T any](
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

func bindLoggerWriterBuilders(
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

func bindLoggerHandlerBuilders(
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

func bindStatsHandlerBuilders(
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

func bindCredentialsBuilders(
	resolved settings.Resolved,
	builders map[string]credentials.Builder,
) map[string]credentials.Builder {
	out := cloneMap(builders)
	if _, ok := out["insecure"]; ok {
		out["insecure"] = insecure.BuiltinBuilder()
	}
	if _, ok := out["local"]; ok {
		out["local"] = local.BuiltinBuilder()
	}
	if _, ok := out["tls"]; ok {
		global, services := resolveTLSBuilderConfig(resolved)
		out["tls"] = ytls.BuiltinBuilderWithConfig(global, services)
	}
	return out
}

func resolveTLSBuilderConfig(resolved settings.Resolved) (ytls.BuilderConfig, map[string]ytls.BuilderConfig) {
	var global ytls.BuilderConfig
	if raw, ok := resolved.Transports.GRPCCredentials["tls"]; ok {
		_ = settings.DecodePayload(&global, raw)
	}
	services := map[string]ytls.BuilderConfig{}
	for serviceName, specs := range resolved.Transports.GRPCServiceCredentials {
		raw, ok := specs["tls"]
		if !ok {
			continue
		}
		cfg := ytls.BuilderConfig{}
		_ = settings.DecodePayload(&cfg, raw)
		services[serviceName] = cfg
	}
	return global, services
}

func loggingInterceptorSource(resolved settings.Resolved) any {
	if cfg := resolved.Logging.Interceptors["logging"]; cfg != nil {
		return cfg
	}
	if cfg := resolved.Logging.Interceptors["logger"]; cfg != nil {
		return cfg
	}
	return nil
}

func newRuntimeMarshalerProvider(snapshot *Snapshot) restmiddleware.Provider {
	supported := []string{marshaler.SchemeJSONPb}
	var jsonpbCfg *marshaler.JSONPbConfig
	if snapshot != nil && snapshot.Resolved.Transports.Rest != nil {
		if len(snapshot.Resolved.Transports.Rest.Marshaler.Support) > 0 {
			supported = append([]string(nil), snapshot.Resolved.Transports.Rest.Marshaler.Support...)
		}
		jsonpbCfg = snapshot.Resolved.Transports.Rest.Marshaler.Config.JSONPB
	}
	registry := marshaler.BuildMarshalerRegistryWithBuilders(
		snapshot.MarshalerBuilderMap,
		jsonpbCfg,
		supported...,
	)
	return restmiddleware.NewProvider(
		"marshaler",
		func() func(http.Handler) http.Handler {
			return restmiddleware.NewMarshalerMiddleware(registry)
		},
	)
}

type namedProvider interface {
	Name() string
}

func mapNamedProviders[T namedProvider](items []T) map[string]T {
	out := make(map[string]T, len(items))
	for _, item := range items {
		out[item.Name()] = item
	}
	return out
}

func mapUnaryServerProviders(
	items []interceptor.UnaryServerInterceptorProvider,
) map[string]interceptor.UnaryServerInterceptorProvider {
	return mapNamedProviders(items)
}

func mapStreamServerProviders(
	items []interceptor.StreamServerInterceptorProvider,
) map[string]interceptor.StreamServerInterceptorProvider {
	return mapNamedProviders(items)
}

func mapUnaryClientProviders(
	items []interceptor.UnaryClientInterceptorProvider,
) map[string]interceptor.UnaryClientInterceptorProvider {
	return mapNamedProviders(items)
}

func mapStreamClientProviders(
	items []interceptor.StreamClientInterceptorProvider,
) map[string]interceptor.StreamClientInterceptorProvider {
	return mapNamedProviders(items)
}

func runtimeRequiresRestart(current, next *Snapshot) bool {
	if current == nil || next == nil {
		return false
	}
	return !reflect.DeepEqual(current.Resolved.Server, next.Resolved.Server) ||
		!reflect.DeepEqual(current.Resolved.Clients, next.Resolved.Clients) ||
		!reflect.DeepEqual(current.Resolved.Discovery, next.Resolved.Discovery) ||
		!reflect.DeepEqual(current.Resolved.Balancers, next.Resolved.Balancers) ||
		!reflect.DeepEqual(current.Resolved.Transports, next.Resolved.Transports) ||
		!reflect.DeepEqual(current.Resolved.Extensions, next.Resolved.Extensions) ||
		!reflect.DeepEqual(current.Resolved.Telemetry.Stats, next.Resolved.Telemetry.Stats)
}
