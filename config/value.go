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

// Package config provides a set of types for configuration.
package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"strconv"
	"time"

	"github.com/creasty/defaults"
	"github.com/mitchellh/mapstructure"
)

type value struct {
	val interface{}
}

func newValue(val interface{}) Value {
	return &value{val: val}
}

func (m *value) Bool(def ...bool) bool {
	b, ok := m.val.(bool)
	if ok {
		return b
	}

	str, ok := m.val.(string)
	if !ok {
		if len(def) == 0 {
			return false
		}
		return def[0]
	}

	b, err := strconv.ParseBool(str)
	if err != nil {
		if len(def) == 0 {
			return false
		}
		return def[0]
	}

	return b
}

func (m *value) Int(def ...int) int {
	i, ok := m.val.(int)
	if ok {
		return i
	}

	str, ok := m.val.(string)
	if !ok {
		if len(def) == 0 {
			return 0
		}
		return def[0]
	}

	i, err := strconv.Atoi(str)
	if err != nil {
		if len(def) == 0 {
			return 0
		}
		return def[0]
	}

	return i
}

func (m *value) Int64(def ...int64) int64 {
	i, ok := m.val.(int64)
	if ok {
		return i
	}

	// Also handle int type
	iInt, ok := m.val.(int)
	if ok {
		return int64(iInt)
	}

	str, ok := m.val.(string)
	if !ok {
		if len(def) == 0 {
			return 0
		}
		return def[0]
	}

	i, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		if len(def) == 0 {
			return 0
		}
		return def[0]
	}

	return i
}

func (m *value) String(def ...string) string {
	if str, ok := m.val.(string); ok {
		return str
	}
	if len(def) == 0 {
		return ""
	}
	return def[0]
}

func (m *value) Float64(def ...float64) float64 {
	f, ok := m.val.(float64)
	if ok {
		return f
	}

	// Also handle int and int64 types
	if iInt, ok := m.val.(int); ok {
		return float64(iInt)
	}
	if iInt64, ok := m.val.(int64); ok {
		return float64(iInt64)
	}

	str, ok := m.val.(string)
	if !ok {
		if len(def) == 0 {
			return 0
		}
		return def[0]
	}

	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		if len(def) == 0 {
			return 0
		}
		return def[0]
	}

	return f
}

func (m *value) Duration(def ...time.Duration) time.Duration {
	switch v := m.val.(type) {
	case time.Duration:
		return v
	case string:
		value, err := time.ParseDuration(v)
		if err != nil {
			if len(def) == 0 {
				return 0
			}
			return def[0]
		}
		return value
	default:
		if len(def) == 0 {
			return 0
		}
		return def[0]
	}
}

func (m *value) StringSlice(def ...[]string) []string {
	switch sl := m.val.(type) {
	case []string:
		return sl
	case []interface{}:
		tmp := make([]string, len(sl))
		for i, item := range sl {
			tmp[i] = fmt.Sprintf("%v", item)
		}
		return tmp
	default:

	}
	sl, ok := m.val.([]string)
	if ok {
		return sl
	}
	if len(def) == 0 {
		return nil
	}
	return def[0]
}

func (m *value) StringMap(def ...map[string]string) map[string]string {
	res, ok := m.val.(map[string]string)
	if ok {
		return maps.Clone(res)
	}
	if len(def) == 0 {
		return map[string]string{}
	}
	return def[0]
}

func (m *value) Map(def ...map[string]any) map[string]any {
	res, ok := m.val.(map[string]any)
	if ok {
		return maps.Clone(res)
	}
	if len(def) == 0 {
		return map[string]any{}
	}
	return def[0]
}

func (m *value) Scan(val interface{}) error {
	switch m.val.(type) {
	case map[string]any:
	case []any:
	default:
		v := reflect.ValueOf(val)
		if v.Kind() != reflect.Ptr || v.IsNil() {
			return nil
		}

		// 获取指针指向的实际元素
		elem := v.Elem()
		// 3. 如果不是结构体，尝试直接赋值
		if elem.Kind() != reflect.Struct {
			if m.val == nil {
				return nil
			}
			// 检查是否可以进行赋值操作
			if elem.CanSet() {
				mVal := reflect.ValueOf(m.val)
				// 严谨起见，检查类型是否匹配或可以分配
				if mVal.Type().AssignableTo(elem.Type()) {
					elem.Set(mVal)
				}
			}
			return nil
		}
		return defaults.Set(val)
	}
	if m.val == nil {
		return defaults.Set(val)
	}
	c := mapstructure.DecoderConfig{
		DecodeHook: newCustomerDecoder(),
		Result:     val,
	}
	decoder, err := mapstructure.NewDecoder(&c)
	if err != nil {
		return err
	}
	if err := decoder.Decode(m.val); err != nil {
		return err
	}
	if reflect.TypeOf(val).Kind() != reflect.Ptr ||
		reflect.ValueOf(val).Elem().Kind() != reflect.Struct {
		return nil
	}
	return defaults.Set(val)
}

func (m *value) Bytes(def ...[]byte) []byte {
	switch v := m.val.(type) {
	case []byte:
		return v
	case string:
		return []byte(v)
	default:
		if m.val != nil {
			if data, _ := json.Marshal(m.val); len(data) > 0 {
				return data
			}
		}
		if len(def) == 0 {
			return nil
		}
		return def[0]
	}
}

func newCustomerDecoder() mapstructure.DecodeHookFuncType {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		switch {
		case f.Kind() == reflect.String:
			if t != reflect.TypeOf(time.Duration(5)) {
				return data, nil
			}
			return time.ParseDuration(data.(string))
		case t == reflect.TypeOf((*Value)(nil)).Elem():
			return newValue(data), nil
		default:
			return data, nil
		}
	}
}
