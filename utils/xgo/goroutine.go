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

// Package xgo provides a simple way to run a function in a new goroutine.
package xgo

import (
	"context"
	"log/slog"
	"runtime/debug"
)

// Go runs a function in a new goroutine,
// recover panic and log it.
func Go(f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("goroutine panic",
					slog.Any("msg", r),
					slog.String("stack", string(debug.Stack())))
			}
		}()
		f()
	}()
}

// GoWithCtx runs a function in a new goroutine with a context,
// recover panic and log it.
func GoWithCtx(ctx context.Context, f func(ctx context.Context)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("goroutine panic",
					slog.Any("msg", r),
					slog.String("stack", string(debug.Stack())))
			}
		}()
		f(ctx)
	}()
}
