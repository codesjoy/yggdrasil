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

// Package netaddr resolves local listen and advertise addresses.
package netaddr

import (
	"context"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync/atomic"
)

const loopbackIPv4 = "127.0.0.1"

// probeTarget is a reserved documentation address (RFC 5737 TEST-NET-3).
// A UDP dial emits no packets; it only asks the kernel to apply the routing
// table, which yields the address of the default-route egress interface.
const probeTarget = "203.0.113.1:9"

// overlayInterfacePrefixes are container, overlay and tunnel interface names
// whose addresses are unreachable from other machines on the local network.
var overlayInterfacePrefixes = []string{
	"br-", "cali", "cni", "docker", "dummy", "flannel", "tap", "tun", "veth",
	"virbr", "vmnet", "vpn", "wg",
}

var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

var (
	cachedAddr  atomic.Value // string
	resolveAddr = probePrimaryIPv4
)

// IsWildcard reports whether host is empty or a wildcard bind host.
func IsWildcard(host string) bool {
	switch strings.TrimSpace(host) {
	case "", "0.0.0.0", "::", "[::]":
		return true
	default:
		return false
	}
}

// SelectPrimaryIPv4 returns the host's primary routable IPv4 address.
//
// It never fails: when no usable address can be probed it returns the loopback
// address. Successful results are cached for the process lifetime.
func SelectPrimaryIPv4(ctx context.Context) string {
	if addr, ok := cachedAddr.Load().(string); ok && addr != "" {
		return addr
	}
	addr := resolveAddr(ctx)
	if addr == "" {
		addr = loopbackIPv4
	}
	cachedAddr.Store(addr)
	return addr
}

func resetCacheForTest() {
	cachedAddr.Store("")
}

func probePrimaryIPv4(ctx context.Context) string {
	if addr := probeDefaultRouteIPv4(ctx); addr != "" {
		return addr
	}
	return scanInterfaceIPv4()
}

func probeDefaultRouteIPv4(ctx context.Context) string {
	if ctx == nil {
		ctx = context.Background()
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "udp4", probeTarget)
	if err != nil {
		return ""
	}
	defer func() { _ = conn.Close() }()
	udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return ""
	}
	return usableIPv4(udpAddr.IP)
}

func scanInterfaceIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	// Container and overlay interfaces can sort ahead of the real LAN address,
	// so they are skipped entirely rather than merely deprioritized.
	private := make([]netip.Addr, 0, 4)
	public := make([]netip.Addr, 0, 4)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if isOverlayInterface(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			// Interfaces can disappear at runtime; skip transient failures.
			continue
		}
		for _, item := range addrs {
			addr, ok := addrToNetip(item)
			if !ok || !addr.Is4() || !addr.IsGlobalUnicast() || isExcludedIPv4(addr) {
				continue
			}
			if addr.IsPrivate() {
				private = append(private, addr)
				continue
			}
			public = append(public, addr)
		}
	}
	sortAddrs(private)
	sortAddrs(public)
	if len(private) > 0 {
		return private[0].String()
	}
	if len(public) > 0 {
		return public[0].String()
	}
	return ""
}

func usableIPv4(ip net.IP) string {
	v4 := ip.To4()
	if v4 == nil {
		return ""
	}
	addr, ok := netip.AddrFromSlice(v4)
	if !ok {
		return ""
	}
	addr = addr.Unmap()
	// The default-route probe is authoritative, so CGNAT is allowed here; only
	// addresses that cannot route to a peer are rejected.
	if isUnroutableIPv4(addr) {
		return ""
	}
	return addr.String()
}

func addrToNetip(addr net.Addr) (netip.Addr, bool) {
	switch v := addr.(type) {
	case *net.IPNet:
		return netip.AddrFromSlice(v.IP)
	case *net.IPAddr:
		return netip.AddrFromSlice(v.IP)
	default:
		return netip.Addr{}, false
	}
}

func isOverlayInterface(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range overlayInterfacePrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func isExcludedIPv4(addr netip.Addr) bool {
	return isUnroutableIPv4(addr) || cgnatPrefix.Contains(addr)
}

func isUnroutableIPv4(addr netip.Addr) bool {
	return addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsUnspecified()
}

func sortAddrs(addrs []netip.Addr) {
	sort.Slice(addrs, func(i, j int) bool {
		return addrs[i].Compare(addrs[j]) < 0
	})
}
