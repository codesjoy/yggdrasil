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

package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/code"
)

// testReason is a test implementation of Reason interface
type testReason struct {
	reason string
	domain string
	code   code.Code
}

func (r *testReason) Reason() string {
	return r.reason
}

func (r *testReason) Domain() string {
	return r.domain
}

func (r *testReason) Code() code.Code {
	return r.code
}

// TestNewReason tests creating a new ErrorInfo reason
func TestNewReason(t *testing.T) {
	t.Run("create reason with all fields", func(t *testing.T) {
		reason := &testReason{
			reason: "TEST_REASON",
			domain: "test.domain",
			code:   code.Code_INVALID_ARGUMENT,
		}
		metadata := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		errorInfo := NewReason(reason, metadata)

		assert.NotNil(t, errorInfo)
		assert.Equal(t, "TEST_REASON", errorInfo.Reason)
		assert.Equal(t, "test.domain", errorInfo.Domain)
		assert.Equal(t, "value1", errorInfo.Metadata["key1"])
		assert.Equal(t, "value2", errorInfo.Metadata["key2"])
	})

	t.Run("create reason with empty metadata", func(t *testing.T) {
		reason := &testReason{
			reason: "TEST_REASON",
			domain: "test.domain",
			code:   code.Code_INVALID_ARGUMENT,
		}

		errorInfo := NewReason(reason, nil)

		assert.NotNil(t, errorInfo)
		assert.Equal(t, "TEST_REASON", errorInfo.Reason)
		assert.Equal(t, "test.domain", errorInfo.Domain)
		assert.Nil(t, errorInfo.Metadata)
	})

	t.Run("create reason with empty metadata map", func(t *testing.T) {
		reason := &testReason{
			reason: "TEST_REASON",
			domain: "test.domain",
			code:   code.Code_INVALID_ARGUMENT,
		}

		errorInfo := NewReason(reason, map[string]string{})

		assert.NotNil(t, errorInfo)
		assert.Equal(t, "TEST_REASON", errorInfo.Reason)
		assert.Equal(t, "test.domain", errorInfo.Domain)
	})
}
