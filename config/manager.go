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
	"errors"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"

	configinternal "github.com/codesjoy/yggdrasil/v3/config/internal"
	"github.com/codesjoy/yggdrasil/v3/config/source"
)

// Priority defines layer precedence.
type Priority uint8

const (
	PriorityDefaults Priority = iota
	PriorityFile
	PriorityRemote
	PriorityEnv
	PriorityFlag
	PriorityOverride
)

type layer struct {
	name     string
	priority Priority
	order    uint64
	data     map[string]any
	src      source.Source
	stop     chan struct{}
}

type watcher struct {
	id   uint64
	path []string
	fn   func(Snapshot)
}

// Manager owns layered configuration sources and a current immutable snapshot.
type Manager struct {
	mu sync.Mutex

	nextOrder   uint64
	nextWatchID uint64
	closed      bool

	layers   map[string]layer
	order    []string
	watchers []watcher

	snapshotMap map[string]any
	snapshot    atomic.Value
}

// NewManager creates an empty configuration manager.
func NewManager() *Manager {
	manager := &Manager{
		layers:      map[string]layer{},
		snapshotMap: map[string]any{},
	}
	manager.snapshot.Store(NewSnapshot(map[string]any{}))
	return manager
}

// Snapshot returns the current immutable snapshot.
func (m *Manager) Snapshot() Snapshot {
	return m.snapshot.Load().(Snapshot)
}

// Section returns a nested current snapshot.
func (m *Manager) Section(path ...string) Snapshot {
	return m.Snapshot().Section(path...)
}

// Bytes returns the JSON representation of the current snapshot.
func (m *Manager) Bytes() []byte {
	return m.Snapshot().Bytes()
}

// LoadLayer reads a source into a named layer and watches it when supported.
func (m *Manager) LoadLayer(name string, priority Priority, src source.Source) error {
	if name == "" {
		return errors.New("config layer name is required")
	}
	if src == nil {
		return errors.New("config source is nil")
	}

	data, err := src.Read()
	if err != nil {
		return err
	}
	normalized, err := decodeSourceData(data)
	if err != nil {
		return err
	}

	var changeCh <-chan source.Data
	var stop chan struct{}
	if watchable, ok := src.(source.Watchable); ok {
		changeCh, err = watchable.Watch()
		if err != nil {
			return err
		}
		if changeCh != nil {
			stop = make(chan struct{})
		}
	}

	oldSrc, oldStop, notify := m.commitLayer(name, layer{
		name:     name,
		priority: priority,
		data:     normalized,
		src:      src,
		stop:     stop,
	})
	if oldStop != nil {
		close(oldStop)
	}
	if oldSrc != nil && oldSrc != src {
		_ = oldSrc.Close()
	}
	m.dispatch(notify)

	if changeCh != nil {
		go m.watchLayer(name, changeCh, stop)
	}
	return nil
}

// Close stops watched sources and closes all owned sources.
func (m *Manager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	layers := make([]layer, 0, len(m.layers))
	for _, item := range m.layers {
		layers = append(layers, item)
	}
	m.watchers = nil
	m.mu.Unlock()

	var err error
	for _, item := range layers {
		if item.stop != nil {
			close(item.stop)
		}
		if item.src != nil {
			err = errors.Join(err, item.src.Close())
		}
	}
	return err
}

type notification struct {
	fn       func(Snapshot)
	snapshot Snapshot
}

func (m *Manager) watch(path []string, fn func(Snapshot)) func() {
	if fn == nil {
		return func() {}
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return func() {}
	}
	m.nextWatchID++
	id := m.nextWatchID
	storedPath := append([]string(nil), path...)
	m.watchers = append(m.watchers, watcher{id: id, path: storedPath, fn: fn})
	snapshot := NewSnapshot(Lookup(m.snapshotMap, storedPath...))
	m.mu.Unlock()

	fn(snapshot)

	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.watchers = slices.DeleteFunc(m.watchers, func(item watcher) bool {
			return item.id == id
		})
	}
}

func (m *Manager) watchLayer(name string, changeCh <-chan source.Data, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case change, ok := <-changeCh:
			if !ok {
				return
			}
			normalized, err := decodeSourceData(change)
			if err != nil {
				continue
			}
			m.replaceLayerData(name, normalized)
		}
	}
}

func (m *Manager) replaceLayerData(name string, data map[string]any) {
	_, _, notify := m.commitLayer(name, layer{name: name, data: data})
	m.dispatch(notify)
}

func (m *Manager) commitLayer(name string, next layer) (source.Source, chan struct{}, []notification) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, nil, nil
	}

	prev, exists := m.layers[name]
	if exists {
		next.order = prev.order
		if next.priority == 0 && prev.priority != 0 {
			next.priority = prev.priority
		}
		if next.src == nil {
			next.src = prev.src
		}
		if next.stop == nil {
			next.stop = prev.stop
		}
	} else {
		m.nextOrder++
		next.order = m.nextOrder
		m.order = append(m.order, name)
	}
	m.layers[name] = next

	merged := m.mergeLocked()
	if reflect.DeepEqual(m.snapshotMap, merged) {
		m.snapshotMap = merged
		if next.src == prev.src {
			return nil, nil, nil
		}
		return prev.src, prev.stop, nil
	}

	oldSnapshot := m.snapshotMap
	m.snapshotMap = merged
	m.snapshot.Store(NewSnapshot(merged))
	return prev.src, prev.stop, m.collectNotificationsLocked(oldSnapshot, merged)
}

func (m *Manager) mergeLocked() map[string]any {
	layers := make([]layer, 0, len(m.order))
	for _, name := range m.order {
		item, ok := m.layers[name]
		if !ok {
			continue
		}
		layers = append(layers, item)
	}

	slices.SortFunc(layers, func(a, b layer) int {
		if a.priority != b.priority {
			return int(a.priority) - int(b.priority)
		}
		switch {
		case a.order < b.order:
			return -1
		case a.order > b.order:
			return 1
		default:
			return 0
		}
	})

	merged := map[string]any{}
	for _, item := range layers {
		merged = configinternal.MergeMaps(merged, item.data)
	}
	return merged
}

func (m *Manager) collectNotificationsLocked(prev, next map[string]any) []notification {
	if len(m.watchers) == 0 {
		return nil
	}

	notify := make([]notification, 0, len(m.watchers))
	for _, item := range m.watchers {
		before := Lookup(prev, item.path...)
		after := Lookup(next, item.path...)
		if reflect.DeepEqual(before, after) {
			continue
		}
		notify = append(notify, notification{
			fn:       item.fn,
			snapshot: NewSnapshot(after),
		})
	}
	return notify
}

func (m *Manager) dispatch(items []notification) {
	for _, item := range items {
		item.fn(item.snapshot)
	}
}

func decodeSourceData(data source.Data) (map[string]any, error) {
	out := map[string]any{}
	if err := data.Unmarshal(&out); err != nil {
		return nil, err
	}
	return configinternal.NormalizeMap(out), nil
}
