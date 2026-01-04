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
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil/pkg/config"
	"github.com/codesjoy/yggdrasil/pkg/interceptor"
	"github.com/codesjoy/yggdrasil/pkg/status"
	"github.com/codesjoy/yggdrasil/pkg/stream"
)

var name = "logger"

var (
	global *logging
	once   sync.Once
)

// Config defines the logger configuration.
type Config struct {
	SlowThreshold  time.Duration `default:"1s"`
	PrintReqAndRes bool
}

func initGlobalLogging() {
	once.Do(func() {
		cfg := Config{}
		key := config.Join(config.KeyBase, "interceptor", "config", name)
		if err := config.Get(key).Scan(&cfg); err != nil {
			slog.Error("fault to load logger config", slog.Any("name", name))
			os.Exit(1)
		}
		global = &logging{cfg: &cfg}
	})
}

func init() {
	interceptor.RegisterUnaryClientIntBuilder(
		name,
		func(_ string) interceptor.UnaryClientInterceptor {
			initGlobalLogging()
			return global.UnaryClientInterceptor
		},
	)
	interceptor.RegisterStreamClientIntBuilder(
		name,
		func(string) interceptor.StreamClientInterceptor {
			initGlobalLogging()
			return global.StreamClientInterceptor
		},
	)
	interceptor.RegisterUnaryServerIntBuilder(name, func() interceptor.UnaryServerInterceptor {
		initGlobalLogging()
		return global.UnaryServerInterceptor
	})
	interceptor.RegisterStreamServerIntBuilder(name, func() interceptor.StreamServerInterceptor {
		initGlobalLogging()
		return global.StreamServerInterceptor
	})
}

type logging struct {
	cfg *Config
}

// UnaryServerInterceptor is a unary server interceptor.
func (l *logging) UnaryServerInterceptor(
	ctx context.Context,
	req interface{},
	info *interceptor.UnaryServerInfo,
	handler interceptor.UnaryHandler,
) (resp interface{}, err error) {
	startTime := time.Now()
	defer func() {
		var (
			st     = status.FromError(err)
			fields = make([]slog.Attr, 0)
			event  = "normal"
			cost   = time.Since(startTime)
		)
		if l.cfg.SlowThreshold <= cost {
			event = "slow"
		}
		if rec := recover(); rec != nil {
			switch rec := rec.(type) {
			case error:
				err = rec
			default:
				err = fmt.Errorf("%v", rec)
			}
			st = status.FromError(err)
			stack := make([]byte, 4096)
			stack = stack[:runtime.Stack(stack, true)]
			fields = append(fields, slog.String("stack", string(stack)))
			event = "recover"
		}
		fields = append(fields,
			slog.String("type", "unary"),
			slog.String("method", info.FullMethod),
			slog.Float64("cost", float64(cost)/float64(time.Millisecond)),
			slog.Int("code", int(st.Code())),
			slog.String("event", event))
		if l.cfg.PrintReqAndRes {
			fields = append(fields, slog.Any("req", req))
		}
		var lv slog.Level
		if err != nil {
			fields = append(fields, slog.Any("error", err))
			if st.HTTPCode() >= http.StatusInternalServerError {
				lv = slog.LevelError
			} else {
				lv = slog.LevelWarn
			}
		} else {
			if l.cfg.PrintReqAndRes {
				fields = append(fields, slog.Any("res", resp))
			}
			lv = slog.LevelInfo
		}
		slog.LogAttrs(ctx, lv, "access", fields...)
	}()
	return handler(ctx, req)
}

