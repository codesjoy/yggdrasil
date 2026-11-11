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
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockHandler is a mock implementation of Handler interface for testing
type mockHandler struct {
	tagRPCCalled     bool
	handleRPCCalled  bool
	tagChanCalled    bool
	handleChanCalled bool
}

func (m *mockHandler) TagRPC(ctx context.Context, _ RPCTagInfo) context.Context {
	m.tagRPCCalled = true
	return ctx
}

func (m *mockHandler) HandleRPC(context.Context, RPCStats) {
	m.handleRPCCalled = true
}

func (m *mockHandler) TagChannel(ctx context.Context, _ ChanTagInfo) context.Context {
	m.tagChanCalled = true
	return ctx
}

func (m *mockHandler) HandleChannel(context.Context, ChanStats) {
	m.handleChanCalled = true
}

// TestRegisterHandlerBuilder tests RegisterHandlerBuilder function
func TestRegisterHandlerBuilder(t *testing.T) {
	t.Run("register new handler builder", func(t *testing.T) {
		builder := func(bool) Handler {
			return &mockHandler{}
		}

		RegisterHandlerBuilder("test", builder)

		retrieved := GetHandlerBuilder("test")
		assert.NotNil(t, retrieved)
	})

	t.Run("register multiple builders", func(t *testing.T) {
		builder1 := func(bool) Handler {
			return &mockHandler{}
		}
		builder2 := func(bool) Handler {
			return &mockHandler{}
		}

		RegisterHandlerBuilder("builder1", builder1)
		RegisterHandlerBuilder("builder2", builder2)

		retrieved1 := GetHandlerBuilder("builder1")
		retrieved2 := GetHandlerBuilder("builder2")

		assert.NotNil(t, retrieved1)
		assert.NotNil(t, retrieved2)
	})

	t.Run("override existing builder", func(t *testing.T) {
		builder1 := func(bool) Handler {
			return &mockHandler{}
		}
		builder2 := func(bool) Handler {
			return &mockHandler{}
		}

		RegisterHandlerBuilder("override-test", builder1)
		RegisterHandlerBuilder("override-test", builder2)

		retrieved := GetHandlerBuilder("override-test")
		assert.NotNil(t, retrieved)
	})

	t.Run("get non-existent builder", func(t *testing.T) {
		retrieved := GetHandlerBuilder("non-existent")
		assert.Nil(t, retrieved)
	})
}

// TestGetHandlerBuilder tests GetHandlerBuilder function
func TestGetHandlerBuilder(t *testing.T) {
	t.Run("get existing builder", func(t *testing.T) {
		builder := func(bool) Handler {
			return &mockHandler{}
		}

		RegisterHandlerBuilder("get-test", builder)

		retrieved := GetHandlerBuilder("get-test")
		assert.NotNil(t, retrieved)
	})

	t.Run("get builder returns the same function", func(t *testing.T) {
		called := false
		builder := func(bool) Handler {
			called = true
			return &mockHandler{}
		}

		RegisterHandlerBuilder("same-func-test", builder)

		retrieved := GetHandlerBuilder("same-func-test")
		assert.NotNil(t, retrieved)

		// Call the builder
		h := retrieved(true)
		assert.NotNil(t, h)
		assert.True(t, called)
	})
}

// TestHandlerChain tests handlerChain implementation
func TestHandlerChain(t *testing.T) {
	t.Run("TagRPC calls all handlers", func(t *testing.T) {
		mock1 := &mockHandler{}
		mock2 := &mockHandler{}
		mock3 := &mockHandler{}

		chain := &handlerChain{
			handlers: []Handler{mock1, mock2, mock3},
		}

		ctx := context.Background()
		info := &RPCTagInfoBase{FullMethod: "/test/method"}

		result := chain.TagRPC(ctx, info)

		assert.True(t, mock1.tagRPCCalled)
		assert.True(t, mock2.tagRPCCalled)
		assert.True(t, mock3.tagRPCCalled)
		assert.NotNil(t, result)
	})

	t.Run("HandleRPC calls all handlers", func(t *testing.T) {
		mock1 := &mockHandler{}
		mock2 := &mockHandler{}

		chain := &handlerChain{
			handlers: []Handler{mock1, mock2},
		}

		ctx := context.Background()
		stats := &RPCBeginBase{}

		chain.HandleRPC(ctx, stats)

		assert.True(t, mock1.handleRPCCalled)
		assert.True(t, mock2.handleRPCCalled)
	})

	t.Run("TagChannel calls all handlers", func(t *testing.T) {
		mock1 := &mockHandler{}
		mock2 := &mockHandler{}

		chain := &handlerChain{
			handlers: []Handler{mock1, mock2},
		}

		ctx := context.Background()
		info := &ChanTagInfoBase{Protocol: "grpc"}

		result := chain.TagChannel(ctx, info)

		assert.True(t, mock1.tagChanCalled)
		assert.True(t, mock2.tagChanCalled)
		assert.NotNil(t, result)
	})

	t.Run("HandleChannel calls all handlers", func(t *testing.T) {
		mock1 := &mockHandler{}
		mock2 := &mockHandler{}

		chain := &handlerChain{
			handlers: []Handler{mock1, mock2},
		}

		ctx := context.Background()
		stats := &ChanBeginBase{}

		chain.HandleChannel(ctx, stats)

		assert.True(t, mock1.handleChanCalled)
		assert.True(t, mock2.handleChanCalled)
	})

	t.Run("empty handler chain", func(t *testing.T) {
		chain := &handlerChain{
			handlers: []Handler{},
		}

		ctx := context.Background()

		// Should not panic
		result := chain.TagRPC(ctx, &RPCTagInfoBase{})
		assert.NotNil(t, result)

		chain.HandleRPC(ctx, &RPCBeginBase{})

		result = chain.TagChannel(ctx, &ChanTagInfoBase{})
		assert.NotNil(t, result)

		chain.HandleChannel(ctx, &ChanBeginBase{})
	})
}

