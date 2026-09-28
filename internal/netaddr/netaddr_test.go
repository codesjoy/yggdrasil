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

package netaddr

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsWildcard(t *testing.T) {
	for _, in := range []string{"", " ", "0.0.0.0", "::", "[::]", " 0.0.0.0 "} {
		assert.True(t, IsWildcard(in), "expected wildcard: %q", in)
	}
	for _, in := range []string{"127.0.0.1", "10.0.0.1", "localhost", "::1"} {
		assert.False(t, IsWildcard(in), "expected non-wildcard: %q", in)
	}
}

func TestSelectPrimaryIPv4_FallbackChain(t *testing.T) {
	tests := []struct {
		name  string
		probe string
		want  string
	}{
		{name: "probe result wins", probe: "10.1.2.3", want: "10.1.2.3"},
		{name: "loopback fallback", probe: "", want: loopbackIPv4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubResolver(t, tc.probe)
			assert.Equal(t, tc.want, SelectPrimaryIPv4(context.Background()))
		})
	}
}

func TestSelectPrimaryIPv4_Caches(t *testing.T) {
	resetCacheForTest()
	t.Cleanup(resetCacheForTest)

	prev := resolveAddr
	calls := 0
	resolveAddr = func(context.Context) string {
		calls++
		return "10.9.9.9"
	}
	t.Cleanup(func() { resolveAddr = prev })

	assert.Equal(t, "10.9.9.9", SelectPrimaryIPv4(context.Background()))
	assert.Equal(t, "10.9.9.9", SelectPrimaryIPv4(context.Background()))
	assert.Equal(t, 1, calls)
}

func TestProbeDefaultRouteIPv4_UsableOrEmpty(t *testing.T) {
	got := probeDefaultRouteIPv4(context.Background())
	if got == "" {
		return
	}
	addr, err := netip.ParseAddr(got)
	require.NoError(t, err)
	assert.True(t, addr.Is4())
	assert.False(t, addr.IsLoopback())
	assert.False(t, addr.IsLinkLocalUnicast())
}

func TestScanInterfaceIPv4_OverlayAndExclusion(t *testing.T) {
	for _, name := range []string{"docker0", "br-abc123", "veth12ab", "wg0", "tun0"} {
		assert.True(t, isOverlayInterface(name), "expected overlay: %q", name)
	}
	for _, name := range []string{"en0", "eth0", "lo0"} {
		assert.False(t, isOverlayInterface(name), "expected physical: %q", name)
	}

	assert.True(t, isExcludedIPv4(netip.MustParseAddr("100.64.0.1")), "CGNAT must be excluded")
	assert.True(
		t,
		isExcludedIPv4(netip.MustParseAddr("169.254.1.1")),
		"link-local must be excluded",
	)
	assert.True(t, isExcludedIPv4(netip.MustParseAddr("127.0.0.1")), "loopback must be excluded")
	assert.False(t, isExcludedIPv4(netip.MustParseAddr("192.168.1.10")))
	assert.False(t, isExcludedIPv4(netip.MustParseAddr("10.0.0.1")))
}

func TestUsableIPv4(t *testing.T) {
	assert.Equal(t, "10.0.0.5", usableIPv4(mustParseIP(t, "10.0.0.5")))
	assert.Equal(t, "", usableIPv4(mustParseIP(t, "127.0.0.1")), "loopback is not usable")
	assert.Equal(t, "", usableIPv4(nil))
}

// stubResolver swaps the resolver with a fixed answer and restores it after.
func stubResolver(t *testing.T, value string) {
	t.Helper()
	resetCacheForTest()
	prev := resolveAddr
	resolveAddr = func(context.Context) string { return value }
	t.Cleanup(func() {
		resolveAddr = prev
		resetCacheForTest()
	})
}

func mustParseIP(t *testing.T, value string) net.IP {
	t.Helper()
	ip := net.ParseIP(value)
	require.NotNil(t, ip)
	return ip
}
