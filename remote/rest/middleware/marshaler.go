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

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/remote/marshaler"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	RegisterBuilder("marshaler", newMarshalerMiddleware)
}

func newMarshalerMiddleware() func(http.Handler) http.Handler {
	key := config.Join(config.KeyBase, "rest", "marshaler", "support")
	schemes := config.GetStringSlice(key, []string{"jsonpb"})

	// Check if we need to load config for jsonpb
	hasJSONPb := false
	for _, scheme := range schemes {
		if scheme == "jsonpb" {
			hasJSONPb = true
			break
		}
	}

	var mr marshaler.Registry
	if hasJSONPb {
		// Manually build registry to inject config for jsonpb
		mr = buildRegistryWithJSONPbConfig(schemes)
	} else {
		mr = marshaler.BuildMarshalerRegistry(schemes...)
	}

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

func buildRegistryWithJSONPbConfig(schemes []string) marshaler.Registry {
	mr := marshaler.NewRegistry()
	cfg := &marshaler.JSONPbConfig{}
	key := config.Join(config.KeyBase, "rest", "marshaler", "config", "jsonpb")
	_ = config.Get(key).Scan(cfg) // Ignore error, use default if failed

	for _, item := range schemes {
		if item == "jsonpb" {
			m := &marshaler.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					Multiline:       cfg.MarshalOptions.Multiline,
					Indent:          cfg.MarshalOptions.Indent,
					AllowPartial:    cfg.MarshalOptions.AllowPartial,
					UseProtoNames:   cfg.MarshalOptions.UseProtoNames,
					UseEnumNumbers:  cfg.MarshalOptions.UseEnumNumbers,
					EmitUnpopulated: cfg.MarshalOptions.EmitUnpopulated,
				},
				UnmarshalOptions: protojson.UnmarshalOptions{
					AllowPartial:   cfg.UnmarshalOptions.AllowPartial,
					DiscardUnknown: cfg.UnmarshalOptions.DiscardUnknown,
					RecursionLimit: cfg.UnmarshalOptions.RecursionLimit,
				},
			}
			_ = mr.Register(item, m)
		} else {
			// For other schemes, use the default builder
			// Ideally we would want to mix configured jsonpb with other default marshalers
			// But buildMarshaller is not easily accessible here directly if we wanted to be 100% correct without duplication
			// However, the logic below handles the remaining items.
		}
	}

	// Re-implementing the loop properly using casting
	for _, item := range schemes {
		if item == "jsonpb" {
			continue // Handled in the first loop logic for jsonpb specifically
		}

		m, err := marshaler.BuildMarshaller(item)
		if err != nil {
			// Ignore error as per original logic's intent (which logged but continued)
			// Ideally we should log a warning here if we had logger access
			continue
		}
		_ = mr.Register(item, m)
	}

	return mr
}
