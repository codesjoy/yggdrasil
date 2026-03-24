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

package source_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/config/source"
	_ "github.com/codesjoy/yggdrasil/v2/config/source/env"
	_ "github.com/codesjoy/yggdrasil/v2/config/source/file"
	_ "github.com/codesjoy/yggdrasil/v2/config/source/flag"
)

type testSource struct{}

func (s *testSource) Type() string                       { return "test" }
func (s *testSource) Name() string                       { return "test" }
func (s *testSource) Read() (source.Data, error)         { return nil, nil }
func (s *testSource) Changeable() bool                   { return false }
func (s *testSource) Watch() (<-chan source.Data, error) { return nil, nil }
func (s *testSource) Close() error                       { return nil }

func TestSourceBuilderRegistry(t *testing.T) {
	const builderType = "unit-test-source"
	source.RegisterBuilder(builderType, func(cfg map[string]any) (source.Source, error) {
		assert.Equal(t, "ok", cfg["case"])
		return &testSource{}, nil
	})

	builder := source.GetBuilder(builderType)
	require.NotNil(t, builder)

	ss, err := source.New(builderType, map[string]any{"case": "ok"})
	require.NoError(t, err)
	require.NotNil(t, ss)
	assert.Equal(t, "test", ss.Type())
}

func TestSourceNewUnknownType(t *testing.T) {
	_, err := source.New("unknown-test-source", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBuiltinSourceBuilders(t *testing.T) {
	_, err := source.New("flag", nil)
	require.NoError(t, err)

	_, err = source.New("env", map[string]any{
		"prefixes": []string{"APP_"},
	})
	require.NoError(t, err)

	_, err = source.New("file", map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path")
}
