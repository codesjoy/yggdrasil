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

// Package config provides a simple way to manage configuration.
package config

import (
	"time"

	"github.com/codesjoy/yggdrasil/pkg/config/source"
)

const keyDelimiter = "."

var cfg = NewConfig(keyDelimiter)

// Get returns the value of the specified key.
func Get(key string) Value {
	return cfg.Get(key)
}

// GetMulti returns the value of the specified keys.
func GetMulti(keys ...string) Value {
	return cfg.GetMulti(keys...)
}

// ValueToValues converts a Value to Values.
func ValueToValues(v Value) Values {
	return cfg.ValueToValues(v)
}

// Set sets the value of the specified key.
func Set(key string, val interface{}) error {
	return cfg.Set(key, val)
}

// SetMulti sets the values of the specified keys.
func SetMulti(keys []string, values []interface{}) error {
	return cfg.SetMulti(keys, values)
}

// Bytes returns the bytes of the configuration.
func Bytes() []byte {
	return cfg.Bytes()
}

// GetBool returns the bool value of the specified key.
func GetBool(key string, def ...bool) bool {
	return cfg.Get(key).Bool(def...)
}

// GetInt returns the int value of the specified key.
func GetInt(key string, def ...int) int {
	return cfg.Get(key).Int(def...)
}

// GetInt64 returns the int64 value of the specified key.
func GetInt64(key string, def ...int64) int64 {
	return cfg.Get(key).Int64(def...)
}

// GetString returns the string value of the specified key.
func GetString(key string, def ...string) string {
	return cfg.Get(key).String(def...)
}

// GetBytes returns the bytes value of the specified key.
func GetBytes(key string, def ...[]byte) []byte {
	return cfg.Get(key).Bytes(def...)
}

// GetStringSlice returns the string slice value of the specified key.
func GetStringSlice(key string, def ...[]string) []string {
	return cfg.Get(key).StringSlice(def...)
}

// GetStringMap returns the string map value of the specified key.
func GetStringMap(key string, def ...map[string]string) map[string]string {
	return cfg.Get(key).StringMap(def...)
}

// GetMap returns the map value of the specified key.
func GetMap(key string, def ...map[string]interface{}) map[string]interface{} {
	return cfg.Get(key).Map(def...)
}

// GetFloat64 returns the float64 value of the specified key.
func GetFloat64(key string, def ...float64) float64 {
	return cfg.Get(key).Float64(def...)
}

// GetDuration returns the duration value of the specified key.
func GetDuration(key string, def ...time.Duration) time.Duration {
	return cfg.Get(key).Duration(def...)
}

// Scan scans the value of the specified key to the specified value.
func Scan(key string, val interface{}) error {
	return cfg.Get(key).Scan(val)
}

// LoadSource loads the source data.
func LoadSource(sources ...source.Source) error {
	return cfg.LoadSource(sources...)
}

// AddWatcher adds a watcher for the specified key.
func AddWatcher(key string, f func(WatchEvent)) error {
	return cfg.AddWatcher(key, f)
}

// DelWatcher deletes the watcher for the specified key.
func DelWatcher(key string, f func(WatchEvent)) error {
	return cfg.DelWatcher(key, f)
}
