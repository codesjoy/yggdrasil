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

// Package security provides protocol-agnostic transport security abstractions.
package security // import "github.com/codesjoy/yggdrasil/v3/transport/support/security"

import (
	"context"
	stdtls "crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
)

// SecurityLevel defines the protection level on an established connection.
type SecurityLevel int //nolint:revive // stutter is acceptable for clarity

const (
	// InvalidSecurityLevel indicates an invalid security level.
	InvalidSecurityLevel SecurityLevel = iota
	// NoSecurity indicates a connection is insecure.
	NoSecurity
	// IntegrityOnly indicates a connection only provides integrity protection.
	IntegrityOnly
	// PrivacyAndIntegrity indicates a connection provides privacy and integrity protection.
	PrivacyAndIntegrity
)

// String returns SecurityLevel in a string format.
func (s SecurityLevel) String() string {
	switch s {
	case NoSecurity:
		return "NoSecurity"
	case IntegrityOnly:
		return "IntegrityOnly"
	case PrivacyAndIntegrity:
		return "PrivacyAndIntegrity"
	}
	return fmt.Sprintf("invalid SecurityLevel: %v", int(s))
}

// CommonAuthInfo contains authenticated information common to AuthInfo implementations.
type CommonAuthInfo struct {
	SecurityLevel SecurityLevel
}

// GetCommonAuthInfo returns the CommonAuthInfo payload.
func (c CommonAuthInfo) GetCommonAuthInfo() CommonAuthInfo {
	return c
}

// BasicAuthInfo is a simple AuthInfo implementation for protocols that only need
// a stable auth type and a security level.
type BasicAuthInfo struct {
	CommonAuthInfo
	Type string
}

// AuthType returns the security protocol type.
func (i BasicAuthInfo) AuthType() string {
	return i.Type
}

// ProtocolInfo provides information regarding the active security protocol.
type ProtocolInfo struct {
	ProtocolVersion  string
	SecurityProtocol string
	SecurityVersion  string
	ServerName       string
}

// AuthInfo defines the common interface for transport authentication information.
type AuthInfo interface {
	AuthType() string
}

// ConnAuthenticator defines connection-oriented transport security behavior.
type ConnAuthenticator interface {
	ClientHandshake(context.Context, string, net.Conn) (net.Conn, AuthInfo, error)
	ServerHandshake(net.Conn) (net.Conn, AuthInfo, error)
	Info() ProtocolInfo
	Clone() ConnAuthenticator
	OverrideServerName(string) error
}

// RequestAuthenticator defines request-oriented transport security behavior.
type RequestAuthenticator interface {
	AuthenticateRequest(*http.Request) (AuthInfo, error)
}

// Side identifies the transport direction material is being built for.
type Side string

const (
	// SideClient identifies client-side transport material.
	SideClient Side = "client"
	// SideServer identifies server-side transport material.
	SideServer Side = "server"
)

// Mode identifies the channel security mode.
type Mode string

const (
	// ModeInsecure disables transport security.
	ModeInsecure Mode = "insecure"
	// ModeLocal restricts use to local channels.
	ModeLocal Mode = "local"
	// ModeTLS enables TLS transport security.
	ModeTLS Mode = "tls"
)

// BuildSpec describes how one protocol instance needs to consume a security profile.
type BuildSpec struct {
	Protocol    string
	Side        Side
	ServiceName string
	Authority   string
}

// Material contains the compiled security material a protocol adapter needs.
type Material struct {
	Mode        Mode
	ClientTLS   *stdtls.Config
	ServerTLS   *stdtls.Config
	RequestAuth RequestAuthenticator
	ConnAuth    ConnAuthenticator
}

// Provider compiles raw config into protocol-agnostic security profiles.
type Provider interface {
	Type() string
	Compile(name string, raw map[string]any) (Profile, error)
}

// Profile materializes protocol-specific security state from a compiled profile.
type Profile interface {
	Name() string
	Type() string
	Build(spec BuildSpec) (Material, error)
}

// CheckSecurityLevel verifies that an AuthInfo value satisfies the requested level.
func CheckSecurityLevel(ai AuthInfo, level SecurityLevel) error {
	type internalInfo interface {
		GetCommonAuthInfo() CommonAuthInfo
	}
	if ai == nil {
		return errors.New("AuthInfo is nil")
	}
	if ci, ok := ai.(internalInfo); ok {
		if ci.GetCommonAuthInfo().SecurityLevel == InvalidSecurityLevel {
			return nil
		}
		if ci.GetCommonAuthInfo().SecurityLevel < level {
			return fmt.Errorf(
				"requires SecurityLevel %v; connection has %v",
				level,
				ci.GetCommonAuthInfo().SecurityLevel,
			)
		}
	}
	return nil
}
