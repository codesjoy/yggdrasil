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

	"github.com/codesjoy/pkg/utils/xnet"
)

// NormalizeListenHost normalizes listen host for empty or wildcard values.
func NormalizeListenHost(host string) (string, error) {
	host = strings.TrimSpace(host)
	if !isWildcardHost(host) {
		return host, nil
	}

	addr, err := xnet.SelectLocalAddr(context.Background(), defaultLocalAddrOptions())
	if err != nil {
		return "", err
	}
	return addr.String(), nil
}

func isWildcardHost(host string) bool {
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		return true
	default:
		return false
	}
}

func defaultLocalAddrOptions() xnet.LocalAddrOptions {
	return xnet.LocalAddrOptions{
		Family:           xnet.FamilyIPv4,
		IncludeLoopback:  true,
		IncludeLinkLocal: true,
	}
}
