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

package interceptor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/stream"
	"github.com/stretchr/testify/assert"
)

// TestRegisterUnaryClientIntBuilder tests RegisterUnaryClientIntBuilder function
func TestRegisterUnaryClientIntBuilder(t *testing.T) {
	t.Run("register builder successfully", func(t *testing.T) {
		builder := func(string) UnaryClientInterceptor {
			return func(ctx context.Context, method string, req, reply any, invoker UnaryInvoker) error {
				return invoker(ctx, method, req, reply)
			}
		}

		RegisterUnaryClientIntBuilder("test-interceptor", builder)

		retrieved := getUnaryClientIntBuilder("test-interceptor")
		assert.NotNil(t, retrieved)

		// Verify the builder works
		interceptor := retrieved("test-service")
		assert.NotNil(t, interceptor)
	})

	t.Run("register multiple builders", func(t *testing.T) {
		builders := []struct {
			name string
		}{
			{"interceptor-1"},
			{"interceptor-2"},
			{"interceptor-3"},
		}

		for _, b := range builders {
			RegisterUnaryClientIntBuilder(b.name, func(string) UnaryClientInterceptor {
				return func(ctx context.Context, method string, req, reply any, invoker UnaryInvoker) error {
					return invoker(ctx, method, req, reply)
				}
			})
		}

		for _, b := range builders {
			retrieved := getUnaryClientIntBuilder(b.name)
			assert.NotNil(t, retrieved, "builder %s should be registered", b.name)
		}
	})

	t.Run("override existing builder", func(t *testing.T) {
		RegisterUnaryClientIntBuilder(
			"override-test",
			func(string) UnaryClientInterceptor {
				return func(context.Context, string, any, any, UnaryInvoker) error {
					return errors.New("old")
				}
			},
		)

		newBuilder := func(string) UnaryClientInterceptor {
			return func(context.Context, string, any, any, UnaryInvoker) error {
				return errors.New("new")
			}
		}
		RegisterUnaryClientIntBuilder("override-test", newBuilder)

		retrieved := getUnaryClientIntBuilder("override-test")
		assert.NotNil(t, retrieved)
		interceptor := retrieved("test-service")
		err := interceptor(
			context.Background(),
			"/test/method",
			"req",
			"reply",
			func(context.Context, string, any, any) error {
				return nil
			},
		)
		assert.EqualError(t, err, "new")
	})
}

// TestRegisterUnaryServerIntBuilder tests RegisterUnaryServerIntBuilder function
func TestRegisterUnaryServerIntBuilder(t *testing.T) {
	t.Run("register builder successfully", func(t *testing.T) {
		builder := func() UnaryServerInterceptor {
			return func(ctx context.Context, req any, _ *UnaryServerInfo, handler UnaryHandler) (any, error) {
				return handler(ctx, req)
			}
		}

		RegisterUnaryServerIntBuilder("test-server-interceptor", builder)

		retrieved := getUnaryServerIntBuilder("test-server-interceptor")
		assert.NotNil(t, retrieved)

		interceptor := retrieved()
		assert.NotNil(t, interceptor)
	})
}

// TestRegisterStreamClientIntBuilder tests RegisterStreamClientIntBuilder function
func TestRegisterStreamClientIntBuilder(t *testing.T) {
	t.Run("register builder successfully", func(t *testing.T) {
		builder := func(_ string) StreamClientInterceptor {
			return func(ctx context.Context, desc *stream.Desc, method string, streamer Streamer) (stream.ClientStream, error) {
				return streamer(ctx, desc, method)
			}
		}

		RegisterStreamClientIntBuilder("test-stream-client", builder)

		retrieved := getStreamClientIntBuilder("test-stream-client")
		assert.NotNil(t, retrieved)

		interceptor := retrieved("test-service")
		assert.NotNil(t, interceptor)
	})
}

// TestRegisterStreamServerIntBuilder tests RegisterStreamServerIntBuilder function
func TestRegisterStreamServerIntBuilder(t *testing.T) {
	t.Run("register builder successfully", func(t *testing.T) {
		builder := func() StreamServerInterceptor {
			return func(srv interface{}, ss stream.ServerStream, _ *StreamServerInfo, handler stream.Handler) error {
				return handler(srv, ss)
			}
		}

		RegisterStreamServerIntBuilder("test-stream-server", builder)

		retrieved := getStreamServerIntBuilder("test-stream-server")
		assert.NotNil(t, retrieved)

		interceptor := retrieved()
		assert.NotNil(t, interceptor)
	})
}

