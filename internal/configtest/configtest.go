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

// Package configtest serializes tests that mutate the package-level config snapshot.
package configtest

import (
	"fmt"
	"sync"
	"testing"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
)

var (
	serialMu sync.Mutex
	stateMu  sync.Mutex
	states   = map[string]*state{}
)

type state struct {
	layerName string
	data      map[string]any
}

// Set updates a dotted configuration key within an isolated test layer.
func Set(t testing.TB, key string, val any) {
	t.Helper()
	st := ensureState(t)
	applySet(st.data, key, val)
	loadState(t, st)
}

func ensureState(t testing.TB) *state {
	t.Helper()

	name := t.Name()
	stateMu.Lock()
	if st, ok := states[name]; ok {
		stateMu.Unlock()
		return st
	}
	stateMu.Unlock()

	serialMu.Lock()
	st := &state{
		layerName: "__configtest__/" + name,
		data:      map[string]any{},
	}
	stateMu.Lock()
	states[name] = st
	stateMu.Unlock()
	loadState(t, st)
	t.Cleanup(func() {
		clearState(st.layerName)
		stateMu.Lock()
		delete(states, name)
		stateMu.Unlock()
		serialMu.Unlock()
	})
	return st
}

func loadState(t testing.TB, st *state) {
	t.Helper()
	if err := config.Default().LoadLayer(st.layerName, config.PriorityOverride, memory.NewSource(st.layerName, st.data)); err != nil {
		t.Fatalf("load test config layer %q: %v", st.layerName, err)
	}
}

func clearState(layerName string) {
	_ = config.Default().
		LoadLayer(layerName, config.PriorityOverride, memory.NewSource(layerName, nil))
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
