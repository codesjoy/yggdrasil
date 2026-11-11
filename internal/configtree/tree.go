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

// Package configtree provides internal configuration tree normalization helpers.
package configtree

import (
	"fmt"
	"reflect"
	"unsafe"
)

// NormalizeValue recursively normalizes nested map types and deep-clones structured values.
func NormalizeValue(v any) any {
	switch v.(type) {
	case nil,
		bool,
		string,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, uintptr,
		float32, float64,
		complex64, complex128:
		return v
	}
	return newCloneState().normalize(reflect.ValueOf(v)).Interface()
}

// NormalizeMap recursively normalizes nested values and returns a deep clone.
func NormalizeMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	return newCloneState().normalize(reflect.ValueOf(src)).Interface().(map[string]any)
}

type cloneKey struct {
	typ  reflect.Type
	ptr  uintptr
	len  int
	cap  int
	kind reflect.Kind
}

type cloneState struct {
	objects map[cloneKey]reflect.Value
}

func newCloneState() *cloneState {
	return &cloneState{objects: make(map[cloneKey]reflect.Value)}
}

func (s *cloneState) normalize(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}

	switch value.Kind() {
	case reflect.Interface:
		return s.normalizeInterface(value)
	case reflect.Pointer:
		return s.normalizePointer(value)
	case reflect.Map:
		return s.normalizeMap(value)
	case reflect.Slice:
		return s.normalizeSlice(value)
	case reflect.Array:
		return s.normalizeArray(value)
	case reflect.Struct:
		return s.normalizeStruct(value)
	default:
		return value
	}
}

func (s *cloneState) normalizeInterface(value reflect.Value) reflect.Value {
	if value.IsNil() {
		return reflect.Zero(value.Type())
	}
	cloned := s.normalize(value.Elem())
	out := reflect.New(value.Type()).Elem()
	out.Set(cloned)
	return out
}

func (s *cloneState) normalizePointer(value reflect.Value) reflect.Value {
	if value.IsNil() {
		return reflect.Zero(value.Type())
	}
	key := objectKey(value)
	if cloned, ok := s.objects[key]; ok {
		return cloned
	}
	out := reflect.New(value.Type().Elem())
	s.objects[key] = out
	out.Elem().Set(s.normalize(value.Elem()))
	return out
}

func (s *cloneState) normalizeMap(value reflect.Value) reflect.Value {
	if value.IsNil() {
		return reflect.Zero(value.Type())
	}
	key := objectKey(value)
	if cloned, ok := s.objects[key]; ok {
		return cloned
	}
	outType := value.Type()
	if value.Type() == reflect.TypeOf(map[any]any{}) {
		outType = reflect.TypeOf(map[string]any{})
	}
	out := reflect.MakeMapWithSize(outType, value.Len())
	s.objects[key] = out
	iter := value.MapRange()
	for iter.Next() {
		if value.Type() == reflect.TypeOf(map[any]any{}) {
			out.SetMapIndex(
				reflect.ValueOf(fmt.Sprintf("%v", iter.Key().Interface())),
				s.normalize(iter.Value()),
			)
			continue
		}
		out.SetMapIndex(
			s.normalize(iter.Key()),
			s.normalize(iter.Value()),
		)
	}
	return out
}

func (s *cloneState) normalizeSlice(value reflect.Value) reflect.Value {
	if value.IsNil() {
		return reflect.Zero(value.Type())
	}
	key := objectKey(value)
	if cloned, ok := s.objects[key]; ok {
		return cloned
	}
	out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
	s.objects[key] = out
	for index := 0; index < value.Len(); index++ {
		out.Index(index).Set(s.normalize(value.Index(index)))
	}
	return out
}

func (s *cloneState) normalizeArray(value reflect.Value) reflect.Value {
	out := reflect.New(value.Type()).Elem()
	for index := 0; index < value.Len(); index++ {
		out.Index(index).Set(s.normalize(value.Index(index)))
	}
	return out
}

func (s *cloneState) normalizeStruct(value reflect.Value) reflect.Value {
	out := reflect.New(value.Type()).Elem()
	source := reflect.New(value.Type()).Elem()
	source.Set(value)
	out.Set(source)
	for index := 0; index < value.NumField(); index++ {
		sourceField := writableField(source.Field(index))
		writableField(out.Field(index)).Set(s.normalize(sourceField))
	}
	return out
}

func objectKey(value reflect.Value) cloneKey {
	key := cloneKey{typ: value.Type(), kind: value.Kind()}
	switch value.Kind() {
	case reflect.Pointer:
		key.ptr = value.Pointer()
	case reflect.Map, reflect.Slice:
		key.ptr = uintptr(value.UnsafePointer())
		if value.Kind() == reflect.Slice {
			key.len = value.Len()
			key.cap = value.Cap()
		}
	}
	return key
}

func writableField(value reflect.Value) reflect.Value {
	if value.CanInterface() && value.CanSet() {
		return value
	}
	//nolint:gosec // Reopens an addressable cloned config field for deep copying.
	address := unsafe.Pointer(value.UnsafeAddr())
	return reflect.NewAt(value.Type(), address).Elem()
}

// MergeMaps recursively overlays src onto dst. Source values replace destination
// values when the types differ.
func MergeMaps(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = map[string]any{}
	}
	for k, srcVal := range src {
		srcMap, srcIsMap := srcVal.(map[string]any)
		dstMap, dstIsMap := dst[k].(map[string]any)
		if srcIsMap && dstIsMap {
			dst[k] = MergeMaps(dstMap, srcMap)
			continue
		}
		dst[k] = NormalizeValue(srcVal)
	}
	return dst
}
