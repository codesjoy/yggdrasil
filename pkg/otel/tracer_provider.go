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

package otel

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{}))
}

// TracerProviderBuilder is a function that returns a TracerProvider.
type TracerProviderBuilder func(name string) trace.TracerProvider

var tracerBuilders = make(map[string]TracerProviderBuilder)

// RegisterTracerProviderBuilder registers a TracerProviderBuilder.
func RegisterTracerProviderBuilder(name string, constructor TracerProviderBuilder) {
	tracerBuilders[name] = constructor
}

// GetTracerProviderBuilder returns a TracerProviderBuilder.
func GetTracerProviderBuilder(name string) (TracerProviderBuilder, bool) {
	constructor, ok := tracerBuilders[name]
	return constructor, ok
}
