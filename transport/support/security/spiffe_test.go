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

package security

import (
	stdtls "crypto/tls"
	"crypto/x509"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSPIFFEIDFromState(t *testing.T) {
	t.Run("empty peer certificates", func(t *testing.T) {
		state := stdtls.ConnectionState{}
		require.Nil(t, SPIFFEIDFromState(state))
	})

	t.Run("cert with no URIs", func(t *testing.T) {
		state := stdtls.ConnectionState{
			PeerCertificates: []*x509.Certificate{{}},
		}
		require.Nil(t, SPIFFEIDFromState(state))
	})

	t.Run("valid SPIFFE ID URI", func(t *testing.T) {
		spiffeURI := &url.URL{Scheme: "spiffe", Host: "example.com", Path: "/service"}
		cert := &x509.Certificate{
			URIs: []*url.URL{spiffeURI},
		}
		state := stdtls.ConnectionState{
			PeerCertificates: []*x509.Certificate{cert},
		}
		result := SPIFFEIDFromState(state)
		require.NotNil(t, result)
		require.Equal(t, "spiffe://example.com/service", result.String())
	})
}

func TestSPIFFEIDFromCert(t *testing.T) {
	tests := []struct {
		name string
		cert *x509.Certificate
		want *url.URL
	}{
		{
			name: "nil cert",
			cert: nil,
			want: nil,
		},
		{
			name: "cert with nil URIs",
			cert: &x509.Certificate{URIs: nil},
			want: nil,
		},
		{
			name: "cert with valid SPIFFE URI",
			cert: &x509.Certificate{
				URIs: []*url.URL{
					{Scheme: "spiffe", Host: "example.com", Path: "/service"},
				},
			},
			want: &url.URL{Scheme: "spiffe", Host: "example.com", Path: "/service"},
		},
		{
			name: "cert with non-spiffe scheme",
			cert: &x509.Certificate{
				URIs: []*url.URL{
					{Scheme: "https", Host: "example.com", Path: "/foo"},
				},
			},
			want: nil,
		},
		{
			name: "cert with URI with opaque part",
			cert: &x509.Certificate{
				URIs: []*url.URL{
					{Scheme: "spiffe", Opaque: "opaque"},
				},
			},
			want: nil,
		},
		{
			name: "cert with userinfo",
			cert: &x509.Certificate{
				URIs: []*url.URL{
					{Scheme: "spiffe", Host: "example.com", Path: "/svc", User: url.User("user")},
				},
			},
			want: nil,
		},
		{
			name: "cert with empty host",
			cert: &x509.Certificate{
				URIs: []*url.URL{
					{Scheme: "spiffe", Host: "", Path: "/svc"},
				},
			},
			want: nil,
		},
		{
			name: "cert with empty path",
			cert: &x509.Certificate{
				URIs: []*url.URL{
					{Scheme: "spiffe", Host: "example.com", Path: ""},
				},
			},
			want: nil,
		},
		{
			name: "cert with multiple URI SANs",
			cert: &x509.Certificate{
				URIs: []*url.URL{
					{Scheme: "spiffe", Host: "example.com", Path: "/svc1"},
					{Scheme: "spiffe", Host: "example.com", Path: "/svc2"},
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SPIFFEIDFromCert(tt.cert)
			if tt.want == nil {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				require.Equal(t, tt.want.String(), result.String())
			}
		})
	}
}

func TestSPIFFEIDFromCert_SpiffeIDTooLong(t *testing.T) {
	// Build a SPIFFE URI > 2048 bytes
	longHost := make([]byte, 2040)
	for i := range longHost {
		longHost[i] = 'a'
	}
	uri := &url.URL{
		Scheme: "spiffe",
		Host:   string(longHost),
		Path:   "/svc",
	}
	cert := &x509.Certificate{
		URIs: []*url.URL{uri},
	}
	require.Nil(t, SPIFFEIDFromCert(cert))
}

func TestSPIFFEIDFromCert_HostTooLong(t *testing.T) {
	// Build a host > 255 chars but total URI <= 2048
	longHost := make([]byte, 256)
	for i := range longHost {
		longHost[i] = 'a'
	}
	uri := &url.URL{
		Scheme: "spiffe",
		Host:   string(longHost),
		Path:   "/svc",
	}
	cert := &x509.Certificate{
		URIs: []*url.URL{uri},
	}
	require.Nil(t, SPIFFEIDFromCert(cert))
}
