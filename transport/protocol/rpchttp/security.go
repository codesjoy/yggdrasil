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

package rpchttp

import (
	stdtls "crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

type staticRequestAuthenticator struct {
	info security.AuthInfo
}

func (a staticRequestAuthenticator) AuthenticateRequest(*http.Request) (security.AuthInfo, error) {
	return a.info, nil
}

func defaultInsecureRequestAuthenticator() security.RequestAuthenticator {
	return staticRequestAuthenticator{
		info: security.BasicAuthInfo{
			CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: security.NoSecurity},
			Type:           string(security.ModeInsecure),
		},
	}
}

func buildSecurityMaterial(
	profiles map[string]security.Profile,
	profileName string,
	serviceName string,
	side security.Side,
) (security.Material, error) {
	if profileName == "" {
		return security.Material{
			Mode:        security.ModeInsecure,
			RequestAuth: defaultInsecureRequestAuthenticator(),
		}, nil
	}
	profile := profiles[profileName]
	if profile == nil {
		return security.Material{}, fmt.Errorf("security profile %q not found", profileName)
	}
	return profile.Build(security.BuildSpec{
		Protocol:    Protocol,
		Side:        side,
		ServiceName: serviceName,
	})
}

func buildHTTPTransport(material security.Material) (*http.Transport, string, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, "", fmt.Errorf(
			"default http transport has unexpected type %T",
			http.DefaultTransport,
		)
	}
	out := transport.Clone()
	scheme := "http"
	switch material.Mode {
	case security.ModeTLS:
		if material.ClientTLS == nil {
			return nil, "", fmt.Errorf("tls security material missing client tls config")
		}
		out.TLSClientConfig = material.ClientTLS.Clone()
		scheme = "https"
	case security.ModeInsecure, security.ModeLocal:
	default:
		return nil, "", fmt.Errorf("unsupported security mode %q", material.Mode)
	}
	return out, scheme, nil
}

func buildServerListener(lis net.Listener, material security.Material) (net.Listener, error) {
	if lis == nil {
		return nil, net.ErrClosed
	}
	switch material.Mode {
	case security.ModeTLS:
		if material.ServerTLS == nil {
			return nil, fmt.Errorf("tls security material missing server tls config")
		}
		return stdtls.NewListener(lis, material.ServerTLS.Clone()), nil
	case security.ModeInsecure, security.ModeLocal:
		return lis, nil
	default:
		return nil, fmt.Errorf("unsupported security mode %q", material.Mode)
	}
}
