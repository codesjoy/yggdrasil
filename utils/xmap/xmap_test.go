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

package xmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMergeStringMap_Simple tests simple map merging
func TestMergeStringMap_Simple(t *testing.T) {
	dst := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	src := map[string]interface{}{
		"key2": "new_value2",
		"key3": "value3",
	}

	MergeStringMap(dst, src)

	assert.Equal(t, "value1", dst["key1"], "key1 should remain unchanged")
	assert.Equal(t, "new_value2", dst["key2"], "key2 should be updated")
	assert.Equal(t, "value3", dst["key3"], "key3 should be added")
}

// TestMergeStringMap_Nested tests merging nested maps
func TestMergeStringMap_Nested(t *testing.T) {
	dst := map[string]interface{}{
		"config": map[string]interface{}{
			"host": "localhost",
			"port": 8080,
			"database": map[string]interface{}{
				"name": "mydb",
			},
		},
	}
	src := map[string]interface{}{
		"config": map[string]interface{}{
			"port":    9090,
			"timeout": 30,
			"database": map[string]interface{}{
				"user": "admin",
			},
		},
	}

	MergeStringMap(dst, src)

	config := dst["config"].(map[string]interface{})
	assert.Equal(t, "localhost", config["host"], "host should remain unchanged")
	assert.Equal(t, 9090, config["port"], "port should be updated")
	assert.Equal(t, 30, config["timeout"], "timeout should be added")

	db := config["database"].(map[string]interface{})
	assert.Equal(t, "mydb", db["name"], "database name should remain unchanged")
	assert.Equal(t, "admin", db["user"], "database user should be added")
}

// TestMergeStringMap_MultipleSources tests merging multiple sources
func TestMergeStringMap_MultipleSources(t *testing.T) {
	dst := map[string]interface{}{
		"key": "original",
	}
	src1 := map[string]interface{}{
		"key": "from_src1",
		"a":   1,
	}
	src2 := map[string]interface{}{
		"key": "from_src2",
		"b":   2,
	}
	src3 := map[string]interface{}{
		"c": 3,
	}

	MergeStringMap(dst, src1, src2, src3)

	assert.Equal(t, "from_src2", dst["key"], "key should be from src2 (last merge)")
	assert.Equal(t, 1, dst["a"], "a should be added")
	assert.Equal(t, 2, dst["b"], "b should be added")
	assert.Equal(t, 3, dst["c"], "c should be added")
}

// TestMergeStringMap_EmptyMaps tests merging with empty maps
func TestMergeStringMap_EmptyMaps(t *testing.T) {
	t.Run("empty destination", func(t *testing.T) {
		dst := map[string]interface{}{}
		src := map[string]interface{}{
			"key": "value",
		}

		MergeStringMap(dst, src)

		assert.Equal(t, "value", dst["key"])
	})

	t.Run("empty source", func(t *testing.T) {
		dst := map[string]interface{}{
			"key": "value",
		}
		src := map[string]interface{}{}

		MergeStringMap(dst, src)

		assert.Equal(t, "value", dst["key"])
	})

	t.Run("both empty", func(t *testing.T) {
		dst := map[string]interface{}{}
		src := map[string]interface{}{}

		MergeStringMap(dst, src)

		assert.Empty(t, dst)
	})

	t.Run("no sources", func(t *testing.T) {
		dst := map[string]interface{}{
			"key": "value",
		}

		MergeStringMap(dst)

		assert.Equal(t, "value", dst["key"])
	})
}

// TestMergeStringMap_TypeMismatch tests handling of type mismatches
func TestMergeStringMap_TypeMismatch(t *testing.T) {
	dst := map[string]interface{}{
		"key": "string_value",
	}
	src := map[string]interface{}{
		"key": 123, // Different type
	}

	MergeStringMap(dst, src)

	// Should skip when types don't match
	assert.Equal(t, "string_value", dst["key"], "key should remain unchanged when types differ")
}

// TestMergeStringMap_DeepNesting tests deeply nested map merging
func TestMergeStringMap_DeepNesting(t *testing.T) {
	dst := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": map[string]interface{}{
					"key": "original",
				},
			},
		},
	}
	src := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": map[string]interface{}{
					"key": "updated",
					"new": "value",
					"deep": map[string]interface{}{
						"nested": "value",
					},
				},
			},
		},
	}

	MergeStringMap(dst, src)

	level1 := dst["level1"].(map[string]interface{})
	level2 := level1["level2"].(map[string]interface{})
	level3 := level2["level3"].(map[string]interface{})

	assert.Equal(t, "updated", level3["key"])
	assert.Equal(t, "value", level3["new"])
	assert.Equal(t, map[string]interface{}{"nested": "value"}, level3["deep"])
}

