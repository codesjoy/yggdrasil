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

package client

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigureAndCurrentConfig(t *testing.T) {
	preserveClientSettings(t)

	Configure(Settings{})
	require.Equal(t, ServiceConfig{}, CurrentConfig("missing"))

	Configure(Settings{
		Services: map[string]ServiceConfig{
			"svc-a": {
				FastFail: true,
				Resolver: "resolver-a",
				Balancer: "default",
			},
			"svc-b": {
				FastFail: false,
				Resolver: "resolver-b",
				Balancer: "custom",
			},
		},
	})

	cfgA := CurrentConfig("svc-a")
	require.True(t, cfgA.FastFail)
	require.Equal(t, "resolver-a", cfgA.Resolver)
	require.Equal(t, "default", cfgA.Balancer)

	cfgB := CurrentConfig("svc-b")
	require.False(t, cfgB.FastFail)
	require.Equal(t, "resolver-b", cfgB.Resolver)
	require.Equal(t, "custom", cfgB.Balancer)
}
