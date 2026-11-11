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
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type contextMetadataAccessor struct {
	with func(context.Context, MD) context.Context
	from func(context.Context) (MD, bool)
}

var (
	inMetadataAccessor = contextMetadataAccessor{
		with: WithInContext,
		from: FromInContext,
	}
	outMetadataAccessor = contextMetadataAccessor{
		with: WithOutContext,
		from: FromOutContext,
	}
)

func testWithContextMetadata(t *testing.T, accessor contextMetadataAccessor) {
	t.Helper()

	t.Run("attach metadata to empty context", func(t *testing.T) {
		ctx := accessor.with(t.Context(), New(map[string]string{
			"key1": "value1",
			"key2": "value2",
		}))

		retrieved, ok := accessor.from(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value1"}, retrieved["key1"])
		assert.Equal(t, []string{"value2"}, retrieved["key2"])
	})

	t.Run("attach metadata to context with existing metadata", func(t *testing.T) {
		ctx := accessor.with(t.Context(), New(map[string]string{"key1": "value1"}))
		ctx = accessor.with(ctx, New(map[string]string{"key2": "value2"}))

		retrieved, ok := accessor.from(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value1"}, retrieved["key1"])
		assert.Equal(t, []string{"value2"}, retrieved["key2"])
	})

	t.Run("overlapping keys are merged", func(t *testing.T) {
		ctx := accessor.with(t.Context(), Pairs("key", "value1"))
		ctx = accessor.with(ctx, Pairs("key", "value2"))

		retrieved, ok := accessor.from(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value1", "value2"}, retrieved["key"])
	})

	t.Run("empty metadata", func(t *testing.T) {
		ctx := accessor.with(t.Context(), MD{})

		retrieved, ok := accessor.from(ctx)
		require.True(t, ok)
		assert.Empty(t, retrieved)
	})
}

func testFromContextMetadata(t *testing.T, accessor contextMetadataAccessor) {
	t.Helper()

	t.Run("retrieve from context with metadata", func(t *testing.T) {
		ctx := accessor.with(t.Context(), New(map[string]string{"key": "value"}))

		retrieved, ok := accessor.from(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value"}, retrieved["key"])
	})

	t.Run("retrieve from context without metadata", func(t *testing.T) {
		retrieved, ok := accessor.from(t.Context())
		require.False(t, ok)
		assert.Empty(t, retrieved)
	})

	t.Run("retrieved metadata is a copy", func(t *testing.T) {
		ctx := accessor.with(t.Context(), New(map[string]string{"key": "value"}))

		retrieved, ok := accessor.from(ctx)
		require.True(t, ok)
		retrieved.Set("key", "modified")

		original, _ := accessor.from(ctx)
		assert.Equal(t, []string{"value"}, original["key"])
	})

	t.Run("multiple retrievals return independent copies", func(t *testing.T) {
		ctx := accessor.with(t.Context(), New(map[string]string{"key": "value"}))
		copy1, _ := accessor.from(ctx)
		copy2, _ := accessor.from(ctx)

		copy1.Set("key", "modified1")
		copy2.Set("key", "modified2")

		original, _ := accessor.from(ctx)
		assert.Equal(t, []string{"value"}, original["key"])
	})
}

// TestWithInContext tests attaching metadata to context.
func TestWithInContext(t *testing.T) {
	testWithContextMetadata(t, inMetadataAccessor)
}

// TestFromInContext tests retrieving metadata from context.
func TestFromInContext(t *testing.T) {
	testFromContextMetadata(t, inMetadataAccessor)
}

// TestWithOutContext tests attaching output metadata to context.
func TestWithOutContext(t *testing.T) {
	testWithContextMetadata(t, outMetadataAccessor)

	t.Run("in and out metadata are independent", func(t *testing.T) {
		ctx := WithInContext(t.Context(), New(map[string]string{"in": "in_value"}))
		ctx = WithOutContext(ctx, New(map[string]string{"out": "out_value"}))

		inMetadata, _ := FromInContext(ctx)
		outMetadata, _ := FromOutContext(ctx)
		assert.Equal(t, []string{"in_value"}, inMetadata["in"])
		assert.Equal(t, []string{"out_value"}, outMetadata["out"])
		assert.NotContains(t, inMetadata, "out")
		assert.NotContains(t, outMetadata, "in")
	})
}

// TestFromOutContext tests retrieving output metadata from context.
func TestFromOutContext(t *testing.T) {
	testFromContextMetadata(t, outMetadataAccessor)
}

// TestWithStreamContext tests creating stream context.
func TestWithStreamContext(t *testing.T) {
	t.Run("create stream context from background", func(t *testing.T) {
		ctx := t.Context()
		streamCtx := WithStreamContext(ctx)

		streamCtx2 := WithStreamContext(streamCtx)
		assert.Equal(t, streamCtx, streamCtx2)
	})

	t.Run("stream context is independent of parent", func(t *testing.T) {
		parent := t.Context()
		streamCtx := WithStreamContext(parent)

		_, ok := parent.Value(streamKey{}).(*stream)
		assert.False(t, ok)

		_, ok = streamCtx.Value(streamKey{}).(*stream)
		assert.True(t, ok)
	})
}

type streamMetadataAccessor struct {
	kind string
	set  func(context.Context, MD) error
	get  func(context.Context) (MD, bool)
}

var (
	headerMetadataAccessor = streamMetadataAccessor{
		kind: "header",
		set:  SetHeader,
		get:  FromHeaderCtx,
	}
	trailerMetadataAccessor = streamMetadataAccessor{
		kind: "trailer",
		set:  SetTrailer,
		get:  FromTrailerCtx,
	}
)

func testSetStreamMetadata(t *testing.T, accessor streamMetadataAccessor) {
	t.Helper()

	t.Run(fmt.Sprintf("set %s on stream context", accessor.kind), func(t *testing.T) {
		ctx := WithStreamContext(t.Context())
		require.NoError(t, accessor.set(ctx, New(map[string]string{"key": "value"})))

		values, ok := accessor.get(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value"}, values["key"])
	})

	t.Run(fmt.Sprintf("set %s multiple times merges", accessor.kind), func(t *testing.T) {
		ctx := WithStreamContext(t.Context())

		require.NoError(t, accessor.set(ctx, Pairs("key", "value1")))
		require.NoError(t, accessor.set(ctx, Pairs("key", "value2")))

		values, _ := accessor.get(ctx)
		assert.Equal(t, []string{"value1", "value2"}, values["key"])
	})

	t.Run(
		fmt.Sprintf("set %s on non-stream context returns error", accessor.kind),
		func(t *testing.T) {
			err := accessor.set(t.Context(), New(map[string]string{"key": "value"}))
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to fetch the stream")
		},
	)

	t.Run(fmt.Sprintf("set empty %s", accessor.kind), func(t *testing.T) {
		ctx := WithStreamContext(t.Context())
		require.NoError(t, accessor.set(ctx, MD{}))

		values, ok := accessor.get(ctx)
		require.True(t, ok)
		assert.Empty(t, values)
	})

	t.Run(fmt.Sprintf("concurrent set %s", accessor.kind), func(t *testing.T) {
		ctx := WithStreamContext(t.Context())
		var wg sync.WaitGroup
		for range 100 {
			wg.Go(func() {
				_ = accessor.set(ctx, New(map[string]string{"key": "value"}))
			})
		}
		wg.Wait()

		values, ok := accessor.get(ctx)
		require.True(t, ok)
		assert.NotEmpty(t, values["key"])
	})
}

func testFromStreamMetadata(t *testing.T, accessor streamMetadataAccessor) {
	t.Helper()

	t.Run(fmt.Sprintf("retrieve %s from stream context", accessor.kind), func(t *testing.T) {
		ctx := WithStreamContext(t.Context())
		require.NoError(t, accessor.set(ctx, New(map[string]string{"key": "value"})))

		values, ok := accessor.get(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value"}, values["key"])
	})

	t.Run(fmt.Sprintf("retrieve %s from non-stream context", accessor.kind), func(t *testing.T) {
		values, ok := accessor.get(t.Context())
		require.False(t, ok)
		assert.Empty(t, values)
	})

	t.Run(fmt.Sprintf("retrieve %s when not set", accessor.kind), func(t *testing.T) {
		values, ok := accessor.get(WithStreamContext(t.Context()))
		require.False(t, ok)
		assert.Empty(t, values)
	})

	t.Run(fmt.Sprintf("retrieved %s is a copy", accessor.kind), func(t *testing.T) {
		ctx := WithStreamContext(t.Context())
		require.NoError(t, accessor.set(ctx, New(map[string]string{"key": "value"})))

		values, ok := accessor.get(ctx)
		require.True(t, ok)
		values.Set("key", "modified")

		original, _ := accessor.get(ctx)
		assert.Equal(t, []string{"value"}, original["key"])
	})

	t.Run(fmt.Sprintf("concurrent read %s", accessor.kind), func(t *testing.T) {
		ctx := WithStreamContext(t.Context())
		require.NoError(t, accessor.set(ctx, New(map[string]string{"key": "value"})))

		type result struct {
			values MD
			ok     bool
		}
		results := make([]result, 100)
		var wg sync.WaitGroup
		for index := range results {
			wg.Go(func() {
				results[index].values, results[index].ok = accessor.get(ctx)
			})
		}
		wg.Wait()

		for _, item := range results {
			require.True(t, item.ok)
			assert.NotEmpty(t, item.values)
		}
	})
}

// TestSetHeader tests setting header metadata.
func TestSetHeader(t *testing.T) {
	testSetStreamMetadata(t, headerMetadataAccessor)
}

// TestFromHeaderCtx tests retrieving header metadata.
func TestFromHeaderCtx(t *testing.T) {
	testFromStreamMetadata(t, headerMetadataAccessor)
}

// TestSetTrailer tests setting trailer metadata.
func TestSetTrailer(t *testing.T) {
	testSetStreamMetadata(t, trailerMetadataAccessor)
}

// TestFromTrailerCtx tests retrieving trailer metadata.
func TestFromTrailerCtx(t *testing.T) {
	testFromStreamMetadata(t, trailerMetadataAccessor)
}

// TestHeaderAndTrailerIndependence tests that header and trailer are independent.
func TestHeaderAndTrailerIndependence(t *testing.T) {
	t.Run("header and trailer are separate", func(t *testing.T) {
		ctx := WithStreamContext(t.Context())
		require.NoError(t, SetHeader(ctx, New(map[string]string{"header": "header_value"})))
		require.NoError(t, SetTrailer(ctx, New(map[string]string{"trailer": "trailer_value"})))

		header, _ := FromHeaderCtx(ctx)
		trailer, _ := FromTrailerCtx(ctx)

		assert.Equal(t, []string{"header_value"}, header["header"])
		assert.Equal(t, []string{"trailer_value"}, trailer["trailer"])
		assert.NotContains(t, header, "trailer")
		assert.NotContains(t, trailer, "header")
	})

	t.Run("modifying header doesn't affect trailer", func(t *testing.T) {
		ctx := WithStreamContext(t.Context())
		require.NoError(t, SetHeader(ctx, New(map[string]string{"key": "header"})))
		require.NoError(t, SetTrailer(ctx, New(map[string]string{"key": "trailer"})))
		require.NoError(t, SetHeader(ctx, Pairs("key", "new_header")))

		header, _ := FromHeaderCtx(ctx)
		trailer, _ := FromTrailerCtx(ctx)
		assert.Equal(t, []string{"trailer"}, trailer["key"])
		assert.Equal(t, []string{"header", "new_header"}, header["key"])
	})
}

// TestContextPropagation tests metadata propagation through context chain.
func TestContextPropagation(t *testing.T) {
	t.Run("propagate metadata through multiple contexts", func(t *testing.T) {
		ctx := WithInContext(t.Context(), New(map[string]string{"request-id": "123"}))
		ctx = WithStreamContext(ctx)
		require.NoError(t, SetHeader(
			ctx,
			New(map[string]string{"content-type": "application/json"}),
		))

		inMetadata, _ := FromInContext(ctx)
		headerMetadata, _ := FromHeaderCtx(ctx)
		assert.Equal(t, []string{"123"}, inMetadata["request-id"])
		assert.Equal(t, []string{"application/json"}, headerMetadata["content-type"])
	})

	t.Run("cancel propagation doesn't affect metadata", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		ctx = WithInContext(ctx, New(map[string]string{"key": "value"}))
		cancel()

		retrieved, ok := FromInContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value"}, retrieved["key"])
	})
}

// TestConcurrentAccess tests concurrent access to context metadata.
func TestConcurrentAccess(t *testing.T) {
	t.Run("concurrent read and write", func(t *testing.T) {
		ctx := WithStreamContext(t.Context())
		var wg sync.WaitGroup

		for range 50 {
			wg.Go(func() {
				_ = SetHeader(ctx, New(map[string]string{"key": "value"}))
			})
		}
		for range 50 {
			wg.Go(func() {
				_, _ = FromHeaderCtx(ctx)
			})
		}
		wg.Wait()

		header, ok := FromHeaderCtx(ctx)
		assert.True(t, ok)
		assert.NotEmpty(t, header["key"])
	})
}

// TestRealWorldScenarios tests real-world usage patterns.
func TestRealWorldScenarios(t *testing.T) {
	t.Run("HTTP request handling", func(t *testing.T) {
		ctx := WithInContext(
			t.Context(),
			Pairs(
				"request-id", "abc-123",
				"user-agent", "MyApp/1.0",
				"authorization", "Bearer token",
			),
		)
		ctx = WithStreamContext(ctx)
		require.NoError(t, SetHeader(ctx, Pairs(
			"content-type", "application/json",
			"status", "200 OK",
		)))

		inMetadata, _ := FromInContext(ctx)
		headerMetadata, _ := FromHeaderCtx(ctx)
		assert.Equal(t, []string{"abc-123"}, inMetadata["request-id"])
		assert.Equal(t, []string{"application/json"}, headerMetadata["content-type"])
	})

	t.Run("gRPC-style metadata", func(t *testing.T) {
		ctx := WithInContext(t.Context(), Pairs(
			":authority", "api.example.com",
			":method", "POST",
			":path", "/v1/resource",
			"content-type", "application/grpc",
		))
		ctx = WithOutContext(ctx, Pairs(
			"grpc-status", "0",
			"grpc-message", "OK",
		))

		inMetadata, _ := FromInContext(ctx)
		outMetadata, _ := FromOutContext(ctx)
		assert.Equal(t, []string{"POST"}, inMetadata[":method"])
		assert.Equal(t, []string{"0"}, outMetadata["grpc-status"])
	})

	t.Run("microservice chain", func(t *testing.T) {
		ctx := WithInContext(t.Context(), Pairs("service", "svc1", "trace-id", "trace-123"))
		ctx = WithInContext(ctx, Pairs("service", "svc2"))
		ctx = WithOutContext(ctx, Pairs("status", "processed"))

		inMetadata, _ := FromInContext(ctx)
		outMetadata, _ := FromOutContext(ctx)
		assert.Equal(t, []string{"svc1", "svc2"}, inMetadata["service"])
		assert.Equal(t, []string{"trace-123"}, inMetadata["trace-id"])
		assert.Equal(t, []string{"processed"}, outMetadata["status"])
	})
}
