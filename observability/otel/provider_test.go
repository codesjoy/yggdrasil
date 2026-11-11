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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

type providerRegistry[T any] struct {
	register func(string, func(string) T)
	get      func(string) (func(string) T, bool)
	build    func(string) T
	check    func(T)
}

func testProviderRegistry[T any](
	t *testing.T,
	registry providerRegistry[T],
) {
	t.Helper()

	t.Run("register and retrieve", func(t *testing.T) {
		registry.register("registered", registry.build)
		builder, ok := registry.get("registered")
		require.True(t, ok)
		registry.check(builder("service"))
	})

	t.Run("multiple builders", func(t *testing.T) {
		registry.register("builder-1", registry.build)
		registry.register("builder-2", registry.build)
		_, ok1 := registry.get("builder-1")
		_, ok2 := registry.get("builder-2")
		assert.True(t, ok1)
		assert.True(t, ok2)
	})

	t.Run("override builder", func(t *testing.T) {
		registry.register("override", func(string) T { return registry.build("old") })
		registry.register("override", func(string) T { return registry.build("new") })
		builder, ok := registry.get("override")
		require.True(t, ok)
		registry.check(builder("service"))
	})

	t.Run("missing builder", func(t *testing.T) {
		builder, ok := registry.get("missing")
		assert.False(t, ok)
		assert.Nil(t, builder)
	})

	t.Run("builder receives service names", func(t *testing.T) {
		var names []string
		registry.register("named", func(name string) T {
			names = append(names, name)
			return registry.build(name)
		})
		builder, ok := registry.get("named")
		require.True(t, ok)
		registry.check(builder("name-1"))
		registry.check(builder("name-2"))
		registry.check(builder("name-3"))
		assert.Equal(t, []string{"name-1", "name-2", "name-3"}, names)
	})

	t.Run("repeated registration and retrieval", func(t *testing.T) {
		for range 100 {
			registry.register("repeat", registry.build)
			builder, ok := registry.get("repeat")
			require.True(t, ok)
			registry.check(builder("service"))
		}
	})
}

func TestTracerProviderBuilders(t *testing.T) {
	testProviderRegistry(t, providerRegistry[trace.TracerProvider]{
		register: func(name string, builder func(string) trace.TracerProvider) {
			RegisterTracerProviderBuilder(name, TracerProviderBuilder(builder))
		},
		get: func(name string) (func(string) trace.TracerProvider, bool) {
			builder, ok := GetTracerProviderBuilder(name)
			if !ok {
				return nil, false
			}
			return func(serviceName string) trace.TracerProvider {
				return builder(serviceName)
			}, true
		},
		build: func(string) trace.TracerProvider {
			return tracenoop.NewTracerProvider()
		},
		check: func(provider trace.TracerProvider) {
			assert.NotNil(t, provider.Tracer("test"))
		},
	})
}

func TestMeterProviderBuilders(t *testing.T) {
	testProviderRegistry(t, providerRegistry[metric.MeterProvider]{
		register: func(name string, builder func(string) metric.MeterProvider) {
			RegisterMeterProviderBuilder(name, MeterProviderBuilder(builder))
		},
		get: func(name string) (func(string) metric.MeterProvider, bool) {
			builder, ok := GetMeterProviderBuilder(name)
			if !ok {
				return nil, false
			}
			return func(serviceName string) metric.MeterProvider {
				return builder(serviceName)
			}, true
		},
		build: func(string) metric.MeterProvider {
			return metricnoop.NewMeterProvider()
		},
		check: func(provider metric.MeterProvider) {
			assert.NotNil(t, provider.Meter("test"))
		},
	})
}
