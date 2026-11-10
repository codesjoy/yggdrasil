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

// Package listenaddr provides transport listen-address normalization helpers.
package listenaddr

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/internal/netaddr"
)

// NormalizeListenHost normalizes listen host for empty or wildcard values.
//
// Empty and wildcard values resolve to the host's primary routable IPv4
// address, so a listener is never bound to an unreachable interface address.
// Explicit host values pass through unchanged.
//
// Deprecated: use ResolveBindHost and ResolveAdvertiseAddress so binding and
// service advertisement are configured independently.
func NormalizeListenHost(host string) (string, error) {
	host = strings.TrimSpace(host)
	if !netaddr.IsWildcard(host) {
		return host, nil
	}
	return netaddr.SelectPrimaryIPv4(context.Background()), nil
}

var selectAdvertiseHost = func() string {
	return netaddr.SelectPrimaryIPv4(context.Background())
}

// ResolveBindHost returns the host that should be passed to net.Listen.
// Empty and wildcard hosts remain wildcards.
func ResolveBindHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "[::]" {
		return "::"
	}
	return host
}

// ResolveAdvertiseAddress returns the address that should be published for a
// running listener. Explicit configured addresses take precedence. Otherwise a
// wildcard bind host is replaced with the primary routable IPv4 address while
// preserving the actual bound port.
func ResolveAdvertiseAddress(
	network, boundAddress, configuredAddress string,
) (string, error) {
	configuredAddress = strings.TrimSpace(configuredAddress)
	if configuredAddress != "" {
		if !isTCPNetwork(network) {
			return configuredAddress, nil
		}
		if _, _, err := net.SplitHostPort(configuredAddress); err != nil {
			return "", fmt.Errorf("invalid advertise address %q: %w", configuredAddress, err)
		}
		return configuredAddress, nil
	}
	if !isTCPNetwork(network) {
		return boundAddress, nil
	}
	host, port, err := net.SplitHostPort(boundAddress)
	if err != nil {
		return "", fmt.Errorf("split bound address %q: %w", boundAddress, err)
	}
	if netaddr.IsWildcard(host) {
		host = selectAdvertiseHost()
	}
	return net.JoinHostPort(host, port), nil
}

func isTCPNetwork(network string) bool {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "", "tcp", "tcp4", "tcp6":
		return true
	default:
		return false
	}
}
