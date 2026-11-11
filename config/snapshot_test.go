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
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshotSectionAndDecode(t *testing.T) {
	snapshot := NewSnapshot(map[string]any{
		"clients": map[string]any{
			"services": map[string]any{
				"github.com.demo.service": map[string]any{
					"resolver": "mock",
				},
			},
		},
	})

	var resolver string
	require.NoError(
		t,
		snapshot.Section("clients", "services", "github.com.demo.service", "resolver").
			Decode(&resolver),
	)
	require.Equal(t, "mock", resolver)
}

func TestSnapshotHelpersAndDecodeEdgeCases(t *testing.T) {
	snapshot := NewSnapshot(map[string]any{
		"app": map[string]any{
			"name": "demo",
		},
		"value": 12,
	})

	m := snapshot.Map()
	m["app"].(map[string]any)["name"] = "changed"
	require.Equal(t, "demo", snapshot.Map()["app"].(map[string]any)["name"])
	require.Contains(t, string(snapshot.Bytes()), `"value":12`)
	require.False(t, snapshot.Empty())

	value := snapshot.Value().(map[string]any)
	value["value"] = 100
	require.Equal(t, 12, snapshot.Value().(map[string]any)["value"])

	empty := NewSnapshot(nil)
	require.True(t, empty.Empty())
	require.Equal(t, map[string]any{}, empty.Map())

	var nonPtr int
	require.Error(t, snapshot.Decode(nonPtr))
	var nilPtr *int
	require.Error(t, snapshot.Decode(nilPtr))

	type withDefault struct {
		Port int `default:"8080"`
	}
	var cfg withDefault
	require.NoError(t, empty.Decode(&cfg))
	require.Equal(t, 8080, cfg.Port)

	var structTarget struct {
		Value int `mapstructure:"value"`
	}
	require.Error(t, NewSnapshot("bad").Decode(&structTarget))

	var intTarget int
	require.Error(t, NewSnapshot("bad").Decode(&intTarget))
}

func TestSnapshotDeepClonesTypedValuesAndDecodeResults(t *testing.T) {
	type child struct {
		Labels map[string]string
	}
	type typedConfig struct {
		Values     []string
		Index      map[string]int
		Fixed      [2]string
		Child      *child
		private    []string
		privateMap map[string][]string
	}

	source := &typedConfig{
		Values: []string{"one", "two"},
		Index:  map[string]int{"one": 1},
		Fixed:  [2]string{"left", "right"},
		Child: &child{
			Labels: map[string]string{"env": "test"},
		},
		private:    []string{"hidden"},
		privateMap: map[string][]string{"hidden": {"value"}},
	}
	snapshot := NewSnapshot(source)

	source.Values[0] = "mutated"
	source.Index["one"] = 99
	source.Child.Labels["env"] = "mutated"
	source.private[0] = "mutated"
	source.privateMap["hidden"][0] = "mutated"

	fromValue, ok := snapshot.Value().(*typedConfig)
	require.True(t, ok)
	require.Equal(t, "one", fromValue.Values[0])
	require.Equal(t, 1, fromValue.Index["one"])
	require.Equal(t, "test", fromValue.Child.Labels["env"])
	require.Equal(t, "hidden", fromValue.private[0])
	require.Equal(t, "value", fromValue.privateMap["hidden"][0])
	fromValue.Values[1] = "mutated"
	fromValue.private[0] = "value-mutated"
	fromValue.privateMap["hidden"][0] = "value-mutated"
	require.Equal(t, "two", snapshot.Value().(*typedConfig).Values[1])
	require.Equal(t, "hidden", snapshot.Value().(*typedConfig).private[0])
	require.Equal(t, "value", snapshot.Value().(*typedConfig).privateMap["hidden"][0])

	var decoded *typedConfig
	require.NoError(t, snapshot.Decode(&decoded))
	decoded.Values[0] = "decoded"
	decoded.Index["one"] = 42
	decoded.Child.Labels["env"] = "decoded"
	decoded.private[0] = "decoded"
	decoded.privateMap["hidden"][0] = "decoded"
	require.Equal(t, "one", snapshot.Value().(*typedConfig).Values[0])
	require.Equal(t, 1, snapshot.Value().(*typedConfig).Index["one"])
	require.Equal(t, "test", snapshot.Value().(*typedConfig).Child.Labels["env"])
	require.Equal(t, "hidden", snapshot.Value().(*typedConfig).private[0])
	require.Equal(t, "value", snapshot.Value().(*typedConfig).privateMap["hidden"][0])
}

func TestSnapshotPreservesCyclesAndRepeatedReferences(t *testing.T) {
	type node struct {
		Next *node
		Data map[string]any
	}
	type graph struct {
		Left  *node
		Right *node
	}

	shared := &node{Data: map[string]any{}}
	shared.Next = shared
	shared.Data["self"] = shared.Data
	snapshot := NewSnapshot(graph{Left: shared, Right: shared})

	cloned := snapshot.Value().(graph)
	require.Same(t, cloned.Left, cloned.Right)
	require.Same(t, cloned.Left, cloned.Left.Next)
	require.Equal(
		t,
		reflect.ValueOf(cloned.Left.Data).UnsafePointer(),
		reflect.ValueOf(cloned.Left.Data["self"]).UnsafePointer(),
	)

	shared.Data["value"] = "source"
	require.NotContains(t, cloned.Left.Data, "value")
	cloned.Left.Data["value"] = "clone"
	value := snapshot.Value().(graph)
	require.NotContains(t, value.Left.Data, "value")
}

func TestSnapshotDecodeDoesNotAliasNestedTypedMapValues(t *testing.T) {
	type typedConfig struct {
		Values []string
		Index  map[string]int
	}
	value := &typedConfig{
		Values: []string{"one"},
		Index:  map[string]int{"one": 1},
	}
	snapshot := NewSnapshot(map[string]any{"typed": value})
	value.Values[0] = "mutated"

	var decoded struct {
		Typed *typedConfig `mapstructure:"typed"`
	}
	require.NoError(t, snapshot.Decode(&decoded))
	decoded.Typed.Values[0] = "decoded"
	decoded.Typed.Index["one"] = 42

	original := snapshot.Value().(map[string]any)["typed"].(*typedConfig)
	require.Equal(t, "one", original.Values[0])
	require.Equal(t, 1, original.Index["one"])
}

func TestLookupAndSetPath(t *testing.T) {
	root := map[string]any{
		"app": map[string]any{
			"name": "demo",
		},
	}

	all := Lookup(root).(map[string]any)
	all["app"].(map[string]any)["name"] = "changed"
	require.Equal(t, "demo", root["app"].(map[string]any)["name"])

	require.Nil(t, Lookup(root, "missing"))
	require.Nil(t, Lookup(map[string]any{"app": "value"}, "app", "name"))

	SetPath(root, 9090, "app", "server", "port")
	require.Equal(t, 9090, root["app"].(map[string]any)["server"].(map[string]any)["port"])

	root["app"] = "not-map"
	SetPath(root, "replaced", "app", "name")
	require.Equal(t, "replaced", root["app"].(map[string]any)["name"])

	before := root["app"]
	SetPath(root, "noop")
	require.Equal(t, before, root["app"])
}
