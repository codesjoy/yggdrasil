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

// Package logging provides a logging interceptor.
package logging

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	"github.com/codesjoy/yggdrasil/v3/rpc/status"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
)

const (
	typeLogging = "logging"
	typeLogger  = "logger"
)

// Config defines the logger configuration.
type Config struct {
	SlowThreshold  time.Duration `default:"1s"`
	PrintReqAndRes bool
}

// ProviderOption configures a built-in logging interceptor provider.
type ProviderOption func(*logging)

// WithProviderLogger binds the built-in providers to an explicit logger.
func WithProviderLogger(logger *slog.Logger) ProviderOption {
	return func(interceptor *logging) {
		interceptor.logger = logger
	}
}

func providerNames() []string {
	return []string{typeLogging, typeLogger}
}

func buildProviders[P any](
	source any,
	opts []ProviderOption,
	build func(string, *logging) P,
) ([]P, error) {
	loggingCfg, err := loadConfig(source)
	if err != nil {
		return nil, err
	}
	names := providerNames()
	out := make([]P, 0, len(names))
	for _, name := range names {
		out = append(out, build(name, newLogging(loggingCfg, opts...)))
	}
	return out, nil
}

// BuiltinUnaryClientProviders returns built-in unary client interceptor providers.
func BuiltinUnaryClientProviders() []interceptor.UnaryClientInterceptorProvider {
	return BuiltinUnaryClientProvidersWithConfig(nil)
}

// BuiltinUnaryClientProvidersE returns built-in unary client providers.
func BuiltinUnaryClientProvidersE() ([]interceptor.UnaryClientInterceptorProvider, error) {
	return BuiltinUnaryClientProvidersWithConfigE(nil)
}

// BuiltinUnaryClientProvidersWithConfig returns built-in unary client interceptor providers bound to explicit config.
//
// Deprecated: use BuiltinUnaryClientProvidersWithConfigE to receive config errors.
func BuiltinUnaryClientProvidersWithConfig(
	source any,
) []interceptor.UnaryClientInterceptorProvider {
	providers, err := BuiltinUnaryClientProvidersWithConfigE(source)
	if err != nil {
		panic(err)
	}
	return providers
}

// BuiltinUnaryClientProvidersWithConfigE returns built-in unary client
// interceptor providers and reports invalid config.
func BuiltinUnaryClientProvidersWithConfigE(
	source any,
	opts ...ProviderOption,
) ([]interceptor.UnaryClientInterceptorProvider, error) {
	return buildProviders(source, opts, func(
		name string,
		l *logging,
	) interceptor.UnaryClientInterceptorProvider {
		return interceptor.NewUnaryClientInterceptorProvider(
			name,
			func(string) interceptor.UnaryClientInterceptor {
				return l.UnaryClientInterceptor
			},
		)
	})
}

// BuiltinStreamClientProviders returns built-in stream client interceptor providers.
func BuiltinStreamClientProviders() []interceptor.StreamClientInterceptorProvider {
	return BuiltinStreamClientProvidersWithConfig(nil)
}

// BuiltinStreamClientProvidersE returns built-in stream client providers.
func BuiltinStreamClientProvidersE() ([]interceptor.StreamClientInterceptorProvider, error) {
	return BuiltinStreamClientProvidersWithConfigE(nil)
}

// BuiltinStreamClientProvidersWithConfig returns built-in stream client interceptor providers bound to explicit config.
//
// Deprecated: use BuiltinStreamClientProvidersWithConfigE to receive config errors.
func BuiltinStreamClientProvidersWithConfig(
	source any,
) []interceptor.StreamClientInterceptorProvider {
	providers, err := BuiltinStreamClientProvidersWithConfigE(source)
	if err != nil {
		panic(err)
	}
	return providers
}

// BuiltinStreamClientProvidersWithConfigE returns built-in stream client
// interceptor providers and reports invalid config.
func BuiltinStreamClientProvidersWithConfigE(
	source any,
	opts ...ProviderOption,
) ([]interceptor.StreamClientInterceptorProvider, error) {
	return buildProviders(source, opts, func(
		name string,
		l *logging,
	) interceptor.StreamClientInterceptorProvider {
		return interceptor.NewStreamClientInterceptorProvider(
			name,
			func(string) interceptor.StreamClientInterceptor {
				return l.StreamClientInterceptor
			},
		)
	})
}

