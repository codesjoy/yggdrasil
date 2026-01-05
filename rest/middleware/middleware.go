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

	"github.com/go-chi/chi/v5"
)

// Builder defines a middleware builder
type Builder func() func(http.Handler) http.Handler

var builder = map[string]Builder{}

// RegisterBuilder registers a new middleware builder
func RegisterBuilder(name string, f Builder) {
	builder[name] = f
}

// GetMiddlewares returns a list of middlewares
func GetMiddlewares(names ...string) chi.Middlewares {
	handlers := make(chi.Middlewares, 0, len(names))
	for _, item := range names {
		if f, ok := builder[item]; ok {
			handlers = append(handlers, f())
		}
	}
	return handlers
}
