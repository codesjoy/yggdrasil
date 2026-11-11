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
	"fmt"
	"log/slog"
	"sync"

	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
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

type clientProvider[I any] struct {
	name    string
	builder func(string) I
}

func (p clientProvider[I]) Name() string { return p.name }
func (p clientProvider[I]) New(serviceName string) I {
	return p.builder(serviceName)
}

type serverProvider[I any] struct {
	name    string
	builder func() I
}

func (p serverProvider[I]) Name() string { return p.name }
func (p serverProvider[I]) New() I {
	return p.builder()
}

// NewUnaryClientInterceptorProvider wraps a builder as unary client provider.
func NewUnaryClientInterceptorProvider(
	name string,
	builder UnaryClientIntBuilder,
) UnaryClientInterceptorProvider {
	return clientProvider[UnaryClientInterceptor]{name: name, builder: builder}
}

// NewStreamClientInterceptorProvider wraps a builder as stream client provider.
func NewStreamClientInterceptorProvider(
	name string,
	builder StreamClientIntBuilder,
) StreamClientInterceptorProvider {
	return clientProvider[StreamClientInterceptor]{name: name, builder: builder}
}

// NewUnaryServerInterceptorProvider wraps a builder as unary server provider.
func NewUnaryServerInterceptorProvider(
	name string,
	builder UnaryServerIntBuilder,
) UnaryServerInterceptorProvider {
	return serverProvider[UnaryServerInterceptor]{name: name, builder: builder}
}

// NewStreamServerInterceptorProvider wraps a builder as stream server provider.
func NewStreamServerInterceptorProvider(
	name string,
	builder StreamServerIntBuilder,
) StreamServerInterceptorProvider {
	return serverProvider[StreamServerInterceptor]{name: name, builder: builder}
}

var (
	mu                    sync.RWMutex
	unaryClientProviders  = map[string]UnaryClientInterceptorProvider{}
	unaryServerProviders  = map[string]UnaryServerInterceptorProvider{}
	streamClientProviders = map[string]StreamClientInterceptorProvider{}
	streamServerProviders = map[string]StreamServerInterceptorProvider{}
)

func configureProviderMap[P interface{ Name() string }](
	next []P,
	kind string,
) (map[string]P, error) {
	target := make(map[string]P, len(next))
	for _, item := range next {
		if any(item) == nil {
			continue
		}
		name := item.Name()
		if name == "" {
			return nil, fmt.Errorf("%s interceptor provider name is empty", kind)
		}
		if _, exists := target[name]; exists {
			return nil, fmt.Errorf("duplicate %s interceptor provider %q", kind, name)
		}
		target[name] = item
	}
	return target, nil
}

// ConfigureUnaryClientProviders replaces all unary client interceptor providers.
func ConfigureUnaryClientProviders(next []UnaryClientInterceptorProvider) error {
	target, err := configureProviderMap(next, "unary client")
	if err != nil {
		return err
	}
	mu.Lock()
	unaryClientProviders = target
	mu.Unlock()
	return nil
}

// ConfigureUnaryServerProviders replaces all unary server interceptor providers.
func ConfigureUnaryServerProviders(next []UnaryServerInterceptorProvider) error {
	target, err := configureProviderMap(next, "unary server")
	if err != nil {
		return err
	}
	mu.Lock()
	unaryServerProviders = target
	mu.Unlock()
	return nil
}

// ConfigureStreamClientProviders replaces all stream client interceptor providers.
func ConfigureStreamClientProviders(next []StreamClientInterceptorProvider) error {
	target, err := configureProviderMap(next, "stream client")
	if err != nil {
		return err
	}
	mu.Lock()
	streamClientProviders = target
	mu.Unlock()
	return nil
}

