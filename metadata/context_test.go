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

// Package metadata provides functions for attaching and retrieving metadata to/from a context.
package metadata

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithInContext tests attaching metadata to context
func TestWithInContext(t *testing.T) {
	t.Run("attach metadata to empty context", func(t *testing.T) {
		ctx := context.Background()
		md := New(map[string]string{
			"key1": "value1",
			"key2": "value2",
		})

		newCtx := WithInContext(ctx, md)

		retrieved, ok := FromInContext(newCtx)
		require.True(t, ok)
		assert.Equal(t, []string{"value1"}, retrieved["key1"])
		assert.Equal(t, []string{"value2"}, retrieved["key2"])
	})

	t.Run("attach metadata to context with existing metadata", func(t *testing.T) {
		ctx := context.Background()
		md1 := New(map[string]string{"key1": "value1"})
		ctx = WithInContext(ctx, md1)

		md2 := New(map[string]string{"key2": "value2"})
		newCtx := WithInContext(ctx, md2)

		retrieved, ok := FromInContext(newCtx)
		require.True(t, ok)
		assert.Equal(t, []string{"value1"}, retrieved["key1"])
		assert.Equal(t, []string{"value2"}, retrieved["key2"])
	})

	t.Run("overlapping keys are merged", func(t *testing.T) {
		ctx := context.Background()
		md1 := Pairs("key", "value1")
		ctx = WithInContext(ctx, md1)

		md2 := Pairs("key", "value2")
		newCtx := WithInContext(ctx, md2)

		retrieved, ok := FromInContext(newCtx)
		require.True(t, ok)
		assert.Equal(t, []string{"value1", "value2"}, retrieved["key"])
	})

	t.Run("empty metadata", func(t *testing.T) {
		ctx := context.Background()
		md := MD{}

		newCtx := WithInContext(ctx, md)

		retrieved, ok := FromInContext(newCtx)
		require.True(t, ok)
		assert.Empty(t, retrieved)
	})
}

// TestFromInContext tests retrieving metadata from context
func TestFromInContext(t *testing.T) {
	t.Run("retrieve from context with metadata", func(t *testing.T) {
		ctx := context.Background()
		md := New(map[string]string{"key": "value"})
		ctx = WithInContext(ctx, md)

		retrieved, ok := FromInContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value"}, retrieved["key"])
	})

	t.Run("retrieve from context without metadata", func(t *testing.T) {
		ctx := context.Background()

		retrieved, ok := FromInContext(ctx)
		require.False(t, ok)
		assert.Empty(t, retrieved)
	})

	t.Run("retrieved metadata is a copy", func(t *testing.T) {
		ctx := context.Background()
		md := New(map[string]string{"key": "value"})
		ctx = WithInContext(ctx, md)

		retrieved, ok := FromInContext(ctx)
		require.True(t, ok)

		// Modify the retrieved metadata
		retrieved.Set("key", "modified")

		// Original should be unchanged
		original, _ := FromInContext(ctx)
		assert.Equal(t, []string{"value"}, original["key"])
	})

	t.Run("multiple retrievals return independent copies", func(t *testing.T) {
		ctx := context.Background()
		md := New(map[string]string{"key": "value"})
		ctx = WithInContext(ctx, md)

		copy1, _ := FromInContext(ctx)
		copy2, _ := FromInContext(ctx)

		copy1.Set("key", "modified1")
		copy2.Set("key", "modified2")

		// Original should be unchanged
		original, _ := FromInContext(ctx)
		assert.Equal(t, []string{"value"}, original["key"])
	})
}

