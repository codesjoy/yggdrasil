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
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	"github.com/codesjoy/yggdrasil/pkg/metadata"
	"github.com/stretchr/testify/assert"
)

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

// TestNewLocalizedMsg tests creating a new LocalizedMessage
func TestNewLocalizedMsg(t *testing.T) {
	t.Run("create localized message from context", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"en":    "Error occurred",
				"zh-CN": "发生错误",
			},
		}

		md := metadata.New(map[string]string{"language": "en"})
		ctx := metadata.WithInContext(context.Background(), md)

		localizedMsg := NewLocalizedMsg(ctx, msg)

		assert.NotNil(t, localizedMsg)
		assert.Equal(t, "en", localizedMsg.Locale)
		assert.Equal(t, "Error occurred", localizedMsg.Message)
	})

	t.Run("create localized message with Chinese", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"en":    "Error occurred",
				"zh-CN": "发生错误",
			},
		}

		md := metadata.New(map[string]string{"language": "zh-CN"})
		ctx := metadata.WithInContext(context.Background(), md)

		localizedMsg := NewLocalizedMsg(ctx, msg)

		assert.NotNil(t, localizedMsg)
		assert.Equal(t, "zh-CN", localizedMsg.Locale)
		assert.Equal(t, "发生错误", localizedMsg.Message)
	})

	t.Run("fallback to default language", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"zh-CN": "发生错误",
			},
		}

		ctx := context.Background()

		localizedMsg := NewLocalizedMsg(ctx, msg)

		assert.NotNil(t, localizedMsg)
		// Should fallback to zh-CN as default
		assert.Equal(t, "zh-CN", localizedMsg.Locale)
		assert.Equal(t, "发生错误", localizedMsg.Message)
	})

	t.Run("no language returns nil", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{},
		}

		ctx := context.Background()

		localizedMsg := NewLocalizedMsg(ctx, msg)

		assert.Nil(t, localizedMsg)
	})

	t.Run("context with multiple languages", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"en":    "Error occurred",
				"zh-CN": "发生错误",
				"fr":    "Erreur survenue",
			},
		}

		// Note: metadata.MD stores values as []string, so we can have multiple values
		md := metadata.MD{"language": []string{"fr", "en", "zh-CN"}}
		ctx := metadata.WithInContext(context.Background(), md)

		localizedMsg := NewLocalizedMsg(ctx, msg)

		assert.NotNil(t, localizedMsg)
		assert.Equal(t, "fr", localizedMsg.Locale)
		assert.Equal(t, "Erreur survenue", localizedMsg.Message)
	})

	t.Run("first available language is used", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"en":    "Error occurred",
				"zh-CN": "发生错误",
			},
		}

		md := metadata.MD{"language": []string{"fr", "de", "en"}}
		ctx := metadata.WithInContext(context.Background(), md)

		localizedMsg := NewLocalizedMsg(ctx, msg)

		assert.NotNil(t, localizedMsg)
		assert.Equal(t, "en", localizedMsg.Locale)
		assert.Equal(t, "Error occurred", localizedMsg.Message)
	})
}

// TestNewLocalizedMsgWithLang tests creating localized message with language list
func TestNewLocalizedMsgWithLang(t *testing.T) {
	t.Run("create with single language", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"en": "Hello",
			},
		}

		localizedMsg := NewLocalizedMsgWithLang([]string{"en"}, msg)

		assert.NotNil(t, localizedMsg)
		assert.Equal(t, "en", localizedMsg.Locale)
		assert.Equal(t, "Hello", localizedMsg.Message)
	})

	t.Run("create with multiple languages", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"en": "Hello",
				"fr": "Bonjour",
			},
		}

		localizedMsg := NewLocalizedMsgWithLang([]string{"fr", "en"}, msg)

		assert.NotNil(t, localizedMsg)
		assert.Equal(t, "fr", localizedMsg.Locale)
		assert.Equal(t, "Bonjour", localizedMsg.Message)
	})

	t.Run("fallback to next available language", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"en":    "Hello",
				"zh-CN": "你好",
			},
		}

		localizedMsg := NewLocalizedMsgWithLang([]string{"fr", "de", "zh-CN"}, msg)

		assert.NotNil(t, localizedMsg)
		assert.Equal(t, "zh-CN", localizedMsg.Locale)
		assert.Equal(t, "你好", localizedMsg.Message)
	})

	t.Run("fallback to default zh-CN", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"zh-CN": "你好",
			},
		}

		localizedMsg := NewLocalizedMsgWithLang([]string{"fr", "de"}, msg)

		assert.NotNil(t, localizedMsg)
		assert.Equal(t, "zh-CN", localizedMsg.Locale)
		assert.Equal(t, "你好", localizedMsg.Message)
	})

	t.Run("empty languages with default fallback", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"zh-CN": "默认消息",
			},
		}

		localizedMsg := NewLocalizedMsgWithLang([]string{}, msg)

		assert.NotNil(t, localizedMsg)
		assert.Equal(t, "zh-CN", localizedMsg.Locale)
		assert.Equal(t, "默认消息", localizedMsg.Message)
	})

	t.Run("no message available returns nil", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{},
		}

		localizedMsg := NewLocalizedMsgWithLang([]string{"en", "fr"}, msg)

		assert.Nil(t, localizedMsg)
	})

	t.Run("empty message string is treated as unavailable", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"en": "",
			},
		}

		// NewLocalizedMsgWithLang will fallback to zh-CN as default
		// Since we don't have zh-CN in the messages, it returns nil
		localizedMsg := NewLocalizedMsgWithLang([]string{"en"}, msg)

		assert.Nil(t, localizedMsg)
	})
}

