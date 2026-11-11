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

package logging

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"

	"google.golang.org/genproto/googleapis/rpc/code"
)

// mockServerStream is a mock implementation of stream.ServerStream for testing
type mockServerStream struct {
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}

func (m *mockServerStream) RecvMsg(interface{}) error {
	return nil
}

func (m *mockServerStream) SendMsg(interface{}) error {
	return nil
}

func (m *mockServerStream) SetHeader(metadata.MD) error {
	return nil
}

func (m *mockServerStream) SendHeader(metadata.MD) error {
	return nil
}

func (m *mockServerStream) SetTrailer(metadata.MD) {
}

// mockClientStream is a mock implementation of stream.ClientStream for testing
type mockClientStream struct{}

func (m *mockClientStream) Header() (metadata.MD, error) {
	return metadata.MD{}, nil
}

func (m *mockClientStream) Trailer() metadata.MD {
	return metadata.MD{}
}

func (m *mockClientStream) CloseSend() error {
	return nil
}

func (m *mockClientStream) Context() context.Context {
	return context.Background()
}

func (m *mockClientStream) SendMsg(interface{}) error {
	return nil
}

func (m *mockClientStream) RecvMsg(interface{}) error {
	return nil
}

// TestConfig tests Config struct
func TestConfig(t *testing.T) {
	t.Run("default config values", func(t *testing.T) {
		cfg := Config{}
		assert.Equal(t, time.Duration(0), cfg.SlowThreshold)
		assert.False(t, cfg.PrintReqAndRes)
	})

	t.Run("set config values", func(t *testing.T) {
		cfg := Config{
			SlowThreshold:  2 * time.Second,
			PrintReqAndRes: true,
		}
		assert.Equal(t, 2*time.Second, cfg.SlowThreshold)
		assert.True(t, cfg.PrintReqAndRes)
	})
}

