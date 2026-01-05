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

// Package metadata provides metadata manipulation utilities
package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNew tests creating MD from a map
func TestNew(t *testing.T) {
	t.Run("simple map", func(t *testing.T) {
		m := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		md := New(m)

		assert.Equal(t, []string{"value1"}, md["key1"])
		assert.Equal(t, []string{"value2"}, md["key2"])
	})

	t.Run("uppercase keys are lowercased", func(t *testing.T) {
		m := map[string]string{
			"Content-Type":  "application/json",
			"AUTHORIZATION": "Bearer token",
		}

		md := New(m)

		assert.Equal(t, []string{"application/json"}, md["content-type"])
		assert.Equal(t, []string{"Bearer token"}, md["authorization"])
		assert.NotContains(t, md, "Content-Type")
		assert.NotContains(t, md, "AUTHORIZATION")
	})

	t.Run("empty map", func(t *testing.T) {
		m := map[string]string{}

		md := New(m)

		assert.Empty(t, md)
		assert.Equal(t, 0, md.Len())
	})

	t.Run("nil map", func(t *testing.T) {
		m := map[string]string(nil)

		md := New(m)

		assert.Empty(t, md)
		assert.Equal(t, 0, md.Len())
	})

	t.Run("special characters in keys", func(t *testing.T) {
		m := map[string]string{
			"key-with-dash":       "value1",
			"key_with_underscore": "value2",
			"key.with.dot":        "value3",
			"Key123":              "value4",
		}

		md := New(m)

		assert.Equal(t, []string{"value1"}, md["key-with-dash"])
		assert.Equal(t, []string{"value2"}, md["key_with_underscore"])
		assert.Equal(t, []string{"value3"}, md["key.with.dot"])
		assert.Equal(t, []string{"value4"}, md["key123"])
	})
}

// TestPairs tests creating MD from key-value pairs
func TestPairs(t *testing.T) {
	t.Run("simple pairs", func(t *testing.T) {
		md := Pairs("key1", "value1", "key2", "value2")

		assert.Equal(t, []string{"value1"}, md["key1"])
		assert.Equal(t, []string{"value2"}, md["key2"])
	})

	t.Run("uppercase keys are lowercased", func(t *testing.T) {
		md := Pairs("Content-Type", "application/json", "Accept", "text/html")

		assert.Equal(t, []string{"application/json"}, md["content-type"])
		assert.Equal(t, []string{"text/html"}, md["accept"])
	})

	t.Run("odd number of arguments panics", func(t *testing.T) {
		assert.Panics(t, func() {
			Pairs("key1", "value1", "key2") // nolint:staticcheck
		})

		assert.Panics(t, func() {
			Pairs("key1") // nolint:staticcheck
		})

		// Empty (even) doesn't panic
		assert.NotPanics(t, func() {
			Pairs()
		})
	})

	t.Run("even number of arguments", func(t *testing.T) {
		assert.NotPanics(t, func() {
			Pairs("key1", "value1")
		})

		assert.NotPanics(t, func() {
			Pairs("key1", "value1", "key2", "value2", "key3", "value3")
		})
	})

	t.Run("multiple values for same key", func(t *testing.T) {
		md := Pairs("key", "value1", "key", "value2", "key", "value3")

		assert.Equal(t, []string{"value1", "value2", "value3"}, md["key"])
	})

	t.Run("empty values", func(t *testing.T) {
		md := Pairs("key1", "", "key2", "")

		assert.Equal(t, []string{""}, md["key1"])
		assert.Equal(t, []string{""}, md["key2"])
	})

	t.Run("special characters in keys", func(t *testing.T) {
		md := Pairs(
			"x-custom-header", "value1",
			"user_id", "123",
			"request.id", "456",
		)

		assert.Equal(t, []string{"value1"}, md["x-custom-header"])
		assert.Equal(t, []string{"123"}, md["user_id"])
		assert.Equal(t, []string{"456"}, md["request.id"])
	})
}

