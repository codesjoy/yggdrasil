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

package tls

import (
	"context"
	stdtls "crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/transport/support/peer"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

type connAuthenticator struct {
	clientTLS          *stdtls.Config
	serverTLS          *stdtls.Config
	serverNameOverride string
}

// AuthInfo contains TLS handshake information.
type AuthInfo struct {
	security.CommonAuthInfo
	State stdtls.ConnectionState
}

// AuthType returns the auth type.
func (AuthInfo) AuthType() string { return name }

func (c *connAuthenticator) ClientHandshake(
	ctx context.Context,
	authority string,
	rawConn net.Conn,
) (net.Conn, security.AuthInfo, error) {
	if rawConn == nil {
		return nil, nil, errors.New("tls: rawConn is nil")
	}
	if c == nil || c.clientTLS == nil {
		return nil, nil, errors.New("tls: client tls config is nil")
	}
	cfg := c.clientTLS.Clone()
	if cfg.ServerName == "" {
		cfg.ServerName = c.serverNameOverride
	}
	if cfg.ServerName == "" {
		cfg.ServerName = authorityToServerName(authority)
	}
	tlsConn := stdtls.Client(rawConn, cfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = rawConn.Close()
		return nil, nil, err
	}
	return tlsConn, AuthInfo{
		CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: security.PrivacyAndIntegrity},
		State:          tlsConn.ConnectionState(),
	}, nil
}

func (c *connAuthenticator) ServerHandshake(rawConn net.Conn) (net.Conn, security.AuthInfo, error) {
	if rawConn == nil {
		return nil, nil, errors.New("tls: rawConn is nil")
	}
	if c == nil || c.serverTLS == nil {
		return nil, nil, errors.New("tls: server tls config is nil")
	}
	cfg := c.serverTLS.Clone()
	tlsConn := stdtls.Server(rawConn, cfg)
	if err := tlsConn.HandshakeContext(context.Background()); err != nil {
		_ = rawConn.Close()
		return nil, nil, err
	}
	return tlsConn, AuthInfo{
		CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: security.PrivacyAndIntegrity},
		State:          tlsConn.ConnectionState(),
	}, nil
}

func (c *connAuthenticator) Info() security.ProtocolInfo {
	info := security.ProtocolInfo{SecurityProtocol: name}
	if c != nil && c.clientTLS != nil {
		info.ServerName = c.clientTLS.ServerName
	}
	return info
}

func (c *connAuthenticator) Clone() security.ConnAuthenticator {
	if c == nil {
		return nil
	}
	return &connAuthenticator{
		clientTLS:          cloneTLSConfig(c.clientTLS),
		serverTLS:          cloneTLSConfig(c.serverTLS),
		serverNameOverride: c.serverNameOverride,
	}
}

func (c *connAuthenticator) OverrideServerName(serverNameOverride string) error {
	c.serverNameOverride = serverNameOverride
	return nil
}

type requestAuthenticator struct{}

func (requestAuthenticator) AuthenticateRequest(r *http.Request) (security.AuthInfo, error) {
	if r == nil || r.TLS == nil {
		return nil, errors.New("tls: request does not carry tls state")
	}
	return AuthInfo{
		CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: security.PrivacyAndIntegrity},
		State:          *r.TLS,
	}, nil
}

// StateFromAuthInfo extracts the tls.ConnectionState from AuthInfo.
func StateFromAuthInfo(ai security.AuthInfo) (stdtls.ConnectionState, bool) {
	switch v := ai.(type) {
	case AuthInfo:
		return v.State, true
	case *AuthInfo:
		if v == nil {
			return stdtls.ConnectionState{}, false
		}
		return v.State, true
	default:
		return stdtls.ConnectionState{}, false
	}
}

// SPIFFEIDFromAuthInfo extracts the SPIFFE ID from AuthInfo.
func SPIFFEIDFromAuthInfo(ai security.AuthInfo) *url.URL {
	state, ok := StateFromAuthInfo(ai)
	if !ok {
		return nil
	}
	return security.SPIFFEIDFromState(state)
}

// SPIFFEIDFromPeer extracts the SPIFFE ID from Peer.
func SPIFFEIDFromPeer(p *peer.Peer) *url.URL {
	if p == nil {
		return nil
	}
	return SPIFFEIDFromAuthInfo(p.AuthInfo)
}

func authorityToServerName(authority string) string {
	authority = strings.TrimSpace(authority)
	if authority == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(authority); err == nil {
		return host
	}
	return authority
}

func cloneTLSConfig(cfg *stdtls.Config) *stdtls.Config {
	if cfg == nil {
		return nil
	}
	return cfg.Clone()
}
