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

// Package httpmeta provides HTTP metadata and peer adapters shared by transport
// protocol implementations.
package httpmeta

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	"github.com/codesjoy/yggdrasil/v3/transport/support/peer"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

const (
	// MetadataHeaderPrefix identifies RPC metadata carried in HTTP headers.
	MetadataHeaderPrefix = "Yggdrasil-Metadata-"
	// MetadataTrailerPrefix identifies RPC metadata carried in HTTP trailers.
	MetadataTrailerPrefix = "Yggdrasil-Trailer-"
)

// HeaderMatcher maps one RPC metadata key to an HTTP header key.
type HeaderMatcher func(string) string

// PrefixMatcher returns a matcher that prepends prefix to each metadata key.
func PrefixMatcher(prefix string) HeaderMatcher {
	return func(key string) string {
		return prefix + key
	}
}

// ExtractMetadata returns metadata whose HTTP header names start with prefix.
func ExtractMetadata(header http.Header, prefix string) metadata.MD {
	out := metadata.MD{}
	for key, values := range header {
		if strings.HasPrefix(key, prefix) {
			out.Append(key[len(prefix):], values...)
		}
	}
	return out
}

// WriteMetadata writes metadata to HTTP headers using match.
func WriteMetadata(w http.ResponseWriter, md metadata.MD, match HeaderMatcher) {
	for key, values := range md {
		header := match(key)
		for _, value := range values {
			w.Header().Add(header, value)
		}
	}
}

// AcceptsTrailers reports whether the request advertises trailer support.
func AcceptsTrailers(req *http.Request) bool {
	return strings.Contains(strings.ToLower(req.Header.Get("TE")), "trailers")
}

// DeclareTrailers reserves HTTP trailer names using match.
func DeclareTrailers(w http.ResponseWriter, md metadata.MD, match HeaderMatcher) {
	for key := range md {
		w.Header().Add("Trailer", match(key))
	}
}

// WriteTrailers writes HTTP trailer values using match.
func WriteTrailers(w http.ResponseWriter, md metadata.MD, match HeaderMatcher) {
	WriteMetadata(w, md, match)
}

// ParseRemoteAddr parses an HTTP RemoteAddr into an IP address and port.
func ParseRemoteAddr(value string) (netip.Addr, int) {
	value = strings.TrimSpace(value)
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		host = strings.Trim(value, "[]")
		portText = ""
	}
	address, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return netip.Addr{}, 0
	}
	port, _ := strconv.Atoi(portText)
	return address.Unmap(), port
}

// AttachPeer stores the HTTP request's peer information in the context.
func AttachPeer(
	ctx context.Context,
	req *http.Request,
	localAddr net.Addr,
	authInfo security.AuthInfo,
) context.Context {
	address, port := ParseRemoteAddr(req.RemoteAddr)
	var ip net.IP
	if address.IsValid() {
		ip = net.IP(address.AsSlice())
	}
	return peer.WithContext(ctx, &peer.Peer{
		Addr: &net.TCPAddr{
			IP:   ip,
			Port: port,
		},
		LocalAddr: localAddr,
		AuthInfo:  authInfo,
		Protocol:  "http",
	})
}
