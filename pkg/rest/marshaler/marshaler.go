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

// Package marshaler provides a marshaler for REST.
package marshaler

import (
	"context"
	"errors"
)

var marshalerBuilder = map[string]MarshallerBuilder{}

// RegisterMarshallerBuilder registers a new marshaler builder
func RegisterMarshallerBuilder(scheme string, builder MarshallerBuilder) {
	marshalerBuilder[scheme] = builder
}

func buildMarshaller(scheme string) (Marshaler, error) {
	f, ok := marshalerBuilder[scheme]
	if !ok {
		return nil, errors.New("rest marshaler builder not found")
	}
	return f()
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
