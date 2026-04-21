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
	"io"
	"log/slog"
	"testing"
)

func newBenchmarkJSONLogger(b *testing.B) *slog.Logger {
	b.Helper()
	h, err := NewJSONHandler(&JSONHandlerConfig{
		CommonHandlerConfig: CommonHandlerConfig{
			Level: slog.LevelInfo,
		},
		Writer: io.Discard,
	})
	if err != nil {
		b.Fatalf("NewJSONHandler() error = %v", err)
	}
	return slog.New(h)
}

func BenchmarkJSONHandlerInfo(b *testing.B) {
	l := newBenchmarkJSONLogger(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info("bench_info")
	}
}

func BenchmarkJSONHandlerAnyScalars(b *testing.B) {
	l := newBenchmarkJSONLogger(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"bench_any_scalars",
			slog.Int("id", i),
			slog.Any("enabled", true),
			slog.Any("name", "worker"),
		)
	}
}

func BenchmarkJSONHandlerAnyObject(b *testing.B) {
	l := newBenchmarkJSONLogger(b)
	type payload struct {
		ID      int               `json:"id"`
		Name    string            `json:"name"`
		Labels  map[string]string `json:"labels"`
		Enabled bool              `json:"enabled"`
	}
	data := payload{
		ID:      42,
		Name:    "demo",
		Labels:  map[string]string{"k1": "v1", "k2": "v2"},
		Enabled: true,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"bench_any_object",
			slog.Any("payload", data),
		)
	}
}

func BenchmarkJSONHandlerGroupTwoLevels(b *testing.B) {
	l := newBenchmarkJSONLogger(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"bench_group",
			slog.Group(
				"request",
				slog.Group(
					"meta",
					slog.String("trace", "t-123"),
					slog.String("span", "s-456"),
				),
				slog.Int("attempt", i%3),
			),
		)
	}
}