// TestMD_Len tests the Len method
func TestMD_Len(t *testing.T) {
	t.Run("empty metadata", func(t *testing.T) {
		md := MD{}
		assert.Equal(t, 0, md.Len())
	})

	t.Run("single key", func(t *testing.T) {
		md := New(map[string]string{"key": "value"})
		assert.Equal(t, 1, md.Len())
	})

	t.Run("multiple keys", func(t *testing.T) {
		md := New(map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		})
		assert.Equal(t, 3, md.Len())
	})

	t.Run("nil metadata", func(t *testing.T) {
		var md MD
		assert.Equal(t, 0, md.Len())
	})
}

// TestMD_Copy tests the Copy method
func TestMD_Copy(t *testing.T) {
	t.Run("copy creates independent copy", func(t *testing.T) {
		original := New(map[string]string{
			"key1": "value1",
			"key2": "value2",
		})

		cpMd := original.Copy()

		// Modify copy
		cpMd.Set("key3", "value3")
		cpMd["key2"] = []string{"modified"}

		// Original should be unchanged
		assert.Equal(t, []string{"value1"}, original["key1"])
		assert.Equal(t, []string{"value2"}, original["key2"])
		assert.NotContains(t, original, "key3")
	})

	t.Run("copy empty metadata", func(t *testing.T) {
		original := MD{}
		cpMd := original.Copy()

		assert.Empty(t, cpMd)
		assert.Equal(t, 0, cpMd.Len())
	})

	t.Run("copy preserves all values", func(t *testing.T) {
		original := Pairs("key", "v1", "key", "v2", "key", "v3")

		cpMd := original.Copy()

		assert.Equal(t, []string{"v1", "v2", "v3"}, cpMd["key"])
	})

	t.Run("copy with multiple values per key", func(t *testing.T) {
		original := MD{
			"key1": []string{"value1", "value2"},
			"key2": []string{"value3"},
		}

		cpMD := original.Copy()

		assert.Equal(t, original["key1"], cpMD["key1"])
		assert.Equal(t, original["key2"], cpMD["key2"])
	})
}

// TestMD_Get tests the Get method
func TestMD_Get(t *testing.T) {
	t.Run("get existing key", func(t *testing.T) {
		md := New(map[string]string{
			"key1": "value1",
			"key2": "value2",
		})

		values := md.Get("key1")
		assert.Equal(t, []string{"value1"}, values)
	})

	t.Run("get non-existent key", func(t *testing.T) {
		md := New(map[string]string{"key": "value"})

		values := md.Get("nonexistent")
		assert.Nil(t, values)
	})

	t.Run("get is case-insensitive", func(t *testing.T) {
		md := New(map[string]string{"content-type": "application/json"})

		assert.Equal(t, []string{"application/json"}, md.Get("content-type"))
		assert.Equal(t, []string{"application/json"}, md.Get("CONTENT-TYPE"))
		assert.Equal(t, []string{"application/json"}, md.Get("Content-Type"))
	})

	t.Run("get returns all values", func(t *testing.T) {
		md := Pairs("key", "value1", "key", "value2", "key", "value3")

		values := md.Get("key")
		assert.Equal(t, []string{"value1", "value2", "value3"}, values)
	})

	t.Run("get from empty metadata", func(t *testing.T) {
		md := MD{}

		values := md.Get("key")
		assert.Nil(t, values)
	})
}

