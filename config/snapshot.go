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
	"fmt"
	"reflect"
	"strings"

	"github.com/creasty/defaults"
	"github.com/mitchellh/mapstructure"

	"github.com/codesjoy/yggdrasil/v3/internal/configtree"
)

// Snapshot is an immutable configuration snapshot or subsection.
type Snapshot struct {
	value any
}

// NewSnapshot creates a snapshot backed by a deep-cloned value.
func NewSnapshot(value any) Snapshot {
	return Snapshot{value: configtree.NormalizeValue(value)}
}

// Section returns a nested immutable snapshot.
func (s Snapshot) Section(path ...string) Snapshot {
	if len(path) == 0 {
		return NewSnapshot(s.value)
	}
	return NewSnapshot(Lookup(s.value, path...))
}

// Decode decodes the snapshot into the target value.
func (s Snapshot) Decode(target any) error {
	return decodeInto(s.value, target)
}

// Map returns the underlying value when it is a map.
func (s Snapshot) Map() map[string]any {
	if out, ok := s.value.(map[string]any); ok {
		return configtree.NormalizeMap(out)
	}
	return map[string]any{}
}

// Bytes returns the JSON representation of the snapshot.
func (s Snapshot) Bytes() []byte {
	data, _ := json.Marshal(s.value)
	return data
}

// Empty reports whether the snapshot contains no value.
func (s Snapshot) Empty() bool {
	return s.value == nil
}

// Value returns a normalized clone of the underlying value.
func (s Snapshot) Value() any {
	return configtree.NormalizeValue(s.value)
}

func decodeInto(src, target any) error {
	value := reflect.ValueOf(target)
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() {
		return errors.New("decode target must be a non-nil pointer")
	}
	elem := value.Elem()
	if src == nil {
		if elem.Kind() == reflect.Struct {
			return defaults.Set(target)
		}
		return nil
	}
	src = configtree.NormalizeValue(src)

	switch src.(type) {
	case map[string]any, []any:
		cfg := mapstructure.DecoderConfig{
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				mapstructure.TextUnmarshallerHookFunc(),
				mapstructure.StringToTimeDurationHookFunc(),
			),
			Result: target,
		}
		decoder, err := mapstructure.NewDecoder(&cfg)
		if err != nil {
			return err
		}
		if err := decoder.Decode(src); err != nil {
			return err
		}
	default:
		if elem.Kind() == reflect.Struct {
			return fmt.Errorf(
				"config snapshot value type %T cannot decode into struct %s",
				src,
				elem.Type(),
			)
		}
		srcValue := reflect.ValueOf(src)
		switch {
		case srcValue.Type().AssignableTo(elem.Type()):
			elem.Set(srcValue)
		case srcValue.Type().ConvertibleTo(elem.Type()):
			elem.Set(srcValue.Convert(elem.Type()))
		default:
			return fmt.Errorf(
				"config snapshot value type %s is not assignable to %s",
				srcValue.Type(),
				elem.Type(),
			)
		}
	}

	if elem.Kind() == reflect.Struct {
		return defaults.Set(target)
	}
	return nil
}

// Lookup returns a deep-cloned value from the given path.
func Lookup(value any, path ...string) any {
	if len(path) == 0 {
		return configtree.NormalizeValue(value)
	}
	current := value
	for i, segment := range path {
		nextMap, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		next, ok := nextMap[segment]
		if !ok {
			return nil
		}
		if i == len(path)-1 {
			return configtree.NormalizeValue(next)
		}
		current = next
	}
	return nil
}

// SetPath writes a normalized value into the provided map using path segments.
func SetPath(dst map[string]any, value any, path ...string) {
	if len(path) == 0 {
		return
	}
	current := dst
	for _, segment := range path[:len(path)-1] {
		next, ok := current[segment].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[segment] = next
		}
		current = next
	}
	current[path[len(path)-1]] = configtree.NormalizeValue(value)
}

// Section is a typed view over a manager subsection.
type Section[T any] struct {
	manager *Manager
	path    []string
}

// Bind creates a typed section bound to the given manager path.
func Bind[T any](manager *Manager, path ...string) Section[T] {
	if manager == nil {
		manager = Default()
	}
	return Section[T]{
		manager: manager,
		path:    append([]string(nil), path...),
	}
}

// Current returns the current typed value for the section.
func (s Section[T]) Current() (T, error) {
	var out T
	err := s.manager.Section(s.path...).Decode(&out)
	return out, err
}

// Watch subscribes to typed section updates. The callback is invoked immediately.
func (s Section[T]) Watch(fn func(T, error)) func() {
	return s.manager.watch(s.path, func(snapshot Snapshot) {
		var out T
		err := snapshot.Decode(&out)
		if fn != nil {
			fn(out, err)
		}
	})
}

// View is an immutable, decode-capable config subsection.
type View interface {
	Path() string
	Decode(target any) error
	Sub(path string) View
	Exists() bool
}

type snapshotView struct {
	path     string
	snapshot Snapshot
}

// NewView creates a view from a snapshot and logical path.
func NewView(path string, snapshot Snapshot) View {
	return snapshotView{
		path:     strings.TrimSpace(path),
		snapshot: snapshot,
	}
}

func (v snapshotView) Path() string {
	return v.path
}

func (v snapshotView) Decode(target any) error {
	return v.snapshot.Decode(target)
}

func (v snapshotView) Sub(path string) View {
	path = strings.TrimSpace(path)
	if path == "" {
		return v
	}
	parts := splitDotPath(path)
	childPath := path
	if v.path != "" {
		childPath = v.path + "." + path
	}
	return snapshotView{
		path:     childPath,
		snapshot: v.snapshot.Section(parts...),
	}
}

func (v snapshotView) Exists() bool {
	return !v.snapshot.Empty()
}

func splitDotPath(path string) []string {
	raw := strings.Split(path, ".")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}
