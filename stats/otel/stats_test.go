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

	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/stretchr/testify/assert"
)

// TestInit tests package initialization
func TestInit(t *testing.T) {
	t.Run("otel handler builder is registered", func(t *testing.T) {
		builder := stats.GetHandlerBuilder("otel")
		assert.NotNil(t, builder, "otel handler builder should be registered")
	})

	t.Run("builder creates server handler", func(t *testing.T) {
		builder := stats.GetHandlerBuilder("otel")
		assert.NotNil(t, builder)

		handler := builder(true)
		assert.NotNil(t, handler, "server handler should not be nil")

		// Verify it implements Handler interface
		assert.Implements(t, (*stats.Handler)(nil), handler)
	})

	t.Run("builder creates client handler", func(t *testing.T) {
		builder := stats.GetHandlerBuilder("otel")
		assert.NotNil(t, builder)

		handler := builder(false)
		assert.NotNil(t, handler, "client handler should not be nil")

		// Verify it implements Handler interface
		assert.Implements(t, (*stats.Handler)(nil), handler)
	})
}
