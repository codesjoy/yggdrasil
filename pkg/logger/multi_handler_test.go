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
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/pkg/logger/buffer"
)

// mockHandler is a mock slog.Handler for testing
type mockHandler struct {
	enabled   bool
	level     slog.Level
	records   []slog.Record
	attrs     [][]slog.Attr
	groups    []string
	handleErr error
}

func newMockHandler(enabled bool, level slog.Level) *mockHandler {
	return &mockHandler{
		enabled: enabled,
		level:   level,
		records: make([]slog.Record, 0),
		attrs:   make([][]slog.Attr, 0),
		groups:  make([]string, 0),
	}
}

// Enabled implements slog.Handler
func (m *mockHandler) Enabled(_ context.Context, level slog.Level) bool {
	return m.enabled && m.level <= level
}

// Handle implements slog.Handler
func (m *mockHandler) Handle(_ context.Context, r slog.Record) error {
	m.records = append(m.records, r)
	if m.handleErr != nil {
		return m.handleErr
	}
	return nil
}

// WithAttrs implements slog.Handler
func (m *mockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Create a new records slice to avoid sharing
	newRecords := make([]slog.Record, len(m.records))
	copy(newRecords, m.records)

	newHandler := &mockHandler{
		enabled:   m.enabled,
		level:     m.level,
		records:   newRecords,
		attrs:     append(m.attrs, attrs),
		groups:    append([]string{}, m.groups...),
		handleErr: m.handleErr,
	}
	return newHandler
}

// WithGroup implements slog.Handler
func (m *mockHandler) WithGroup(group string) slog.Handler {
	// Create a new records slice to avoid sharing
	newRecords := make([]slog.Record, len(m.records))
	copy(newRecords, m.records)

	newHandler := &mockHandler{
		enabled:   m.enabled,
		level:     m.level,
		records:   newRecords,
		attrs:     m.attrs,
		groups:    append(m.groups, group),
		handleErr: m.handleErr,
	}
	return newHandler
}

func (m *mockHandler) recordCount() int {
	return len(m.records)
}