// ConfigureStreamServerProviders replaces all stream server interceptor providers.
func ConfigureStreamServerProviders(next []StreamServerInterceptorProvider) error {
	target, err := configureProviderMap(next, "stream server")
	if err != nil {
		return err
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

func buildInterceptorList[P interface{ Name() string }, I any](
	names []string,
	providers map[string]P,
	kind string,
	isProviderNil func(P) bool,
	build func(P) I,
	isNil func(I) bool,
) ([]I, error) {
	out := make([]I, 0, len(names))
	for _, name := range names {
		provider, ok := providers[name]
		if !ok || isProviderNil(provider) {
			return nil, fmt.Errorf("%s interceptor provider %q not found", kind, name)
		}
		built := build(provider)
		if isNil(built) {
			return nil, fmt.Errorf("%s interceptor provider %q returned nil", kind, name)
		}
		out = append(out, built)
	}
	return out, nil
}

func buildInterceptorListLenient[P interface{ Name() string }, I any](
	names []string,
	providers map[string]P,
	kind string,
	build func(P) I,
) []I {
	out := make([]I, 0, len(names))
	for _, name := range names {
		provider, ok := providers[name]
		if !ok || any(provider) == nil {
			slog.Warn("not found "+kind+" interceptor", slog.String("name", name))
			continue
		}
		out = append(out, build(provider))
	}
	return out
}

// ChainUnaryClientInterceptors chains all unary client interceptors into one.
//
// Deprecated: use ChainUnaryClientInterceptorsE to observe missing providers.
func ChainUnaryClientInterceptors(serviceName string, names []string) UnaryClientInterceptor {
	mu.RLock()
	providers := unaryClientProviders
	mu.RUnlock()
	return ChainUnaryClientInterceptorsWithProviders(serviceName, names, providers)
}

// ChainUnaryClientInterceptorsE chains unary client interceptors or returns a
// configuration error for a missing provider.
func ChainUnaryClientInterceptorsE(
	serviceName string,
	names []string,
) (UnaryClientInterceptor, error) {
	mu.RLock()
	providers := unaryClientProviders
	mu.RUnlock()
	return ChainUnaryClientInterceptorsWithProvidersE(serviceName, names, providers)
}

// ChainUnaryClientInterceptorsWithProviders chains unary client interceptors from an explicit provider map.
func ChainUnaryClientInterceptorsWithProviders(
	serviceName string,
	names []string,
	providers map[string]UnaryClientInterceptorProvider,
) UnaryClientInterceptor {
	return chainUnaryClientInterceptors(buildInterceptorListLenient(
		names,
		providers,
		"unary client",
		func(provider UnaryClientInterceptorProvider) UnaryClientInterceptor {
			return provider.New(serviceName)
		},
	))
}

// ChainUnaryClientInterceptorsWithProvidersE chains unary client interceptors
// and rejects missing providers.
func ChainUnaryClientInterceptorsWithProvidersE(
	serviceName string,
	names []string,
	providers map[string]UnaryClientInterceptorProvider,
) (UnaryClientInterceptor, error) {
	interceptors, err := buildInterceptorList(
		names,
		providers,
		"unary client",
		func(provider UnaryClientInterceptorProvider) bool { return provider == nil },
		func(provider UnaryClientInterceptorProvider) UnaryClientInterceptor {
			return provider.New(serviceName)
		},
		func(interceptor UnaryClientInterceptor) bool { return interceptor == nil },
	)
	if err != nil {
		return nil, err
	}
	return chainUnaryClientInterceptors(interceptors), nil
}

func chainUnaryClientInterceptors(
	interceptors []UnaryClientInterceptor,
) UnaryClientInterceptor {
	if len(interceptors) == 0 {
		return func(
			ctx context.Context,
			method string,
			req, reply interface{},
			invoker UnaryInvoker,
		) error {
			return invoker(ctx, method, req, reply)
		}
	}
	if len(interceptors) == 1 {
		return interceptors[0]
	}
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		invoker UnaryInvoker,
	) error {
		next := invoker
		for i := len(interceptors) - 1; i >= 0; i-- {
			current := interceptors[i]
			downstream := next
			next = func(ctx context.Context, method string, req, reply any) error {
				return current(ctx, method, req, reply, downstream)
			}
		}
		return next(ctx, method, req, reply)
	}
}

// ChainStreamClientInterceptors chains all stream client interceptors into one.
//
// Deprecated: use ChainStreamClientInterceptorsE to observe missing providers.
func ChainStreamClientInterceptors(serviceName string, names []string) StreamClientInterceptor {
	mu.RLock()
	providers := streamClientProviders
	mu.RUnlock()
	return ChainStreamClientInterceptorsWithProviders(serviceName, names, providers)
}

// ChainStreamClientInterceptorsE chains stream client interceptors or returns a
// configuration error for a missing provider.
func ChainStreamClientInterceptorsE(
	serviceName string,
	names []string,
) (StreamClientInterceptor, error) {
	mu.RLock()
	providers := streamClientProviders
	mu.RUnlock()
	return ChainStreamClientInterceptorsWithProvidersE(serviceName, names, providers)
}

// ChainStreamClientInterceptorsWithProviders chains stream client interceptors from an explicit provider map.
func ChainStreamClientInterceptorsWithProviders(
	serviceName string,
	names []string,
	providers map[string]StreamClientInterceptorProvider,
) StreamClientInterceptor {
	return chainStreamClientInterceptors(buildInterceptorListLenient(
		names,
		providers,
		"stream client",
		func(provider StreamClientInterceptorProvider) StreamClientInterceptor {
			return provider.New(serviceName)
		},
	))
}

// ChainStreamClientInterceptorsWithProvidersE chains stream client
// interceptors and rejects missing providers.
func ChainStreamClientInterceptorsWithProvidersE(
	serviceName string,
	names []string,
	providers map[string]StreamClientInterceptorProvider,
) (StreamClientInterceptor, error) {
	interceptors, err := buildInterceptorList(
		names,
		providers,
		"stream client",
		func(provider StreamClientInterceptorProvider) bool { return provider == nil },
		func(provider StreamClientInterceptorProvider) StreamClientInterceptor {
			return provider.New(serviceName)
		},
		func(interceptor StreamClientInterceptor) bool { return interceptor == nil },
	)
	if err != nil {
		return nil, err
	}
	return chainStreamClientInterceptors(interceptors), nil
}

func chainStreamClientInterceptors(
	interceptors []StreamClientInterceptor,
) StreamClientInterceptor {
	if len(interceptors) == 0 {
		return func(
			ctx context.Context,
			desc *stream.Desc,
			method string,
			streamer Streamer,
		) (stream.ClientStream, error) {
			return streamer(ctx, desc, method)
		}
	}
	if len(interceptors) == 1 {
		return interceptors[0]
	}
	return func(
		ctx context.Context,
		desc *stream.Desc,
		method string,
		streamer Streamer,
	) (stream.ClientStream, error) {
		next := streamer
		for i := len(interceptors) - 1; i >= 0; i-- {
			current := interceptors[i]
			downstream := next
			next = func(
				ctx context.Context,
				desc *stream.Desc,
				method string,
			) (stream.ClientStream, error) {
				return current(ctx, desc, method, downstream)
			}
		}
		return next(ctx, desc, method)
	}
}

// ChainUnaryServerInterceptors chains all unary server interceptors into one.
//
// Deprecated: use ChainUnaryServerInterceptorsE to observe missing providers.
func ChainUnaryServerInterceptors(names []string) UnaryServerInterceptor {
	mu.RLock()
	providers := unaryServerProviders
	mu.RUnlock()
	return ChainUnaryServerInterceptorsWithProviders(names, providers)
}

// ChainUnaryServerInterceptorsE chains unary server interceptors or returns a
// configuration error for a missing provider.
func ChainUnaryServerInterceptorsE(
	names []string,
) (UnaryServerInterceptor, error) {
	mu.RLock()
	providers := unaryServerProviders
	mu.RUnlock()
	return ChainUnaryServerInterceptorsWithProvidersE(names, providers)
}

// ChainUnaryServerInterceptorsWithProviders chains unary server interceptors from an explicit provider map.
func ChainUnaryServerInterceptorsWithProviders(
	names []string,
	providers map[string]UnaryServerInterceptorProvider,
) UnaryServerInterceptor {
	return chainUnaryServerInterceptors(buildInterceptorListLenient(
		names,
		providers,
		"unary server",
		func(provider UnaryServerInterceptorProvider) UnaryServerInterceptor {
			return provider.New()
		},
	))
}

// ChainUnaryServerInterceptorsWithProvidersE chains unary server interceptors
// and rejects missing providers.
func ChainUnaryServerInterceptorsWithProvidersE(
	names []string,
	providers map[string]UnaryServerInterceptorProvider,
) (UnaryServerInterceptor, error) {
	interceptors, err := buildInterceptorList(
		names,
		providers,
		"unary server",
		func(provider UnaryServerInterceptorProvider) bool { return provider == nil },
		func(provider UnaryServerInterceptorProvider) UnaryServerInterceptor {
			return provider.New()
		},
		func(interceptor UnaryServerInterceptor) bool { return interceptor == nil },
	)
	if err != nil {
		return nil, err
	}
	return chainUnaryServerInterceptors(interceptors), nil
}

func chainUnaryServerInterceptors(
	interceptors []UnaryServerInterceptor,
) UnaryServerInterceptor {
	if len(interceptors) == 0 {
		return func(
			ctx context.Context,
			req interface{},
			_ *UnaryServerInfo,
			handler UnaryHandler,
		) (interface{}, error) {
			return handler(ctx, req)
		}
	}
	if len(interceptors) == 1 {
		return interceptors[0]
	}
	return func(
		ctx context.Context,
		req interface{},
		info *UnaryServerInfo,
		handler UnaryHandler,
	) (interface{}, error) {
		next := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			current := interceptors[i]
			downstream := next
			next = func(ctx context.Context, req any) (any, error) {
				return current(ctx, req, info, downstream)
			}
		}
		return next(ctx, req)
	}
}