// TestChainUnaryClientInterceptors tests ChainUnaryClientInterceptors function
func TestChainUnaryClientInterceptors(t *testing.T) {
	t.Run("empty chain returns passthrough", func(t *testing.T) {
		chain := ChainUnaryClientInterceptors("test-service", []string{})

		assert.NotNil(t, chain)

		invokerCalled := false
		err := chain(
			context.Background(),
			"/test/method",
			"req",
			"reply",
			func(context.Context, string, any, any) error {
				invokerCalled = true
				return nil
			},
		)

		assert.True(t, invokerCalled)
		assert.NoError(t, err)
	})

	t.Run("single interceptor", func(t *testing.T) {
		RegisterUnaryClientIntBuilder(
			"single-test",
			func(string) UnaryClientInterceptor {
				return func(ctx context.Context, method string, req, reply any, invoker UnaryInvoker) error {
					return invoker(ctx, method, req, reply)
				}
			},
		)

		chain := ChainUnaryClientInterceptors("test-service", []string{"single-test"})

		invokerCalled := false
		err := chain(
			context.Background(),
			"/test/method",
			"req",
			"reply",
			func(context.Context, string, any, any) error {
				invokerCalled = true
				return nil
			},
		)

		assert.True(t, invokerCalled)
		assert.NoError(t, err)
	})

	t.Run("multiple interceptors in order", func(t *testing.T) {
		callOrder := []string{}
		RegisterUnaryClientIntBuilder("first", func(string) UnaryClientInterceptor {
			return func(ctx context.Context, method string, req, reply any, invoker UnaryInvoker) error {
				callOrder = append(callOrder, "first")
				return invoker(ctx, method, req, reply)
			}
		})
		RegisterUnaryClientIntBuilder("second", func(string) UnaryClientInterceptor {
			return func(ctx context.Context, method string, req, reply any, invoker UnaryInvoker) error {
				callOrder = append(callOrder, "second")
				return invoker(ctx, method, req, reply)
			}
		})
		RegisterUnaryClientIntBuilder("third", func(string) UnaryClientInterceptor {
			return func(ctx context.Context, method string, req, reply any, invoker UnaryInvoker) error {
				callOrder = append(callOrder, "third")
				return invoker(ctx, method, req, reply)
			}
		})

		chain := ChainUnaryClientInterceptors("test-service", []string{"first", "second", "third"})

		err := chain(
			context.Background(),
			"/test/method",
			"req",
			"reply",
			func(_ context.Context, _ string, _, _ interface{}) error {
				callOrder = append(callOrder, "invoker")
				return nil
			},
		)
		assert.Nil(t, err)
		assert.Equal(t, []string{"first", "second", "third", "invoker"}, callOrder)
	})

	t.Run("missing interceptor is skipped", func(t *testing.T) {
		RegisterUnaryClientIntBuilder(
			"existing-1",
			func(string) UnaryClientInterceptor {
				return func(ctx context.Context, method string, req, reply any, invoker UnaryInvoker) error {
					return invoker(ctx, method, req, reply)
				}
			},
		)
		RegisterUnaryClientIntBuilder(
			"existing-2",
			func(string) UnaryClientInterceptor {
				return func(ctx context.Context, method string, req, reply any, invoker UnaryInvoker) error {
					return invoker(ctx, method, req, reply)
				}
			},
		)

		chain := ChainUnaryClientInterceptors(
			"test-service",
			[]string{"existing-1", "non-existent", "existing-2"},
		)

		invokerCalled := false
		err := chain(
			context.Background(),
			"/test/method",
			"req",
			"reply",
			func(context.Context, string, any, any) error {
				invokerCalled = true
				return nil
			},
		)

		assert.True(t, invokerCalled)
		assert.NoError(t, err)
	})

	t.Run("interceptor can modify request", func(t *testing.T) {
		RegisterUnaryClientIntBuilder("modifier", func(string) UnaryClientInterceptor {
			return func(ctx context.Context, method string, _, reply any, invoker UnaryInvoker) error {
				return invoker(ctx, method, "modified-req", reply)
			}
		})

		chain := ChainUnaryClientInterceptors("test-service", []string{"modifier"})

		var receivedReq interface{}
		_ = chain(
			context.Background(),
			"/test/method",
			"req",
			"reply",
			func(_ context.Context, _ string, req, _ any) error {
				receivedReq = req
				return nil
			},
		)

		assert.Equal(t, "modified-req", receivedReq)
	})

	t.Run("interceptor can modify response", func(t *testing.T) {
		RegisterUnaryClientIntBuilder(
			"reply-modifier",
			func(string) UnaryClientInterceptor {
				// nolint:staticcheck
				return func(ctx context.Context, method string, req, reply any, invoker UnaryInvoker) error {
					reply = "modified-reply"
					return invoker(ctx, method, req, reply)
				}
			},
		)

		chain := ChainUnaryClientInterceptors("test-service", []string{"reply-modifier"})

		var receivedReply interface{}
		_ = chain(
			context.Background(),
			"/test/method",
			"req",
			nil,
			func(_ context.Context, _ string, _ interface{}, reply interface{}) error {
				receivedReply = reply
				return nil
			},
		)

		assert.Equal(t, "modified-reply", receivedReply)
	})
}

