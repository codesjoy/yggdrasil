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

// Package insecure provides an insecure transport security profile provider.
package insecure

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

const name = "insecure"

// BuiltinProvider returns the built-in insecure security profile provider.
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
	return security.Material{
		Mode:        security.ModeInsecure,
		RequestAuth: requestAuthenticator{},
		ConnAuth:    connAuthenticator{},
	}, nil
}

type connAuthenticator struct{}

func (connAuthenticator) ClientHandshake(
	_ context.Context,
	_ string,
	conn net.Conn,
) (net.Conn, security.AuthInfo, error) {
	return conn, authInfo(), nil
}

func (connAuthenticator) ServerHandshake(conn net.Conn) (net.Conn, security.AuthInfo, error) {
	return conn, authInfo(), nil
}

func (connAuthenticator) Info() security.ProtocolInfo {
	return security.ProtocolInfo{SecurityProtocol: name}
}

func (connAuthenticator) Clone() security.ConnAuthenticator {
	return connAuthenticator{}
}

func (connAuthenticator) OverrideServerName(string) error {
	return nil
}

type requestAuthenticator struct{}

func (requestAuthenticator) AuthenticateRequest(*http.Request) (security.AuthInfo, error) {
	return authInfo(), nil
}

func authInfo() security.AuthInfo {
	return security.BasicAuthInfo{
		CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: security.NoSecurity},
		Type:           name,
	}
}
