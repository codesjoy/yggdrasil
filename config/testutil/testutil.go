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

// Package testutil provides isolated config helpers for tests.
// Each T instance owns an independent *config.Manager so tests can run in parallel.
package testutil

import (
	"fmt"
	"testing"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
)

// T wraps an independent config.Manager for use in a single test.
type T struct {
	tb   testing.TB
	mgr  *config.Manager
	data map[string]any
}

// New creates a *T with its own *config.Manager.
// The manager is closed automatically via t.Cleanup.
func New(t testing.TB) *T {
	t.Helper()
	ct := &T{
		tb:   t,
		mgr:  config.NewManager(),
		data: map[string]any{},
	}
	t.Cleanup(func() {
		_ = ct.mgr.Close()
	})
	return ct
}

// Set sets a dotted configuration key (with brace support) and returns the
// receiver for chaining. The value is flushed to the underlying Manager on
// every call.
func (ct *T) Set(key string, val any) *T {
	ct.tb.Helper()
	applySet(ct.data, key, val)
	ct.flush()
	return ct
}

// Manager returns the underlying *config.Manager.
func (ct *T) Manager() *config.Manager {
	return ct.mgr
}

func (ct *T) flush() {
	ct.tb.Helper()
	if err := ct.mgr.LoadLayer(
		"__testutil__",
		config.PriorityOverride,
		memory.NewSource("__testutil__", ct.data),
	); err != nil {
		ct.tb.Fatalf("testutil: load config layer: %v", err)
	}
}

func applySet(dst map[string]any, key string, val any) {
	parts := pathSegments(key)
	tmp := dst
	for _, part := range parts[:len(parts)-1] {
		next, ok := tmp[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			tmp[part] = next
		}
		tmp = next
	}
	tmp[parts[len(parts)-1]] = val
}

func pathSegments(key string) []string {
	parts := make([]string, 0, 8)
	current := ""
	inBraces := false
	for _, r := range key {
		switch {
		case r == '{':
			inBraces = true
		case r == '}':
			inBraces = false
		case r == '.' && !inBraces:
			parts = append(parts, current)
			current = ""
		default:
			current += string(r)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	if len(parts) == 0 {
		panic(fmt.Sprintf("invalid config key %q", key))
	}
	return parts
}
