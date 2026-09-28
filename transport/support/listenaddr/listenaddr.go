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
	"strings"

	"github.com/codesjoy/yggdrasil/v3/internal/netaddr"
)

// NormalizeListenHost normalizes listen host for empty or wildcard values.
//
// Empty and wildcard values resolve to the host's primary routable IPv4
// address, so a listener is never bound to an unreachable interface address.
// Explicit host values pass through unchanged.
func NormalizeListenHost(host string) (string, error) {
	host = strings.TrimSpace(host)
	if !netaddr.IsWildcard(host) {
		return host, nil
	}
	return netaddr.SelectPrimaryIPv4(context.Background()), nil
}
