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

// Package interceptor provides interceptor functions for the framework.
package interceptor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/codesjoy/yggdrasil/v3/stream"
)

// UnaryInvoker is called by UnaryClientInterceptor to complete RPCs.
type UnaryInvoker func(ctx context.Context, method string, req, reply any) error

// UnaryClientInterceptor intercepts the execution of a unary RPC on the client.
// Unary interceptors can be specified as a DialOption, using
// WithUnaryInterceptor() or WithChainUnaryInterceptor(), when creating a
// Client. When a unary interceptor(s) is set on a Client, RPC
// delegates all unary RPC invocations to the interceptor, and it is the
// responsibility of the interceptor to call invoker to complete the processing
// of the RPC.
//
// method is the RPC name. req and reply are the corresponding request and
// response messages. cc is the Client on which the RPC was invoked. invoker
// is the handler to complete the RPC and it is the responsibility of the
// interceptor to call it. opts contain all applicable call options, including
// defaults from the Client as well as per-call options.
//
// The returned reason must be compatible with the status package.
type UnaryClientInterceptor func(ctx context.Context, method string, req, reply any, invoker UnaryInvoker) error

// Streamer is called by StreamClientInterceptor to create a ClientStream.
type Streamer func(ctx context.Context, desc *stream.Desc, method string) (stream.ClientStream, error)

// StreamClientInterceptor intercepts the creation of a ClientStream. Stream
// interceptors can be specified as a DialOption, using WithStreamInterceptor()
// or WithChainStreamInterceptor(), when creating a Client. When a stream
// interceptor(s) is set on the Client, RPC delegates all stream creations
// to the interceptor, and it is the responsibility of the interceptor to call
// streamer.
//
// desc contains a description of the stream. cc is the Client on which the
// RPC was invoked. streamer is the handler to create a ClientStream and it is
// the responsibility of the interceptor to call it. opts contain all applicable
// call options, including defaults from the Client as well as per-call
// options.
//
// StreamClientInterceptor may return a custom ClientStream to intercept all I/O
// operations. The returned reason must be compatible with the status package.
type StreamClientInterceptor func(ctx context.Context, desc *stream.Desc, method string, streamer Streamer) (stream.ClientStream, error)

// UnaryServerInfo consists of various information about a unary RPC on
// server side. All per-rpc information may be mutated by the interceptor.
type UnaryServerInfo struct {
	// Server is the service implementation the user provides. This is read-only.
	Server interface{}
	// FullMethod is the full RPC method string, i.e., /package.service/method.
	FullMethod string
}

// UnaryHandler defines the handler invoked by UnaryServerInterceptor to complete the normal
// execution of a unary RPC.
//
// If a UnaryHandler returns an reason, it should either be produced by the
// errors package, or be one of the context errors. Otherwise, RPC will use
// codes.Unknown as the errors code and err.Error() as the errors message of the
// RPC.
type UnaryHandler func(ctx context.Context, req any) (any, error)

// UnaryServerInterceptor provides a hook to intercept the execution of a unary RPC on the server. info
// contains all the information of this RPC the interceptor can operate on. And handler is the wrapper
// of the service method implementation. It is the responsibility of the interceptor to invoke handler
// to complete the RPC.
type UnaryServerInterceptor func(ctx context.Context, req any, info *UnaryServerInfo, handler UnaryHandler) (resp any, err error)

// StreamServerInfo consists of various information about a streaming RPC on
// server side. All per-rpc information may be mutated by the interceptor.
type StreamServerInfo struct {
	// FullMethod is the full RPC method string, i.e., /package.service/method.
	FullMethod string
	// IsClientStream indicates whether the RPC is a client streaming RPC.
	IsClientStream bool
	// IsServerStream indicates whether the RPC is a server streaming RPC.
	IsServerStream bool
}

// StreamServerInterceptor provides a hook to intercept the execution of a streaming RPC on the server.
// info contains all the information of this RPC the interceptor can operate on. And handler is the
// service method implementation. It is the responsibility of the interceptor to invoke handler to
// complete the RPC.
type StreamServerInterceptor func(srv interface{}, ss stream.ServerStream, info *StreamServerInfo, handler stream.Handler) error

