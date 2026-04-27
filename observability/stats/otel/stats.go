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
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/codesjoy/yggdrasil/v3/observability/stats"
)

// HandlerRuntime contains App-local OTel dependencies for the built-in stats
// handler.
type HandlerRuntime struct {
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	Propagator     propagation.TextMapPropagator
}

// BuiltinHandlerBuilder returns the built-in otel stats builder.
func BuiltinHandlerBuilder() stats.HandlerBuilder {
	return BuiltinHandlerBuilderWithConfig(*getCfg())
}

// BuiltinHandlerBuilderWithConfig returns the built-in otel stats builder bound to one explicit config.
func BuiltinHandlerBuilderWithConfig(cfg Config) stats.HandlerBuilder {
	return BuiltinHandlerBuilderWithRuntime(cfg, HandlerRuntime{})
}

// BuiltinHandlerBuilderWithRuntime returns the built-in otel stats builder
// bound to explicit config and App-local OTel dependencies.
func BuiltinHandlerBuilderWithRuntime(
	cfg Config,
	runtime HandlerRuntime,
) stats.HandlerBuilder {
	return func(isServer bool) stats.Handler {
		if isServer {
			return newSvrHandlerWithRuntime(&cfg, runtime)
		}
		return newCliHandlerWithRuntime(&cfg, runtime)
	}
}

// RegisterBuiltinHandler registers the built-in otel stats handler.
func RegisterBuiltinHandler() {
	stats.RegisterHandlerBuilder("otel", BuiltinHandlerBuilder())
}