// TestToMapStringInterface tests converting map[interface{}]interface{} to map[string]interface{}
func TestToMapStringInterface(t *testing.T) {
	t.Run("simple keys", func(t *testing.T) {
		src := map[interface{}]interface{}{
			"key1": "value1",
			"key2": 123,
			"key3": true,
		}

		result := ToMapStringInterface(src)

		assert.Equal(t, "value1", result["key1"])
		assert.Equal(t, 123, result["key2"])
		assert.Equal(t, true, result["key3"])
	})

	t.Run("integer keys", func(t *testing.T) {
		src := map[interface{}]interface{}{
			1: "one",
			2: "two",
			3: "three",
		}

		result := ToMapStringInterface(src)

		assert.Equal(t, "one", result["1"])
		assert.Equal(t, "two", result["2"])
		assert.Equal(t, "three", result["3"])
	})

	t.Run("mixed type keys", func(t *testing.T) {
		src := map[interface{}]interface{}{
			"string": "string_value",
			123:      "int_value",
			true:     "bool_value",
			3.14:     "float_value",
		}

		result := ToMapStringInterface(src)

		assert.Equal(t, "string_value", result["string"])
		assert.Equal(t, "int_value", result["123"])
		assert.Equal(t, "bool_value", result["true"])
		assert.Equal(t, "float_value", result["3.14"])
	})

	t.Run("empty map", func(t *testing.T) {
		src := map[interface{}]interface{}{}

		result := ToMapStringInterface(src)

		assert.Empty(t, result)
	})

	t.Run("nested values", func(t *testing.T) {
		src := map[interface{}]interface{}{
			"nested": map[interface{}]interface{}{
				"inner": "value",
			},
		}

		result := ToMapStringInterface(src)

		nested := result["nested"].(map[interface{}]interface{})
		assert.Equal(t, "value", nested["inner"])
	})
}

// TestCoverInterfaceMapToStringMap tests converting nested maps
func TestCoverInterfaceMapToStringMap(t *testing.T) {
	t.Run("simple nested map", func(t *testing.T) {
		src := map[string]interface{}{
			"key1": "value1",
			"key2": map[interface{}]interface{}{
				"inner_key": "inner_value",
			},
		}

		CoverInterfaceMapToStringMap(src)

		assert.Equal(t, "value1", src["key1"])
		assert.IsType(t, map[string]interface{}{}, src["key2"])

		key2 := src["key2"].(map[string]interface{})
		assert.Equal(t, "inner_value", key2["inner_key"])
	})

	t.Run("deeply nested maps", func(t *testing.T) {
		src := map[string]interface{}{
			"level1": map[interface{}]interface{}{
				"level2": map[interface{}]interface{}{
					"level3": map[interface{}]interface{}{
						"value": "deep",
					},
				},
			},
		}

		CoverInterfaceMapToStringMap(src)

		level1 := src["level1"].(map[string]interface{})
		level2 := level1["level2"].(map[string]interface{})
		level3 := level2["level3"].(map[string]interface{})

		assert.Equal(t, "deep", level3["value"])
	})

	t.Run("array of maps", func(t *testing.T) {
		src := map[string]interface{}{
			"items": []interface{}{
				map[interface{}]interface{}{
					"id":   1,
					"name": "item1",
				},
				map[interface{}]interface{}{
					"id":   2,
					"name": "item2",
				},
			},
		}

		CoverInterfaceMapToStringMap(src)

		items := src["items"].([]interface{})
		item1 := items[0].(map[string]interface{})
		item2 := items[1].(map[string]interface{})

		assert.Equal(t, 1, item1["id"])
		assert.Equal(t, "item1", item1["name"])
		assert.Equal(t, 2, item2["id"])
		assert.Equal(t, "item2", item2["name"])
	})

	t.Run("mixed nested structures", func(t *testing.T) {
		src := map[string]interface{}{
			"simple": "value",
			"nested": map[interface{}]interface{}{
				"key": "value",
			},
			"array": []interface{}{
				"string",
				123,
				map[interface{}]interface{}{
					"array_key": "array_value",
				},
			},
			"already_string_map": map[string]interface{}{
				"key": "value",
			},
		}

		CoverInterfaceMapToStringMap(src)

		assert.Equal(t, "value", src["simple"])

		nested := src["nested"].(map[string]interface{})
		assert.Equal(t, "value", nested["key"])

		array := src["array"].([]interface{})
		assert.Equal(t, "string", array[0])
		assert.Equal(t, 123, array[1])

		arrayMap := array[2].(map[string]interface{})
		assert.Equal(t, "array_value", arrayMap["array_key"])

		alreadyMap := src["already_string_map"].(map[string]interface{})
		assert.Equal(t, "value", alreadyMap["key"])
	})

	t.Run("empty map", func(t *testing.T) {
		src := map[string]interface{}{}

		CoverInterfaceMapToStringMap(src)

		assert.Empty(t, src)
	})

	t.Run("nil and non-map values", func(t *testing.T) {
		src := map[string]interface{}{
			"string": "value",
			"number": 123,
			"bool":   true,
			"nil":    nil,
		}

		CoverInterfaceMapToStringMap(src)

		assert.Equal(t, "value", src["string"])
		assert.Equal(t, 123, src["number"])
		assert.Equal(t, true, src["bool"])
		assert.Nil(t, src["nil"])
	})
}

