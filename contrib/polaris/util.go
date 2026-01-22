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