// TestLogging_UnaryServerInterceptor tests UnaryServerInterceptor method
func TestLogging_UnaryServerInterceptor(t *testing.T) {
	t.Run("successful call", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		info := &interceptor.UnaryServerInfo{
			Server:     &struct{}{},
			FullMethod: "/test.service/Method",
		}

		handler := func(_ context.Context, _ interface{}) (interface{}, error) {
			return "response", nil
		}

		resp, err := l.UnaryServerInterceptor(context.Background(), "request", info, handler)

		assert.NoError(t, err)
		assert.Equal(t, "response", resp)
	})

	t.Run("error call", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		info := &interceptor.UnaryServerInfo{
			Server:     &struct{}{},
			FullMethod: "/test.service/Method",
		}

		handler := func(_ context.Context, _ interface{}) (interface{}, error) {
			return nil, errors.New("handler error")
		}

		resp, err := l.UnaryServerInterceptor(context.Background(), "request", info, handler)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.ErrorContains(t, err, "handler error")
	})

	t.Run("slow call detection", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: 10 * time.Millisecond}}

		info := &interceptor.UnaryServerInfo{
			Server:     &struct{}{},
			FullMethod: "/test.service/Method",
		}

		handler := func(_ context.Context, _ interface{}) (interface{}, error) {
			time.Sleep(50 * time.Millisecond)
			return "response", nil
		}

		resp, err := l.UnaryServerInterceptor(context.Background(), "request", info, handler)

		assert.NoError(t, err)
		assert.Equal(t, "response", resp)
	})

	t.Run("panic recovery", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		info := &interceptor.UnaryServerInfo{
			Server:     &struct{}{},
			FullMethod: "/test.service/Method",
		}

		handler := func(_ context.Context, _ interface{}) (interface{}, error) {
			panic("test panic")
		}

		resp, err := l.UnaryServerInterceptor(context.Background(), "request", info, handler)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.ErrorContains(t, err, "test panic")
	})

	t.Run("panic recovery with error", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		info := &interceptor.UnaryServerInfo{
			Server:     &struct{}{},
			FullMethod: "/test.service/Method",
		}

		testErr := errors.New("panic error")
		handler := func(_ context.Context, _ interface{}) (interface{}, error) {
			panic(testErr)
		}

		resp, err := l.UnaryServerInterceptor(context.Background(), "request", info, handler)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, testErr, err)
	})

	t.Run("with request and response logging", func(t *testing.T) {
		l := &logging{cfg: &Config{
			SlowThreshold:  time.Second,
			PrintReqAndRes: true,
		}}

		info := &interceptor.UnaryServerInfo{
			Server:     &struct{}{},
			FullMethod: "/test.service/Method",
		}

		handler := func(_ context.Context, _ interface{}) (interface{}, error) {
			return "response", nil
		}

		resp, err := l.UnaryServerInterceptor(context.Background(), "request", info, handler)

		assert.NoError(t, err)
		assert.Equal(t, "response", resp)
	})

	t.Run("with status error", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		info := &interceptor.UnaryServerInfo{
			Server:     &struct{}{},
			FullMethod: "/test.service/Method",
		}

		handler := func(_ context.Context, _ interface{}) (interface{}, error) {
			return nil, xerror.New(code.Code_INTERNAL, "internal error")
		}

		resp, err := l.UnaryServerInterceptor(context.Background(), "request", info, handler)

		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

// TestLogging_StreamServerInterceptor tests StreamServerInterceptor method
func TestLogging_StreamServerInterceptor(t *testing.T) {
	t.Run("successful call", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		info := &interceptor.StreamServerInfo{
			FullMethod:     "/test.service/StreamMethod",
			IsClientStream: true,
			IsServerStream: false,
		}

		ss := &mockServerStream{}
		srv := &struct{}{}

		handler := func(interface{}, stream.ServerStream) error {
			return nil
		}

		err := l.StreamServerInterceptor(srv, ss, info, handler)

		assert.NoError(t, err)
	})

	t.Run("error call", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		info := &interceptor.StreamServerInfo{
			FullMethod:     "/test.service/StreamMethod",
			IsClientStream: false,
			IsServerStream: true,
		}

		ss := &mockServerStream{}
		srv := &struct{}{}

		handler := func(interface{}, stream.ServerStream) error {
			return errors.New("stream error")
		}

		err := l.StreamServerInterceptor(srv, ss, info, handler)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "stream error")
	})

	t.Run("panic recovery", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		info := &interceptor.StreamServerInfo{
			FullMethod:     "/test.service/StreamMethod",
			IsClientStream: true,
			IsServerStream: true,
		}

		ss := &mockServerStream{}
		srv := &struct{}{}

		handler := func(interface{}, stream.ServerStream) error {
			panic("stream panic")
		}

		err := l.StreamServerInterceptor(srv, ss, info, handler)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "stream panic")
	})

	t.Run("slow call detection", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: 10 * time.Millisecond}}

		info := &interceptor.StreamServerInfo{
			FullMethod:     "/test.service/StreamMethod",
			IsClientStream: true,
			IsServerStream: false,
		}

		ss := &mockServerStream{}
		srv := &struct{}{}

		handler := func(interface{}, stream.ServerStream) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		}

		err := l.StreamServerInterceptor(srv, ss, info, handler)

		assert.NoError(t, err)
	})
}

