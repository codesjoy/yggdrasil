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

	"go.opentelemetry.io/otel/attribute"

	"github.com/stretchr/testify/assert"
)

// TestParseAttributes_Bool tests parsing boolean attributes
func TestParseAttributes_Bool(t *testing.T) {
	t.Run("single bool", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"enabled": true,
			"active":  false,
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 2)
		assert.Contains(t, attrs, attribute.Bool("enabled", true))
		assert.Contains(t, attrs, attribute.Bool("active", false))
	})

	t.Run("bool slice", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"flags": []bool{true, false, true},
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 1)
		assert.Contains(t, attrs, attribute.BoolSlice("flags", []bool{true, false, true}))
	})
}

// TestParseAttributes_String tests parsing string attributes
func TestParseAttributes_String(t *testing.T) {
	t.Run("single string", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"service": "my-service",
			"version": "1.0.0",
			"env":     "production",
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 3)
		assert.Contains(t, attrs, attribute.String("service", "my-service"))
		assert.Contains(t, attrs, attribute.String("version", "1.0.0"))
		assert.Contains(t, attrs, attribute.String("env", "production"))
	})

	t.Run("string slice", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"tags": []string{"tag1", "tag2", "tag3"},
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 1)
		assert.Contains(t, attrs, attribute.StringSlice("tags", []string{"tag1", "tag2", "tag3"}))
	})

	t.Run("empty string", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"empty": "",
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 1)
		assert.Contains(t, attrs, attribute.String("empty", ""))
	})
}

// TestParseAttributes_Int64 tests parsing int64 attributes
func TestParseAttributes_Int64(t *testing.T) {
	t.Run("single int64", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"port":     int64(8080),
			"count":    int64(42),
			"duration": int64(1000),
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 3)
		assert.Contains(t, attrs, attribute.Int64("port", 8080))
		assert.Contains(t, attrs, attribute.Int64("count", 42))
		assert.Contains(t, attrs, attribute.Int64("duration", 1000))
	})

	t.Run("int slice", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"ports": []int64{8080, 8081, 8082},
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 1)
		assert.Contains(t, attrs, attribute.Int64Slice("ports", []int64{8080, 8081, 8082}))
	})

	t.Run("negative int64", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"offset": int64(-100),
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 1)
		assert.Contains(t, attrs, attribute.Int64("offset", -100))
	})
}

// TestParseAttributes_Float64 tests parsing float64 attributes
func TestParseAttributes_Float64(t *testing.T) {
	t.Run("single float64", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"ratio":   float64(0.95),
			"percent": float64(100.0),
			"price":   float64(19.99),
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 3)
		assert.Contains(t, attrs, attribute.Float64("ratio", 0.95))
		assert.Contains(t, attrs, attribute.Float64("percent", 100.0))
		assert.Contains(t, attrs, attribute.Float64("price", 19.99))
	})

	t.Run("float64 slice", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"ratios": []float64{0.1, 0.5, 0.9},
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 1)
		assert.Contains(t, attrs, attribute.Float64Slice("ratios", []float64{0.1, 0.5, 0.9}))
	})

	t.Run("negative float64", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"temperature": float64(-5.5),
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 1)
		assert.Contains(t, attrs, attribute.Float64("temperature", -5.5))
	})
}

// TestParseAttributes_Default tests parsing unknown types as strings
func TestParseAttributes_Default(t *testing.T) {
	t.Run("unknown type converts to string", func(t *testing.T) {
		type customType struct {
			Field string
		}

		attrsMap := map[string]interface{}{
			"custom": customType{Field: "value"},
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 1)
		// Should be converted to string representation
		assert.NotEmpty(t, attrs[0].Value.AsString())
	})

	t.Run("nil value", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"nil_value": nil,
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 1)
		assert.NotEmpty(t, attrs[0].Value.AsString())
	})

	t.Run("integer (not int64)", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"int_value": int(42),
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 1)
		// Should be converted to string representation
		assert.NotEmpty(t, attrs[0].Value.AsString())
	})
}

