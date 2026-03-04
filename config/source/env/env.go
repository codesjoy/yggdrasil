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

// Package env provides a source for loading configuration from environment variables.
package env

import (
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/codesjoy/pkg/utils/xmap"
	"github.com/codesjoy/yggdrasil/v2/config/internal"
	"github.com/codesjoy/yggdrasil/v2/config/source"
	"github.com/mitchellh/mapstructure"
)

type env struct {
	name             string
	prefixes         []string
	strippedPrefixes []string
	parseArray       bool
	arraySep         string
	delimiter        string
}

type builderConfig struct {
	Prefixes         []string `mapstructure:"prefixes"`
	StrippedPrefixes []string `mapstructure:"stripped_prefixes"`
	Delimiter        string   `mapstructure:"delimiter"`
	ParseArray       bool     `mapstructure:"parse_array"`
	ArraySep         string   `mapstructure:"array_sep"`
	Name             string   `mapstructure:"name"`
}

func init() {
	source.RegisterBuilder("env", func(cfg map[string]any) (source.Source, error) {
		buildCfg := &builderConfig{}
		if err := mapstructure.Decode(cfg, buildCfg); err != nil {
			return nil, err
		}
		opts := make([]Option, 0, 3)
		if buildCfg.Delimiter != "" {
			opts = append(opts, SetKeyDelimiter(buildCfg.Delimiter))
		}
		if buildCfg.ParseArray {
			opts = append(opts, WithParseArray(buildCfg.ArraySep))
		}
		if buildCfg.Name != "" {
			opts = append(opts, WithName(buildCfg.Name))
		}
		return NewSource(buildCfg.Prefixes, buildCfg.StrippedPrefixes, opts...), nil
	})
}

func (e *env) parseValue(value string) interface{} {
	if intValue, err := strconv.Atoi(value); err == nil {
		return intValue
	} else if boolValue, err := strconv.ParseBool(value); err == nil {
		return boolValue
	} else if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
		return floatValue
	}
	return value
}

func (e *env) Read() (source.Data, error) {
	result := make(map[string]interface{})
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		value := pair[1]
		key := strings.ToLower(pair[0])
		if len(e.prefixes) > 0 || len(e.strippedPrefixes) > 0 {
			notFound := true
			if _, ok := e.matchPrefix(e.prefixes, key); ok {
				notFound = false
			}
			if match, ok := e.matchPrefix(e.strippedPrefixes, key); ok {
				key = strings.TrimPrefix(key, match+e.delimiter)
				notFound = false
			}
			if notFound {
				continue
			}
		}
		keys := strings.Split(key, e.delimiter)
		slices.Reverse(keys)
		tmp := make(map[string]interface{})
		for i, k := range keys {
			if i == 0 {
				if e.parseArray {
					values := strings.Split(value, e.arraySep)
					if len(values) > 1 {
						tmpVal := make([]interface{}, len(values))
						for j, item := range values {
							tmpVal[j] = e.parseValue(item)
						}
						tmp[k] = tmpVal
						continue
					}
				}
				tmp[k] = e.parseValue(value)
				continue
			}
			tmp = map[string]interface{}{k: tmp}
		}

		xmap.MergeStringMap(result, tmp)
	}

	cs := source.NewMapSourceData(source.PriorityEnv, result)
	return cs, nil
}

func (e *env) matchPrefix(pre []string, s string) (string, bool) {
	for _, p := range pre {
		if internal.HasPrefix(s, p, e.delimiter) {
			return p, true
		}
	}

	return "", false
}

func (e *env) Changeable() bool {
	return false
}

func (e *env) Watch() (<-chan source.Data, error) {
	return nil, nil
}

func (e *env) Name() string {
	return e.name
}

func (e *env) Type() string {
	return "env"
}

func (e *env) Close() error {
	return nil
}

// NewSource returns a new env source.
func NewSource(pre, sp []string, opts ...Option) source.Source {
	for i, item := range pre {
		pre[i] = strings.ToLower(item)
	}

	for i, item := range sp {
		sp[i] = strings.ToLower(item)
	}
	e := &env{prefixes: pre, strippedPrefixes: sp, delimiter: "_", name: strings.Join(pre, "_")}
	for _, opt := range opts {
		opt(e)
	}
	return e
}
