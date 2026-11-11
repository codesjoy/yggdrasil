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

package httpmeta

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	"github.com/codesjoy/yggdrasil/v3/transport/support/peer"
)

func TestMetadataHeaders(t *testing.T) {
	t.Run("extract", func(t *testing.T) {
		header := http.Header{}
		header.Set(MetadataHeaderPrefix+"key1", "value1")
		header.Set(MetadataHeaderPrefix+"key2", "value2")
		header.Set("Other", "ignored")

		md := ExtractMetadata(header, MetadataHeaderPrefix)
		assert.Equal(t, []string{"value1"}, md.Get("key1"))
		assert.Equal(t, []string{"value2"}, md.Get("key2"))
		assert.Empty(t, md.Get("Other"))
	})

	t.Run("write mixed matcher", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		WriteMetadata(recorder, metadata.Pairs(
			"exposed", "one",
			"hidden", "two",
		), func(key string) string {
			if key == "exposed" {
				return key
			}
			return MetadataHeaderPrefix + key
		})

		assert.Equal(t, "one", recorder.Header().Get("exposed"))
		assert.Equal(t, "two", recorder.Header().Get(MetadataHeaderPrefix+"hidden"))
	})

	t.Run("prefix matcher", func(t *testing.T) {
		assert.Equal(t, "x-key", PrefixMatcher("x-")("key"))
	})
}

func TestTrailers(t *testing.T) {
	t.Run("accepts trailers", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/", nil)
		request.Header.Set("TE", "gzip, trailers")
		assert.True(t, AcceptsTrailers(request))
	})

	t.Run("does not accept trailers", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/", nil)
		request.Header.Set("TE", "gzip")
		assert.False(t, AcceptsTrailers(request))
	})

	t.Run("declare and write", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		md := metadata.Pairs("status", "ok", "trace", "abc")
		match := PrefixMatcher(MetadataTrailerPrefix)

		DeclareTrailers(recorder, md, match)
		WriteTrailers(recorder, md, match)

		assert.ElementsMatch(t, []string{
			MetadataTrailerPrefix + "status",
			MetadataTrailerPrefix + "trace",
		}, recorder.Header().Values("Trailer"))
		assert.Equal(t, "ok", recorder.Header().Get(MetadataTrailerPrefix+"status"))
		assert.Equal(t, "abc", recorder.Header().Get(MetadataTrailerPrefix+"trace"))
	})
}

func TestParseRemoteAddr(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		wantAddr netip.Addr
		wantPort int
	}{
		{
			name:     "ipv4",
			value:    "192.0.2.1:8080",
			wantAddr: netip.MustParseAddr("192.0.2.1"),
			wantPort: 8080,
		},
		{
			name:     "ipv6",
			value:    "[2001:db8::1]:443",
			wantAddr: netip.MustParseAddr("2001:db8::1"),
			wantPort: 443,
		},
		{
			name:     "missing port",
			value:    "justahost",
			wantAddr: netip.Addr{},
		},
		{
			name:     "invalid",
			value:    "bad address",
			wantAddr: netip.Addr{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			address, port := ParseRemoteAddr(test.value)
			assert.Equal(t, test.wantAddr, address)
			assert.Equal(t, test.wantPort, port)
		})
	}
}

func TestAttachPeer(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	localAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}

	ctx := AttachPeer(context.Background(), request, localAddr, nil)
	attached, ok := peer.FromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, "192.0.2.10:1234", attached.Addr.String())
	assert.Equal(t, localAddr, attached.LocalAddr)
	assert.Equal(t, "http", attached.Protocol)
}