// ChainStreamServerInterceptors chains all stream server interceptors into one.
//
// Deprecated: use ChainStreamServerInterceptorsE to observe missing providers.
func ChainStreamServerInterceptors(names []string) StreamServerInterceptor {
	mu.RLock()
	providers := streamServerProviders
	mu.RUnlock()
	return ChainStreamServerInterceptorsWithProviders(names, providers)
}

// ChainStreamServerInterceptorsE chains stream server interceptors or returns a
// configuration error for a missing provider.
func ChainStreamServerInterceptorsE(
	names []string,
) (StreamServerInterceptor, error) {
	mu.RLock()
	providers := streamServerProviders
	mu.RUnlock()
	return ChainStreamServerInterceptorsWithProvidersE(names, providers)
}

// ChainStreamServerInterceptorsWithProviders chains stream server interceptors from an explicit provider map.
func ChainStreamServerInterceptorsWithProviders(
	names []string,
	providers map[string]StreamServerInterceptorProvider,
) StreamServerInterceptor {
	return chainStreamServerInterceptors(buildInterceptorListLenient(
		names,
		providers,
		"stream server",
		func(provider StreamServerInterceptorProvider) StreamServerInterceptor {
			return provider.New()
		},
	))
}

// ChainStreamServerInterceptorsWithProvidersE chains stream server
// interceptors and rejects missing providers.
func ChainStreamServerInterceptorsWithProvidersE(
	names []string,
	providers map[string]StreamServerInterceptorProvider,
) (StreamServerInterceptor, error) {
	interceptors, err := buildInterceptorList(
		names,
		providers,
		"stream server",
		func(provider StreamServerInterceptorProvider) bool { return provider == nil },
		func(provider StreamServerInterceptorProvider) StreamServerInterceptor {
			return provider.New()
		},
		func(interceptor StreamServerInterceptor) bool { return interceptor == nil },
	)
	if err != nil {
		return nil, err
	}
	return chainStreamServerInterceptors(interceptors), nil
}

func chainStreamServerInterceptors(
	interceptors []StreamServerInterceptor,
) StreamServerInterceptor {
	if len(interceptors) == 0 {
		return func(
			srv interface{},
			ss stream.ServerStream,
			_ *StreamServerInfo,
			handler stream.Handler,
		) error {
			return handler(srv, ss)
		}
	}
	if len(interceptors) == 1 {
		return interceptors[0]
	}
	return func(
		srv interface{},
		ss stream.ServerStream,
		info *StreamServerInfo,
		handler stream.Handler,
	) error {
		next := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			current := interceptors[i]
			downstream := next
			next = func(srv any, stream stream.ServerStream) error {
				return current(srv, stream, info, downstream)
			}
		}
		return next(srv, ss)
	}
}