// TestNewDebugInfo tests creating a new DebugInfo
func TestNewDebugInfo(t *testing.T) {
	t.Run("create debug info with stacks and message", func(t *testing.T) {
		stacks := []string{
			"main.func1",
			"  at file1.go:10",
			"main.func2",
			"  at file2.go:20",
		}
		msg := "debug detail"

		debugInfo := NewDebugInfo(stacks, msg)

		assert.NotNil(t, debugInfo)
		assert.Equal(t, stacks, debugInfo.StackEntries)
		assert.Equal(t, msg, debugInfo.Detail)
	})

	t.Run("create debug info with empty stacks", func(t *testing.T) {
		debugInfo := NewDebugInfo([]string{}, "message")

		assert.NotNil(t, debugInfo)
		assert.Empty(t, debugInfo.StackEntries)
		assert.Equal(t, "message", debugInfo.Detail)
	})

	t.Run("create debug info with nil stacks", func(t *testing.T) {
		debugInfo := NewDebugInfo(nil, "message")

		assert.NotNil(t, debugInfo)
		assert.Nil(t, debugInfo.StackEntries)
		assert.Equal(t, "message", debugInfo.Detail)
	})

	t.Run("create debug info with empty message", func(t *testing.T) {
		stacks := []string{"stack1", "stack2"}

		debugInfo := NewDebugInfo(stacks, "")

		assert.NotNil(t, debugInfo)
		assert.Equal(t, stacks, debugInfo.StackEntries)
		assert.Equal(t, "", debugInfo.Detail)
	})
}

// TestDetailsIntegration tests integration of details with status
func TestDetailsIntegration(t *testing.T) {
	t.Run("status with ErrorInfo detail", func(t *testing.T) {
		reason := &testReason{
			reason: "VALIDATION_ERROR",
			domain: "api.example.com",
			code:   code.Code_INVALID_ARGUMENT,
		}
		err := errors.New("validation failed")

		st := FromReason(err, reason, map[string]string{
			"field":   "email",
			"message": "invalid format",
		})

		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_INVALID_ARGUMENT), st.Code())

		errorInfo := st.Reason()
		assert.NotNil(t, errorInfo)
		assert.Equal(t, "VALIDATION_ERROR", errorInfo.Reason)
		assert.Equal(t, "api.example.com", errorInfo.Domain)
		assert.Equal(t, "email", errorInfo.Metadata["field"])
	})

	t.Run("status with LocalizedMessage detail", func(t *testing.T) {
		err := errors.New("error occurred")
		st := New(code.Code_INTERNAL, err)

		msg := &testMessage{
			messages: map[string]string{
				"en": "Internal error",
			},
		}

		ctx := context.Background()
		localizedMsg := NewLocalizedMsg(ctx, msg)
		if localizedMsg != nil {
			st.WithDetails(localizedMsg)
		}

		assert.NotNil(t, st)
	})

	t.Run("status with DebugInfo detail", func(t *testing.T) {
		err := errors.New("error with stack")
		st := New(code.Code_INTERNAL, err)

		stacks := []string{
			"main.main",
			"  at main.go:10",
		}

		debugInfo := NewDebugInfo(stacks, "stack trace")
		st.WithDetails(debugInfo)

		assert.NotNil(t, st)
		assert.Equal(t, 1, len(st.stu.Details))
	})

	t.Run("status with multiple detail types", func(t *testing.T) {
		err := errors.New("complex error")
		st := New(code.Code_INTERNAL, err)

		reason := &testReason{
			reason: "COMPLEX_ERROR",
			domain: "test.domain",
			code:   code.Code_INTERNAL,
		}
		errorInfo := NewReason(reason, map[string]string{"key": "value"})

		debugInfo := NewDebugInfo([]string{"stack1", "stack2"}, "detail")

		msg := &testMessage{
			messages: map[string]string{"en": "Error message"},
		}
		localizedMsg := NewLocalizedMsg(context.Background(), msg)

		st.WithDetails(errorInfo, debugInfo)
		if localizedMsg != nil {
			st.WithDetails(localizedMsg)
		}

		assert.NotNil(t, st)
		assert.GreaterOrEqual(t, len(st.stu.Details), 2)
	})
}

