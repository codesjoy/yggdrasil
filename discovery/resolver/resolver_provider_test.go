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

package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	p := NewProvider("test-type", func(name string) (Resolver, error) {
		return &testResolver{kind: name}, nil
	})
	assert.Equal(t, "test-type", p.Type())
	r, err := p.New("svc")
	require.NoError(t, err)
	assert.Equal(t, "svc", r.Type())
}

func TestConfigureProvidersErrors(t *testing.T) {
	t.Run("nil items skipped", func(t *testing.T) {
		err := ConfigureProviders([]Provider{nil})
		require.NoError(t, err)
	})

	t.Run("empty type returns error", func(t *testing.T) {
		p := NewProvider("", func(name string) (Resolver, error) { return nil, nil })
		err := ConfigureProviders([]Provider{p})
		require.Error(t, err)
		assert.ErrorContains(t, err, "type is empty")
	})

	t.Run("duplicate type returns error", func(t *testing.T) {
		p := NewProvider("dup-res", func(name string) (Resolver, error) { return nil, nil })
		err := ConfigureProviders([]Provider{p, p})
		require.Error(t, err)
		assert.ErrorContains(t, err, "duplicate")
	})
}

func TestGetProvider(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		p := NewProvider("get-test-res", func(name string) (Resolver, error) {
			return &testResolver{kind: name}, nil
		})
		err := ConfigureProviders([]Provider{p})
		require.NoError(t, err)
		got := GetProvider("get-test-res")
		require.NotNil(t, got)
	})

	t.Run("not found", func(t *testing.T) {
		assert.Nil(t, GetProvider("nonexistent-res"))
	})
}

func TestGet_NonDefaultWithoutConfig(t *testing.T) {
	Configure(nil)
	_, err := Get("nonexistent")
	require.Error(t, err)
	assert.ErrorContains(t, err, "not found resolver type")
}

func TestGet_NonexistentProvider(t *testing.T) {
	Configure(map[string]Spec{"missing": {Type: "no-such-provider"}})
	_, err := Get("missing")
	require.Error(t, err)
	assert.ErrorContains(t, err, "not found resolver provider")
}

func TestGet_CachedResolver(t *testing.T) {
	RegisterBuilder("cache-test", func(name string) (Resolver, error) {
		return &testResolver{kind: "cached"}, nil
	})
	Configure(map[string]Spec{"cached": {Type: "cache-test"}})

	r1, err := Get("cached")
	require.NoError(t, err)
	r2, err := Get("cached")
	require.NoError(t, err)
	assert.Equal(t, r1, r2)
}

func TestCurrentSpec(t *testing.T) {
	Configure(map[string]Spec{"svc": {Type: "mock", Config: map[string]any{"k": "v"}}})
	spec := CurrentSpec("svc")
	assert.Equal(t, "mock", spec.Type)
}

func TestBaseEndpoint(t *testing.T) {
	be := BaseEndpoint{
		Address:    "127.0.0.1:8080",
		Protocol:   "grpc",
		Attributes: map[string]any{"zone": "us-west"},
	}
	assert.Equal(t, "grpc/127.0.0.1:8080", be.Name())
	assert.Equal(t, "127.0.0.1:8080", be.GetAddress())
	assert.Equal(t, "grpc", be.GetProtocol())
	assert.Equal(t, map[string]any{"zone": "us-west"}, be.GetAttributes())
}

func TestBaseState(t *testing.T) {
	bs := BaseState{
		Attributes: map[string]any{"region": "us"},
		Endpoints:  []Endpoint{BaseEndpoint{Address: "localhost:9090", Protocol: "http"}},
	}
	assert.Equal(t, map[string]any{"region": "us"}, bs.GetAttributes())
	require.Len(t, bs.GetEndpoints(), 1)
	assert.Equal(t, "http/localhost:9090", bs.GetEndpoints()[0].Name())
}
