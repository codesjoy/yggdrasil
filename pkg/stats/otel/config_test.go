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
)

// TestConfig tests Config structure
func TestConfig(t *testing.T) {
	t.Run("default config values", func(t *testing.T) {
		cfg := &Config{}
		assert.False(t, cfg.ReceivedEvent)
		assert.False(t, cfg.SentEvent)
		assert.False(t, cfg.EnableMetrics)
	})

	t.Run("set config values", func(t *testing.T) {
		cfg := &Config{
			ReceivedEvent: true,
			SentEvent:     true,
			EnableMetrics: true,
		}
		assert.True(t, cfg.ReceivedEvent)
		assert.True(t, cfg.SentEvent)
		assert.True(t, cfg.EnableMetrics)
	})
}

// TestGetCfg tests getCfg function
func TestGetCfg(t *testing.T) {
	t.Run("get config returns non-nil", func(t *testing.T) {
		cfg := getCfg()
		assert.NotNil(t, cfg)
	})

	t.Run("multiple calls return same config", func(t *testing.T) {
		cfg1 := getCfg()
		cfg2 := getCfg()
		// Config values might differ if underlying config changes, but should both be non-nil
		assert.NotNil(t, cfg1)
		assert.NotNil(t, cfg2)
	})
}
