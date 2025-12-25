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

	"github.com/codesjoy/yggdrasil/pkg/metadata"
	"github.com/stretchr/testify/assert"
)

// TestNewMetadataReaderWriter tests creating a new MetadataReaderWriter
func TestNewMetadataReaderWriter(t *testing.T) {
	t.Run("create with valid metadata", func(t *testing.T) {
		md := metadata.MD{}
		carrier := NewMetadataReaderWriter(&md)

		assert.NotNil(t, carrier)
		assert.Same(t, &md, carrier.md)
	})

	t.Run("create with nil metadata", func(t *testing.T) {
		carrier := NewMetadataReaderWriter(nil)

		assert.NotNil(t, carrier)
		// This will panic if accessed, which is expected behavior
	})
}

// TestMetadataReaderWriter_Get tests the Get method
func TestMetadataReaderWriter_Get(t *testing.T) {
	t.Run("get single value", func(t *testing.T) {
		md := metadata.New(map[string]string{
			"key1": "value1",
			"key2": "value2",
		})
		carrier := NewMetadataReaderWriter(&md)

		value := carrier.Get("key1")
		assert.Equal(t, "value1", value)
	})

	t.Run("get multiple values joined", func(t *testing.T) {
		md := metadata.Pairs("key", "value1", "key", "value2", "key", "value3")
		carrier := NewMetadataReaderWriter(&md)

		value := carrier.Get("key")
		assert.Equal(t, "value1;value2;value3", value)
	})

	t.Run("get non-existent key", func(t *testing.T) {
		md := metadata.New(map[string]string{"key": "value"})
		carrier := NewMetadataReaderWriter(&md)

		value := carrier.Get("nonexistent")
		assert.Equal(t, "", value)
	})

	t.Run("get case insensitive", func(t *testing.T) {
		md := metadata.New(map[string]string{"Content-Type": "application/json"})
		carrier := NewMetadataReaderWriter(&md)

		value := carrier.Get("content-type")
		assert.Equal(t, "application/json", value)
	})

	t.Run("get from empty metadata", func(t *testing.T) {
		md := metadata.MD{}
		carrier := NewMetadataReaderWriter(&md)

		value := carrier.Get("key")
		assert.Equal(t, "", value)
	})
}

// TestMetadataReaderWriter_Set tests the Set method
func TestMetadataReaderWriter_Set(t *testing.T) {
	t.Run("set new key", func(t *testing.T) {
		md := metadata.MD{}
		carrier := NewMetadataReaderWriter(&md)

		carrier.Set("key", "value")

		values := md.Get("key")
		assert.Equal(t, []string{"value"}, values)
	})

	t.Run("set overwrites existing values", func(t *testing.T) {
		md := metadata.Pairs("key", "oldvalue")
		carrier := NewMetadataReaderWriter(&md)

		carrier.Set("key", "newvalue")

		values := md.Get("key")
		assert.Equal(t, []string{"newvalue"}, values)
	})

	t.Run("set case insensitive", func(t *testing.T) {
		md := metadata.New(map[string]string{"key": "value"})
		carrier := NewMetadataReaderWriter(&md)

		carrier.Set("KEY", "newvalue")

		values := md.Get("key")
		assert.Equal(t, []string{"newvalue"}, values)
	})

	t.Run("set multiple keys", func(t *testing.T) {
		md := metadata.MD{}
		carrier := NewMetadataReaderWriter(&md)

		carrier.Set("key1", "value1")
		carrier.Set("key2", "value2")
		carrier.Set("key3", "value3")

		assert.Equal(t, []string{"value1"}, md.Get("key1"))
		assert.Equal(t, []string{"value2"}, md.Get("key2"))
		assert.Equal(t, []string{"value3"}, md.Get("key3"))
	})
}

// TestMetadataReaderWriter_Keys tests the Keys method
func TestMetadataReaderWriter_Keys(t *testing.T) {
	t.Run("get all keys", func(t *testing.T) {
		md := metadata.New(map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		})
		carrier := NewMetadataReaderWriter(&md)

		keys := carrier.Keys()

		assert.Len(t, keys, 3)
		assert.Contains(t, keys, "key1")
		assert.Contains(t, keys, "key2")
		assert.Contains(t, keys, "key3")
	})

	t.Run("keys from empty metadata", func(t *testing.T) {
		md := metadata.MD{}
		carrier := NewMetadataReaderWriter(&md)

		keys := carrier.Keys()

		assert.Empty(t, keys)
	})

	t.Run("keys with multiple values", func(t *testing.T) {
		md := metadata.Pairs("key", "v1", "key", "v2")
		carrier := NewMetadataReaderWriter(&md)

		keys := carrier.Keys()

		assert.Len(t, keys, 1)
		assert.Contains(t, keys, "key")
	})
}

// TestMetadataReaderWriter_TextMapCarrier tests TextMapCarrier interface compliance
func TestMetadataReaderWriter_TextMapCarrier(t *testing.T) {
	t.Run("propagate trace context", func(t *testing.T) {
		md := metadata.MD{}
		carrier := NewMetadataReaderWriter(&md)

		// Simulate setting trace context
		carrier.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
		carrier.Set("tracestate", "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE")

		assert.Equal(t, "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
			carrier.Get("traceparent"))
		assert.Equal(t, "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE",
			carrier.Get("tracestate"))
	})

	t.Run("get keys for propagation", func(t *testing.T) {
		md := metadata.Pairs(
			"traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
			"tracestate", "rojo=00f067aa0ba902b7",
			"baggage", "key1=value1,key2=value2",
		)
		carrier := NewMetadataReaderWriter(&md)

		keys := carrier.Keys()

		assert.Len(t, keys, 3)
		assert.Contains(t, keys, "traceparent")
		assert.Contains(t, keys, "tracestate")
		assert.Contains(t, keys, "baggage")
	})
}

// TestMetadataReaderWriter_RealWorldScenarios tests real-world usage
func TestMetadataReaderWriter_RealWorldScenarios(t *testing.T) {
	t.Run("HTTP headers propagation", func(t *testing.T) {
		md := metadata.Pairs(
			"traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			"x-request-id", "abc-123",
			"user-agent", "MyApp/1.0",
		)
		carrier := NewMetadataReaderWriter(&md)

		traceParent := carrier.Get("traceparent")
		requestID := carrier.Get("x-request-id")

		assert.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			traceParent)
		assert.Equal(t, "abc-123", requestID)
	})

	t.Run("gRPC metadata propagation", func(t *testing.T) {
		md := metadata.Pairs(
			"traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
			":authority", "api.example.com",
			":method", "POST",
			"grpc-accept-encoding", "gzip",
		)
		carrier := NewMetadataReaderWriter(&md)

		keys := carrier.Keys()

		assert.Contains(t, keys, "traceparent")
		assert.Contains(t, keys, ":authority")
		assert.Contains(t, keys, ":method")
	})
}