// TestDeepSearchInMap tests deep search in nested maps
func TestDeepSearchInMap(t *testing.T) {
	t.Run("find single level key", func(t *testing.T) {
		m := map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}

		result := DeepSearchInMap(m, "key1")

		assert.Equal(t, "value1", result)
	})

	t.Run("find nested key", func(t *testing.T) {
		m := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": "found_value",
				},
			},
		}

		result := DeepSearchInMap(m, "level1", "level2", "level3")

		assert.Equal(t, "found_value", result)
	})

	t.Run("non-existent key returns nil", func(t *testing.T) {
		m := map[string]interface{}{
			"key": "value",
		}

		result := DeepSearchInMap(m, "nonexistent")

		assert.Nil(t, result)
	})

	t.Run("partial path returns nil", func(t *testing.T) {
		m := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": "value",
			},
		}

		result := DeepSearchInMap(m, "level1", "level2", "level3", "level4")

		assert.Nil(t, result)
	})

	t.Run("path terminates at non-map value", func(t *testing.T) {
		m := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": "final_value",
			},
		}

		result := DeepSearchInMap(m, "level1", "level2")

		assert.Equal(t, "final_value", result)
	})

	t.Run("intermediate value is not a map", func(t *testing.T) {
		m := map[string]interface{}{
			"level1": "not_a_map",
		}

		result := DeepSearchInMap(m, "level1", "level2")

		assert.Nil(t, result)
	})

	t.Run("empty path returns nil", func(t *testing.T) {
		m := map[string]interface{}{
			"key": "value",
		}

		result := DeepSearchInMap(m)

		assert.Equal(t, map[string]interface{}{"key": "value"}, result)
	})

	t.Run("complex nested structure", func(t *testing.T) {
		m := map[string]interface{}{
			"config": map[string]interface{}{
				"database": map[string]interface{}{
					"host":     "localhost",
					"port":     5432,
					"username": "admin",
					"password": "secret",
				},
				"server": map[string]interface{}{
					"port": 8080,
					"host": "0.0.0.0",
				},
			},
		}

		result := DeepSearchInMap(m, "config", "database", "host")

		assert.Equal(t, "localhost", result)
	})

	t.Run("original map is not modified", func(t *testing.T) {
		m := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"value": "original",
				},
			},
		}

		originalValue := m["level1"].(map[string]interface{})["level2"].(map[string]interface{})["value"]

		_ = DeepSearchInMap(m, "level1", "level2", "value")

		// Original map should not be modified
		assert.Equal(t, "original", originalValue)
	})

	t.Run("find numeric value", func(t *testing.T) {
		m := map[string]interface{}{
			"config": map[string]interface{}{
				"port": 8080,
			},
		}

		result := DeepSearchInMap(m, "config", "port")

		assert.Equal(t, 8080, result)
	})

	t.Run("find boolean value", func(t *testing.T) {
		m := map[string]interface{}{
			"settings": map[string]interface{}{
				"enabled": true,
			},
		}

		result := DeepSearchInMap(m, "settings", "enabled")

		assert.Equal(t, true, result)
	})

	t.Run("find nil value", func(t *testing.T) {
		m := map[string]interface{}{
			"key": nil,
		}

		result := DeepSearchInMap(m, "key")

		assert.Nil(t, result)
	})
}