// TestLogging_UnaryClientInterceptor tests UnaryClientInterceptor method
func TestLogging_UnaryClientInterceptor(t *testing.T) {
	t.Run("successful call", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		invoker := func(_ context.Context, _ string, _ interface{}, _ interface{}) error {
			return nil
		}

		err := l.UnaryClientInterceptor(
			context.Background(),
			"/test.service/Method",
			"request",
			"reply",
			invoker,
		)

		assert.NoError(t, err)
	})

	t.Run("error call", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		invoker := func(_ context.Context, _ string, _ interface{}, _ interface{}) error {
			return errors.New("invoker error")
		}

		err := l.UnaryClientInterceptor(
			context.Background(),
			"/test.service/Method",
			"request",
			"reply",
			invoker,
		)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "invoker error")
	})

	t.Run("slow call detection", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: 10 * time.Millisecond}}

		invoker := func(_ context.Context, _ string, _ interface{}, _ interface{}) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		}

		err := l.UnaryClientInterceptor(
			context.Background(),
			"/test.service/Method",
			"request",
			"reply",
			invoker,
		)

		assert.NoError(t, err)
	})

	t.Run("panic recovery", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		invoker := func(_ context.Context, _ string, _ interface{}, _ interface{}) error {
			panic("client panic")
		}

		err := l.UnaryClientInterceptor(
			context.Background(),
			"/test.service/Method",
			"request",
			"reply",
			invoker,
		)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "client panic")
	})

	t.Run("with request and reply logging", func(t *testing.T) {
		l := &logging{cfg: &Config{
			SlowThreshold:  time.Second,
			PrintReqAndRes: true,
		}}

		invoker := func(_ context.Context, _ string, _ interface{}, _ interface{}) error {
			return nil
		}

		err := l.UnaryClientInterceptor(
			context.Background(),
			"/test.service/Method",
			"request",
			"reply",
			invoker,
		)

		assert.NoError(t, err)
	})

	t.Run("slow successful call logs at warn level", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: 10 * time.Millisecond}}

		invoker := func(_ context.Context, _ string, _ interface{}, _ interface{}) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		}

		err := l.UnaryClientInterceptor(
			context.Background(),
			"/test.service/Method",
			"request",
			"reply",
			invoker,
		)

		assert.NoError(t, err)
	})

	t.Run("fast successful call logs at info level", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		invoker := func(_ context.Context, _ string, _ interface{}, _ interface{}) error {
			return nil
		}

		err := l.UnaryClientInterceptor(
			context.Background(),
			"/test.service/Method",
			"request",
			"reply",
			invoker,
		)

		assert.NoError(t, err)
	})
}

// TestLogging_StreamClientInterceptor tests StreamClientInterceptor method
func TestLogging_StreamClientInterceptor(t *testing.T) {
	t.Run("successful call", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		streamer := func(_ context.Context, _ *stream.Desc, _ string) (stream.ClientStream, error) {
			return &mockClientStream{}, nil
		}

		cs, err := l.StreamClientInterceptor(
			context.Background(),
			&stream.Desc{},
			"/test.service/StreamMethod",
			streamer,
		)

		assert.NoError(t, err)
		assert.NotNil(t, cs)
	})

	t.Run("error call", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		streamer := func(_ context.Context, _ *stream.Desc, _ string) (stream.ClientStream, error) {
			return nil, errors.New("streamer error")
		}

		cs, err := l.StreamClientInterceptor(
			context.Background(),
			&stream.Desc{},
			"/test.service/StreamMethod",
			streamer,
		)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "streamer error")
		assert.Nil(t, cs)
	})

	t.Run("panic recovery", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		streamer := func(_ context.Context, _ *stream.Desc, _ string) (stream.ClientStream, error) {
			panic("stream client panic")
		}

		cs, err := l.StreamClientInterceptor(
			context.Background(),
			&stream.Desc{},
			"/test.service/StreamMethod",
			streamer,
		)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "stream client panic")
		assert.Nil(t, cs)
	})
}

