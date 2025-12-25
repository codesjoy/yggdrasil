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

// Package xmap provides some useful functions for map.
package xmap

import (
	"fmt"
	"reflect"
)

// MergeStringMap merge src to dst
func MergeStringMap(dst map[string]interface{}, src ...map[string]interface{}) {
	for _, item := range src {
		mergeStringMap(dst, item)
	}
}

func mergeStringMap(dest, src map[string]interface{}) {
	for sk, sv := range src {
		tv, ok := dest[sk]
		if !ok {
			// val不存在时，直接赋值
			dest[sk] = sv
			continue
		}

		svType := reflect.TypeOf(sv)
		tvType := reflect.TypeOf(tv)
		if svType != tvType {
			continue
		}

		switch ttv := tv.(type) {
		case map[interface{}]interface{}:
			tsv := sv.(map[interface{}]interface{})
			ssv := ToMapStringInterface(tsv)
			stv := ToMapStringInterface(ttv)
			mergeStringMap(stv, ssv)
			dest[sk] = stv
		case map[string]interface{}:
			mergeStringMap(ttv, sv.(map[string]interface{}))
			dest[sk] = ttv
		default:
			dest[sk] = sv
		}
	}
}

// ToMapStringInterface cast map[interface{}]interface{} to map[string]interface{}
func ToMapStringInterface(src map[interface{}]interface{}) map[string]interface{} {
	tgt := map[string]interface{}{}
	for k, v := range src {
		tgt[fmt.Sprintf("%v", k)] = v
	}
	return tgt
}

// CoverInterfaceMapToStringMap cover map[interface{}]interface{} to map[string]interface{}
func CoverInterfaceMapToStringMap(src map[string]interface{}) {
	for k, v := range src {
		switch v := v.(type) {
		case map[interface{}]interface{}:
			src[k] = ToMapStringInterface(v)
			CoverInterfaceMapToStringMap(src[k].(map[string]interface{}))
		case map[string]interface{}:
			CoverInterfaceMapToStringMap(src[k].(map[string]interface{}))
		case []interface{}:
			for i, item := range v {
				switch item := item.(type) {
				case map[interface{}]interface{}:
					v[i] = ToMapStringInterface(item)
					CoverInterfaceMapToStringMap(v[i].(map[string]interface{}))
				case map[string]interface{}:
					CoverInterfaceMapToStringMap(v[i].(map[string]interface{}))
				default:
				}
			}
		default:
		}
	}
}

// DeepSearchInMap deep search in map
func DeepSearchInMap(m map[string]interface{}, paths ...string) interface{} {
	tmp := make(map[string]interface{})
	for k, v := range m {
		tmp[k] = v
	}
	for i, k := range paths {
		v, ok := tmp[k]
		if !ok {
			return nil
		}
		tmp, ok = v.(map[string]interface{})
		if !ok {
			if i != len(paths)-1 {
				return nil
			}
			return v
		}
	}
	return tmp
}
