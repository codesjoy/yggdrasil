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

package config

import (
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
)

type testSource struct {
	name       string
	kind       string
	data       source.Data
	readErr    error
	closeErr   error
	closeCount int32
}

func (s *testSource) Kind() string { return s.kind }
func (s *testSource) Name() string { return s.name }
func (s *testSource) Read() (source.Data, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	return s.data, nil
}

func (s *testSource) Close() error {
	atomic.AddInt32(&s.closeCount, 1)
	return s.closeErr
}

type watchableTestSource struct {
	*testSource
	watchCh  <-chan source.Data
	watchErr error
}

func (s *watchableTestSource) Watch() (<-chan source.Data, error) {
	if s.watchErr != nil {
		return nil, s.watchErr
	}
	return s.watchCh, nil
}

type blockingReadSource struct {
	*testSource
	started chan struct{}
	release chan struct{}
}

func (s *blockingReadSource) Read() (source.Data, error) {
	close(s.started)
	<-s.release
	return s.testSource.Read()
}

type blockingData struct {
	data    source.Data
	started chan struct{}
	release chan struct{}
}

func (d *blockingData) Bytes() []byte {
	return d.data.Bytes()
}

func (d *blockingData) Unmarshal(target any) error {
	close(d.started)
	<-d.release
	return d.data.Unmarshal(target)
}

func TestDefaultSetDefaultBindCurrent(t *testing.T) {
	prev := Default()
	t.Cleanup(func() { SetDefault(prev) })

	manager := NewManager()
	old := SetDefault(manager)
	require.Equal(t, prev, old)
	require.Equal(t, manager, Default())

	require.NoError(
		t,
		manager.LoadLayer("defaults", PriorityDefaults, memory.NewSource("defaults", map[string]any{
			"app": map[string]any{"port": 8088},
		})),
	)
	section := Bind[struct {
		Port int `mapstructure:"port"`
	}](nil, "app")
	current, err := section.Current()
	require.NoError(t, err)
	require.Equal(t, 8088, current.Port)

	returned := SetDefault(nil)
	require.Equal(t, manager, returned)
	require.NotNil(t, Default())
	require.NotEqual(t, manager, Default())
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, cond(), "condition not met before timeout")
}

func TestManagerLoadLayerRespectsPriorityAndReplacement(t *testing.T) {
	manager := NewManager()

	require.NoError(
		t,
		manager.LoadLayer("defaults", PriorityDefaults, memory.NewSource("defaults", map[string]any{
			"app": map[string]any{
				"name": "base",
				"port": 8080,
			},
		})),
	)
	require.NoError(t, manager.LoadLayer("env", PriorityEnv, memory.NewSource("env", map[string]any{
		"app": map[string]any{
			"port": 9090,
		},
	})))

	var cfg struct {
		Name string `mapstructure:"name"`
		Port int    `mapstructure:"port"`
	}
	require.NoError(t, manager.Section("app").Decode(&cfg))
	require.Equal(t, "base", cfg.Name)
	require.Equal(t, 9090, cfg.Port)

	require.NoError(t, manager.LoadLayer("env", PriorityEnv, memory.NewSource("env", map[string]any{
		"app": map[string]any{
			"port": 10001,
		},
	})))
	require.NoError(t, manager.Section("app").Decode(&cfg))
	require.Equal(t, 10001, cfg.Port)
}

func TestManagerLoadLayerReplacementUsesNewPriority(t *testing.T) {
	manager := NewManager()
	t.Cleanup(func() { require.NoError(t, manager.Close()) })

	require.NoError(
		t,
		manager.LoadLayer("shared", PriorityFile, memory.NewSource("shared", map[string]any{
			"value": "file",
		})),
	)
	require.NoError(
		t,
		manager.LoadLayer("other", PriorityDefaults, memory.NewSource("other", map[string]any{
			"value": "default",
		})),
	)
	require.Equal(t, "file", manager.Snapshot().Value().(map[string]any)["value"])

	require.NoError(
		t,
		manager.LoadLayer("shared", PriorityDefaults, memory.NewSource("shared", map[string]any{
			"value": "replacement",
		})),
	)
	require.Equal(t, "default", manager.Snapshot().Value().(map[string]any)["value"])
}

func TestTypedSectionWatchIsScoped(t *testing.T) {
	manager := NewManager()
	type appCfg struct {
		Enabled bool `mapstructure:"enabled"`
	}

	section := Bind[appCfg](manager, "app")
	events := make([]bool, 0, 2)
	cancel := section.Watch(func(next appCfg, err error) {
		require.NoError(t, err)
		events = append(events, next.Enabled)
	})
	defer cancel()

	require.NoError(
		t,
		manager.LoadLayer("first", PriorityFile, memory.NewSource("first", map[string]any{
			"app": map[string]any{"enabled": true},
		})),
	)
	require.NoError(
		t,
		manager.LoadLayer("other", PriorityOverride, memory.NewSource("other", map[string]any{
			"other": map[string]any{"value": 1},
		})),
	)

	require.Equal(t, []bool{false, true}, events)
}