// TestHandlerChain_ContextPropagation tests context propagation through handler chain
func TestHandlerChain_ContextPropagation(t *testing.T) {
	t.Run("context is passed through handlers", func(t *testing.T) {
		type ctxKey struct{}
		expectedValue := "test-value"

		handler1 := &contextTestHandler{
			tagRPCFunc: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, ctxKey{}, expectedValue)
			},
		}
		handler2 := &contextTestHandler{
			tagRPCFunc: func(ctx context.Context) context.Context {
				// Verify context value from handler1
				value := ctx.Value(ctxKey{})
				assert.Equal(t, expectedValue, value)
				return ctx
			},
		}

		chain := &handlerChain{
			handlers: []Handler{handler1, handler2},
		}

		ctx := context.Background()
		info := &RPCTagInfoBase{FullMethod: "/test/method"}

		result := chain.TagRPC(ctx, info)

		assert.NotNil(t, result)
	})
}

// contextTestHandler is a test handler that allows custom behavior
type contextTestHandler struct {
	tagRPCFunc     func(context.Context) context.Context
	tagChanFunc    func(context.Context) context.Context
	handleRPCFunc  func(context.Context, RPCStats)
	handleChanFunc func(context.Context, ChanStats)
}

func (c *contextTestHandler) TagRPC(ctx context.Context, _ RPCTagInfo) context.Context {
	if c.tagRPCFunc != nil {
		return c.tagRPCFunc(ctx)
	}
	return ctx
}

func (c *contextTestHandler) HandleRPC(ctx context.Context, rs RPCStats) {
	if c.handleRPCFunc != nil {
		c.handleRPCFunc(ctx, rs)
	}
}

func (c *contextTestHandler) TagChannel(ctx context.Context, _ ChanTagInfo) context.Context {
	if c.tagChanFunc != nil {
		return c.tagChanFunc(ctx)
	}
	return ctx
}

func (c *contextTestHandler) HandleChannel(ctx context.Context, cs ChanStats) {
	if c.handleChanFunc != nil {
		c.handleChanFunc(ctx, cs)
	}
}

// TestGetServerHandler tests GetServerHandler function
func TestGetServerHandler(t *testing.T) {
	testScopedHandler(t, "server", "GetServerHandler", GetServerHandler)
}

// TestGetClientHandler tests GetClientHandler function
func TestGetClientHandler(t *testing.T) {
	testScopedHandler(t, "client", "GetClientHandler", GetClientHandler)
}

func testScopedHandler(
	t *testing.T,
	kind string,
	getterName string,
	get func() Handler,
) {
	t.Helper()

	t.Run("get "+kind+" handler", func(t *testing.T) {
		assert.NotNil(t, get())
	})

	t.Run(getterName+" is idempotent", func(t *testing.T) {
		assert.Same(t, get(), get())
	})

	t.Run(getterName+" is thread-safe", func(t *testing.T) {
		handlers := make([]Handler, 10)
		var wg sync.WaitGroup
		for index := range handlers {
			wg.Go(func() {
				handlers[index] = get()
			})
		}
		wg.Wait()

		for index := 1; index < len(handlers); index++ {
			assert.Same(t, handlers[0], handlers[index])
		}
	})
}