// TestChainStreamClientInterceptors tests ChainStreamClientInterceptors function
func TestChainStreamClientInterceptors(t *testing.T) {
	t.Run("empty chain returns passthrough", func(t *testing.T) {
		chain := ChainStreamClientInterceptors("test-service", []string{})

		assert.NotNil(t, chain)

		streamerCalled := false
		_, err := chain(
			context.Background(),
			&stream.Desc{},
			"/test/method",
			func(context.Context, *stream.Desc, string) (stream.ClientStream, error) {
				streamerCalled = true
				return nil, nil
			},
		)

		assert.True(t, streamerCalled)
		assert.NoError(t, err)
	})

	t.Run("single interceptor", func(*testing.T) {
		RegisterStreamClientIntBuilder(
			"single-stream",
			func(_ string) StreamClientInterceptor {
				return func(ctx context.Context, desc *stream.Desc, method string, streamer Streamer) (stream.ClientStream, error) {
					return streamer(ctx, desc, method)
				}
			},
		)

		chain := ChainStreamClientInterceptors("test-service", []string{"single-stream"})

		streamerCalled := false
		_, err := chain(
			context.Background(),
			&stream.Desc{},
			"/test/method",
			func(context.Context, *stream.Desc, string) (stream.ClientStream, error) {
				streamerCalled = true
				return nil, nil
			},
		)

		assert.True(t, streamerCalled)
		assert.NoError(t, err)
	})

	t.Run("multiple interceptors in order", func(*testing.T) {
		callOrder := []string{}
		RegisterStreamClientIntBuilder(
			"stream-first",
			func(string) StreamClientInterceptor {
				return func(ctx context.Context, desc *stream.Desc, method string, streamer Streamer) (stream.ClientStream, error) {
					callOrder = append(callOrder, "stream-first")
					return streamer(ctx, desc, method)
				}
			},
		)
		RegisterStreamClientIntBuilder(
			"stream-second",
			func(_ string) StreamClientInterceptor {
				return func(ctx context.Context, desc *stream.Desc, method string, streamer Streamer) (stream.ClientStream, error) {
					callOrder = append(callOrder, "stream-second")
					return streamer(ctx, desc, method)
				}
			},
		)

		chain := ChainStreamClientInterceptors(
			"test-service",
			[]string{"stream-first", "stream-second"},
		)

		_, _ = chain(
			context.Background(),
			&stream.Desc{},
			"/test/method",
			func(context.Context, *stream.Desc, string) (stream.ClientStream, error) {
				callOrder = append(callOrder, "streamer")
				return nil, nil
			},
		)

		assert.Equal(t, []string{"stream-first", "stream-second", "streamer"}, callOrder)
	})
}