// BuiltinUnaryServerProviders returns built-in unary server interceptor providers.
func BuiltinUnaryServerProviders() []interceptor.UnaryServerInterceptorProvider {
	return BuiltinUnaryServerProvidersWithConfig(nil)
}

// BuiltinUnaryServerProvidersE returns built-in unary server providers.
func BuiltinUnaryServerProvidersE() ([]interceptor.UnaryServerInterceptorProvider, error) {
	return BuiltinUnaryServerProvidersWithConfigE(nil)
}

// BuiltinUnaryServerProvidersWithConfig returns built-in unary server interceptor providers bound to explicit config.
//
// Deprecated: use BuiltinUnaryServerProvidersWithConfigE to receive config errors.
func BuiltinUnaryServerProvidersWithConfig(
	source any,
) []interceptor.UnaryServerInterceptorProvider {
	providers, err := BuiltinUnaryServerProvidersWithConfigE(source)
	if err != nil {
		panic(err)
	}
	return providers
}

// BuiltinUnaryServerProvidersWithConfigE returns built-in unary server
// interceptor providers and reports invalid config.
func BuiltinUnaryServerProvidersWithConfigE(
	source any,
	opts ...ProviderOption,
) ([]interceptor.UnaryServerInterceptorProvider, error) {
	return buildProviders(source, opts, func(
		name string,
		l *logging,
	) interceptor.UnaryServerInterceptorProvider {
		return interceptor.NewUnaryServerInterceptorProvider(
			name,
			func() interceptor.UnaryServerInterceptor {
				return l.UnaryServerInterceptor
			},
		)
	})
}

// BuiltinStreamServerProviders returns built-in stream server interceptor providers.
func BuiltinStreamServerProviders() []interceptor.StreamServerInterceptorProvider {
	return BuiltinStreamServerProvidersWithConfig(nil)
}

// BuiltinStreamServerProvidersE returns built-in stream server providers.
func BuiltinStreamServerProvidersE() ([]interceptor.StreamServerInterceptorProvider, error) {
	return BuiltinStreamServerProvidersWithConfigE(nil)
}

// BuiltinStreamServerProvidersWithConfig returns built-in stream server interceptor providers bound to explicit config.
//
// Deprecated: use BuiltinStreamServerProvidersWithConfigE to receive config errors.
func BuiltinStreamServerProvidersWithConfig(
	source any,
) []interceptor.StreamServerInterceptorProvider {
	providers, err := BuiltinStreamServerProvidersWithConfigE(source)
	if err != nil {
		panic(err)
	}
	return providers
}

// BuiltinStreamServerProvidersWithConfigE returns built-in stream server
// interceptor providers and reports invalid config.
func BuiltinStreamServerProvidersWithConfigE(
	source any,
	opts ...ProviderOption,
) ([]interceptor.StreamServerInterceptorProvider, error) {
	return buildProviders(source, opts, func(
		name string,
		l *logging,
	) interceptor.StreamServerInterceptorProvider {
		return interceptor.NewStreamServerInterceptorProvider(
			name,
			func() interceptor.StreamServerInterceptor {
				return l.StreamServerInterceptor
			},
		)
	})
}

func loadConfig(source any) (*Config, error) {
	cfg := Config{}
	if err := config.NewSnapshot(source).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("load logging interceptor config: %w", err)
	}
	return &cfg, nil
}

type logging struct {
	cfg    *Config
	logger *slog.Logger
}

func newLogging(cfg *Config, opts ...ProviderOption) *logging {
	out := &logging{cfg: cfg}
	for _, opt := range opts {
		if opt != nil {
			opt(out)
		}
	}
	return out
}

func (l *logging) logAttrs(ctx context.Context, level slog.Level, fields ...slog.Attr) {
	if l != nil && l.logger != nil {
		l.logger.LogAttrs(ctx, level, "access", fields...)
		return
	}
	slog.LogAttrs(ctx, level, "access", fields...)
}

type observation struct {
	ctx           context.Context
	kind          string
	method        string
	request       any
	markSlow      bool
	warnOnSlow    bool
	printRequest  bool
	printResponse bool
}

