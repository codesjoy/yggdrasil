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

// Package tls provides tls credentials for rpc framework.
package tls

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"strings"

	"github.com/codesjoy/yggdrasil/v2/remote/credentials"
	"github.com/codesjoy/yggdrasil/v2/remote/peer"
)

const name = "tls"

// Config contains the configuration for tls credentials.
type Config struct {
	Client    bool
	TLSConfig *tls.Config
}

// New returns a new tls credentials.
func New(cfg Config) (credentials.TransportCredentials, error) {
	if cfg.TLSConfig == nil {
		return nil, errors.New("tls: TLSConfig is nil")
	}
	c := cfg.TLSConfig.Clone()
	return &transportCredentials{
		client: cfg.Client,
		cfg:    c,
	}, nil
}

type transportCredentials struct {
	client             bool
	cfg                *tls.Config
	serverNameOverride string
}

// AuthInfo contains the information from a security handshake.
type AuthInfo struct {
	credentials.CommonAuthInfo
	State tls.ConnectionState
}

// AuthType returns the type of info as a string.
func (AuthInfo) AuthType() string { return name }

// ClientHandshake performs a client-side TLS handshake.
func (c *transportCredentials) ClientHandshake(
	ctx context.Context,
	authority string,
	rawConn net.Conn,
) (net.Conn, credentials.AuthInfo, error) {
	if rawConn == nil {
		return nil, nil, errors.New("tls: rawConn is nil")
	}
	cfg := c.cfg.Clone()
	if cfg.ServerName == "" {
		cfg.ServerName = c.serverNameOverride
	}
	if cfg.ServerName == "" {
		cfg.ServerName = authorityToServerName(authority)
	}
	tlsConn := tls.Client(rawConn, cfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = rawConn.Close()
		return nil, nil, err
	}
	return tlsConn, AuthInfo{
		CommonAuthInfo: credentials.CommonAuthInfo{SecurityLevel: credentials.PrivacyAndIntegrity},
		State:          tlsConn.ConnectionState(),
	}, nil
}

// ServerHandshake performs a server-side TLS handshake.
func (c *transportCredentials) ServerHandshake(
	rawConn net.Conn,
) (net.Conn, credentials.AuthInfo, error) {
	if rawConn == nil {
		return nil, nil, errors.New("tls: rawConn is nil")
	}
	cfg := c.cfg.Clone()
	tlsConn := tls.Server(rawConn, cfg)
	if err := tlsConn.HandshakeContext(context.Background()); err != nil {
		_ = rawConn.Close()
		return nil, nil, err
	}
	return tlsConn, AuthInfo{
		CommonAuthInfo: credentials.CommonAuthInfo{SecurityLevel: credentials.PrivacyAndIntegrity},
		State:          tlsConn.ConnectionState(),
	}, nil
}

// Info returns the ProtocolInfo for this TransportCredentials.
func (c *transportCredentials) Info() credentials.ProtocolInfo {
	cfg := c.cfg
	info := credentials.ProtocolInfo{
		SecurityProtocol: name,
	}
	if cfg != nil {
		info.ServerName = cfg.ServerName
	}
	return info
}

// Clone makes a copy of TransportCredentials.
func (c *transportCredentials) Clone() credentials.TransportCredentials {
	if c == nil || c.cfg == nil {
		return nil
	}
	return &transportCredentials{
		client:             c.client,
		cfg:                c.cfg.Clone(),
		serverNameOverride: c.serverNameOverride,
	}
}

// OverrideServerName overrides the server name for SNI.
func (c *transportCredentials) OverrideServerName(serverNameOverride string) error {
	c.serverNameOverride = serverNameOverride
	return nil
}

// Name returns the name of the security protocol.
func (c *transportCredentials) Name() string { return name }

// StateFromAuthInfo extracts the tls.ConnectionState from AuthInfo.
func StateFromAuthInfo(ai credentials.AuthInfo) (tls.ConnectionState, bool) {
	switch v := ai.(type) {
	case AuthInfo:
		return v.State, true
	case *AuthInfo:
		if v == nil {
			return tls.ConnectionState{}, false
		}
		return v.State, true
	default:
		return tls.ConnectionState{}, false
	}
}

// SPIFFEIDFromAuthInfo extracts the SPIFFE ID from AuthInfo.
func SPIFFEIDFromAuthInfo(ai credentials.AuthInfo) *url.URL {
	state, ok := StateFromAuthInfo(ai)
	if !ok {
		return nil
	}
	return credentials.SPIFFEIDFromState(state)
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
