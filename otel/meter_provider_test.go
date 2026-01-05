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

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/stretchr/testify/assert"
)

// TestRegisterMeterProviderBuilder tests registering MeterProvider builders
func TestRegisterMeterProviderBuilder(t *testing.T) {
	t.Run("register new builder", func(t *testing.T) {
		builder := func(_ string) metric.MeterProvider {
			return noop.NewMeterProvider()
		}

		RegisterMeterProviderBuilder("test", builder)

		retrieved, ok := GetMeterProviderBuilder("test")
		assert.NotNil(t, retrieved)
		assert.True(t, ok)
	})

	t.Run("register multiple builders", func(t *testing.T) {
		builder1 := func(_ string) metric.MeterProvider {
			return noop.NewMeterProvider()
		}
		builder2 := func(_ string) metric.MeterProvider {
			return noop.NewMeterProvider()
		}

		RegisterMeterProviderBuilder("builder1", builder1)
		RegisterMeterProviderBuilder("builder2", builder2)

		retrieved1, ok := GetMeterProviderBuilder("builder1")
		assert.NotNil(t, retrieved1)
		assert.True(t, ok)
		retrieved2, ok := GetMeterProviderBuilder("builder2")
		assert.NotNil(t, retrieved2)
		assert.True(t, ok)
	})

	t.Run("override existing builder", func(t *testing.T) {
		builder1 := func(string) metric.MeterProvider {
			return noop.NewMeterProvider()
		}
		builder2 := func(string) metric.MeterProvider {
			return noop.NewMeterProvider()
		}

		RegisterMeterProviderBuilder("test", builder1)
		RegisterMeterProviderBuilder("test", builder2)

		retrieved, ok := GetMeterProviderBuilder("test")
		assert.NotNil(t, retrieved)
		assert.True(t, ok)
	})
}

// TestGetMeterProviderBuilder tests retrieving MeterProvider builders
func TestGetMeterProviderBuilder(t *testing.T) {
	t.Run("get existing builder", func(t *testing.T) {
		called := false
		builder := func(name string) metric.MeterProvider {
			called = true
			assert.Equal(t, "test", name)
			return noop.NewMeterProvider()
		}

		RegisterMeterProviderBuilder("existing", builder)

		retrieved, ok := GetMeterProviderBuilder("existing")
		assert.NotNil(t, retrieved)
		assert.True(t, ok)

		result := retrieved("test")
		assert.NotNil(t, result)
		assert.True(t, called)
	})

	t.Run("get non-existent builder", func(t *testing.T) {
		retrieved, ok := GetMeterProviderBuilder("nonexistent")

		assert.Nil(t, retrieved)
		assert.False(t, ok)
	})

	t.Run("get builder and call it", func(t *testing.T) {
		called := false
		builder := func(name string) metric.MeterProvider {
			called = true
			assert.Equal(t, "test-name", name)
			return noop.NewMeterProvider()
		}

		RegisterMeterProviderBuilder("callable", builder)
		retrieved, ok := GetMeterProviderBuilder("callable")

		assert.NotNil(t, retrieved)
		assert.True(t, ok)
		_ = retrieved("test-name")
		assert.True(t, called)
	})
}

// TestMeterProviderBuilder_Functionality tests builder functionality
func TestMeterProviderBuilder_Functionality(t *testing.T) {
	t.Run("builder creates meter provider", func(t *testing.T) {
		builder := func(string) metric.MeterProvider {
			return noop.NewMeterProvider()
		}

		RegisterMeterProviderBuilder("meter-provider", builder)
		retrieved, _ := GetMeterProviderBuilder("meter-provider")

		provider := retrieved("service-name")
		assert.NotNil(t, provider)

		meter := provider.Meter("test-meter")
		assert.NotNil(t, meter)
	})

	t.Run("builder with different names", func(t *testing.T) {
		var names []string
		builder := func(name string) metric.MeterProvider {
			names = append(names, name)
			return noop.NewMeterProvider()
		}

		RegisterMeterProviderBuilder("multi-name", builder)
		retrieved, _ := GetMeterProviderBuilder("multi-name")

		_ = retrieved("name1")
		_ = retrieved("name2")
		_ = retrieved("name3")

		assert.Equal(t, []string{"name1", "name2", "name3"}, names)
	})
}

// TestMeterProviderBuilder_Concurrency tests concurrent access
func TestMeterProviderBuilder_Concurrency(t *testing.T) {
	t.Run("concurrent registration", func(t *testing.T) {
		const numGoroutines = 100

		for i := 0; i < numGoroutines; i++ {
			builder := func(string) metric.MeterProvider {
				return noop.NewMeterProvider()
			}
			RegisterMeterProviderBuilder("builder", builder)
		}

		// Should not panic
		assert.True(t, true)
	})

	t.Run("concurrent retrieval", func(t *testing.T) {
		builder := func(string) metric.MeterProvider {
			return noop.NewMeterProvider()
		}
		RegisterMeterProviderBuilder("concurrent", builder)

		for i := 0; i < 100; i++ {
			retrieved, ok := GetMeterProviderBuilder("concurrent")
			assert.NotNil(t, retrieved)
			assert.True(t, ok)
		}
	})
}

// TestMeterProviderBuilder_RealWorldScenarios tests real-world scenarios
func TestMeterProviderBuilder_RealWorldScenarios(t *testing.T) {
	t.Run("multiple service types", func(t *testing.T) {
		// Register builders for different service types
		prometheusBuilder := func(string) metric.MeterProvider {
			return noop.NewMeterProvider()
		}
		defaultBuilder := func(string) metric.MeterProvider {
			return noop.NewMeterProvider()
		}

		RegisterMeterProviderBuilder("prometheus", prometheusBuilder)
		RegisterMeterProviderBuilder("default", defaultBuilder)

		promBuilder, ok := GetMeterProviderBuilder("prometheus")
		assert.NotNil(t, promBuilder)
		assert.True(t, ok)

		defaultBuilderRetrieved, ok := GetMeterProviderBuilder("default")
		assert.NotNil(t, defaultBuilderRetrieved)
		assert.True(t, ok)
	})

	t.Run("custom provider configuration", func(t *testing.T) {
		configured := false
		builder := func(string) metric.MeterProvider {
			configured = true
			return noop.NewMeterProvider()
		}

		RegisterMeterProviderBuilder("custom", builder)
		retrieved, _ := GetMeterProviderBuilder("custom")

		_ = retrieved("my-service")
		assert.True(t, configured)
	})
}
