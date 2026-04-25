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

package local

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

func TestBuiltinProvider(t *testing.T) {
	provider := BuiltinProvider()
	profile, err := provider.Compile("default", nil)
	require.NoError(t, err)

	material, err := profile.Build(security.BuildSpec{})
	require.NoError(t, err)
	require.Equal(t, security.ModeLocal, material.Mode)
	require.NotNil(t, material.ConnAuth)
	require.NotNil(t, material.RequestAuth)
}

func TestCompileRejectsConfig(t *testing.T) {
	_, err := BuiltinProvider().Compile("default", map[string]any{"key": "value"})
	require.Error(t, err)
}

func TestProvider_Type(t *testing.T) {
	require.Equal(t, "local", BuiltinProvider().Type())
}

func TestGetSecurityLevel(t *testing.T) {
	tests := []struct {
		name    string
		addr    net.Addr
		want    security.SecurityLevel
		wantErr bool
	}{
		{
			name: "UnixAddr",
			addr: &net.UnixAddr{Name: "/tmp/test.sock", Net: "unix"},
			want: security.PrivacyAndIntegrity,
		},
		{
			name: "TCPAddr loopback",
			addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1234},
			want: security.NoSecurity,
		},
		{
			name:    "TCPAddr non-loopback",
			addr:    &net.TCPAddr{IP: net.ParseIP("8.8.8.8"), Port: 443},
			wantErr: true,
		},
		{
			name: "IPAddr loopback",
			addr: &net.IPAddr{IP: net.ParseIP("127.0.0.1")},
			want: security.NoSecurity,
		},
		{
			name:    "nil addr",
			addr:    nil,
			wantErr: true,
		},
		{
			name:    "non-local custom addr",
			addr:    &customAddr{network: "tcp", addr: "8.8.8.8:443"},
			wantErr: true,
		},
		{
			name: "network unix custom addr",
			addr: &customAddr{network: "unix", addr: "/tmp/test.sock"},
			want: security.PrivacyAndIntegrity,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, err := getSecurityLevel(tt.addr)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, level)
			}
		})
	}
}

func TestGetSecurityLevelFromString(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		want    security.SecurityLevel
		wantErr bool
	}{
		{
			name: "loopback with port",
			addr: "127.0.0.1:8080",
			want: security.NoSecurity,
		},
		{
			name: "loopback without port",
			addr: "127.0.0.1",
			want: security.NoSecurity,
		},
		{
			name:    "non-local with port",
			addr:    "8.8.8.8:443",
			wantErr: true,
		},
		{
			name:    "non-local without port",
			addr:    "8.8.8.8",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, err := getSecurityLevelFromString(tt.addr)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, level)
			}
		})
	}
}

func TestConnAuthenticator_ClientHandshake(t *testing.T) {
	t.Run("loopback addr success", func(t *testing.T) {
		ca := &connAuthenticator{info: security.ProtocolInfo{SecurityProtocol: "local"}}
		conn := &mockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1234}}
		_, auth, err := ca.ClientHandshake(context.Background(), "localhost", conn)
		require.NoError(t, err)
		require.NotNil(t, auth)

		bai, ok := auth.(security.BasicAuthInfo)
		require.True(t, ok)
		require.Equal(t, security.NoSecurity, bai.SecurityLevel)
	})

	t.Run("remote addr error", func(t *testing.T) {
		ca := &connAuthenticator{info: security.ProtocolInfo{SecurityProtocol: "local"}}
		conn := &mockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("8.8.8.8"), Port: 443}}
		_, _, err := ca.ClientHandshake(context.Background(), "localhost", conn)
		require.Error(t, err)
	})
}

func TestConnAuthenticator_ServerHandshake(t *testing.T) {
	t.Run("loopback addr success", func(t *testing.T) {
		ca := &connAuthenticator{info: security.ProtocolInfo{SecurityProtocol: "local"}}
		conn := &mockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1234}}
		_, auth, err := ca.ServerHandshake(conn)
		require.NoError(t, err)
		require.NotNil(t, auth)

		bai, ok := auth.(security.BasicAuthInfo)
		require.True(t, ok)
		require.Equal(t, security.NoSecurity, bai.SecurityLevel)
	})

	t.Run("remote addr error", func(t *testing.T) {
		ca := &connAuthenticator{info: security.ProtocolInfo{SecurityProtocol: "local"}}
		conn := &mockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("8.8.8.8"), Port: 443}}
		_, _, err := ca.ServerHandshake(conn)
		require.Error(t, err)
	})
}

func TestConnAuthenticator_Info(t *testing.T) {
	info := security.ProtocolInfo{SecurityProtocol: "local", ServerName: "test"}
	ca := &connAuthenticator{info: info}
	require.Equal(t, "local", ca.Info().SecurityProtocol)
	require.Equal(t, "test", ca.Info().ServerName)
}

func TestConnAuthenticator_Clone(t *testing.T) {
	t.Run("non-nil receiver", func(t *testing.T) {
		ca := &connAuthenticator{info: security.ProtocolInfo{SecurityProtocol: "local"}}
		cloned := ca.Clone()
		require.NotNil(t, cloned)

		clonedCA := cloned.(*connAuthenticator)
		require.Equal(t, "local", clonedCA.info.SecurityProtocol)
	})

	t.Run("nil receiver", func(t *testing.T) {
		var ca *connAuthenticator
		require.Nil(t, ca.Clone())
	})
}

func TestConnAuthenticator_OverrideServerName(t *testing.T) {
	ca := &connAuthenticator{info: security.ProtocolInfo{SecurityProtocol: "local"}}
	err := ca.OverrideServerName("new-server")
	require.NoError(t, err)
	require.Equal(t, "new-server", ca.info.ServerName)
}

func TestRequestAuthenticator(t *testing.T) {
	ra := requestAuthenticator{}

	t.Run("nil request", func(t *testing.T) {
		_, err := ra.AuthenticateRequest(nil)
		require.Error(t, err)
	})

	t.Run("Unix socket via LocalAddrContextKey", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		ctx := context.WithValue(
			req.Context(),
			http.LocalAddrContextKey,
			&net.UnixAddr{Name: "/tmp/test.sock", Net: "unix"},
		)
		req = req.WithContext(ctx)

		auth, err := ra.AuthenticateRequest(req)
		require.NoError(t, err)
		require.NotNil(t, auth)

		bai, ok := auth.(security.BasicAuthInfo)
		require.True(t, ok)
		require.Equal(t, security.PrivacyAndIntegrity, bai.SecurityLevel)
	})

	t.Run("loopback RemoteAddr", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		auth, err := ra.AuthenticateRequest(req)
		require.NoError(t, err)
		require.NotNil(t, auth)

		bai, ok := auth.(security.BasicAuthInfo)
		require.True(t, ok)
		require.Equal(t, security.NoSecurity, bai.SecurityLevel)
	})

	t.Run("non-local RemoteAddr", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		req.RemoteAddr = "8.8.8.8:443"
		_, err := ra.AuthenticateRequest(req)
		require.Error(t, err)
	})
}

// customAddr implements net.Addr for testing.
type customAddr struct {
	network string
	addr    string
}

func (c *customAddr) Network() string { return c.network }
func (c *customAddr) String() string  { return c.addr }

// mockConn implements net.Conn for testing.
type mockConn struct {
	remoteAddr net.Addr
}

func (m *mockConn) Read([]byte) (n int, err error)     { return 0, nil }
func (m *mockConn) Write([]byte) (n int, err error)    { return 0, nil }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return m.remoteAddr }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }
