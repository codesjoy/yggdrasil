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

package polaris

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"
)

func splitHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func mergeStringMap(parts ...map[string]string) map[string]string {
	out := make(map[string]string, 8)
	for _, m := range parts {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

func effectiveTimeout(ctx context.Context, fallback time.Duration) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		d := time.Until(deadline)
		if d > 0 {
			return d
		}
	}
	if fallback > 0 {
		return fallback
	}
	return 0
}

func netAddr(host string, port uint32) string {
	p := strconv.FormatUint(uint64(port), 10)
	if host == "" {
		return ":" + p
	}
	if shouldBracketIPv6(host) {
		return "[" + host + "]:" + p
	}
	return host + ":" + p
}

func shouldBracketIPv6(host string) bool {
	return len(host) > 0 && host[0] != '[' && strings.Contains(host, ":")
}
