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

package stats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHandlerNames(t *testing.T) {
	t.Run("trim empty and dedup", func(t *testing.T) {
		names := ParseHandlerNames(" otel,otel, , custom , otel ,custom ")
		assert.Equal(t, []string{"otel", "custom"}, names)
	})

	t.Run("empty input", func(t *testing.T) {
		names := ParseHandlerNames("")
		assert.Empty(t, names)
	})
}