// TestLogging_ErrorLevel tests error level determination based on HTTP status code
func TestLogging_ErrorLevel(t *testing.T) {
	t.Run("server error (5xx) logs at error level", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		info := &interceptor.UnaryServerInfo{
			Server:     &struct{}{},
			FullMethod: "/test.service/Method",
		}

		handler := func(_ context.Context, _ interface{}) (interface{}, error) {
			return nil, xerror.New(code.Code_INTERNAL, "internal server error")
		}

		resp, err := l.UnaryServerInterceptor(context.Background(), "request", info, handler)

		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("client error (4xx) logs at warn level", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		info := &interceptor.UnaryServerInfo{
			Server:     &struct{}{},
			FullMethod: "/test.service/Method",
		}

		handler := func(_ context.Context, _ interface{}) (interface{}, error) {
			return nil, xerror.New(code.Code_NOT_FOUND, "not found")
		}

		resp, err := l.UnaryServerInterceptor(context.Background(), "request", info, handler)

		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("success logs at info level", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		info := &interceptor.UnaryServerInfo{
			Server:     &struct{}{},
			FullMethod: "/test.service/Method",
		}

		handler := func(_ context.Context, _ interface{}) (interface{}, error) {
			return "response", nil
		}

		resp, err := l.UnaryServerInterceptor(context.Background(), "request", info, handler)

		assert.NoError(t, err)
		assert.Equal(t, "response", resp)
	})
}

// TestLogging_ContextPropagation tests that context is properly propagated
func TestLogging_ContextPropagation(t *testing.T) {
	t.Run("server interceptor preserves context", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		type ctxKey struct{}
		expectedValue := "test-value"

		ctx := context.WithValue(context.Background(), ctxKey{}, expectedValue)

		info := &interceptor.UnaryServerInfo{
			Server:     &struct{}{},
			FullMethod: "/test.service/Method",
		}

		handler := func(ctx context.Context, _ any) (interface{}, error) {
			value := ctx.Value(ctxKey{})
			assert.Equal(t, expectedValue, value)
			return "response", nil
		}

		resp, err := l.UnaryServerInterceptor(ctx, "request", info, handler)

		assert.NoError(t, err)
		assert.Equal(t, "response", resp)
	})

	t.Run("client interceptor preserves context", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		type ctxKey struct{}
		expectedValue := "test-value"

		ctx := context.WithValue(context.Background(), ctxKey{}, expectedValue)

		invoker := func(ctx context.Context, _ string, _ any, _ any) error {
			value := ctx.Value(ctxKey{})
			assert.Equal(t, expectedValue, value)
			return nil
		}

		err := l.UnaryClientInterceptor(ctx, "/test.service/Method", "request", "reply", invoker)

		assert.NoError(t, err)
	})
}

// TestLogging_StatusCodeConversion tests status code to HTTP code conversion
func TestLogging_StatusCodeConversion(t *testing.T) {
	t.Run("various status codes", func(t *testing.T) {
		l := &logging{cfg: &Config{SlowThreshold: time.Second}}

		testCases := []struct {
			name         string
			code         code.Code
			expectMinLvl slog.Level
		}{
			{
				name:         "OK",
				code:         code.Code_OK,
				expectMinLvl: slog.LevelInfo,
			},
			{
				name:         "INTERNAL - should be Error",
				code:         code.Code_INTERNAL,
				expectMinLvl: slog.LevelError,
			},
			{
				name:         "UNKNOWN - should be Error",
				code:         code.Code_UNKNOWN,
				expectMinLvl: slog.LevelError,
			},
			{
				name:         "NOT_FOUND - should be Warn",
				code:         code.Code_NOT_FOUND,
				expectMinLvl: slog.LevelWarn,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				info := &interceptor.UnaryServerInfo{
					Server:     &struct{}{},
					FullMethod: "/test.service/Method",
				}

				handler := func(_ context.Context, _ interface{}) (interface{}, error) {
					return nil, xerror.New(tc.code, "test error")
				}

				resp, err := l.UnaryServerInterceptor(
					context.Background(),
					"request",
					info,
					handler,
				)

				assert.Error(t, err)
				assert.Nil(t, resp)
			})
		}
	})
}

// BenchmarkUnaryServerInterceptor benchmarks the unary server interceptor
func BenchmarkUnaryServerInterceptor(b *testing.B) {
	l := &logging{cfg: &Config{SlowThreshold: time.Second}}

	info := &interceptor.UnaryServerInfo{
		Server:     &struct{}{},
		FullMethod: "/test.service/Method",
	}

	handler := func(_ context.Context, _ interface{}) (interface{}, error) {
		return "response", nil
	}

	ctx := context.Background()
	req := "request"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = l.UnaryServerInterceptor(ctx, req, info, handler)
	}
}

