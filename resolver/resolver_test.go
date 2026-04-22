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

	"github.com/stretchr/testify/require"
)

type testResolver struct{ kind string }

func (t *testResolver) AddWatch(string, Client) error { return nil }
func (t *testResolver) DelWatch(string, Client) error { return nil }
func (t *testResolver) Type() string                  { return t.kind }

func TestGetDefaultResolverWithoutConfigReturnsNil(t *testing.T) {
	r, err := Get(DefaultResolverName)
	require.NoError(t, err)
	require.Nil(t, r)
}

func TestGetResolverFromDiscoveryTree(t *testing.T) {
	RegisterBuilder("mock", func(name string) (Resolver, error) {
		return &testResolver{kind: name}, nil
	})
	Configure(map[string]Spec{"demo": {Type: "mock"}})

	r, err := Get("demo")
	require.NoError(t, err)
	require.Equal(t, "demo", r.Type())
}

func RegisterBuilder(typeName string, f func(string) (Resolver, error)) {
	mu.Lock()
	defer mu.Unlock()
	providers[typeName] = NewProvider(typeName, f)
	resolver = map[string]Resolver{}
}