// TestWithOutContext tests attaching output metadata to context
func TestWithOutContext(t *testing.T) {
	t.Run("attach metadata to empty context", func(t *testing.T) {
		ctx := context.Background()
		md := New(map[string]string{
			"key1": "value1",
			"key2": "value2",
		})

		newCtx := WithOutContext(ctx, md)

		retrieved, ok := FromOutContext(newCtx)
		require.True(t, ok)
		assert.Equal(t, []string{"value1"}, retrieved["key1"])
		assert.Equal(t, []string{"value2"}, retrieved["key2"])
	})

	t.Run("attach metadata to context with existing metadata", func(t *testing.T) {
		ctx := context.Background()
		md1 := New(map[string]string{"key1": "value1"})
		ctx = WithOutContext(ctx, md1)

		md2 := New(map[string]string{"key2": "value2"})
		newCtx := WithOutContext(ctx, md2)

		retrieved, ok := FromOutContext(newCtx)
		require.True(t, ok)
		assert.Equal(t, []string{"value1"}, retrieved["key1"])
		assert.Equal(t, []string{"value2"}, retrieved["key2"])
	})

	t.Run("in and out metadata are independent", func(t *testing.T) {
		ctx := context.Background()
		inMD := New(map[string]string{"in": "in_value"})
		outMD := New(map[string]string{"out": "out_value"})

		ctx = WithInContext(ctx, inMD)
		ctx = WithOutContext(ctx, outMD)

		inRetrieved, _ := FromInContext(ctx)
		outRetrieved, _ := FromOutContext(ctx)

		assert.Equal(t, []string{"in_value"}, inRetrieved["in"])
		assert.Equal(t, []string{"out_value"}, outRetrieved["out"])
		assert.NotContains(t, inRetrieved, "out")
		assert.NotContains(t, outRetrieved, "in")
	})
}

// TestFromOutContext tests retrieving output metadata from context
func TestFromOutContext(t *testing.T) {
	t.Run("retrieve from context with metadata", func(t *testing.T) {
		ctx := context.Background()
		md := New(map[string]string{"key": "value"})
		ctx = WithOutContext(ctx, md)

		retrieved, ok := FromOutContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value"}, retrieved["key"])
	})

	t.Run("retrieve from context without metadata", func(t *testing.T) {
		ctx := context.Background()

		retrieved, ok := FromOutContext(ctx)
		require.False(t, ok)
		assert.Empty(t, retrieved)
	})

	t.Run("retrieved metadata is a copy", func(t *testing.T) {
		ctx := context.Background()
		md := New(map[string]string{"key": "value"})
		ctx = WithOutContext(ctx, md)

		retrieved, ok := FromOutContext(ctx)
		require.True(t, ok)

		// Modify the retrieved metadata
		retrieved.Set("key", "modified")

		// Original should be unchanged
		original, _ := FromOutContext(ctx)
		assert.Equal(t, []string{"value"}, original["key"])
	})
}

// TestWithStreamContext tests creating stream context
func TestWithStreamContext(t *testing.T) {
	t.Run("create stream context from background", func(t *testing.T) {
		ctx := context.Background()
		streamCtx := WithStreamContext(ctx)

		// Should not panic and should return same context if called again
		streamCtx2 := WithStreamContext(streamCtx)
		assert.Equal(t, streamCtx, streamCtx2)
	})

	t.Run("stream context is independent of parent", func(t *testing.T) {
		parent := context.Background()
		streamCtx := WithStreamContext(parent)

		// Parent should not have stream context
		_, ok := parent.Value(streamKey{}).(*stream)
		assert.False(t, ok)

		// Stream context should have it
		_, ok = streamCtx.Value(streamKey{}).(*stream)
		assert.True(t, ok)
	})
}