// TestMergeStringMap_RealWorldScenario tests a real-world configuration merging scenario
func TestMergeStringMap_RealWorldScenario(t *testing.T) {
	defaultConfig := map[string]interface{}{
		"server": map[string]interface{}{
			"host": "0.0.0.0",
			"port": 8080,
		},
		"database": map[string]interface{}{
			"host": "localhost",
			"port": 5432,
			"name": "myapp",
			"ssl":  false,
			"pool": map[string]interface{}{
				"max":     10,
				"min":     2,
				"timeout": 30,
			},
		},
		"logging": map[string]interface{}{
			"level":  "info",
			"format": "json",
		},
	}

	userConfig := map[string]interface{}{
		"server": map[string]interface{}{
			"port": 9000,
		},
		"database": map[string]interface{}{
			"host": "db.example.com",
			"pool": map[string]interface{}{
				"max": 20,
			},
		},
	}

	envConfig := map[string]interface{}{
		"database": map[string]interface{}{
			"password": "secret",
			"ssl":      true,
		},
		"logging": map[string]interface{}{
			"level": "debug",
		},
	}

	MergeStringMap(defaultConfig, userConfig, envConfig)

	// Verify server config
	server := defaultConfig["server"].(map[string]interface{})
	assert.Equal(t, "0.0.0.0", server["host"], "default host should be preserved")
	assert.Equal(t, 9000, server["port"], "user port should override default")

	// Verify database config
	db := defaultConfig["database"].(map[string]interface{})
	assert.Equal(t, "db.example.com", db["host"], "user host should override default")
	assert.Equal(t, 5432, db["port"], "default port should be preserved")
	assert.Equal(t, "myapp", db["name"], "default name should be preserved")
	assert.Equal(t, true, db["ssl"], "env ssl should override user and default")
	assert.Equal(t, "secret", db["password"], "env password should be added")

	pool := db["pool"].(map[string]interface{})
	assert.Equal(t, 20, pool["max"], "user max pool should override default")
	assert.Equal(t, 2, pool["min"], "default min pool should be preserved")
	assert.Equal(t, 30, pool["timeout"], "default timeout should be preserved")

	// Verify logging config
	logging := defaultConfig["logging"].(map[string]interface{})
	assert.Equal(t, "debug", logging["level"], "env level should override default")
	assert.Equal(t, "json", logging["format"], "default format should be preserved")
}

// TestCoverInterfaceMapToStringMap_ComplexStructure tests complex structure conversion
func TestCoverInterfaceMapToStringMap_ComplexStructure(t *testing.T) {
	src := map[string]interface{}{
		"config": map[interface{}]interface{}{
			"servers": []interface{}{
				map[interface{}]interface{}{
					"host": "server1.example.com",
					"port": 8080,
				},
				map[interface{}]interface{}{
					"host": "server2.example.com",
					"port": 8081,
				},
			},
			"database": map[interface{}]interface{}{
				"primary": map[interface{}]interface{}{
					"host":     "db1.example.com",
					"port":     5432,
					"database": "production",
				},
				"replica": map[interface{}]interface{}{
					"host":     "db2.example.com",
					"port":     5433,
					"database": "production",
				},
			},
			"features": map[interface{}]interface{}{
				"enabled": []interface{}{"feature1", "feature2"},
				"beta":    []interface{}{"beta1"},
			},
		},
	}

	CoverInterfaceMapToStringMap(src)

	config := src["config"].(map[string]interface{})
	servers := config["servers"].([]interface{})

	server1 := servers[0].(map[string]interface{})
	assert.Equal(t, "server1.example.com", server1["host"])
	assert.Equal(t, 8080, server1["port"])

	server2 := servers[1].(map[string]interface{})
	assert.Equal(t, "server2.example.com", server2["host"])
	assert.Equal(t, 8081, server2["port"])

	database := config["database"].(map[string]interface{})
	primary := database["primary"].(map[string]interface{})
	assert.Equal(t, "db1.example.com", primary["host"])
	assert.Equal(t, 5432, primary["port"])
	assert.Equal(t, "production", primary["database"])

	replica := database["replica"].(map[string]interface{})
	assert.Equal(t, "db2.example.com", replica["host"])
	assert.Equal(t, 5433, replica["port"])
	assert.Equal(t, "production", replica["database"])

	features := config["features"].(map[string]interface{})
	enabled := features["enabled"].([]interface{})
	assert.Equal(t, "feature1", enabled[0])
	assert.Equal(t, "feature2", enabled[1])
}
