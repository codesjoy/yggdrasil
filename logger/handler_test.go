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

package logger

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"

	"github.com/codesjoy/yggdrasil/v2/config"
)

type emptyHandler struct{}

func (emptyHandler) Enabled(context.Context, slog.Level) bool { return true }

func (emptyHandler) Handle(context.Context, slog.Record) error { return nil }

func (h emptyHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h emptyHandler) WithGroup(string) slog.Handler { return h }

func resetHandlerBuilders() {
	handlerBuilderMu.Lock()
	defer handlerBuilderMu.Unlock()
	handlerBuilder = make(map[string]HandlerBuilder)
	handlerBuilder["json"] = newJSONHandler
	handlerBuilder["console"] = newConsoleHandler
}

func TestHandlerBuilderConcurrentRegisterAndGet(t *testing.T) {
	resetHandlerBuilders()

	const total = 64
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("handler_%d", i)
			RegisterHandlerBuilder(name, func(string, config.Value) (slog.Handler, error) {
				return emptyHandler{}, nil
			})
			_, _ = GetHandlerBuilder(name)
			_, _ = GetHandlerBuilder("json")
		}(i)
	}
	wg.Wait()

	for i := 0; i < total; i++ {
		name := fmt.Sprintf("handler_%d", i)
		if _, err := GetHandlerBuilder(name); err != nil {
			t.Fatalf("GetHandlerBuilder(%q) error = %v", name, err)
		}
	}
}