// TestParseAttributes_MixedTypes tests parsing mixed type attributes
func TestParseAttributes_MixedTypes(t *testing.T) {
	t.Run("mixed types", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"name":      "service",
			"port":      int64(8080),
			"enabled":   true,
			"ratio":     float64(0.95),
			"tags":      []string{"tag1", "tag2"},
			"endpoints": []int64{8080, 8081},
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 6)

		// Verify each type
		foundTypes := make(map[string]bool)
		for _, attr := range attrs {
			switch attr.Key {
			case "name":
				foundTypes["string"] = true
			case "port":
				foundTypes["int64"] = true
			case "enabled":
				foundTypes["bool"] = true
			case "ratio":
				foundTypes["float64"] = true
			case "tags":
				foundTypes["string_slice"] = true
			case "endpoints":
				foundTypes["int64_slice"] = true
			}
		}

		assert.True(t, foundTypes["string"])
		assert.True(t, foundTypes["int64"])
		assert.True(t, foundTypes["bool"])
		assert.True(t, foundTypes["float64"])
		assert.True(t, foundTypes["string_slice"])
		assert.True(t, foundTypes["int64_slice"])
	})
}

// TestParseAttributes_EmptyAndEdgeCases tests edge cases
func TestParseAttributes_EmptyAndEdgeCases(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		attrsMap := map[string]interface{}{}

		attrs := ParseAttributes(attrsMap)

		assert.Empty(t, attrs)
	})

	t.Run("nil map", func(t *testing.T) {
		var attrsMap map[string]interface{}

		attrs := ParseAttributes(attrsMap)

		assert.Empty(t, attrs)
	})

	t.Run("empty slices", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"empty_strings": []string{},
			"empty_ints":    []int64{},
			"empty_floats":  []float64{},
			"empty_bools":   []bool{},
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 4)
	})

	t.Run("zero values", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"zero_string": "",
			"zero_int":    int64(0),
			"zero_float":  float64(0.0),
			"zero_bool":   false,
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 4)
	})
}

// TestParseAttributes_OrderAndConsistency tests attribute order and consistency
func TestParseAttributes_OrderAndConsistency(t *testing.T) {
	t.Run("attributes preserve insertion order", func(t *testing.T) {
		// Note: Go maps don't preserve order, but we can verify all attributes are present
		attrsMap := map[string]interface{}{
			"key1": "value1",
			"key2": int64(100),
			"key3": true,
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 3)

		// Verify all keys are present
		keys := make(map[string]bool)
		for _, attr := range attrs {
			keys[string(attr.Key)] = true
		}

		assert.True(t, keys["key1"])
		assert.True(t, keys["key2"])
		assert.True(t, keys["key3"])
	})
}

// TestParseAttributes_RealWorldScenarios tests real-world usage scenarios
func TestParseAttributes_RealWorldScenarios(t *testing.T) {
	t.Run("HTTP request attributes", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"http.method":      "GET",
			"http.status_code": int64(200),
			"http.scheme":      "https",
			"http.host":        "api.example.com",
			"http.port":        int64(443),
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 5)
	})

	t.Run("service resource attributes", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"service.name":      "my-service",
			"service.version":   "1.0.0",
			"service.namespace": "production",
			"deployment.env":    "prod",
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 4)
	})

	t.Run("database call attributes", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"db.system":           "postgresql",
			"db.name":             "mydb",
			"db.statement":        "SELECT * FROM users",
			"db.port":             int64(5432),
			"db.connection.count": int64(10),
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 5)
	})

	t.Run("RPC span attributes", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"rpc.system":      "grpc",
			"rpc.service":     "my.Service",
			"rpc.method":      "TestMethod",
			"rpc.status_code": "OK",
			"net.peer.name":   "localhost",
			"net.peer.port":   int64(9090),
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 6)
	})

	t.Run("cloud resource attributes", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"cloud.provider":          "aws",
			"cloud.account.id":        "123456789",
			"cloud.region":            "us-east-1",
			"cloud.availability_zone": "us-east-1a",
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 4)
	})

	t.Run("k8s pod attributes", func(t *testing.T) {
		attrsMap := map[string]interface{}{
			"k8s.pod.name":       "my-pod",
			"k8s.pod.uid":        "abc-123",
			"k8s.namespace.name": "default",
			"k8s.node.name":      "node-1",
		}

		attrs := ParseAttributes(attrsMap)

		assert.Len(t, attrs, 4)
	})
}