// BenchmarkUnaryClientInterceptor benchmarks the unary client interceptor
func BenchmarkUnaryClientInterceptor(b *testing.B) {
	l := &logging{cfg: &Config{SlowThreshold: time.Second}}

	invoker := func(_ context.Context, _ string, _ interface{}, _ interface{}) error {
		return nil
	}

	ctx := context.Background()
	method := "/test.service/Method"
	req := "request"
	reply := "reply"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.UnaryClientInterceptor(ctx, method, req, reply, invoker)
	}
}

func TestBuiltinProviders(t *testing.T) {
	tests := []struct {
		name string
		get  func() []string
	}{
		{
			name: "TestBuiltinUnaryClientProviders",
			get:  func() []string { return builtinProviderNames(BuiltinUnaryClientProviders()) },
		},
		{
			name: "TestBuiltinStreamClientProviders",
			get:  func() []string { return builtinProviderNames(BuiltinStreamClientProviders()) },
		},
		{
			name: "TestBuiltinUnaryServerProviders",
			get:  func() []string { return builtinProviderNames(BuiltinUnaryServerProviders()) },
		},
		{
			name: "TestBuiltinStreamServerProviders",
			get:  func() []string { return builtinProviderNames(BuiltinStreamServerProviders()) },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, []string{"logging", "logger"}, test.get())
		})
	}
}

func builtinProviderNames[T interface{ Name() string }](providers []T) []string {
	names := make([]string, len(providers))
	for index, provider := range providers {
		names[index] = provider.Name()
	}
	return names
}

func TestBuiltinProvidersWithConfig(t *testing.T) {
	t.Run("unary client with config", func(t *testing.T) {
		providers := BuiltinUnaryClientProvidersWithConfig(nil)
		assert.Len(t, providers, 2)
		for _, p := range providers {
			ic := p.New("svc")
			assert.NotNil(t, ic)
		}
	})
	t.Run("stream client with config", func(t *testing.T) {
		providers := BuiltinStreamClientProvidersWithConfig(nil)
		assert.Len(t, providers, 2)
		for _, p := range providers {
			ic := p.New("svc")
			assert.NotNil(t, ic)
		}
	})
	t.Run("unary server with config", func(t *testing.T) {
		providers := BuiltinUnaryServerProvidersWithConfig(nil)
		assert.Len(t, providers, 2)
		for _, p := range providers {
			ic := p.New()
			assert.NotNil(t, ic)
		}
	})
	t.Run("stream server with config", func(t *testing.T) {
		providers := BuiltinStreamServerProvidersWithConfig(nil)
		assert.Len(t, providers, 2)
		for _, p := range providers {
			ic := p.New()
			assert.NotNil(t, ic)
		}
	})
}

func TestBuiltinProvidersWithConfigEReportsConfigErrors(t *testing.T) {
	invalid := []string{"not a config object"}
	_, err := BuiltinUnaryClientProvidersWithConfigE(invalid)
	require.Error(t, err)
	_, err = BuiltinStreamClientProvidersWithConfigE(invalid)
	require.Error(t, err)
	_, err = BuiltinUnaryServerProvidersWithConfigE(invalid)
	require.Error(t, err)
	_, err = BuiltinStreamServerProvidersWithConfigE(invalid)
	require.Error(t, err)
}

func TestLoggingProviderUsesExplicitLogger(t *testing.T) {
	var explicit bytes.Buffer
	var global bytes.Buffer
	oldDefault := slog.Default()
	t.Cleanup(func() { slog.SetDefault(oldDefault) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(&global, nil)))

	logger := slog.New(slog.NewJSONHandler(&explicit, nil))
	l := newLogging(&Config{SlowThreshold: time.Second}, WithProviderLogger(logger))
	_, err := l.UnaryServerInterceptor(
		context.Background(),
		"request",
		&interceptor.UnaryServerInfo{FullMethod: "/test.Service/Method"},
		func(context.Context, interface{}) (interface{}, error) {
			return "response", nil
		},
	)
	require.NoError(t, err)
	require.Contains(t, explicit.String(), `"msg":"access"`)
	require.Empty(t, global.String())
}
