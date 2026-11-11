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
	"fmt"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/internal/configtree"
)

// Priority defines layer precedence.
type Priority uint8

// Layer priority levels, from lowest to highest precedence.
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
	watchID  uint64
}

type watcher struct {
	id      uint64
	path    []string
	fn      func(Snapshot)
	onError func(error)
}

var errManagerClosed = errors.New("config manager is closed")

var (
	defaultMu      sync.RWMutex
	defaultManager = NewManager()
)

// Manager owns layered configuration sources and a current immutable snapshot.
type Manager struct {
	mu sync.Mutex

	nextOrder   uint64
	nextWatchID uint64
	closed      bool

	layers   map[string]layer
	order    []string
	watchers []watcher
	watchWG  sync.WaitGroup

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
	m.mu.Lock()
	closed := m.closed
	m.mu.Unlock()
	if closed {
		return errManagerClosed
	}
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

	changeCh, stop, err := prepareSourceWatch(src)
	if err != nil {
		return err
	}

	oldSrc, oldStop, notify, watchID, committed := m.commitExternalLayer(name, layer{
		name:     name,
		priority: priority,
		data:     normalized,
		src:      src,
		stop:     stop,
	})
	if !committed {
		if stop != nil {
			close(stop)
		}
		return errManagerClosed
	}
	retireSource(oldSrc, oldStop, src)
	m.dispatch(notify)
	m.startSourceWatch(name, watchID, changeCh, stop)
	return nil
}

func prepareSourceWatch(src source.Source) (<-chan source.Data, chan struct{}, error) {
	watchable, ok := src.(source.Watchable)
	if !ok {
		return nil, nil, nil
	}
	changeCh, err := watchable.Watch()
	if err != nil {
		return nil, nil, err
	}
	if changeCh == nil {
		return nil, nil, nil
	}
	return changeCh, make(chan struct{}), nil
}

func retireSource(previous source.Source, stop chan struct{}, current source.Source) {
	if stop != nil {
		close(stop)
	}
	if previous != nil && previous != current {
		_ = previous.Close()
	}
}

func (m *Manager) startSourceWatch(
	name string,
	watchID uint64,
	changeCh <-chan source.Data,
	stop chan struct{},
) {
	if changeCh == nil {
		return
	}
	m.mu.Lock()
	active := !m.closed
	if active {
		m.watchWG.Add(1)
	}
	m.mu.Unlock()
	if active {
		go m.watchLayer(name, watchID, changeCh, stop)
	}
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
	m.watchWG.Wait()
	return err
}

type notification struct {
	fn       func(Snapshot)
	snapshot Snapshot
}

func (m *Manager) watch(path []string, fn func(Snapshot), onError ...func(error)) func() {
	if fn == nil {
		return func() {}
	}
	var errorFn func(error)
	if len(onError) > 0 {
		errorFn = onError[0]
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return func() {}
	}
	m.nextWatchID++
	id := m.nextWatchID
	storedPath := append([]string(nil), path...)
	m.watchers = append(m.watchers, watcher{
		id:      id,
		path:    storedPath,
		fn:      fn,
		onError: errorFn,
	})
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

// Watch subscribes changes for a snapshot subsection.
// The callback is invoked once immediately with the current snapshot.
func (m *Manager) Watch(path []string, fn func(Snapshot)) func() {
	return m.watch(path, fn)
}

// WatchWithError subscribes to changes and reports source decode errors.
// The success callback is invoked once immediately with the current snapshot.
func (m *Manager) WatchWithError(
	path []string,
	fn func(Snapshot),
	onError func(error),
) func() {
	return m.watch(path, fn, onError)
}

func (m *Manager) watchLayer(
	name string,
	watchID uint64,
	changeCh <-chan source.Data,
	stop <-chan struct{},
) {
	defer m.watchWG.Done()
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
				m.dispatchWatchError(watchID, fmt.Errorf(
					"config layer %q watch decode failed: %w",
					name,
					err,
				))
				continue
			}
			m.replaceLayerData(name, watchID, normalized)
		}
	}
}

func (m *Manager) replaceLayerData(name string, watchID uint64, data map[string]any) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	prev, exists := m.layers[name]
	if !exists || prev.watchID == 0 || prev.watchID != watchID {
		m.mu.Unlock()
		return
	}

	next := prev
	next.data = data
	if reflect.DeepEqual(prev.data, data) {
		m.mu.Unlock()
		return
	}
	m.layers[name] = next
	notify := m.publishSnapshotLocked()
	m.mu.Unlock()
	m.dispatch(notify)
}

func (m *Manager) commitExternalLayer(
	name string,
	next layer,
) (source.Source, chan struct{}, []notification, uint64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, nil, nil, 0, false
	}

	prev, exists := m.layers[name]
	if exists {
		next.order = prev.order
	} else {
		m.nextOrder++
		next.order = m.nextOrder
		m.order = append(m.order, name)
	}
	if next.stop != nil {
		m.nextWatchID++
		next.watchID = m.nextWatchID
	}
	m.layers[name] = next

	return prev.src, prev.stop, m.publishSnapshotLocked(), next.watchID, true
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
		merged = configtree.MergeMaps(merged, item.data)
	}
	return merged
}

func (m *Manager) publishSnapshotLocked() []notification {
	next := m.mergeLocked()
	previous := m.snapshotMap
	if reflect.DeepEqual(previous, next) {
		m.snapshotMap = next
		return nil
	}
	m.snapshotMap = next
	m.snapshot.Store(NewSnapshot(next))
	return m.collectNotificationsLocked(previous, next)
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

func (m *Manager) dispatchWatchError(watchID uint64, err error) {
	if err == nil {
		return
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	active := false
	for _, item := range m.layers {
		if item.watchID == watchID {
			active = true
			break
		}
	}
	if !active {
		m.mu.Unlock()
		return
	}
	callbacks := make([]func(error), 0, len(m.watchers))
	for _, item := range m.watchers {
		if item.onError != nil {
			callbacks = append(callbacks, item.onError)
		}
	}
	m.mu.Unlock()
	for _, fn := range callbacks {
		fn(err)
	}
}

func decodeSourceData(data source.Data) (map[string]any, error) {
	out := map[string]any{}
	if err := data.Unmarshal(&out); err != nil {
		return nil, err
	}
	return configtree.NormalizeMap(out), nil
}

// Default returns the process-global configuration manager used by yggdrasil bootstrapping.
func Default() *Manager {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultManager
}

// SetDefault swaps the process-global configuration manager and returns the previous one.
func SetDefault(next *Manager) *Manager {
	if next == nil {
		next = NewManager()
	}
	defaultMu.Lock()
	prev := defaultManager
	defaultManager = next
	defaultMu.Unlock()
	return prev
}
