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
	"time"

	"github.com/codesjoy/yggdrasil/pkg/config/source"
)

// Config defines the interface for configuration.
type Config interface {
	Values
	LoadSource(...source.Source) error
	AddWatcher(string, func(WatchEvent)) error
	DelWatcher(string, func(WatchEvent)) error
	ValueToValues(Value) Values
}

// WatchEventType defines the type of watch event.
type WatchEventType uint32

const (
	_ WatchEventType = iota
	// WatchEventUpd represents an update event.
	WatchEventUpd
	// WatchEventAdd represents an add event.
	WatchEventAdd
	// WatchEventDel represents a delete event.
	WatchEventDel
)

// WatchEvent represents a watch event.
type WatchEvent interface {
	Type() WatchEventType
	Value() Value
	Version() uint64
}

// Values defines the interface for values.
type Values interface {
	Get(key string) Value
	GetMulti(keys ...string) Value
	Set(key string, val interface{}) error
	SetMulti(keys []string, values []interface{}) error
	Del(key string) error
	Map() map[string]interface{}
	Scan(v interface{}) error
	Bytes() []byte
}

// Value defines the interface for value.
type Value interface {
	Bool(def ...bool) bool
	Int(def ...int) int
	Int64(def ...int64) int64
	String(def ...string) string
	Float64(def ...float64) float64
	Duration(def ...time.Duration) time.Duration
	StringSlice(def ...[]string) []string
	StringMap(def ...map[string]string) map[string]string
	Map(def ...map[string]interface{}) map[string]interface{}
	Scan(val interface{}) error
	Bytes(def ...[]byte) []byte
}