// StreamServerInterceptor is a stream server interceptor.
func (l *logging) StreamServerInterceptor(
	srv interface{},
	ss stream.ServerStream,
	info *interceptor.StreamServerInfo,
	handler stream.Handler,
) (err error) {
	startTime := time.Now()
	defer func() {
		var (
			st     = status.FromError(err)
			fields = make([]slog.Attr, 0)
			event  = "normal"
			cost   = time.Since(startTime)
		)
		if rec := recover(); rec != nil {
			switch rec := rec.(type) {
			case error:
				err = rec
			default:
				err = fmt.Errorf("%v", rec)
			}
			st = status.FromError(err)
			stack := make([]byte, 4096)
			stack = stack[:runtime.Stack(stack, true)]
			fields = append(fields, slog.String("stack", string(stack)))
			event = "recover"
		}
		fields = append(fields,
			slog.String("type", "stream"),
			slog.String("method", info.FullMethod),
			slog.Float64("cost", float64(cost)/float64(time.Millisecond)),
			slog.String("event", event),
			slog.Int("code", int(st.Code())))
		var lv slog.Level
		if err != nil {
			fields = append(fields, slog.Any("error", err))
			if st.HTTPCode() >= http.StatusInternalServerError {
				lv = slog.LevelError
			} else {
				lv = slog.LevelWarn
			}
		} else {
			lv = slog.LevelInfo
		}
		slog.LogAttrs(ss.Context(), lv, "access", fields...)
	}()
	return handler(srv, ss)
}

// UnaryClientInterceptor is a unary client interceptor.
func (l *logging) UnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply any,
	invoker interceptor.UnaryInvoker,
) (err error) {
	startTime := time.Now()
	defer func() {
		var (
			st     = status.FromError(err)
			fields = make([]slog.Attr, 0)
			event  = "normal"
			cost   = time.Since(startTime)
		)
		if l.cfg.SlowThreshold <= cost {
			event = "slow"
		}
		if rec := recover(); rec != nil {
			switch rec := rec.(type) {
			case error:
				err = rec
			default:
				err = fmt.Errorf("%v", rec)
			}
			st = status.FromError(err)
			stack := make([]byte, 4096)
			stack = stack[:runtime.Stack(stack, true)]
			fields = append(fields, slog.String("stack", string(stack)))
			event = "recover"
		}
		fields = append(fields,
			slog.String("type", "unary"),
			slog.String("method", method),
			slog.Float64("cost", float64(cost)/float64(time.Millisecond)),
			slog.Int("code", int(st.Code())),
			slog.String("event", event))
		if l.cfg.PrintReqAndRes {
			fields = append(fields, slog.Any("req", req))
		}

		var lv slog.Level
		if err != nil {
			fields = append(fields, slog.Any("error", err))
			if st.HTTPCode() >= http.StatusInternalServerError {
				lv = slog.LevelError
			} else {
				lv = slog.LevelWarn
			}
		} else {
			if l.cfg.PrintReqAndRes {
				fields = append(fields, slog.Any("res", reply))
			}
			if l.cfg.SlowThreshold <= cost {
				lv = slog.LevelWarn
			} else {
				lv = slog.LevelInfo
			}
		}
		slog.LogAttrs(ctx, lv, "access", fields...)
	}()
	err = invoker(ctx, method, req, reply)
	return
}

// StreamClientInterceptor is a stream client interceptor.
func (l *logging) StreamClientInterceptor(
	ctx context.Context,
	desc *stream.Desc,
	method string,
	streamer interceptor.Streamer,
) (res stream.ClientStream, err error) {
	startTime := time.Now()
	defer func() {
		var (
			st     = status.FromError(err)
			fields = make([]slog.Attr, 0)
			event  = "normal"
			cost   = time.Since(startTime)
		)
		if rec := recover(); rec != nil {
			switch rec := rec.(type) {
			case error:
				err = rec
			default:
				err = fmt.Errorf("%v", rec)
			}
			st = status.FromError(err)
			stack := make([]byte, 4096)
			stack = stack[:runtime.Stack(stack, true)]
			fields = append(fields, slog.String("stack", string(stack)))
			event = "recover"
		}
		fields = append(fields,
			slog.String("type", "stream"),
			slog.String("method", method),
			slog.Float64("cost", float64(cost)/float64(time.Millisecond)),
			slog.String("event", event),
			slog.Int("code", int(st.Code())))

		var lv slog.Level
		if err != nil {
			fields = append(fields, slog.Any("error", err))
			if st.HTTPCode() >= http.StatusInternalServerError {
				lv = slog.LevelError
			} else {
				lv = slog.LevelWarn
			}
		} else {
			lv = slog.LevelInfo
		}
		slog.LogAttrs(ctx, lv, "access", fields...)
	}()
	return streamer(ctx, desc, method)
}