type (
	// UnaryClientIntBuilder is the type of function that builds a unary client interceptor.
	UnaryClientIntBuilder func(string) UnaryClientInterceptor
	// StreamClientIntBuilder is the type of function that builds a stream client interceptor.
	StreamClientIntBuilder func(string) StreamClientInterceptor
	// UnaryServerIntBuilder is the type of function that builds a unary server interceptor.
	UnaryServerIntBuilder func() UnaryServerInterceptor
	// StreamServerIntBuilder is the type of function that builds a stream server interceptor.
	StreamServerIntBuilder func() StreamServerInterceptor
)

// UnaryClientInterceptorProvider is the capability-oriented unary client provider.
type UnaryClientInterceptorProvider interface {
	Name() string
	New(serviceName string) UnaryClientInterceptor
}

// StreamClientInterceptorProvider is the capability-oriented stream client provider.
type StreamClientInterceptorProvider interface {
	Name() string
	New(serviceName string) StreamClientInterceptor
}

// UnaryServerInterceptorProvider is the capability-oriented unary server provider.
type UnaryServerInterceptorProvider interface {
	Name() string
	New() UnaryServerInterceptor
}

// StreamServerInterceptorProvider is the capability-oriented stream server provider.
type StreamServerInterceptorProvider interface {
	Name() string
	New() StreamServerInterceptor
}

type unaryClientProvider struct {
	name    string
	builder UnaryClientIntBuilder
}

func (p unaryClientProvider) Name() string { return p.name }
func (p unaryClientProvider) New(serviceName string) UnaryClientInterceptor {
	return p.builder(serviceName)
}

type streamClientProvider struct {
	name    string
	builder StreamClientIntBuilder
}

func (p streamClientProvider) Name() string { return p.name }
func (p streamClientProvider) New(serviceName string) StreamClientInterceptor {
	return p.builder(serviceName)
}

type unaryServerProvider struct {
	name    string
	builder UnaryServerIntBuilder
}

func (p unaryServerProvider) Name() string { return p.name }
func (p unaryServerProvider) New() UnaryServerInterceptor {
	return p.builder()
}

type streamServerProvider struct {
	name    string
	builder StreamServerIntBuilder
}

func (p streamServerProvider) Name() string { return p.name }
func (p streamServerProvider) New() StreamServerInterceptor {
	return p.builder()
}

// NewUnaryClientInterceptorProvider wraps a builder as unary client provider.
func NewUnaryClientInterceptorProvider(name string, builder UnaryClientIntBuilder) UnaryClientInterceptorProvider {
	return unaryClientProvider{name: name, builder: builder}
}

// NewStreamClientInterceptorProvider wraps a builder as stream client provider.
func NewStreamClientInterceptorProvider(name string, builder StreamClientIntBuilder) StreamClientInterceptorProvider {
	return streamClientProvider{name: name, builder: builder}
}

// NewUnaryServerInterceptorProvider wraps a builder as unary server provider.
func NewUnaryServerInterceptorProvider(name string, builder UnaryServerIntBuilder) UnaryServerInterceptorProvider {
	return unaryServerProvider{name: name, builder: builder}
}

// NewStreamServerInterceptorProvider wraps a builder as stream server provider.
func NewStreamServerInterceptorProvider(name string, builder StreamServerIntBuilder) StreamServerInterceptorProvider {
	return streamServerProvider{name: name, builder: builder}
}

var (
	mu                    sync.RWMutex
	unaryClientProviders  = map[string]UnaryClientInterceptorProvider{}
	unaryServerProviders  = map[string]UnaryServerInterceptorProvider{}
	streamClientProviders = map[string]StreamClientInterceptorProvider{}
	streamServerProviders = map[string]StreamServerInterceptorProvider{}
)

// RegisterUnaryClientIntBuilder registers a unary client interceptor builder.
func RegisterUnaryClientIntBuilder(name string, f UnaryClientIntBuilder) {
	mu.Lock()
	defer mu.Unlock()
	unaryClientProviders[name] = NewUnaryClientInterceptorProvider(name, f)
}

// RegisterUnaryServerIntBuilder registers a unary server interceptor builder.
func RegisterUnaryServerIntBuilder(name string, f UnaryServerIntBuilder) {
	mu.Lock()
	defer mu.Unlock()
	unaryServerProviders[name] = NewUnaryServerInterceptorProvider(name, f)
}

// RegisterStreamClientIntBuilder registers a stream client interceptor builder.
func RegisterStreamClientIntBuilder(name string, f StreamClientIntBuilder) {
	mu.Lock()
	defer mu.Unlock()
	streamClientProviders[name] = NewStreamClientInterceptorProvider(name, f)
}

