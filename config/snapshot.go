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

	"github.com/creasty/defaults"
	"github.com/mitchellh/mapstructure"

	configinternal "github.com/codesjoy/yggdrasil/v3/config/internal"
)

// Snapshot is an immutable configuration snapshot or subsection.
type Snapshot struct {
	value any
}

// NewSnapshot creates a snapshot backed by a deep-cloned value.
func NewSnapshot(value any) Snapshot {
	return Snapshot{value: configinternal.NormalizeValue(value)}
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
		return configinternal.NormalizeMap(out)
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
	return configinternal.NormalizeValue(s.value)
}

func decodeInto(src, target any) error {
	value := reflect.ValueOf(target)
	if !value.IsValid() || value.Kind() != reflect.Ptr || value.IsNil() {
		return errors.New("decode target must be a non-nil pointer")
	}
	elem := value.Elem()
	if src == nil {
		if elem.Kind() == reflect.Struct {
			return defaults.Set(target)
		}
		return nil
	}

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
			return fmt.Errorf("config snapshot value type %T cannot decode into struct %s", src, elem.Type())
		}
		srcValue := reflect.ValueOf(src)
		switch {
		case srcValue.Type().AssignableTo(elem.Type()):
			elem.Set(srcValue)
		case srcValue.Type().ConvertibleTo(elem.Type()):
			elem.Set(srcValue.Convert(elem.Type()))
		default:
			return fmt.Errorf("config snapshot value type %s is not assignable to %s", srcValue.Type(), elem.Type())
		}
	}

	if elem.Kind() == reflect.Struct {
		return defaults.Set(target)
	}
	return nil
}