func TestManagerLoadLayerValidationAndErrors(t *testing.T) {
	manager := NewManager()

	err := manager.LoadLayer("", PriorityDefaults, memory.NewSource("x", map[string]any{}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "layer name")

	err = manager.LoadLayer("x", PriorityDefaults, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "source is nil")

	readErr := errors.New("read failed")
	err = manager.LoadLayer("bad-read", PriorityFile, &testSource{
		name:    "bad-read",
		kind:    "test",
		readErr: readErr,
	})
	require.ErrorIs(t, err, readErr)

	err = manager.LoadLayer("bad-decode", PriorityFile, &testSource{
		name: "bad-decode",
		kind: "test",
		data: source.NewBytesData([]byte(`[]`), json.Unmarshal),
	})
	require.Error(t, err)

	watchErr := errors.New("watch failed")
	err = manager.LoadLayer("bad-watch", PriorityFile, &watchableTestSource{
		testSource: &testSource{
			name: "bad-watch",
			kind: "test",
			data: source.NewMapData(map[string]any{"app": map[string]any{"enabled": true}}),
		},
		watchErr: watchErr,
	})
	require.ErrorIs(t, err, watchErr)
}

func TestManagerCloseDuringLoadLayerRejectsLateLayer(t *testing.T) {
	manager := NewManager()
	src := &blockingReadSource{
		testSource: &testSource{
			name: "blocking",
			kind: "test",
			data: source.NewMapData(map[string]any{"app": map[string]any{"enabled": true}}),
		},
		started: make(chan struct{}),
		release: make(chan struct{}),
	}

	loadDone := make(chan error, 1)
	go func() {
		loadDone <- manager.LoadLayer("blocking", PriorityFile, src)
	}()
	<-src.started
	require.NoError(t, manager.Close())
	close(src.release)

	require.ErrorIs(t, <-loadDone, errManagerClosed)
}

func TestManagerLoadLayerReplaceClosesPreviousSourceAndSkipsNotifyOnUnchanged(t *testing.T) {
	manager := NewManager()
	defer func() { require.NoError(t, manager.Close()) }()

	var notified int32
	cancel := manager.watch(nil, func(_ Snapshot) { atomic.AddInt32(&notified, 1) })
	defer cancel()

	first := &testSource{
		name: "shared",
		kind: "test",
		data: source.NewMapData(map[string]any{
			"app": map[string]any{"port": 8080},
		}),
	}
	require.NoError(t, manager.LoadLayer("shared", PriorityFile, first))
	waitFor(t, func() bool { return atomic.LoadInt32(&notified) >= 2 }) // initial + first load

	second := &testSource{
		name: "shared",
		kind: "test",
		data: source.NewMapData(map[string]any{
			"app": map[string]any{"port": 8080},
		}),
	}
	before := atomic.LoadInt32(&notified)
	require.NoError(t, manager.LoadLayer("shared", PriorityFile, second))
	require.Equal(t, int32(1), atomic.LoadInt32(&first.closeCount))
	require.Equal(t, before, atomic.LoadInt32(&notified))

	third := &testSource{
		name: "shared",
		kind: "test",
		data: source.NewMapData(map[string]any{
			"app": map[string]any{"port": 8081},
		}),
	}
	require.NoError(t, manager.LoadLayer("shared", PriorityFile, third))
	waitFor(t, func() bool { return atomic.LoadInt32(&notified) > before })
}

func TestManagerWatchLayerUpdatesAndIgnoresInvalidPayload(t *testing.T) {
	manager := NewManager()
	defer func() { require.NoError(t, manager.Close()) }()

	changeCh := make(chan source.Data, 2)
	src := &watchableTestSource{
		testSource: &testSource{
			name: "watch",
			kind: "test",
			data: source.NewMapData(map[string]any{
				"app": map[string]any{"enabled": false},
			}),
		},
		watchCh: changeCh,
	}

	require.NoError(t, manager.LoadLayer("watch", PriorityFile, src))

	section := Bind[struct {
		Enabled bool `mapstructure:"enabled"`
	}](manager, "app")
	var last atomic.Bool
	cancel := section.Watch(func(next struct {
		Enabled bool `mapstructure:"enabled"`
	}, err error,
	) {
		require.NoError(t, err)
		last.Store(next.Enabled)
	})
	defer cancel()
	require.False(t, last.Load())

	changeCh <- source.NewMapData(map[string]any{
		"app": map[string]any{"enabled": true},
	})
	waitFor(t, last.Load)

	changeCh <- source.NewBytesData([]byte(`not-json`), json.Unmarshal)
	time.Sleep(50 * time.Millisecond)
	require.True(t, last.Load())
}

func TestManagerWatchableReplacementCanCloseAndDropsRetiredUpdates(t *testing.T) {
	manager := NewManager()
	t.Cleanup(func() { _ = manager.Close() })
	changeCh := make(chan source.Data, 1)
	src := &watchableTestSource{
		testSource: &testSource{
			name: "watch",
			kind: "test",
			data: source.NewMapData(map[string]any{
				"app": map[string]any{"value": "initial"},
			}),
		},
		watchCh: changeCh,
	}
	require.NoError(t, manager.LoadLayer("shared", PriorityFile, src))

	blocked := &blockingData{
		data: source.NewMapData(map[string]any{
			"app": map[string]any{"value": "stale"},
		}),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	changeCh <- blocked
	select {
	case <-blocked.started:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not start decoding the stale update")
	}

	require.NoError(
		t,
		manager.LoadLayer(
			"shared",
			PriorityOverride,
			memory.NewSource("replacement", map[string]any{
				"app": map[string]any{"value": "fresh"},
			}),
		),
	)
	close(blocked.release)

	waitFor(t, func() bool {
		value, _ := Lookup(manager.Snapshot().Value(), "app", "value").(string)
		return value == "fresh"
	})
	require.NotPanics(t, func() {
		require.NoError(t, manager.Close())
	})
}

func TestManagerRetiredWatcherDoesNotReportErrors(t *testing.T) {
	manager := NewManager()
	defer func() { require.NoError(t, manager.Close()) }()

	changeCh := make(chan source.Data, 1)
	src := &watchableTestSource{
		testSource: &testSource{
			name: "watch",
			kind: "test",
			data: source.NewMapData(map[string]any{
				"app": map[string]any{"value": "initial"},
			}),
		},
		watchCh: changeCh,
	}
	require.NoError(t, manager.LoadLayer("shared", PriorityFile, src))

	errorsCh := make(chan error, 1)
	cancel := manager.WatchWithError(nil, func(Snapshot) {}, func(err error) {
		select {
		case errorsCh <- err:
		default:
		}
	})
	defer cancel()

	blocked := &blockingData{
		data:    source.NewBytesData([]byte(`not-json`), json.Unmarshal),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	changeCh <- blocked
	select {
	case <-blocked.started:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not start decoding the stale error")
	}
	require.NoError(
		t,
		manager.LoadLayer(
			"shared",
			PriorityOverride,
			memory.NewSource("replacement", map[string]any{
				"app": map[string]any{"value": "fresh"},
			}),
		),
	)
	close(blocked.release)

	select {
	case err := <-errorsCh:
		t.Fatalf("retired watcher reported an error: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestManagerWatchWithErrorReportsInvalidPayload(t *testing.T) {
	manager := NewManager()
	defer func() { require.NoError(t, manager.Close()) }()

	changeCh := make(chan source.Data, 1)
	src := &watchableTestSource{
		testSource: &testSource{
			name: "watch-error",
			kind: "test",
			data: source.NewMapData(map[string]any{
				"app": map[string]any{"enabled": true},
			}),
		},
		watchCh: changeCh,
	}
	require.NoError(t, manager.LoadLayer("watch-error", PriorityFile, src))

	errorsCh := make(chan error, 1)
	cancel := manager.WatchWithError(
		nil,
		func(Snapshot) {},
		func(err error) {
			select {
			case errorsCh <- err:
			default:
			}
		},
	)
	defer cancel()

	changeCh <- source.NewBytesData([]byte(`not-json`), json.Unmarshal)
	select {
	case err := <-errorsCh:
		require.Error(t, err)
		require.Contains(t, err.Error(), "watch decode failed")
	case <-time.After(2 * time.Second):
		t.Fatal("watch error was not reported")
	}
}

func TestManagerCloseIsIdempotentAndJoinsErrors(t *testing.T) {
	manager := NewManager()
	closeErr := errors.New("close failed")
	require.NoError(t, manager.LoadLayer("a", PriorityDefaults, &testSource{
		name: "a",
		kind: "test",
		data: source.NewMapData(map[string]any{
			"a": 1,
		}),
		closeErr: closeErr,
	}))
	require.NoError(t, manager.LoadLayer("b", PriorityDefaults, &testSource{
		name: "b",
		kind: "test",
		data: source.NewMapData(map[string]any{
			"b": 1,
		}),
	}))

	err := manager.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), closeErr.Error())
	require.NoError(t, manager.Close())

	called := false
	manager.watch([]string{"a"}, func(_ Snapshot) { called = true })
	require.False(t, called)
}
