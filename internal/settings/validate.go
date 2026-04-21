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
	"errors"
	"fmt"
	"log/slog"

	"github.com/codesjoy/yggdrasil/v2/interceptor"
	xotel "github.com/codesjoy/yggdrasil/v2/otel"
	"github.com/codesjoy/yggdrasil/v2/registry"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/remote/credentials"
	"github.com/codesjoy/yggdrasil/v2/remote/marshaler"
	"github.com/codesjoy/yggdrasil/v2/remote/rest/middleware"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
)

// Validate validates the resolved framework configuration.
func Validate(resolved Resolved) error {
	strict := resolved.Admin.Validation.Strict
	enable := strict || resolved.Admin.Validation.Enable
	if !enable {
		return nil
	}

	var multiErr error
	addErr := func(msg string, err error, attrs ...slog.Attr) {
		if err == nil {
			return
		}
		if strict {
			multiErr = errors.Join(multiErr, fmt.Errorf("%s: %w", msg, err))
			return
		}
		attrs = append(attrs, slog.Any("error", err))
		args := make([]any, 0, len(attrs))
		for _, a := range attrs {
			args = append(args, a)
		}
		slog.Warn(msg, args...)
	}

	if typeName := resolved.Discovery.Registry.Type; typeName != "" && registry.GetBuilder(typeName) == nil {
		addErr("registry builder not found", fmt.Errorf("type=%s", typeName), slog.String("type", typeName))
	}
	for name, spec := range resolved.Discovery.Resolvers {
		if spec.Type == "" {
			continue
		}
		if !resolver.HasBuilder(spec.Type) {
			addErr("resolver builder not found", fmt.Errorf("type=%s", spec.Type), slog.String("name", name))
		}
	}
	if tracerName := resolved.Telemetry.Tracer; tracerName != "" {
		if _, ok := xotel.GetTracerProviderBuilder(tracerName); !ok {
			addErr("tracer provider builder not found", fmt.Errorf("name=%s", tracerName), slog.String("name", tracerName))
		}
	}
	if meterName := resolved.Telemetry.Meter; meterName != "" {
		if _, ok := xotel.GetMeterProviderBuilder(meterName); !ok {
			addErr("meter provider builder not found", fmt.Errorf("name=%s", meterName), slog.String("name", meterName))
		}
	}
	validateStatsHandlers := func(raw string, key string) {
		for _, name := range stats.ParseHandlerNames(raw) {
			if stats.GetHandlerBuilder(name) == nil {
				addErr(
					"stats handler builder not found",
					fmt.Errorf("name=%s", name),
					slog.String("name", name),
					slog.String("key", key),
				)
			}
		}
	}
	validateStatsHandlers(resolved.Telemetry.Stats.Server, "yggdrasil.telemetry.stats.server")
	validateStatsHandlers(resolved.Telemetry.Stats.Client, "yggdrasil.telemetry.stats.client")
	for _, protocol := range resolved.Server.Transports {
		if remote.GetServerBuilder(protocol) == nil {
			addErr("remote server builder not found", fmt.Errorf("protocol=%s", protocol), slog.String("protocol", protocol))
		}
	}
	validateCredential := func(protoName, serviceName string, client bool, key string) {
		if protoName == "" {
			return
		}
		builder := credentials.GetBuilder(protoName)
		if builder == nil {
			addErr(
				"remote credentials builder not found",
				fmt.Errorf("name=%s", protoName),
				slog.String("name", protoName),
				slog.String("key", key),
			)
			return
		}
		if protoName == "tls" {
			if err := validateTLSCredentialConfig(resolved, serviceName, client); err != nil {
				addErr(
					"remote credentials config invalid",
					fmt.Errorf("name=%s: %w", protoName, err),
					slog.String("name", protoName),
					slog.String("key", key),
				)
			}
			return
		}
		if builder(serviceName, client) == nil {
			addErr(
				"remote credentials config invalid",
				fmt.Errorf("name=%s", protoName),
				slog.String("name", protoName),
				slog.String("key", key),
			)
		}
	}
	validateCredential(
		resolved.Transports.GRPC.Server.CredsProto,
		"",
		false,
		"yggdrasil.transports.grpc.server.creds_proto",
	)
	validateCredential(
		resolved.Transports.GRPC.Client.Transport.CredsProto,
		"",
		true,
		"yggdrasil.transports.grpc.client.transport.creds_proto",
	)
	for serviceName, cfg := range resolved.Transports.GRPC.ClientServices {
		validateCredential(
			cfg.Transport.CredsProto,
			serviceName,
			true,
			fmt.Sprintf("yggdrasil.clients.services.%s.transports.grpc.transport.creds_proto", serviceName),
		)
	}
	for _, name := range resolved.Server.Interceptors.Unary {
		if !interceptor.HasUnaryServerIntBuilder(name) {
			addErr("unary server interceptor not found", fmt.Errorf("name=%s", name), slog.String("name", name))
		}
	}
	for _, name := range resolved.Server.Interceptors.Stream {
		if !interceptor.HasStreamServerIntBuilder(name) {
			addErr("stream server interceptor not found", fmt.Errorf("name=%s", name), slog.String("name", name))
		}
	}
	for _, name := range resolved.Root.Yggdrasil.Clients.Defaults.Interceptors.Unary {
		if !interceptor.HasUnaryClientIntBuilder(name) {
			addErr("unary client interceptor not found", fmt.Errorf("name=%s", name), slog.String("name", name))
		}
	}
	for _, name := range resolved.Root.Yggdrasil.Clients.Defaults.Interceptors.Stream {
		if !interceptor.HasStreamClientIntBuilder(name) {
			addErr("stream client interceptor not found", fmt.Errorf("name=%s", name), slog.String("name", name))
		}
	}
	for serviceName, cfg := range resolved.Clients.Services {
		for _, name := range cfg.Interceptors.Unary {
			if !interceptor.HasUnaryClientIntBuilder(name) {
				addErr(
					"unary client interceptor not found",
					fmt.Errorf("name=%s", name),
					slog.String("name", name),
					slog.String("app", serviceName),
				)
			}
		}
		for _, name := range cfg.Interceptors.Stream {
			if !interceptor.HasStreamClientIntBuilder(name) {
				addErr(
					"stream client interceptor not found",
					fmt.Errorf("name=%s", name),
					slog.String("name", name),
					slog.String("app", serviceName),
				)
			}
		}
	}
	if resolved.Transports.Rest != nil {
		for _, name := range resolved.Transports.Rest.Middleware.All {
			if !middleware.HasBuilder(name) {
				addErr("rest middleware not found", fmt.Errorf("name=%s", name), slog.String("name", name))
			}
		}
		for _, name := range resolved.Transports.Rest.Middleware.RPC {
			if !middleware.HasBuilder(name) {
				addErr("rest middleware not found", fmt.Errorf("name=%s", name), slog.String("name", name))
			}
		}
		for _, name := range resolved.Transports.Rest.Middleware.Web {
			if !middleware.HasBuilder(name) {
				addErr("rest middleware not found", fmt.Errorf("name=%s", name), slog.String("name", name))
			}
		}
		if !middleware.HasBuilder("marshaler") {
			addErr("rest middleware not found", fmt.Errorf("name=marshaler"), slog.String("name", "marshaler"))
		}
		schemes := resolved.Transports.Rest.Marshaler.Support
		if len(schemes) == 0 {
			schemes = []string{marshaler.SchemeJSONPb}
		}
		for _, scheme := range schemes {
			if !marshaler.HasMarshallerBuilder(scheme) {
				addErr(
					"rest marshaler builder not found",
					fmt.Errorf("scheme=%s", scheme),
					slog.String("scheme", scheme),
				)
			}
		}
	}
	return multiErr
}