// TestSetHeader tests setting header metadata
func TestSetHeader(t *testing.T) {
	t.Run("set header on stream context", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())
		md := New(map[string]string{"key": "value"})

		err := SetHeader(ctx, md)
		require.NoError(t, err)

		header, ok := FromHeaderCtx(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value"}, header["key"])
	})

	t.Run("set header multiple times merges", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())

		md1 := Pairs("key", "value1")
		err1 := SetHeader(ctx, md1)
		require.NoError(t, err1)

		md2 := Pairs("key", "value2")
		err2 := SetHeader(ctx, md2)
		require.NoError(t, err2)

		header, _ := FromHeaderCtx(ctx)
		assert.Equal(t, []string{"value1", "value2"}, header["key"])
	})

	t.Run("set header on non-stream context returns error", func(t *testing.T) {
		ctx := context.Background()
		md := New(map[string]string{"key": "value"})

		err := SetHeader(ctx, md)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch the stream")
	})

	t.Run("set empty header", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())
		md := MD{}

		err := SetHeader(ctx, md)
		require.NoError(t, err)

		header, ok := FromHeaderCtx(ctx)
		require.True(t, ok)
		assert.Empty(t, header)
	})

	t.Run("concurrent set header", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(int) {
				defer wg.Done()
				md := New(map[string]string{"key": "value"})
				_ = SetHeader(ctx, md)
			}(i)
		}

		wg.Wait()

		header, ok := FromHeaderCtx(ctx)
		require.True(t, ok)
		// Should have all the values merged
		assert.True(t, len(header["key"]) > 0)
	})
}

// TestFromHeaderCtx tests retrieving header metadata
func TestFromHeaderCtx(t *testing.T) {
	t.Run("retrieve header from stream context", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())
		md := New(map[string]string{"key": "value"})
		_ = SetHeader(ctx, md)

		header, ok := FromHeaderCtx(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value"}, header["key"])
	})

	t.Run("retrieve header from non-stream context", func(t *testing.T) {
		ctx := context.Background()

		header, ok := FromHeaderCtx(ctx)
		require.False(t, ok)
		assert.Empty(t, header)
	})

	t.Run("retrieve header when not set", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())

		header, ok := FromHeaderCtx(ctx)
		require.False(t, ok)
		assert.Empty(t, header)
	})

	t.Run("retrieved header is a copy", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())
		md := New(map[string]string{"key": "value"})
		_ = SetHeader(ctx, md)

		header, ok := FromHeaderCtx(ctx)
		require.True(t, ok)

		// Modify the retrieved header
		header.Set("key", "modified")

		// Original should be unchanged
		original, _ := FromHeaderCtx(ctx)
		assert.Equal(t, []string{"value"}, original["key"])
	})

	t.Run("concurrent read header", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())
		md := New(map[string]string{"key": "value"})
		_ = SetHeader(ctx, md)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				header, ok := FromHeaderCtx(ctx)
				assert.True(t, ok)
				assert.NotEmpty(t, header)
			}()
		}

		wg.Wait()
	})
}

// TestSetTrailer tests setting trailer metadata
func TestSetTrailer(t *testing.T) {
	t.Run("set trailer on stream context", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())
		md := New(map[string]string{"key": "value"})

		err := SetTrailer(ctx, md)
		require.NoError(t, err)

		trailer, ok := FromTrailerCtx(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value"}, trailer["key"])
	})

	t.Run("set trailer multiple times merges", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())

		md1 := Pairs("key", "value1")
		err1 := SetTrailer(ctx, md1)
		require.NoError(t, err1)

		md2 := Pairs("key", "value2")
		err2 := SetTrailer(ctx, md2)
		require.NoError(t, err2)

		trailer, _ := FromTrailerCtx(ctx)
		assert.Equal(t, []string{"value1", "value2"}, trailer["key"])
	})

	t.Run("set trailer on non-stream context returns error", func(t *testing.T) {
		ctx := context.Background()
		md := New(map[string]string{"key": "value"})

		err := SetTrailer(ctx, md)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch the stream")
	})

	t.Run("set empty trailer", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())
		md := MD{}

		err := SetTrailer(ctx, md)
		require.NoError(t, err)

		trailer, ok := FromTrailerCtx(ctx)
		require.True(t, ok)
		assert.Empty(t, trailer)
	})

	t.Run("concurrent set trailer", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(int) {
				defer wg.Done()
				md := New(map[string]string{"key": "value"})
				_ = SetTrailer(ctx, md)
			}(i)
		}

		wg.Wait()

		trailer, ok := FromTrailerCtx(ctx)
		require.True(t, ok)
		// Should have all the values merged
		assert.True(t, len(trailer["key"]) > 0)
	})
}