// TestMD_Set tests the Set method
func TestMD_Set(t *testing.T) {
	t.Run("set new key", func(t *testing.T) {
		md := MD{}
		md.Set("key", "value")

		assert.Equal(t, []string{"value"}, md["key"])
	})

	t.Run("set overwrites existing values", func(t *testing.T) {
		md := Pairs("key", "value1", "key", "value2")
		assert.Equal(t, []string{"value1", "value2"}, md["key"])

		md.Set("key", "newvalue")
		assert.Equal(t, []string{"newvalue"}, md["key"])
	})

	t.Run("set with multiple values", func(t *testing.T) {
		md := MD{}
		md.Set("key", "value1", "value2", "value3")

		assert.Equal(t, []string{"value1", "value2", "value3"}, md["key"])
	})

	t.Run("set with empty values does nothing", func(t *testing.T) {
		md := New(map[string]string{"key": "oldvalue"})
		md.Set("key")

		// Should keep the old value
		assert.Equal(t, []string{"oldvalue"}, md["key"])
	})

	t.Run("set key is case-insensitive", func(t *testing.T) {
		md := New(map[string]string{"content-type": "application/json"})
		md.Set("CONTENT-TYPE", "text/html")

		assert.Equal(t, []string{"text/html"}, md["content-type"])
	})

	t.Run("set empty string value", func(t *testing.T) {
		md := MD{}
		md.Set("key", "")

		assert.Equal(t, []string{""}, md["key"])
	})
}

// TestMD_Append tests the Append method
func TestMD_Append(t *testing.T) {
	t.Run("append to existing key", func(t *testing.T) {
		md := New(map[string]string{"key": "value1"})
		md.Append("key", "value2")

		assert.Equal(t, []string{"value1", "value2"}, md["key"])
	})

	t.Run("append to non-existent key", func(t *testing.T) {
		md := MD{}
		md.Append("key", "value")

		assert.Equal(t, []string{"value"}, md["key"])
	})

	t.Run("append multiple values", func(t *testing.T) {
		md := New(map[string]string{"key": "value1"})
		md.Append("key", "value2", "value3", "value4")

		assert.Equal(t, []string{"value1", "value2", "value3", "value4"}, md["key"])
	})

	t.Run("append with empty values does nothing", func(t *testing.T) {
		md := New(map[string]string{"key": "value1"})
		md.Append("key")

		assert.Equal(t, []string{"value1"}, md["key"])
	})

	t.Run("append is case-insensitive", func(t *testing.T) {
		md := New(map[string]string{"content-type": "application/json"})
		md.Append("CONTENT-TYPE", "utf-8")

		assert.Equal(t, []string{"application/json", "utf-8"}, md["content-type"])
	})

	t.Run("append preserves existing values", func(t *testing.T) {
		md := Pairs("key", "v1", "key", "v2")
		md.Append("key", "v3", "v4")

		assert.Equal(t, []string{"v1", "v2", "v3", "v4"}, md["key"])
	})
}

// TestJoin tests the Join function
func TestJoin(t *testing.T) {
	t.Run("join multiple metadata", func(t *testing.T) {
		md1 := New(map[string]string{"key1": "value1"})
		md2 := New(map[string]string{"key2": "value2"})
		md3 := New(map[string]string{"key3": "value3"})

		joined := Join(md1, md2, md3)

		assert.Equal(t, []string{"value1"}, joined["key1"])
		assert.Equal(t, []string{"value2"}, joined["key2"])
		assert.Equal(t, []string{"value3"}, joined["key3"])
	})

	t.Run("join with overlapping keys", func(t *testing.T) {
		md1 := Pairs("key", "value1")
		md2 := Pairs("key", "value2")
		md3 := Pairs("key", "value3")

		joined := Join(md1, md2, md3)

		assert.Equal(t, []string{"value1", "value2", "value3"}, joined["key"])
	})

	t.Run("join empty metadata", func(t *testing.T) {
		md1 := New(map[string]string{"key": "value"})
		md2 := MD{}
		md3 := MD{}

		joined := Join(md1, md2, md3)

		assert.Equal(t, []string{"value"}, joined["key"])
	})

	t.Run("join no arguments", func(t *testing.T) {
		joined := Join()

		assert.Empty(t, joined)
		assert.Equal(t, 0, joined.Len())
	})

	t.Run("join preserves order", func(t *testing.T) {
		md1 := Pairs("key", "v1", "key", "v2")
		md2 := Pairs("key", "v3")
		md3 := Pairs("key", "v4", "key", "v5")

		joined := Join(md1, md2, md3)

		assert.Equal(t, []string{"v1", "v2", "v3", "v4", "v5"}, joined["key"])
	})

	t.Run("join creates independent copy", func(t *testing.T) {
		md1 := New(map[string]string{"key1": "value1"})
		md2 := New(map[string]string{"key2": "value2"})

		joined := Join(md1, md2)

		// Modify joined
		joined.Set("key3", "value3")
		joined["key1"] = []string{"modified"}

		// Originals should be unchanged
		assert.Equal(t, []string{"value1"}, md1["key1"])
		assert.Equal(t, []string{"value2"}, md2["key2"])
		assert.NotContains(t, md1, "key3")
		assert.NotContains(t, md2, "key3")
	})

	t.Run("join with nil metadata", func(t *testing.T) {
		md1 := New(map[string]string{"key1": "value1"})
		var md2 MD

		joined := Join(md1, md2)

		assert.Equal(t, []string{"value1"}, joined["key1"])
	})
}