func TestMultiHandlerEnabled(t *testing.T) {
	tests := []struct {
		name     string
		handlers []*mockHandler
		level    slog.Level
		want     bool
	}{
		{
			name:     "no handlers",
			handlers: []*mockHandler{},
			level:    slog.LevelInfo,
			want:     false,
		},
		{
			name: "all handlers enabled",
			handlers: []*mockHandler{
				newMockHandler(true, slog.LevelInfo),
				newMockHandler(true, slog.LevelInfo),
			},
			level: slog.LevelInfo,
			want:  true,
		},
		{
			name: "some handlers enabled",
			handlers: []*mockHandler{
				newMockHandler(false, slog.LevelInfo),
				newMockHandler(true, slog.LevelInfo),
			},
			level: slog.LevelInfo,
			want:  true,
		},
		{
			name: "no handlers enabled",
			handlers: []*mockHandler{
				newMockHandler(true, slog.LevelError),
				newMockHandler(true, slog.LevelError),
			},
			level: slog.LevelInfo,
			want:  false,
		},
		{
			name: "handler level too high",
			handlers: []*mockHandler{
				newMockHandler(true, slog.LevelError),
			},
			level: slog.LevelWarn,
			want:  false,
		},
		{
			name: "handler level sufficient",
			handlers: []*mockHandler{
				newMockHandler(true, slog.LevelInfo),
			},
			level: slog.LevelError,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers := make([]slog.Handler, len(tt.handlers))
			for i, mh := range tt.handlers {
				handlers[i] = mh
			}
			mh := &multiHandler{handlers: handlers}

			got := mh.Enabled(context.Background(), tt.level)
			if got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMultiHandlerHandle(t *testing.T) {
	tests := []struct {
		name      string
		handlers  []*mockHandler
		level     slog.Level
		wantCount int
		wantErr   bool
	}{
		{
			name:      "no handlers",
			handlers:  []*mockHandler{},
			level:     slog.LevelInfo,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "all handlers handle",
			handlers: []*mockHandler{
				newMockHandler(true, slog.LevelInfo),
				newMockHandler(true, slog.LevelInfo),
			},
			level:     slog.LevelInfo,
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "some handlers skip",
			handlers: []*mockHandler{
				newMockHandler(true, slog.LevelError),
				newMockHandler(true, slog.LevelInfo),
			},
			level:     slog.LevelInfo,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "one handler errors",
			handlers: []*mockHandler{
				newMockHandler(true, slog.LevelInfo),
			},
			level:     slog.LevelInfo,
			wantCount: 1,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				tt.handlers[0].handleErr = errors.New("handle error")
			}

			handlers := make([]slog.Handler, len(tt.handlers))
			for i, mh := range tt.handlers {
				handlers[i] = mh
			}
			mh := &multiHandler{handlers: handlers}

			now := time.Now()
			record := slog.NewRecord(now, tt.level, "test message", 0)

			err := mh.Handle(context.Background(), record)
			if (err != nil) != tt.wantErr {
				t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Count how many handlers actually processed the record
			actualCount := 0
			for _, h := range tt.handlers {
				actualCount += h.recordCount()
			}
			if actualCount != tt.wantCount {
				t.Errorf("Total records processed = %d, want %d", actualCount, tt.wantCount)
			}
		})
	}
}

func TestMultiHandlerWithAttrs(t *testing.T) {
	tests := []struct {
		name         string
		handlers     []*mockHandler
		attrs        []slog.Attr
		wantSame     bool
		wantAttrCopy bool
	}{
		{
			name: "all empty groups",
			handlers: []*mockHandler{
				newMockHandler(true, slog.LevelInfo),
			},
			attrs: []slog.Attr{
				slog.Group("empty"),
			},
			wantSame:     true,
			wantAttrCopy: false,
		},
		{
			name: "non-empty attrs",
			handlers: []*mockHandler{
				newMockHandler(true, slog.LevelInfo),
			},
			attrs: []slog.Attr{
				slog.String("key", "value"),
			},
			wantSame:     false,
			wantAttrCopy: true,
		},
		{
			name: "mixed empty and non-empty",
			handlers: []*mockHandler{
				newMockHandler(true, slog.LevelInfo),
			},
			attrs: []slog.Attr{
				slog.Group("empty"),
				slog.String("key", "value"),
			},
			wantSame:     false,
			wantAttrCopy: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers := make([]slog.Handler, len(tt.handlers))
			for i, mh := range tt.handlers {
				handlers[i] = mh
			}
			mh := &multiHandler{handlers: handlers}

			newHandler := mh.WithAttrs(tt.attrs)

			if tt.wantSame {
				if newHandler != mh {
					t.Error("WithAttrs() should return same handler for all empty groups")
				}
			} else {
				if newHandler == mh {
					t.Error("WithAttrs() should return new handler")
				}

				newMh, ok := newHandler.(*multiHandler)
				if !ok {
					t.Fatal("WithAttrs() did not return multiHandler")
				}

				// Verify handlers were cloned
				for i := range tt.handlers {
					if newMh.handlers[i] == mh.handlers[i] {
						t.Errorf("Handler %d was not cloned", i)
					}
				}
			}
		})
	}
}

func TestMultiHandlerWithGroup(t *testing.T) {
	handlers := []*mockHandler{
		newMockHandler(true, slog.LevelInfo),
		newMockHandler(true, slog.LevelInfo),
	}

	handlerSlice := make([]slog.Handler, len(handlers))
	for i, mh := range handlers {
		handlerSlice[i] = mh
	}
	mh := &multiHandler{handlers: handlerSlice}

	groupName := "test_group"
	newHandler := mh.WithGroup(groupName)

	if newHandler == mh {
		t.Error("WithGroup() should return new handler")
	}

	newMh, ok := newHandler.(*multiHandler)
	if !ok {
		t.Fatal("WithGroup() did not return multiHandler")
	}

	// Verify the handler slice was cloned
	if len(newMh.handlers) != len(mh.handlers) {
		t.Errorf("WithGroup() handlers length mismatch")
	}
	// Check that the slices are different (pointing to different backing arrays)
	if cap(newMh.handlers) == cap(mh.handlers) && &newMh.handlers[0] == &mh.handlers[0] {
		t.Error("WithGroup() did not clone handlers slice")
	}

	// Verify each handler had WithGroup called
	for i, h := range newMh.handlers {
		mockH, ok := h.(*mockHandler)
		if !ok {
			continue
		}
		if len(mockH.groups) != 1 || mockH.groups[0] != groupName {
			t.Errorf("Handler %d groups = %v, want [%s]", i, mockH.groups, groupName)
		}
	}
}

func TestMultiHandlerWithGroupChaining(t *testing.T) {
	handlers := []*mockHandler{
		newMockHandler(true, slog.LevelInfo),
	}

	handlerSlice := make([]slog.Handler, len(handlers))
	handlerSlice[0] = handlers[0]
	mh := &multiHandler{handlers: handlerSlice}

	h1 := mh.WithGroup("group1")
	h2 := h1.WithGroup("group2")

	newMh, ok := h2.(*multiHandler)
	if !ok {
		t.Fatal("WithGroup() did not return multiHandler")
	}

	mockH, ok := newMh.handlers[0].(*mockHandler)
	if !ok {
		t.Fatal("Handler is not mockHandler")
	}

	if len(mockH.groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(mockH.groups))
	}

	if mockH.groups[0] != "group1" || mockH.groups[1] != "group2" {
		t.Errorf("Groups = %v, want [group1, group2]", mockH.groups)
	}
}

