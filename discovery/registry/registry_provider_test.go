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

package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	p := NewProvider("test-type", func(cfg map[string]any) (Registry, error) {
		return &testRegistry{kind: "test-type"}, nil
	})
	assert.Equal(t, "test-type", p.Type())
	reg, err := p.New(nil)
	require.NoError(t, err)
	assert.Equal(t, "test-type", reg.Type())
}

func TestConfigureProvidersErrors(t *testing.T) {
	t.Run("nil items skipped", func(t *testing.T) {
		err := ConfigureProviders([]Provider{nil})
		require.NoError(t, err)
	})

	t.Run("empty type returns error", func(t *testing.T) {
		p := NewProvider("", func(cfg map[string]any) (Registry, error) { return nil, nil })
		err := ConfigureProviders([]Provider{p})
		require.Error(t, err)
		assert.ErrorContains(t, err, "type is empty")
	})

	t.Run("duplicate type returns error", func(t *testing.T) {
		p := NewProvider("dup-reg", func(cfg map[string]any) (Registry, error) { return nil, nil })
		err := ConfigureProviders([]Provider{p, p})
		require.Error(t, err)
		assert.ErrorContains(t, err, "duplicate")
	})
}

func TestGetProvider(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		p := NewProvider("get-test-reg", func(cfg map[string]any) (Registry, error) {
			return &testRegistry{kind: "get-test-reg"}, nil
		})
		err := ConfigureProviders([]Provider{p})
		require.NoError(t, err)
		got := GetProvider("get-test-reg")
		require.NotNil(t, got)
		assert.Equal(t, "get-test-reg", got.Type())
	})

	t.Run("not found", func(t *testing.T) {
		assert.Nil(t, GetProvider("nonexistent-reg"))
	})
}

func TestNew(t *testing.T) {
	t.Run("creates registry", func(t *testing.T) {
		RegisterBuilder("new-test", func(cfg map[string]any) (Registry, error) {
			return &testRegistry{kind: "new-test"}, nil
		})
		reg, err := New("new-test", nil)
		require.NoError(t, err)
		assert.Equal(t, "new-test", reg.Type())
	})

	t.Run("unknown type returns error", func(t *testing.T) {
		_, err := New("unknown-type", nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "not found")
	})
}

func TestGet_NoType(t *testing.T) {
	// Reset with empty spec
	Configure(Spec{})
	_, err := Get()
	require.Error(t, err)
	assert.ErrorContains(t, err, "not found registry type")
}

func TestCurrentSpec(t *testing.T) {
	spec := Spec{Type: "mock", Config: map[string]any{"k": "v"}}
	Configure(spec)
	got := CurrentSpec()
	assert.Equal(t, "mock", got.Type)
}

func TestBuiltinProvider(t *testing.T) {
	p := BuiltinProvider()
	assert.Equal(t, "multi_registry", p.Type())
}

func TestBuiltinProviderWithFactory(t *testing.T) {
	called := false
	factory := func(typeName string, cfg map[string]any) (Registry, error) {
		called = true
		return &testRegistry{kind: typeName}, nil
	}
	p := BuiltinProviderWithFactory(factory)
	assert.Equal(t, "multi_registry", p.Type())

	reg, err := p.New(map[string]any{
		"registries": []any{
			map[string]any{"type": "child", "config": map[string]any{}},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "multi_registry", reg.Type())
	assert.True(t, called)
}

func TestMultiRegistry_RegisterDeregister(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mr := &multiRegistry{
			registries: []Registry{
				&testRegistry{kind: "a"},
				&testRegistry{kind: "b"},
			},
		}
		err := mr.Register(context.Background(), nil)
		assert.NoError(t, err)
		err = mr.Deregister(context.Background(), nil)
		assert.NoError(t, err)
	})

	t.Run("failFast", func(t *testing.T) {
		failReg := &errorRegistry{}
		mr := &multiRegistry{
			failFast:   true,
			registries: []Registry{failReg},
		}
		err := mr.Register(context.Background(), nil)
		assert.Error(t, err)
		err = mr.Deregister(context.Background(), nil)
		assert.Error(t, err)
	})

	t.Run("join errors", func(t *testing.T) {
		failReg := &errorRegistry{}
		mr := &multiRegistry{
			failFast:   false,
			registries: []Registry{failReg, failReg},
		}
		err := mr.Register(context.Background(), nil)
		assert.Error(t, err)
	})
}

type errorRegistry struct{}

func (e *errorRegistry) Register(context.Context, Instance) error   { return assert.AnError }
func (e *errorRegistry) Deregister(context.Context, Instance) error { return assert.AnError }
func (e *errorRegistry) Type() string                               { return "error" }