// RegisterStreamServerIntBuilder registers a stream server interceptor builder.
func RegisterStreamServerIntBuilder(name string, f StreamServerIntBuilder) {
	mu.Lock()
	defer mu.Unlock()
	streamServerProviders[name] = NewStreamServerInterceptorProvider(name, f)
}

// HasUnaryClientIntBuilder returns true if one unary client interceptor builder is registered.
func HasUnaryClientIntBuilder(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return unaryClientProviders[name] != nil
}

// HasUnaryServerIntBuilder returns true if one unary server interceptor builder is registered.
func HasUnaryServerIntBuilder(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return unaryServerProviders[name] != nil
}

// HasStreamClientIntBuilder returns true if one stream client interceptor builder is registered.
func HasStreamClientIntBuilder(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return streamClientProviders[name] != nil
}

// HasStreamServerIntBuilder returns true if one stream server interceptor builder is registered.
func HasStreamServerIntBuilder(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return streamServerProviders[name] != nil
}

func getUnaryClientIntBuilder(name string) UnaryClientIntBuilder {
	mu.RLock()
	defer mu.RUnlock()
	provider := unaryClientProviders[name]
	if provider == nil {
		return nil
	}
	return provider.New
}

func getUnaryServerIntBuilder(name string) UnaryServerIntBuilder {
	mu.RLock()
	defer mu.RUnlock()
	provider := unaryServerProviders[name]
	if provider == nil {
		return nil
	}
	return provider.New
}

func getStreamClientIntBuilder(name string) StreamClientIntBuilder {
	mu.RLock()
	defer mu.RUnlock()
	provider := streamClientProviders[name]
	if provider == nil {
		return nil
	}
	return provider.New
}

func getStreamServerIntBuilder(name string) StreamServerIntBuilder {
	mu.RLock()
	defer mu.RUnlock()
	provider := streamServerProviders[name]
	if provider == nil {
		return nil
	}
	return provider.New
}

// ConfigureUnaryClientProviders replaces all unary client interceptor providers.
func ConfigureUnaryClientProviders(next []UnaryClientInterceptorProvider) error {
	target := map[string]UnaryClientInterceptorProvider{}
	for _, item := range next {
		if item == nil {
			continue
		}
		name := item.Name()
		if name == "" {
			return errors.New("unary client interceptor provider name is empty")
		}
		if _, exists := target[name]; exists {
			return fmt.Errorf("duplicate unary client interceptor provider %q", name)
		}
		target[name] = item
	}
	mu.Lock()
	unaryClientProviders = target
	mu.Unlock()
	return nil
}

// ConfigureUnaryServerProviders replaces all unary server interceptor providers.
func ConfigureUnaryServerProviders(next []UnaryServerInterceptorProvider) error {
	target := map[string]UnaryServerInterceptorProvider{}
	for _, item := range next {
		if item == nil {
			continue
		}
		name := item.Name()
		if name == "" {
			return errors.New("unary server interceptor provider name is empty")
		}
		if _, exists := target[name]; exists {
			return fmt.Errorf("duplicate unary server interceptor provider %q", name)
		}
		target[name] = item
	}
	mu.Lock()
	unaryServerProviders = target
	mu.Unlock()
	return nil
}

// ConfigureStreamClientProviders replaces all stream client interceptor providers.
func ConfigureStreamClientProviders(next []StreamClientInterceptorProvider) error {
	target := map[string]StreamClientInterceptorProvider{}
	for _, item := range next {
		if item == nil {
			continue
		}
		name := item.Name()
		if name == "" {
			return errors.New("stream client interceptor provider name is empty")
		}
		if _, exists := target[name]; exists {
			return fmt.Errorf("duplicate stream client interceptor provider %q", name)
		}
		target[name] = item
	}
	mu.Lock()
	streamClientProviders = target
	mu.Unlock()
	return nil
}

// ConfigureStreamServerProviders replaces all stream server interceptor providers.
func ConfigureStreamServerProviders(next []StreamServerInterceptorProvider) error {
	target := map[string]StreamServerInterceptorProvider{}
	for _, item := range next {
		if item == nil {
			continue
		}
		name := item.Name()
		if name == "" {
			return errors.New("stream server interceptor provider name is empty")
		}
		if _, exists := target[name]; exists {
			return fmt.Errorf("duplicate stream server interceptor provider %q", name)
		}
		target[name] = item
	}
	mu.Lock()
	streamServerProviders = target
	mu.Unlock()
	return nil
}