// TestChainUnaryServerInterceptors tests ChainUnaryServerInterceptors function
func TestChainUnaryServerInterceptors(t *testing.T) {
	t.Run("empty chain returns passthrough", func(t *testing.T) {
		chain := ChainUnaryServerInterceptors([]string{})

		assert.NotNil(t, chain)

		info := &UnaryServerInfo{FullMethod: "/test/service/method"}
		handlerCalled := false
		resp, err := chain(
			context.Background(),
			"req",
			info,
			func(_ context.Context, _ any) (any, error) {
				handlerCalled = true
				return "resp", nil
			},
		)

		assert.True(t, handlerCalled)
		assert.Equal(t, "resp", resp)
		assert.NoError(t, err)
	})

	t.Run("single interceptor", func(t *testing.T) {
		RegisterUnaryServerIntBuilder("single-server", func() UnaryServerInterceptor {
			return func(ctx context.Context, req interface{}, _ *UnaryServerInfo, handler UnaryHandler) (any, error) {
				return handler(ctx, req)
			}
		})

		chain := ChainUnaryServerInterceptors([]string{"single-server"})

		info := &UnaryServerInfo{FullMethod: "/test/service/method"}
		resp, err := chain(
			context.Background(),
			"req",
			info,
			func(_ context.Context, _ any) (any, error) {
				return "resp", nil
			},
		)

		assert.Equal(t, "resp", resp)
		assert.NoError(t, err)
	})

	t.Run("multiple interceptors in order", func(*testing.T) {
		callOrder := []string{}
		RegisterUnaryServerIntBuilder("server-1", func() UnaryServerInterceptor {
			return func(ctx context.Context, req interface{}, _ *UnaryServerInfo, handler UnaryHandler) (any, error) {
				callOrder = append(callOrder, "server-1")
				return handler(ctx, req)
			}
		})
		RegisterUnaryServerIntBuilder("server-2", func() UnaryServerInterceptor {
			return func(ctx context.Context, req interface{}, _ *UnaryServerInfo, handler UnaryHandler) (any, error) {
				callOrder = append(callOrder, "server-2")
				return handler(ctx, req)
			}
		})
		RegisterUnaryServerIntBuilder("server-3", func() UnaryServerInterceptor {
			return func(ctx context.Context, req interface{}, _ *UnaryServerInfo, handler UnaryHandler) (any, error) {
				callOrder = append(callOrder, "server-3")
				return handler(ctx, req)
			}
		})

		chain := ChainUnaryServerInterceptors([]string{"server-1", "server-2", "server-3"})

		info := &UnaryServerInfo{FullMethod: "/test/service/method"}
		_, _ = chain(
			context.Background(),
			"req",
			info,
			func(context.Context, any) (any, error) {
				callOrder = append(callOrder, "handler")
				return nil, nil
			},
		)

		assert.Equal(t, []string{"server-1", "server-2", "server-3", "handler"}, callOrder)
	})

	t.Run("interceptor receives correct info", func(t *testing.T) {
		RegisterUnaryServerIntBuilder("info-test", func() UnaryServerInterceptor {
			return func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (any, error) {
				assert.Equal(t, "/my.service/method", info.FullMethod)
				assert.NotNil(t, info.Server)
				return handler(ctx, req)
			}
		})

		chain := ChainUnaryServerInterceptors([]string{"info-test"})

		info := &UnaryServerInfo{
			Server:     &struct{}{},
			FullMethod: "/my.service/method",
		}
		_, _ = chain(
			context.Background(),
			"req",
			info,
			func(context.Context, any) (any, error) {
				return nil, nil
			},
		)
	})
}