// TestFromTrailerCtx tests retrieving trailer metadata
func TestFromTrailerCtx(t *testing.T) {
	t.Run("retrieve trailer from stream context", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())
		md := New(map[string]string{"key": "value"})
		_ = SetTrailer(ctx, md)

		trailer, ok := FromTrailerCtx(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value"}, trailer["key"])
	})

	t.Run("retrieve trailer from non-stream context", func(t *testing.T) {
		ctx := context.Background()

		trailer, ok := FromTrailerCtx(ctx)
		require.False(t, ok)
		assert.Empty(t, trailer)
	})

	t.Run("retrieve trailer when not set", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())

		trailer, ok := FromTrailerCtx(ctx)
		require.False(t, ok)
		assert.Empty(t, trailer)
	})

	t.Run("retrieved trailer is a copy", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())
		md := New(map[string]string{"key": "value"})
		_ = SetTrailer(ctx, md)

		trailer, ok := FromTrailerCtx(ctx)
		require.True(t, ok)

		// Modify the retrieved trailer
		trailer.Set("key", "modified")

		// Original should be unchanged
		original, _ := FromTrailerCtx(ctx)
		assert.Equal(t, []string{"value"}, original["key"])
	})

	t.Run("concurrent read trailer", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())
		md := New(map[string]string{"key": "value"})
		_ = SetTrailer(ctx, md)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				trailer, ok := FromTrailerCtx(ctx)
				assert.True(t, ok)
				assert.NotEmpty(t, trailer)
			}()
		}

		wg.Wait()
	})
}

// TestHeaderAndTrailerIndependence tests that header and trailer are independent
func TestHeaderAndTrailerIndependence(t *testing.T) {
	t.Run("header and trailer are separate", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())

		headerMD := New(map[string]string{"header": "header_value"})
		_ = SetHeader(ctx, headerMD)

		trailerMD := New(map[string]string{"trailer": "trailer_value"})
		_ = SetTrailer(ctx, trailerMD)

		header, _ := FromHeaderCtx(ctx)
		trailer, _ := FromTrailerCtx(ctx)

		assert.Equal(t, []string{"header_value"}, header["header"])
		assert.Equal(t, []string{"trailer_value"}, trailer["trailer"])
		assert.NotContains(t, header, "trailer")
		assert.NotContains(t, trailer, "header")
	})

	t.Run("modifying header doesn't affect trailer", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())

		headerMD := New(map[string]string{"key": "header"})
		_ = SetHeader(ctx, headerMD)

		trailerMD := New(map[string]string{"key": "trailer"})
		_ = SetTrailer(ctx, trailerMD)

		// Modify header
		newHeaderMD := Pairs("key", "new_header")
		_ = SetHeader(ctx, newHeaderMD)

		header, _ := FromHeaderCtx(ctx)
		trailer, _ := FromTrailerCtx(ctx)

		// Trailer should be unchanged
		assert.Equal(t, []string{"trailer"}, trailer["key"])
		// Header should have both values
		assert.Equal(t, []string{"header", "new_header"}, header["key"])
	})
}

// TestContextPropagation tests metadata propagation through context chain
func TestContextPropagation(t *testing.T) {
	t.Run("propagate metadata through multiple contexts", func(t *testing.T) {
		ctx := context.Background()

		// Add input metadata
		inMD := New(map[string]string{"request-id": "123"})
		ctx = WithInContext(ctx, inMD)

		// Add stream context
		ctx = WithStreamContext(ctx)

		// Add header
		headerMD := New(map[string]string{"content-type": "application/json"})
		_ = SetHeader(ctx, headerMD)

		// All metadata should be retrievable
		inRetrieved, _ := FromInContext(ctx)
		headerRetrieved, _ := FromHeaderCtx(ctx)

		assert.Equal(t, []string{"123"}, inRetrieved["request-id"])
		assert.Equal(t, []string{"application/json"}, headerRetrieved["content-type"])
	})

	t.Run("cancel propagation doesn't affect metadata", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		md := New(map[string]string{"key": "value"})
		ctx = WithInContext(ctx, md)

		// Cancel context
		cancel()

		// Metadata should still be retrievable
		retrieved, ok := FromInContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value"}, retrieved["key"])
	})
}

