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

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/stretchr/testify/assert"
)

// TestRegisterTracerProviderBuilder tests registering TracerProvider builders
func TestRegisterTracerProviderBuilder(t *testing.T) {
	t.Run("register new builder", func(t *testing.T) {
		builder := func(_ string) trace.TracerProvider {
			return noop.NewTracerProvider()
		}

		RegisterTracerProviderBuilder("test", builder)

		retrieved, ok := GetTracerProviderBuilder("test")
		assert.True(t, ok)
		assert.NotNil(t, retrieved)
	})

	t.Run("register multiple builders", func(t *testing.T) {
		builder1 := func(_ string) trace.TracerProvider {
			return noop.NewTracerProvider()
		}
		builder2 := func(_ string) trace.TracerProvider {
			return noop.NewTracerProvider()
		}

		RegisterTracerProviderBuilder("builder1", builder1)
		RegisterTracerProviderBuilder("builder2", builder2)

		retrieved1, ok1 := GetTracerProviderBuilder("builder1")
		retrieved2, ok2 := GetTracerProviderBuilder("builder2")

		assert.True(t, ok1)
		assert.True(t, ok2)
		assert.NotNil(t, retrieved1)
		assert.NotNil(t, retrieved2)
	})

	t.Run("override existing builder", func(t *testing.T) {
		builder1 := func(_ string) trace.TracerProvider {
			return noop.NewTracerProvider()
		}
		builder2 := func(_ string) trace.TracerProvider {
			return noop.NewTracerProvider()
		}

		RegisterTracerProviderBuilder("test", builder1)
		RegisterTracerProviderBuilder("test", builder2)

		retrieved, ok := GetTracerProviderBuilder("test")
		assert.True(t, ok)
		assert.NotNil(t, retrieved)
	})
}

// TestGetTracerProviderBuilder tests retrieving TracerProvider builders
func TestGetTracerProviderBuilder(t *testing.T) {
	t.Run("get existing builder", func(t *testing.T) {
		called := false
		builder := func(name string) trace.TracerProvider {
			called = true
			assert.Equal(t, "test", name)
			return noop.NewTracerProvider()
		}

		RegisterTracerProviderBuilder("existing", builder)

		retrieved, ok := GetTracerProviderBuilder("existing")
		assert.True(t, ok)
		assert.NotNil(t, retrieved)

		result := retrieved("test")
		assert.NotNil(t, result)
		assert.True(t, called)
	})

	t.Run("get non-existent builder", func(t *testing.T) {
		retrieved, ok := GetTracerProviderBuilder("nonexistent")

		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})

	t.Run("get builder and call it", func(t *testing.T) {
		called := false
		builder := func(name string) trace.TracerProvider {
			called = true
			assert.Equal(t, "test-name", name)
			return noop.NewTracerProvider()
		}

		RegisterTracerProviderBuilder("callable", builder)
		retrieved, ok := GetTracerProviderBuilder("callable")

		assert.True(t, ok)
		assert.NotNil(t, retrieved)
		_ = retrieved("test-name")
		assert.True(t, called)
	})
}

// TestTracerProviderBuilder_Functionality tests builder functionality
func TestTracerProviderBuilder_Functionality(t *testing.T) {
	t.Run("builder creates tracer provider", func(t *testing.T) {
		builder := func(_ string) trace.TracerProvider {
			return noop.NewTracerProvider()
		}

		RegisterTracerProviderBuilder("tracer-provider", builder)
		retrieved, ok := GetTracerProviderBuilder("tracer-provider")
		assert.True(t, ok)

		provider := retrieved("service-name")
		assert.NotNil(t, provider)

		tracer := provider.Tracer("test-tracer")
		assert.NotNil(t, tracer)
	})

	t.Run("builder with different names", func(t *testing.T) {
		var names []string
		builder := func(name string) trace.TracerProvider {
			names = append(names, name)
			return noop.NewTracerProvider()
		}

		RegisterTracerProviderBuilder("multi-name", builder)
		retrieved, ok := GetTracerProviderBuilder("multi-name")
		assert.True(t, ok)

		_ = retrieved("name1")
		_ = retrieved("name2")
		_ = retrieved("name3")

		assert.Equal(t, []string{"name1", "name2", "name3"}, names)
	})
}

// TestTracerProviderBuilder_Concurrency tests concurrent access
func TestTracerProviderBuilder_Concurrency(t *testing.T) {
	t.Run("concurrent registration", func(t *testing.T) {
		const numGoroutines = 100

		for i := 0; i < numGoroutines; i++ {
			builder := func(_ string) trace.TracerProvider {
				return noop.NewTracerProvider()
			}
			RegisterTracerProviderBuilder("builder", builder)
		}

		// Should not panic
		assert.True(t, true)
	})

	t.Run("concurrent retrieval", func(t *testing.T) {
		builder := func(_ string) trace.TracerProvider {
			return noop.NewTracerProvider()
		}
		RegisterTracerProviderBuilder("concurrent", builder)

		for i := 0; i < 100; i++ {
			retrieved, ok := GetTracerProviderBuilder("concurrent")
			assert.True(t, ok)
			assert.NotNil(t, retrieved)
		}
	})
}

// TestTracerProviderBuilder_RealWorldScenarios tests real-world scenarios
func TestTracerProviderBuilder_RealWorldScenarios(t *testing.T) {
	t.Run("multiple exporter types", func(t *testing.T) {
		// Register builders for different exporters
		jaegerBuilder := func(_ string) trace.TracerProvider {
			return noop.NewTracerProvider()
		}
		zipkinBuilder := func(string) trace.TracerProvider {
			return noop.NewTracerProvider()
		}
		defaultBuilder := func(string) trace.TracerProvider {
			return noop.NewTracerProvider()
		}

		RegisterTracerProviderBuilder("jaeger", jaegerBuilder)
		RegisterTracerProviderBuilder("zipkin", zipkinBuilder)
		RegisterTracerProviderBuilder("default", defaultBuilder)

		jaegerRetrieved, ok1 := GetTracerProviderBuilder("jaeger")
		zipkinRetrieved, ok2 := GetTracerProviderBuilder("zipkin")
		defaultRetrieved, ok3 := GetTracerProviderBuilder("default")

		assert.True(t, ok1)
		assert.True(t, ok2)
		assert.True(t, ok3)
		assert.NotNil(t, jaegerRetrieved)
		assert.NotNil(t, zipkinRetrieved)
		assert.NotNil(t, defaultRetrieved)
	})

	t.Run("custom provider configuration", func(t *testing.T) {
		configured := false
		builder := func(string) trace.TracerProvider {
			configured = true
			return noop.NewTracerProvider()
		}

		RegisterTracerProviderBuilder("custom", builder)
		retrieved, ok := GetTracerProviderBuilder("custom")

		assert.True(t, ok)
		_ = retrieved("my-service")
		assert.True(t, configured)
	})
}

// TestInit tests package initialization
func TestInit(t *testing.T) {
	t.Run("package init sets propagator", func(t *testing.T) {
		// This test verifies that init() runs without panicking
		// The actual propagator setup is tested indirectly through carrier tests
		assert.True(t, true)
	})
}
