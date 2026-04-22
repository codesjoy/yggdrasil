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

// Package middleware provides middleware for REST.
package middleware

import (
	"net/http"

	"github.com/codesjoy/yggdrasil/v3/remote/marshaler"
)

// BuiltinMarshalerProvider returns the built-in marshaler middleware provider.
func BuiltinMarshalerProvider() Provider {
	return NewProvider("marshaler", newMarshalerMiddleware)
}

func newMarshalerMiddleware() func(http.Handler) http.Handler {
	schemes, _ := currentMarshalerConfig()
	if len(schemes) == 0 {
		schemes = []string{"jsonpb"}
	}
	mr := buildRegistry(schemes)

	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inbound, outbound := mr.GetMarshaler(r)
			ctx := marshaler.WithInboundContext(r.Context(), inbound)
			ctx = marshaler.WithOutboundContext(ctx, outbound)
			r = r.WithContext(ctx)
			handler.ServeHTTP(w, r)
		})
	}
}

// NewMarshalerMiddleware returns a new marshaler middleware with the given registry.
func NewMarshalerMiddleware(registry marshaler.Registry) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inbound, outbound := registry.GetMarshaler(r)
			ctx := marshaler.WithInboundContext(r.Context(), inbound)
			ctx = marshaler.WithOutboundContext(ctx, outbound)
			r = r.WithContext(ctx)
			handler.ServeHTTP(w, r)
		})
	}
}

func buildRegistry(schemes []string) marshaler.Registry {
	mr := marshaler.NewRegistry()
	_, cfg := currentMarshalerConfig()

	for _, item := range schemes {
		var marshalerCfg *marshaler.JSONPbConfig
		if item == marshaler.SchemeJSONPb {
			marshalerCfg = cfg
		}
		m, err := marshaler.BuildMarshallerWithConfig(item, marshalerCfg)
		if err != nil {
			continue
		}
		_ = mr.Register(item, m)
	}

	return mr
}
