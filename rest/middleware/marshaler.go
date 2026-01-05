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
	"github.com/codesjoy/yggdrasil/v2/rest/marshaler"
)

func init() {
	RegisterBuilder("marshaler", newMarshalerMiddleware)
}

func newMarshalerMiddleware() func(http.Handler) http.Handler {
	key := config.Join(config.KeyBase, "rest", "marshaler", "support")
	schemes := config.GetStringSlice(key, []string{"jsonpb"})
	marshalerRegistry := marshaler.BuildMarshalerRegistry(schemes...)
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inbound, outbound := marshalerRegistry.GetMarshaler(r)
			ctx := marshaler.WithInboundContext(r.Context(), inbound)
			ctx = marshaler.WithOutboundContext(ctx, outbound)
			r = r.WithContext(ctx)
			handler.ServeHTTP(w, r)
		})
	}
}