// TestConcurrentAccess tests concurrent access to context metadata
func TestConcurrentAccess(t *testing.T) {
	t.Run("concurrent read and write", func(t *testing.T) {
		ctx := WithStreamContext(context.Background())
		var wg sync.WaitGroup

		// Writers
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(int) {
				defer wg.Done()
				md := New(map[string]string{"key": "value"})
				_ = SetHeader(ctx, md)
			}(i)
		}

		// Readers
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = FromHeaderCtx(ctx)
			}()
		}

		wg.Wait()

		// Should not panic and have some data
		header, ok := FromHeaderCtx(ctx)
		assert.True(t, ok)
		assert.True(t, len(header["key"]) > 0)
	})
}

// TestRealWorldScenarios tests real-world usage patterns
func TestRealWorldScenarios(t *testing.T) {
	t.Run("HTTP request handling", func(t *testing.T) {
		// Simulate HTTP request context
		ctx := context.Background()

		// Add request metadata
		reqMD := Pairs(
			"request-id", "abc-123",
			"user-agent", "MyApp/1.0",
			"authorization", "Bearer token",
		)
		ctx = WithInContext(ctx, reqMD)

		// Create stream context
		ctx = WithStreamContext(ctx)

		// Set response headers
		respHeaders := Pairs(
			"content-type", "application/json",
			"status", "200 OK",
		)
		_ = SetHeader(ctx, respHeaders)

		// Verify all metadata
		inMD, _ := FromInContext(ctx)
		headerMD, _ := FromHeaderCtx(ctx)

		assert.Equal(t, []string{"abc-123"}, inMD["request-id"])
		assert.Equal(t, []string{"application/json"}, headerMD["content-type"])
	})

	t.Run("gRPC-style metadata", func(t *testing.T) {
		ctx := context.Background()

		// Initial metadata
		initialMD := Pairs(
			":authority", "api.example.com",
			":method", "POST",
			":path", "/v1/resource",
			"content-type", "application/grpc",
		)
		ctx = WithInContext(ctx, initialMD)

		// Process request and add output metadata
		outputMD := Pairs(
			"grpc-status", "0",
			"grpc-message", "OK",
		)
		ctx = WithOutContext(ctx, outputMD)

		// Verify metadata
		inMD, _ := FromInContext(ctx)
		outMD, _ := FromOutContext(ctx)

		assert.Equal(t, []string{"POST"}, inMD[":method"])
		assert.Equal(t, []string{"0"}, outMD["grpc-status"])
	})

	t.Run("microservice chain", func(t *testing.T) {
		// Service 1 creates context
		ctx := context.Background()
		svc1MD := Pairs("service", "svc1", "trace-id", "trace-123")
		ctx = WithInContext(ctx, svc1MD)

		// Service 2 adds its metadata
		svc2MD := Pairs("service", "svc2")
		ctx = WithInContext(ctx, svc2MD)

		// Service 3 adds output metadata
		svc3OutMD := Pairs("status", "processed")
		ctx = WithOutContext(ctx, svc3OutMD)

		// Verify all metadata is present
		inMD, _ := FromInContext(ctx)
		outMD, _ := FromOutContext(ctx)

		assert.Equal(t, []string{"svc1", "svc2"}, inMD["service"])
		assert.Equal(t, []string{"trace-123"}, inMD["trace-id"])
		assert.Equal(t, []string{"processed"}, outMD["status"])
	})
}