// TestMD_edgeCases tests edge cases
func TestMD_edgeCases(t *testing.T) {
	t.Run("very long key", func(t *testing.T) {
		longKey := string(make([]byte, 1000))
		for i := range longKey {
			longKey = longKey[:i] + "a" + longKey[i+1:]
		}

		md := New(map[string]string{longKey: "value"})
		assert.Equal(t, []string{"value"}, md[longKey])
	})

	t.Run("unicode values", func(t *testing.T) {
		md := Pairs("key", "你好", "key", "世界", "key", "🎉")

		assert.Equal(t, []string{"你好", "世界", "🎉"}, md["key"])
	})

	t.Run("special characters in values", func(t *testing.T) {
		md := Pairs("key", "value\nwith\nnewlines", "key2", "value\twith\ttabs")

		assert.Equal(t, []string{"value\nwith\nnewlines"}, md["key"])
		assert.Equal(t, []string{"value\twith\ttabs"}, md["key2"])
	})

	t.Run("empty string key", func(t *testing.T) {
		md := New(map[string]string{"": "value"})

		assert.Equal(t, []string{"value"}, md[""])
	})

	t.Run("keys with dots", func(t *testing.T) {
		md := Pairs("user.id", "123", "request.id", "456")

		assert.Equal(t, []string{"123"}, md["user.id"])
		assert.Equal(t, []string{"456"}, md["request.id"])
	})
}

// TestMD_RealWorldScenarios tests real-world usage scenarios
func TestMD_RealWorldScenarios(t *testing.T) {
	t.Run("HTTP headers", func(t *testing.T) {
		md := Pairs(
			"Content-Type", "application/json",
			"Authorization", "Bearer token123",
			"Accept", "application/json",
			"User-Agent", "MyApp/1.0",
		)

		assert.Equal(t, []string{"application/json"}, md["content-type"])
		assert.Equal(t, []string{"Bearer token123"}, md["authorization"])
		assert.Equal(t, 4, md.Len())
	})

	t.Run("multiple values for same header", func(t *testing.T) {
		md := Pairs(
			"Accept", "text/html",
			"Accept", "application/xhtml+xml",
			"Accept", "application/xml",
		)

		assert.Equal(
			t,
			[]string{"text/html", "application/xhtml+xml", "application/xml"},
			md["accept"],
		)
	})

	t.Run("metadata propagation", func(t *testing.T) {
		// Simulate metadata flowing through a system
		initial := Pairs("request-id", "123", "user-id", "456")

		// Add more metadata downstream (independent metadata)
		downstream1 := Pairs("service", "auth", "request-id", "123")

		// Join both metadata
		downstream2 := Join(initial, downstream1)
		downstream2.Set("status", "processed")

		// Initial should be unchanged
		assert.Equal(t, []string{"123"}, initial["request-id"])
		assert.NotContains(t, initial, "service")

		// Downstream should have all metadata
		assert.Equal(t, []string{"123", "123"}, downstream2["request-id"])
		assert.Equal(t, []string{"auth"}, downstream2["service"])
		assert.Equal(t, []string{"processed"}, downstream2["status"])
		assert.Equal(t, []string{"456"}, downstream2["user-id"])
	})
}