func observeRPC[T any](
	logger *logging,
	item observation,
	call func() (T, error),
	response func(T) any,
) (result T, err error) {
	startTime := time.Now()
	defer func() {
		cost := time.Since(startTime)
		statusInfo := status.FromError(err)
		event := "normal"
		fields := make([]slog.Attr, 0, 8)
		if logger.cfg.SlowThreshold <= cost && item.markSlow {
			event = "slow"
		}
		if recovered := recover(); recovered != nil {
			switch recovered := recovered.(type) {
			case error:
				err = recovered
			default:
				err = fmt.Errorf("%v", recovered)
			}
			statusInfo = status.FromError(err)
			stack := make([]byte, 4096)
			stack = stack[:runtime.Stack(stack, true)]
			fields = append(fields, slog.String("stack", string(stack)))
			event = "recover"
		}
		fields = append(fields,
			slog.String("type", item.kind),
			slog.String("method", item.method),
			slog.Float64("cost", float64(cost)/float64(time.Millisecond)))
		if item.kind == "unary" {
			fields = append(
				fields,
				slog.Int("code", int(statusInfo.Code())),
				slog.String("event", event),
			)
		} else {
			fields = append(
				fields,
				slog.String("event", event),
				slog.Int("code", int(statusInfo.Code())),
			)
		}
		if logger.cfg.PrintReqAndRes && item.printRequest {
			fields = append(fields, slog.Any("req", item.request))
		}

		level := slog.LevelInfo
		if err != nil {
			fields = append(fields, slog.Any("error", err))
			if statusInfo.HTTPCode() >= http.StatusInternalServerError {
				level = slog.LevelError
			} else {
				level = slog.LevelWarn
			}
		} else {
			if logger.cfg.PrintReqAndRes && item.printResponse && response != nil {
				fields = append(fields, slog.Any("res", response(result)))
			}
			if item.warnOnSlow && event == "slow" {
				level = slog.LevelWarn
			}
		}
		logger.logAttrs(item.ctx, level, fields...)
	}()
	return call()
}

// UnaryServerInterceptor is a unary server interceptor.
func (l *logging) UnaryServerInterceptor(
	ctx context.Context,
	req interface{},
	info *interceptor.UnaryServerInfo,
	handler interceptor.UnaryHandler,
) (any, error) {
	return observeRPC(
		l,
		observation{
			ctx:           ctx,
			kind:          "unary",
			method:        info.FullMethod,
			request:       req,
			markSlow:      true,
			printRequest:  true,
			printResponse: true,
		},
		func() (any, error) { return handler(ctx, req) },
		func(resp any) any { return resp },
	)
}

// StreamServerInterceptor is a stream server interceptor.
func (l *logging) StreamServerInterceptor(
	srv interface{},
	ss stream.ServerStream,
	info *interceptor.StreamServerInfo,
	handler stream.Handler,
) error {
	_, err := observeRPC(
		l,
		observation{
			ctx:    ss.Context(),
			kind:   "stream",
			method: info.FullMethod,
		},
		func() (struct{}, error) {
			return struct{}{}, handler(srv, ss)
		},
		nil,
	)
	return err
}

// UnaryClientInterceptor is a unary client interceptor.
func (l *logging) UnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply any,
	invoker interceptor.UnaryInvoker,
) error {
	_, err := observeRPC(
		l,
		observation{
			ctx:           ctx,
			kind:          "unary",
			method:        method,
			request:       req,
			markSlow:      true,
			warnOnSlow:    true,
			printRequest:  true,
			printResponse: true,
		},
		func() (struct{}, error) {
			return struct{}{}, invoker(ctx, method, req, reply)
		},
		func(struct{}) any { return reply },
	)
	return err
}

// StreamClientInterceptor is a stream client interceptor.
func (l *logging) StreamClientInterceptor(
	ctx context.Context,
	desc *stream.Desc,
	method string,
	streamer interceptor.Streamer,
) (stream.ClientStream, error) {
	return observeRPC(
		l,
		observation{
			ctx:    ctx,
			kind:   "stream",
			method: method,
		},
		func() (stream.ClientStream, error) {
			return streamer(ctx, desc, method)
		},
		nil,
	)
}
