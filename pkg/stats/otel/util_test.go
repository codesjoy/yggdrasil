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
	"context"
	"testing"

	"github.com/codesjoy/yggdrasil/pkg/metadata"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/propagation"
)

// TestParseFullMethod tests parseFullMethod function
func TestParseFullMethod(t *testing.T) {
	t.Run("valid full method", func(t *testing.T) {
		tests := []struct {
			name         string
			fullMethod   string
			expectedName string
			expectAttrs  bool
		}{
			{
				name:         "standard gRPC method",
				fullMethod:   "/package.service/method",
				expectedName: "package.service/method",
				expectAttrs:  true,
			},
			{
				name:         "simple service method",
				fullMethod:   "/myservice/mymethod",
				expectedName: "myservice/mymethod",
				expectAttrs:  true,
			},
			{
				name:         "deeply nested package",
				fullMethod:   "/com.example.package.service/method",
				expectedName: "com.example.package.service/method",
				expectAttrs:  true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				name, attrs := parseFullMethod(tt.fullMethod)
				assert.Equal(t, tt.expectedName, name)
				if tt.expectAttrs {
					assert.NotNil(t, attrs)
					assert.NotEmpty(t, attrs, "should have attributes")
				}
			})
		}
	})

	t.Run("invalid full method format", func(t *testing.T) {
		tests := []struct {
			name            string
			fullMethod      string
			expectEmptyName bool
		}{
			{
				name:            "missing leading slash",
				fullMethod:      "package.service/method",
				expectEmptyName: false,
			},
			{
				name:            "missing service part",
				fullMethod:      "/method",
				expectEmptyName: false,
			},
			{
				name:            "only slash",
				fullMethod:      "/",
				expectEmptyName: true,
			},
			{
				name:            "empty string",
				fullMethod:      "",
				expectEmptyName: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				name, attrs := parseFullMethod(tt.fullMethod)
				// For invalid format, returns the original or modified string
				if tt.expectEmptyName {
					assert.Empty(t, name)
				} else {
					assert.NotEmpty(t, name)
				}
				assert.Nil(t, attrs, "should not have attributes for invalid format")
			})
		}
	})
}

// TestInject tests inject function
func TestInject(t *testing.T) {
	t.Run("inject propagator into context", func(t *testing.T) {
		ctx := context.Background()
		propagators := propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)

		result := inject(ctx, propagators)

		assert.NotNil(t, result)

		// Verify metadata is set in out context
		md, ok := metadata.FromOutContext(result)
		assert.True(t, ok, "metadata should be in out context")
		assert.NotNil(t, md)
	})

	t.Run("inject with existing metadata", func(t *testing.T) {
		existingMD := metadata.MD{"existing-key": []string{"existing-value"}}
		ctx := metadata.WithOutContext(context.Background(), existingMD)

		propagators := propagation.TraceContext{}

		result := inject(ctx, propagators)

		assert.NotNil(t, result)

		md, ok := metadata.FromOutContext(result)
		assert.True(t, ok)
		assert.NotNil(t, md)
	})
}

// TestExtract tests extract function
func TestExtract(t *testing.T) {
	t.Run("extract from context with metadata", func(t *testing.T) {
		// Set up propagator
		propagators := propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
		)

		// Create carrier with trace context
		carrier := propagation.MapCarrier{
			"traceparent": "00-12345678901234567890123456789012-1234567890123456-01",
		}

		// Convert to metadata
		md := make(metadata.MD)
		for k, v := range carrier {
			md[k] = []string{v}
		}

		ctx := metadata.WithInContext(context.Background(), md)

		result := extract(ctx, propagators)

		assert.NotNil(t, result)
	})

	t.Run("extract from context without metadata", func(t *testing.T) {
		ctx := context.Background()
		propagators := propagation.TraceContext{}

		result := extract(ctx, propagators)

		assert.NotNil(t, result)
	})
}

// TestParseFullMethodAttributes tests parseFullMethod attributes
func TestParseFullMethodAttributes(t *testing.T) {
	t.Run("attributes contain service and method", func(t *testing.T) {
		_, attrs := parseFullMethod("/test.service/TestMethod")

		assert.NotNil(t, attrs)
		assert.GreaterOrEqual(t, len(attrs), 2)

		// Check for expected attribute keys
		attrMap := make(map[string]string)
		for _, attr := range attrs {
			attrMap[string(attr.Key)] = attr.Value.AsString()
		}

		// Verify service and method are set
		assert.Contains(t, attrMap, "rpc.service")
		assert.Equal(t, "test.service", attrMap["rpc.service"])

		assert.Contains(t, attrMap, "rpc.method")
		assert.Equal(t, "TestMethod", attrMap["rpc.method"])
	})

	t.Run("single component method", func(t *testing.T) {
		_, attrs := parseFullMethod("/service")

		assert.Nil(t, attrs, "single component should not have attributes")
	})
}