// TestHandlerInterface tests Handler interface implementation
func TestHandlerInterface(t *testing.T) {
	t.Run("handlerChain implements Handler interface", func(t *testing.T) {
		var handler Handler = &handlerChain{
			handlers: []Handler{&mockHandler{}},
		}

		ctx := context.Background()
		info := &RPCTagInfoBase{FullMethod: "/test/method"}

		// Should not panic
		result := handler.TagRPC(ctx, info)
		assert.NotNil(t, result)

		handler.HandleRPC(ctx, &RPCBeginBase{})

		result = handler.TagChannel(ctx, &ChanTagInfoBase{})
		assert.NotNil(t, result)

		handler.HandleChannel(ctx, &ChanBeginBase{})
	})
}

// TestRegisterHandlerBuilder_Concurrency tests concurrent registration
func TestRegisterHandlerBuilder_Concurrency(t *testing.T) {
	t.Run("concurrent registration", func(t *testing.T) {
		const numGoroutines = 100

		for i := 0; i < numGoroutines; i++ {
			builder := func(bool) Handler {
				return &mockHandler{}
			}
			RegisterHandlerBuilder("concurrent", builder)
		}

		// Should not panic
		retrieved := GetHandlerBuilder("concurrent")
		assert.NotNil(t, retrieved)
	})
}

func TestParseHandlerNames(t *testing.T) {
	t.Run("trim empty and dedup", func(t *testing.T) {
		names := ParseHandlerNames(" otel,otel, , custom , otel ,custom ")
		assert.Equal(t, []string{"otel", "custom"}, names)
	})

	t.Run("empty input", func(t *testing.T) {
		assert.Empty(t, ParseHandlerNames(""))
	})
}

// TestChanTagInfoBase tests ChanTagInfoBase methods.
func TestChanTagInfoBase(t *testing.T) {
	t.Run("create and get fields", func(t *testing.T) {
		info := &ChanTagInfoBase{
			RemoteEndpoint: "127.0.0.1:8080",
			LocalEndpoint:  "127.0.0.1:3000",
			Protocol:       "grpc",
		}

		assert.Equal(t, "127.0.0.1:8080", info.GetRemoteEndpoint())
		assert.Equal(t, "127.0.0.1:3000", info.GetLocalEndpoint())
		assert.Equal(t, "grpc", info.GetProtocol())
	})

	t.Run("implements ChanTagInfo interface", func(t *testing.T) {
		var info ChanTagInfo = &ChanTagInfoBase{
			RemoteEndpoint: "remote",
			LocalEndpoint:  "local",
			Protocol:       "http",
		}

		assert.Equal(t, "remote", info.GetRemoteEndpoint())
		assert.Equal(t, "local", info.GetLocalEndpoint())
		assert.Equal(t, "http", info.GetProtocol())
	})
}

// TestChanBeginBase tests ChanBeginBase methods.
func TestChanBeginBase(t *testing.T) {
	t.Run("client channel begin", func(t *testing.T) {
		begin := &ChanBeginBase{Client: true}
		assert.True(t, begin.IsClient())
		assert.Implements(t, (*ChanStats)(nil), begin)
		assert.Implements(t, (*ChanBegin)(nil), begin)
	})

	t.Run("server channel begin", func(t *testing.T) {
		begin := &ChanBeginBase{Client: false}
		assert.False(t, begin.IsClient())
	})
}

// TestChanEndBase tests ChanEndBase methods.
func TestChanEndBase(t *testing.T) {
	t.Run("client channel end", func(t *testing.T) {
		end := &ChanEndBase{Client: true}
		assert.True(t, end.IsClient())
		assert.Implements(t, (*ChanStats)(nil), end)
		assert.Implements(t, (*ChanEnd)(nil), end)
	})

	t.Run("server channel end", func(t *testing.T) {
		end := &ChanEndBase{Client: false}
		assert.False(t, end.IsClient())
	})
}

// TestChanStatsInterface tests ChanStats interface implementations.
func TestChanStatsInterface(t *testing.T) {
	t.Run("ChanBeginBase implements ChanStats", func(t *testing.T) {
		var stats ChanStats = &ChanBeginBase{Client: true}
		assert.True(t, stats.IsClient())
	})

	t.Run("ChanEndBase implements ChanStats", func(t *testing.T) {
		var stats ChanStats = &ChanEndBase{Client: false}
		assert.False(t, stats.IsClient())
	})
}

// TestChanTagInfoInterface tests ChanTagInfo interface implementation.
func TestChanTagInfoInterface(t *testing.T) {
	t.Run("ChanTagInfoBase implements ChanTagInfo", func(t *testing.T) {
		var info ChanTagInfo = &ChanTagInfoBase{
			RemoteEndpoint: "example.com:443",
			LocalEndpoint:  "localhost:8080",
			Protocol:       "https",
		}

		assert.Equal(t, "example.com:443", info.GetRemoteEndpoint())
		assert.Equal(t, "localhost:8080", info.GetLocalEndpoint())
		assert.Equal(t, "https", info.GetProtocol())
	})
}
