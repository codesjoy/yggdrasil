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

package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func init() {
	RegisterBuilder("logger", requestLogger)
}

type logEntry struct {
	r *http.Request
}

func (l *logEntry) Write(status, _ int, _ http.Header, elapsed time.Duration, _ interface{}) {
	fields := []slog.Attr{
		slog.String("method", l.r.Method),
		slog.String("path", l.r.URL.Path),
		slog.Int("status", status),
		slog.Float64("cost", float64(elapsed)/float64(time.Millisecond)),
	}
	var lv slog.Level
	if status < 400 {
		lv = slog.LevelInfo
	} else if status < 500 {
		lv = slog.LevelWarn
	} else {
		lv = slog.LevelError
	}
	slog.LogAttrs(l.r.Context(), lv, "http access", fields...)
}

func (l *logEntry) Panic(v interface{}, stack []byte) {
	slog.ErrorContext(l.r.Context(), "http access panic",
		slog.String("method", l.r.Method),
		slog.String("path", l.r.RequestURI),
		slog.String("panic", fmt.Sprintf("%v", v)),
		slog.String("stack", string(stack)),
	)
}

type logFormatter struct{}

func (l *logFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	return &logEntry{r: r}
}

func requestLogger() func(next http.Handler) http.Handler {
	return middleware.RequestLogger(&logFormatter{})
}