func TestMultiHandlerWithAttrsChaining(t *testing.T) {
	handlers := []*mockHandler{
		newMockHandler(true, slog.LevelInfo),
	}

	handlerSlice := make([]slog.Handler, len(handlers))
	handlerSlice[0] = handlers[0]
	mh := &multiHandler{handlers: handlerSlice}

	attrs1 := []slog.Attr{slog.String("key1", "value1")}
	attrs2 := []slog.Attr{slog.Int("key2", 42)}

	h1 := mh.WithAttrs(attrs1)
	h2 := h1.WithAttrs(attrs2)

	newMh, ok := h2.(*multiHandler)
	if !ok {
		t.Fatal("WithAttrs() did not return multiHandler")
	}

	// Verify handlers were cloned at each step
	if newMh.handlers[0] == mh.handlers[0] {
		t.Error("Handler was not cloned")
	}
}

func TestCountEmptyGroups(t *testing.T) {
	tests := []struct {
		name  string
		attrs []slog.Attr
		want  int
	}{
		{
			name:  "no attrs",
			attrs: []slog.Attr{},
			want:  0,
		},
		{
			name: "no empty groups",
			attrs: []slog.Attr{
				slog.String("key", "value"),
				slog.Int("num", 42),
			},
			want: 0,
		},
		{
			name: "one empty group",
			attrs: []slog.Attr{
				slog.Group("empty"),
			},
			want: 1,
		},
		{
			name: "multiple empty groups",
			attrs: []slog.Attr{
				slog.Group("empty1"),
				slog.Group("empty2"),
			},
			want: 2,
		},
		{
			name: "mixed empty and non-empty groups",
			attrs: []slog.Attr{
				slog.Group("empty"),
				slog.Group("non_empty", slog.String("key", "value")),
			},
			want: 1,
		},
		{
			name: "group with multiple attrs",
			attrs: []slog.Attr{
				slog.Group("with_attrs",
					slog.String("key1", "value1"),
					slog.Int("key2", 42),
				),
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countEmptyGroups(tt.attrs)
			if got != tt.want {
				t.Errorf("countEmptyGroups() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMultiHandlerHandleAllLevels(t *testing.T) {
	levels := []slog.Level{
		slog.LevelDebug,
		slog.LevelInfo,
		slog.LevelWarn,
		slog.LevelError,
	}

	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			// Create fresh handlers for each test run
			handlers := []*mockHandler{
				newMockHandler(true, slog.LevelDebug),
				newMockHandler(true, slog.LevelInfo),
			}

			handlerSlice := make([]slog.Handler, len(handlers))
			for i, mh := range handlers {
				handlerSlice[i] = mh
			}
			mh := &multiHandler{handlers: handlerSlice}

			now := time.Now()
			record := slog.NewRecord(now, level, "test message", 0)

			_ = mh.Handle(context.Background(), record)

			// Handler 1 (Debug level) should handle all
			if handlers[0].recordCount() != 1 {
				t.Errorf("Debug handler record count = %d, want 1", handlers[0].recordCount())
			}

			// Handler 2 (Info level) should handle Info, Warn, Error
			expectedCount := 0
			if level >= slog.LevelInfo {
				expectedCount = 1
			}
			if handlers[1].recordCount() != expectedCount {
				t.Errorf(
					"Info handler record count = %d, want %d",
					handlers[1].recordCount(),
					expectedCount,
				)
			}
		})
	}
}

func TestMultiHandlerErrorJoin(t *testing.T) {
	handlers := []*mockHandler{
		newMockHandler(true, slog.LevelInfo),
		newMockHandler(true, slog.LevelInfo),
		newMockHandler(true, slog.LevelInfo),
	}

	// Make two handlers error
	handlers[0].handleErr = errors.New("error 1")
	handlers[2].handleErr = errors.New("error 2")

	handlerSlice := make([]slog.Handler, len(handlers))
	for i, mh := range handlers {
		handlerSlice[i] = mh
	}
	mh := &multiHandler{handlers: handlerSlice}

	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)

	err := mh.Handle(context.Background(), record)
	if err == nil {
		t.Error("Handle() expected error, got nil")
	}

	// Check that error message contains both errors
	errStr := err.Error()
	if !strings.Contains(errStr, "error 1") || !strings.Contains(errStr, "error 2") {
		t.Errorf("Error message = %s, expected to contain both errors", errStr)
	}
}

func TestDefaultContextHandle(t *testing.T) {
	ctx := context.Background()
	enc := &jsonEncoder{}
	buf := buffer.Get()
	enc.SetBuffer(buf)

	err := DefaultContextHandle(ctx, enc)
	if err != nil {
		t.Errorf("DefaultContextHandle() error = %v", err)
	}
}

func TestDefaultErrorHandle(t *testing.T) {
	err := errors.New("test error")
	enc := &jsonEncoder{}
	buf := buffer.Get()
	enc.SetBuffer(buf)

	DefaultErrorHandle("error_key", err, enc)

	result := buf.String()
	if !strings.Contains(result, "error_key") {
		t.Error("DefaultErrorHandle() did not add error key")
	}
	if !strings.Contains(result, "test error") {
		t.Error("DefaultErrorHandle() did not add error message")
	}
}

func TestMultiHandlerWithEmptyHandlers(t *testing.T) {
	mh := &multiHandler{handlers: []slog.Handler{}}

	now := time.Now()
	record := slog.NewRecord(now, slog.LevelInfo, "test message", 0)

	// Should not panic with empty handlers
	err := mh.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("Handle() with no handlers error = %v", err)
	}

	if mh.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Enabled() should return false with no handlers")
	}
}

func TestMultiHandlerCloneWithGroupAndAttrs(t *testing.T) {
	h1 := newMockHandler(true, slog.LevelInfo)
	h2 := newMockHandler(true, slog.LevelInfo)

	handlers := []slog.Handler{h1, h2}
	mh := &multiHandler{handlers: handlers}

	// Chain WithGroup and WithAttrs
	result := mh.WithGroup("group").WithAttrs([]slog.Attr{slog.String("key", "value")})

	newMh, ok := result.(*multiHandler)
	if !ok {
		t.Fatal("Did not return multiHandler")
	}

	// Verify the slice itself is different
	if len(newMh.handlers) != len(mh.handlers) {
		t.Error("Handlers slice length mismatch")
	}
	// Check that the backing arrays are different
	if &newMh.handlers == &mh.handlers {
		t.Error("Handlers slice was not cloned")
	}
}