// TestChainStreamServerInterceptors tests ChainStreamServerInterceptors function
func TestChainStreamServerInterceptors(t *testing.T) {
	t.Run("empty chain returns passthrough", func(t *testing.T) {
		chain := ChainStreamServerInterceptors([]string{})

		assert.NotNil(t, chain)

		info := &StreamServerInfo{FullMethod: "/test/service/method"}
		handlerCalled := false
		err := chain(
			&struct{}{},
			&mockServerStream{},
			info,
			func(interface{}, stream.ServerStream) error {
				handlerCalled = true
				return nil
			},
		)

		assert.True(t, handlerCalled)
		assert.NoError(t, err)
	})

	t.Run("single interceptor", func(t *testing.T) {
		RegisterStreamServerIntBuilder("single-stream-server", func() StreamServerInterceptor {
			return func(srv interface{}, ss stream.ServerStream, _ *StreamServerInfo, handler stream.Handler) error {
				return handler(srv, ss)
			}
		})

		chain := ChainStreamServerInterceptors([]string{"single-stream-server"})

		info := &StreamServerInfo{FullMethod: "/test/service/method"}
		err := chain(
			&struct{}{},
			&mockServerStream{},
			info,
			func(interface{}, stream.ServerStream) error {
				return nil
			},
		)

		assert.NoError(t, err)
	})

	t.Run("multiple interceptors in order", func(t *testing.T) {
		callOrder := []string{}
		RegisterStreamServerIntBuilder("stream-server-1", func() StreamServerInterceptor {
			return func(srv interface{}, ss stream.ServerStream, _ *StreamServerInfo, handler stream.Handler) error {
				callOrder = append(callOrder, "stream-server-1")
				return handler(srv, ss)
			}
		})
		RegisterStreamServerIntBuilder("stream-server-2", func() StreamServerInterceptor {
			return func(srv interface{}, ss stream.ServerStream, _ *StreamServerInfo, handler stream.Handler) error {
				callOrder = append(callOrder, "stream-server-2")
				return handler(srv, ss)
			}
		})

		chain := ChainStreamServerInterceptors([]string{"stream-server-1", "stream-server-2"})

		info := &StreamServerInfo{FullMethod: "/test/service/method"}
		_ = chain(
			&struct{}{},
			&mockServerStream{},
			info,
			func(interface{}, stream.ServerStream) error {
				callOrder = append(callOrder, "handler")
				return nil
			},
		)

		assert.Equal(t, []string{"stream-server-1", "stream-server-2", "handler"}, callOrder)
	})

	t.Run("interceptor receives correct stream info", func(t *testing.T) {
		RegisterStreamServerIntBuilder("stream-info-test", func() StreamServerInterceptor {
			return func(srv interface{}, ss stream.ServerStream, info *StreamServerInfo, handler stream.Handler) error {
				assert.Equal(t, "/my.service/stream", info.FullMethod)
				assert.True(t, info.IsClientStream)
				assert.False(t, info.IsServerStream)
				return handler(srv, ss)
			}
		})

		chain := ChainStreamServerInterceptors([]string{"stream-info-test"})

		info := &StreamServerInfo{
			FullMethod:     "/my.service/stream",
			IsClientStream: true,
			IsServerStream: false,
		}
		_ = chain(
			&struct{}{},
			&mockServerStream{},
			info,
			func(interface{}, stream.ServerStream) error {
				return nil
			},
		)
	})
}

// TestConcurrentRegistration tests thread safety of registration functions
func TestConcurrentRegistration(t *testing.T) {
	t.Run("concurrent unary client registration", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				name := fmt.Sprintf("concurrent-unary-client-%d", idx)
				RegisterUnaryClientIntBuilder(
					name,
					func(string) UnaryClientInterceptor {
						return func(ctx context.Context, method string, req, reply any, invoker UnaryInvoker) error {
							return invoker(ctx, method, req, reply)
						}
					},
				)
			}(i)
		}
		wg.Wait()

		// Verify all builders are registered
		for i := 0; i < 100; i++ {
			name := fmt.Sprintf("concurrent-unary-client-%d", i)
			builder := getUnaryClientIntBuilder(name)
			assert.NotNil(t, builder, "builder %s should be registered", name)
		}
	})

	t.Run("concurrent unary server registration", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				name := fmt.Sprintf("concurrent-unary-server-%d", idx)
				RegisterUnaryServerIntBuilder(name, func() UnaryServerInterceptor {
					return func(ctx context.Context, req interface{}, _ *UnaryServerInfo, handler UnaryHandler) (any, error) {
						return handler(ctx, req)
					}
				})
			}(i)
		}
		wg.Wait()

		// Verify all builders are registered
		for i := 0; i < 100; i++ {
			name := fmt.Sprintf("concurrent-unary-server-%d", i)
			builder := getUnaryServerIntBuilder(name)
			assert.NotNil(t, builder, "builder %s should be registered", name)
		}
	})
}

// TestGetBuilders tests get* functions that retrieve builders
func TestGetBuilders(t *testing.T) {
	t.Run("get non-existent builder returns nil", func(t *testing.T) {
		unaryClientBuilder := getUnaryClientIntBuilder("non-existent")
		assert.Nil(t, unaryClientBuilder)

		unaryServerBuilder := getUnaryServerIntBuilder("non-existent")
		assert.Nil(t, unaryServerBuilder)

		streamClientBuilder := getStreamClientIntBuilder("non-existent")
		assert.Nil(t, streamClientBuilder)

		streamServerBuilder := getStreamServerIntBuilder("non-existent")
		assert.Nil(t, streamServerBuilder)
	})
}

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
