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

// Package local provides a local-only security profile provider.
package local

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

const name = "local"

// BuiltinProvider returns the built-in local security profile provider.
func BuiltinProvider() security.Provider {
	return provider{}
}

type provider struct{}

func (provider) Type() string { return name }

func (provider) Compile(profileName string, raw map[string]any) (security.Profile, error) {
	if len(raw) != 0 {
		return nil, fmt.Errorf(
			"security profile %q type %q does not accept config",
			profileName,
			name,
		)
	}
	return profile{name: profileName}, nil
}

type profile struct {
	name string
}

func (p profile) Name() string { return p.name }

func (p profile) Type() string { return name }

func (p profile) Build(security.BuildSpec) (security.Material, error) {
	info := security.ProtocolInfo{SecurityProtocol: name}
	return security.Material{
		Mode:        security.ModeLocal,
		RequestAuth: requestAuthenticator{},
		ConnAuth:    &connAuthenticator{info: info},
	}, nil
}

type connAuthenticator struct {
	info security.ProtocolInfo
}

func (c *connAuthenticator) ClientHandshake(
	_ context.Context,
	_ string,
	conn net.Conn,
) (net.Conn, security.AuthInfo, error) {
	secLevel, err := getSecurityLevel(conn.RemoteAddr())
	if err != nil {
		return nil, nil, err
	}
	return conn, authInfo(secLevel), nil
}

func (c *connAuthenticator) ServerHandshake(conn net.Conn) (net.Conn, security.AuthInfo, error) {
	secLevel, err := getSecurityLevel(conn.RemoteAddr())
	if err != nil {
		return nil, nil, err
	}
	return conn, authInfo(secLevel), nil
}

func (c *connAuthenticator) Info() security.ProtocolInfo {
	return c.info
}

func (c *connAuthenticator) Clone() security.ConnAuthenticator {
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
}

func (c *connAuthenticator) OverrideServerName(serverNameOverride string) error {
	c.info.ServerName = serverNameOverride
	return nil
}

type requestAuthenticator struct{}

func (requestAuthenticator) AuthenticateRequest(r *http.Request) (security.AuthInfo, error) {
	if r == nil {
		return nil, fmt.Errorf("local security rejected nil request")
	}
	if addr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
		if network := addr.Network(); network == "unix" || network == "unixpacket" {
			return authInfo(security.PrivacyAndIntegrity), nil
		}
		if _, ok := addr.(*net.UnixAddr); ok {
			return authInfo(security.PrivacyAndIntegrity), nil
		}
	}
	secLevel, err := getSecurityLevelFromString(r.RemoteAddr)
	if err != nil {
		return nil, err
	}
	return authInfo(secLevel), nil
}

// getSecurityLevel returns the security level for a local connection.
func getSecurityLevel(addr net.Addr) (security.SecurityLevel, error) {
	if addr == nil {
		return security.InvalidSecurityLevel, fmt.Errorf(
			"local security rejected nil remote address",
		)
	}
	switch a := addr.(type) {
	case *net.UnixAddr:
		return security.PrivacyAndIntegrity, nil
	case *net.TCPAddr:
		if a.IP != nil && a.IP.IsLoopback() {
			return security.NoSecurity, nil
		}
	case *net.IPAddr:
		if a.IP != nil && a.IP.IsLoopback() {
			return security.NoSecurity, nil
		}
	}

	host, _, err := net.SplitHostPort(addr.String())
	if err == nil {
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return security.NoSecurity, nil
		}
	}

	if network := addr.Network(); network == "unix" || network == "unixpacket" {
		return security.PrivacyAndIntegrity, nil
	}

	return security.InvalidSecurityLevel, fmt.Errorf(
		"local security rejected connection to non-local address %q",
		addr.String(),
	)
}

func getSecurityLevelFromString(remoteAddr string) (security.SecurityLevel, error) {
	host := remoteAddr
	if parsedHost, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = parsedHost
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return security.NoSecurity, nil
	}
	return security.InvalidSecurityLevel, fmt.Errorf(
		"local security rejected request from non-local address %q",
		remoteAddr,
	)
}

func authInfo(level security.SecurityLevel) security.AuthInfo {
	return security.BasicAuthInfo{
		CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: level},
		Type:           name,
	}
}
