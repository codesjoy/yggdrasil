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

// Package marshaler provides shared payload marshaling for HTTP-facing transports.
package marshaler

import (
	"context"
	"errors"
	"strings"
)

// HasMarshalerBuilder returns true if the marshaler builder exists
func HasMarshalerBuilder(scheme string) bool {
	return builtInMarshalerBuilder(normalizeScheme(scheme)) != nil
}

// BuildMarshaler builds a marshaler for the given scheme
func BuildMarshaler(scheme string) (Marshaler, error) {
	builder := builtInMarshalerBuilder(normalizeScheme(scheme))
	if builder == nil {
		return nil, errors.New("transport marshaler builder not found")
	}
	return builder()
}

// BuildMarshalerWithConfig builds a marshaler for the given scheme and optional config.
func BuildMarshalerWithConfig(scheme string, jsonpbCfg *JSONPbConfig) (Marshaler, error) {
	switch normalizeScheme(scheme) {
	case SchemeJSONPb:
		return NewJSONPbMarshalerWithConfig(jsonpbCfg), nil
	default:
		return BuildMarshaler(scheme)
	}
}

// BuildMarshalerWithBuilders builds a marshaler for the given scheme using an explicit builder map.
func BuildMarshalerWithBuilders(
	builders map[string]MarshalerBuilder,
	scheme string,
	jsonpbCfg *JSONPbConfig,
) (Marshaler, error) {
	switch normalizeScheme(scheme) {
	case SchemeJSONPb:
		return NewJSONPbMarshalerWithConfig(jsonpbCfg), nil
	default:
		normalized := normalizeScheme(scheme)
		if builder, ok := builders[normalized]; ok {
			return builder()
		}
		return BuildMarshaler(normalized)
	}
}

func builtInMarshalerBuilder(scheme string) MarshalerBuilder {
	switch scheme {
	case SchemeJSONPb:
		return JSONPbBuilder()
	case SchemeProto:
		return ProtoBuilder()
	default:
		return nil
	}
}

func normalizeScheme(scheme string) string {
	return strings.ToLower(strings.TrimSpace(scheme))
}

type (
	inbound  = struct{}
	outbound = struct{}
)

// InboundFromContext returns the marshaler for inbound
func InboundFromContext(ctx context.Context) Marshaler {
	m, ok := ctx.Value(inbound{}).(Marshaler)
	if !ok {
		return defaultMarshaler
	}
	return m
}

// WithInboundContext returns a new context with the marshaler for inbound
func WithInboundContext(ctx context.Context, m Marshaler) context.Context {
	return context.WithValue(ctx, inbound{}, m)
}

// OutboundFromContext returns the marshaler for outbound
func OutboundFromContext(ctx context.Context) Marshaler {
	m, ok := ctx.Value(outbound{}).(Marshaler)
	if !ok {
		return defaultMarshaler
	}
	return m
}

// WithOutboundContext returns a new context with the marshaler for outbound
func WithOutboundContext(ctx context.Context, m Marshaler) context.Context {
	return context.WithValue(ctx, outbound{}, m)
}
