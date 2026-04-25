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

package listenaddr

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeListenHost_ExplicitHost(t *testing.T) {
	host, err := NormalizeListenHost("localhost")
	require.NoError(t, err)
	assert.Equal(t, "localhost", host)

	host, err = NormalizeListenHost("127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", host)
}

func TestNormalizeListenHost_WildcardAndEmpty(t *testing.T) {
	tests := []string{"", "0.0.0.0", "::", "[::]", " 0.0.0.0 ", " [::] "}

	for _, in := range tests {
		t.Run(in, func(t *testing.T) {
			host, err := NormalizeListenHost(in)
			require.NoError(t, err)
			require.NotEmpty(t, host)
			addr, parseErr := netip.ParseAddr(host)
			require.NoError(t, parseErr)
			assert.True(t, addr.IsValid())
		})
	}
}

func TestNormalizeListenHost_TrimSpace(t *testing.T) {
	host, err := NormalizeListenHost(" localhost ")
	require.NoError(t, err)
	assert.Equal(t, "localhost", host)
	assert.Equal(t, strings.TrimSpace(" localhost "), host)
}