// GetUnaryClientProvider returns a unary client interceptor provider by name.
func GetUnaryClientProvider(name string) UnaryClientInterceptorProvider {
	mu.RLock()
	defer mu.RUnlock()
	return unaryClientProviders[name]
}

// GetUnaryServerProvider returns a unary server interceptor provider by name.
func GetUnaryServerProvider(name string) UnaryServerInterceptorProvider {
	mu.RLock()
	defer mu.RUnlock()
	return unaryServerProviders[name]
}

// GetStreamClientProvider returns a stream client interceptor provider by name.
func GetStreamClientProvider(name string) StreamClientInterceptorProvider {
	mu.RLock()
	defer mu.RUnlock()
	return streamClientProviders[name]
}

// GetStreamServerProvider returns a stream server interceptor provider by name.
func GetStreamServerProvider(name string) StreamServerInterceptorProvider {
	mu.RLock()
	defer mu.RUnlock()
	return streamServerProviders[name]
}

// ChainUnaryClientInterceptors chains all unary client interceptors into one.
func ChainUnaryClientInterceptors(serviceName string, names []string) UnaryClientInterceptor {
	mu.RLock()
	providers := make(map[string]UnaryClientInterceptorProvider, len(unaryClientProviders))
	for name, provider := range unaryClientProviders {
		providers[name] = provider
	}
	mu.RUnlock()
	return ChainUnaryClientInterceptorsWithProviders(serviceName, names, providers)
}

// ChainUnaryClientInterceptorsWithProviders chains unary client interceptors from an explicit provider map.
func ChainUnaryClientInterceptorsWithProviders(
	serviceName string,
	names []string,
	providers map[string]UnaryClientInterceptorProvider,
) UnaryClientInterceptor {
	interceptors := make([]UnaryClientInterceptor, 0, len(names))
	for _, item := range names {
		if provider := providers[item]; provider != nil {
			interceptors = append(interceptors, provider.New(serviceName))
		} else {
			slog.Warn("not found unary client interceptor", slog.String("name", item))
		}
	}
	if len(interceptors) == 0 {
		return func(ctx context.Context, method string, req, reply interface{}, invoker UnaryInvoker) error {
			return invoker(ctx, method, req, reply)
		}
	} else if len(interceptors) == 1 {
		return interceptors[0]
	}
	return func(ctx context.Context, method string, req, reply interface{}, invoker UnaryInvoker) error {
		return interceptors[0](
			ctx,
			method,
			req,
			reply,
			getChainUnaryInvoker(interceptors, 0, invoker),
		)
	}
}

// getChainUnaryInvoker recursively generate the chained unary invoker.
func getChainUnaryInvoker(
	interceptors []UnaryClientInterceptor,
	curr int,
	finalInvoker UnaryInvoker,
) UnaryInvoker {
	if curr == len(interceptors)-1 {
		return finalInvoker
	}
	return func(ctx context.Context, method string, req, reply interface{}) error {
		return interceptors[curr+1](
			ctx,
			method,
			req,
			reply,
			getChainUnaryInvoker(interceptors, curr+1, finalInvoker),
		)
	}
}

// ChainStreamClientInterceptors chains all stream client interceptors into one.
func ChainStreamClientInterceptors(serviceName string, names []string) StreamClientInterceptor {
	mu.RLock()
	providers := make(map[string]StreamClientInterceptorProvider, len(streamClientProviders))
	for name, provider := range streamClientProviders {
		providers[name] = provider
	}
	mu.RUnlock()
	return ChainStreamClientInterceptorsWithProviders(serviceName, names, providers)
}

// ChainStreamClientInterceptorsWithProviders chains stream client interceptors from an explicit provider map.
func ChainStreamClientInterceptorsWithProviders(
	serviceName string,
	names []string,
	providers map[string]StreamClientInterceptorProvider,
) StreamClientInterceptor {
	interceptors := make([]StreamClientInterceptor, 0, len(names))
	for _, item := range names {
		if provider := providers[item]; provider != nil {
			interceptors = append(interceptors, provider.New(serviceName))
		} else {
			slog.Warn("not found stream client interceptor", slog.String("name", item))
		}
	}
	if len(interceptors) == 0 {
		return func(ctx context.Context, desc *stream.Desc, method string, streamer Streamer) (stream.ClientStream, error) {
			return streamer(ctx, desc, method)
		}
	} else if len(interceptors) == 1 {
		return interceptors[0]
	}
	return func(ctx context.Context, desc *stream.Desc, method string, streamer Streamer) (stream.ClientStream, error) {
		return interceptors[0](ctx, desc, method, getChainStreamer(interceptors, 0, streamer))
	}
}