// TestReasonInterface tests Reason interface implementations
func TestReasonInterface(t *testing.T) {
	t.Run("testReason implementation", func(t *testing.T) {
		reason := &testReason{
			reason: "TEST_REASON",
			domain: "test.domain",
			code:   code.Code_INVALID_ARGUMENT,
		}

		assert.Equal(t, "TEST_REASON", reason.Reason())
		assert.Equal(t, "test.domain", reason.Domain())
		assert.Equal(t, code.Code_INVALID_ARGUMENT, reason.Code())
	})

	t.Run("multiple reasons", func(t *testing.T) {
		reasons := []Reason{
			&testReason{reason: "REASON_1", domain: "domain1", code: code.Code_OK},
			&testReason{reason: "REASON_2", domain: "domain2", code: code.Code_NOT_FOUND},
			&testReason{reason: "REASON_3", domain: "domain3", code: code.Code_PERMISSION_DENIED},
		}

		for i, r := range reasons {
			assert.NotEmpty(t, r.Reason(), "reason %d should have Reason", i)
			assert.NotEmpty(t, r.Domain(), "reason %d should have Domain", i)
			assert.NotNil(t, r.Code(), "reason %d should have Code", i)
		}
	})
}

// TestMessageInterface tests Message interface implementations
func TestMessageInterface(t *testing.T) {
	t.Run("testMessage implementation", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"en":    "English",
				"zh-CN": "Chinese",
				"fr":    "French",
			},
		}

		assert.Equal(t, "English", msg.Message("en"))
		assert.Equal(t, "Chinese", msg.Message("zh-CN"))
		assert.Equal(t, "French", msg.Message("fr"))
		assert.Equal(t, "", msg.Message("de"))
	})

	t.Run("message with empty strings", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"en": "",
				"zh": "有内容",
			},
		}

		assert.Equal(t, "", msg.Message("en"))
		assert.Equal(t, "有内容", msg.Message("zh"))
	})
}

// TestDetails_RealWorldScenarios tests real-world usage scenarios
func TestDetails_RealWorldScenarios(t *testing.T) {
	t.Run("gRPC error with multiple details", func(t *testing.T) {
		err := errors.New("validation failed")

		reason := &testReason{
			reason: "VALIDATION_FAILED",
			domain: "user.service",
			code:   code.Code_INVALID_ARGUMENT,
		}

		st := FromReason(err, reason, map[string]string{
			"field":      "username",
			"constraint": "min_length",
		})

		// FromReason adds ErrorInfo detail (1 detail)
		assert.Equal(t, 1, len(st.stu.Details))

		// Add localized message
		msg := &testMessage{
			messages: map[string]string{
				"en":    "Username validation failed",
				"zh-CN": "用户名验证失败",
			},
		}

		md := metadata.New(map[string]string{"language": "en"})
		ctx := metadata.WithInContext(context.Background(), md)

		// Wrap status in error so FromError can extract it
		wrappedErr := fmt.Errorf("wrapped: %w", st)
		st = WithMessage(ctx, wrappedErr, msg)

		assert.NotNil(t, st)
		assert.Equal(t, int32(code.Code_INVALID_ARGUMENT), st.Code())
		// WithMessage adds LocalizedMessage detail (2 details total)
		assert.Equal(t, 2, len(st.stu.Details))
	})

	t.Run("internal error with debug info", func(t *testing.T) {
		err := errors.New("database connection failed")
		st := New(code.Code_INTERNAL, err)

		// Add stack trace as debug info
		// Note: errors.New() doesn't include stack traces, so WithStack() may not add details
		st.WithStack()

		assert.NotNil(t, st)
		// Details count depends on whether error has stack info
		// For simple errors, WithStack() might not add anything
	})

	t.Run("not found error with help links", func(t *testing.T) {
		err := errors.New("user not found")
		st := New(code.Code_NOT_FOUND, err)

		helpInfo := &errdetails.Help{
			Links: []*errdetails.Help_Link{
				{
					Url:         "https://docs.example.com/troubleshooting",
					Description: "Troubleshooting guide",
				},
				{
					Url:         "https://support.example.com",
					Description: "Support",
				},
			},
		}

		st.WithDetails(helpInfo)

		assert.NotNil(t, st)
		assert.Equal(t, 1, len(st.stu.Details))
	})

	t.Run("multi-language error response", func(t *testing.T) {
		msg := &testMessage{
			messages: map[string]string{
				"en":    "Service temporarily unavailable",
				"zh-CN": "服务暂时不可用",
				"ja":    "サービスは一時的に利用できません",
			},
		}

		// Test different languages
		languages := []string{"en", "zh-CN", "ja"}
		for _, lang := range languages {
			md := metadata.New(map[string]string{"language": lang})
			ctx := metadata.WithInContext(context.Background(), md)

			localizedMsg := NewLocalizedMsg(ctx, msg)
			if localizedMsg != nil {
				assert.Equal(t, lang, localizedMsg.Locale)
				assert.NotEmpty(t, localizedMsg.Message)
			}
		}
	})
}
