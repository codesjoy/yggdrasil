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

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codesjoy/yggdrasil/v2/rest/marshaler"
	"github.com/stretchr/testify/assert"
)

func TestRegistry(t *testing.T) {
	// "logger" and "marshaler" are registered in init()
	mws := GetMiddlewares("logger", "marshaler", "unknown")
	assert.Len(t, mws, 2)
}

func TestLoggerMiddleware(t *testing.T) {
	mw := requestLogger()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMarshalerMiddleware(t *testing.T) {
	mw := newMarshalerMiddleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify context values are set
		in := marshaler.InboundFromContext(r.Context())
		out := marshaler.OutboundFromContext(r.Context())

		assert.NotNil(t, in, "Inbound marshaler should be set")
		assert.NotNil(t, out, "Outbound marshaler should be set")

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