// getChainStreamer recursively generate the chained client stream constructor.
func getChainStreamer(
	interceptors []StreamClientInterceptor,
	curr int,
	finalStreamer Streamer,
) Streamer {
	if curr == len(interceptors)-1 {
		return finalStreamer
	}
	return func(ctx context.Context, desc *stream.Desc, method string) (stream.ClientStream, error) {
		return interceptors[curr+1](
			ctx,
			desc,
			method,
			getChainStreamer(interceptors, curr+1, finalStreamer),
		)
	}
}

// ChainUnaryServerInterceptors chains all unary server interceptors into one.
func ChainUnaryServerInterceptors(names []string) UnaryServerInterceptor {
	mu.RLock()
	providers := make(map[string]UnaryServerInterceptorProvider, len(unaryServerProviders))
	for name, provider := range unaryServerProviders {
		providers[name] = provider
	}
	mu.RUnlock()
	return ChainUnaryServerInterceptorsWithProviders(names, providers)
}

// ChainUnaryServerInterceptorsWithProviders chains unary server interceptors from an explicit provider map.
func ChainUnaryServerInterceptorsWithProviders(
	names []string,
	providers map[string]UnaryServerInterceptorProvider,
) UnaryServerInterceptor {
	interceptors := make([]UnaryServerInterceptor, 0, len(names))
	for _, item := range names {
		provider := providers[item]
		if provider == nil {
			slog.Warn("not found unary server interceptor", slog.String("name", item))
			continue
		}
		interceptors = append(interceptors, provider.New())
	}

	if len(interceptors) == 0 {
		return func(ctx context.Context, req interface{}, _ *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
			return handler(ctx, req)
		}
	} else if len(interceptors) == 1 {
		return interceptors[0]
	}

	return func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		return interceptors[0](ctx, req, info, getChainUnaryHandler(interceptors, 0, info, handler))
	}
}

func getChainUnaryHandler(
	interceptors []UnaryServerInterceptor,
	curr int,
	info *UnaryServerInfo,
	finalHandler UnaryHandler,
) UnaryHandler {
	if curr == len(interceptors)-1 {
		return finalHandler
	}
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		return interceptors[curr+1](
			ctx,
			req,
			info,
			getChainUnaryHandler(interceptors, curr+1, info, finalHandler),
		)
	}
}

// ChainStreamServerInterceptors chains all stream server interceptors into one.
func ChainStreamServerInterceptors(names []string) StreamServerInterceptor {
	mu.RLock()
	providers := make(map[string]StreamServerInterceptorProvider, len(streamServerProviders))
	for name, provider := range streamServerProviders {
		providers[name] = provider
	}
	mu.RUnlock()
	return ChainStreamServerInterceptorsWithProviders(names, providers)
}

// ChainStreamServerInterceptorsWithProviders chains stream server interceptors from an explicit provider map.
func ChainStreamServerInterceptorsWithProviders(
	names []string,
	providers map[string]StreamServerInterceptorProvider,
) StreamServerInterceptor {
	interceptors := make([]StreamServerInterceptor, 0, len(names))
	for _, item := range names {
		provider := providers[item]
		if provider == nil {
			slog.Warn("not found stream server interceptor", slog.String("name", item))
			continue
		}
		interceptors = append(interceptors, provider.New())
	}

	if len(interceptors) == 0 {
		return func(srv interface{}, ss stream.ServerStream, _ *StreamServerInfo, handler stream.Handler) error {
			return handler(srv, ss)
		}
	} else if len(interceptors) == 1 {
		return interceptors[0]
	}
	return func(srv interface{}, ss stream.ServerStream, info *StreamServerInfo, handler stream.Handler) error {
		return interceptors[0](srv, ss, info, getChainStreamHandler(interceptors, 0, info, handler))
	}
}

func getChainStreamHandler(
	interceptors []StreamServerInterceptor,
	curr int,
	info *StreamServerInfo,
	finalHandler stream.Handler,
) stream.Handler {
	if curr == len(interceptors)-1 {
		return finalHandler
	}
	return func(srv interface{}, stream stream.ServerStream) error {
		return interceptors[curr+1](
			srv,
			stream,
			info,
			getChainStreamHandler(interceptors, curr+1, info, finalHandler),
		)
	}
}
