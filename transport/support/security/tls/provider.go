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
	stdtls "crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

const name = "tls"

// BuiltinProvider returns the built-in TLS security profile provider.
func BuiltinProvider() security.Provider {
	return provider{}
}

type provider struct{}

func (provider) Type() string { return name }

func (provider) Compile(profileName string, raw map[string]any) (security.Profile, error) {
	cfg := Config{}
	if len(raw) != 0 {
		if err := config.NewSnapshot(raw).Decode(&cfg); err != nil {
			return nil, err
		}
	}
	clientTLS, err := buildTLSConfig(cfg, true)
	if err != nil {
		return nil, err
	}
	serverTLS, err := buildTLSConfig(cfg, false)
	if err != nil {
		return nil, err
	}
	return &profile{
		name:      profileName,
		cfg:       cfg,
		clientTLS: clientTLS,
		serverTLS: serverTLS,
	}, nil
}

// Config contains the TLS security profile configuration.
type Config struct {
	MinVersion *string    `mapstructure:"min_version"`
	Client     SideConfig `mapstructure:"client"`
	Server     SideConfig `mapstructure:"server"`
}

// SideConfig contains one side of a TLS profile configuration.
type SideConfig struct {
	ServerName         string  `mapstructure:"server_name"`
	InsecureSkipVerify *bool   `mapstructure:"insecure_skip_verify"`
	CAFile             string  `mapstructure:"ca_file"`
	ClientCAFile       string  `mapstructure:"client_ca_file"`
	CertFile           string  `mapstructure:"cert_file"`
	KeyFile            string  `mapstructure:"key_file"`
	ClientAuth         *string `mapstructure:"client_auth"`
	RequireClientCert  *bool   `mapstructure:"require_client_cert"`
	SPIFFEID           string  `mapstructure:"spiffe_id"`
	SPIFFETrustDomain  string  `mapstructure:"spiffe_trust_domain"`
}

type profile struct {
	name      string
	cfg       Config
	clientTLS *stdtls.Config
	serverTLS *stdtls.Config
}

func (p *profile) Name() string { return p.name }

func (p *profile) Type() string { return name }

func (p *profile) Build(spec security.BuildSpec) (security.Material, error) {
	material := security.Material{
		Mode:        security.ModeTLS,
		RequestAuth: requestAuthenticator{},
		ConnAuth: &connAuthenticator{
			clientTLS: p.clientTLS.Clone(),
			serverTLS: p.serverTLS.Clone(),
		},
	}
	if p.clientTLS != nil {
		clientTLS := p.clientTLS.Clone()
		if clientTLS.ServerName == "" {
			clientTLS.ServerName = authorityToServerName(spec.Authority)
		}
		material.ClientTLS = clientTLS
	}
	if p.serverTLS != nil {
		material.ServerTLS = p.serverTLS.Clone()
	}
	return material, nil
}

func buildTLSConfig(cfg Config, client bool) (*stdtls.Config, error) {
	tlsCfg := &stdtls.Config{
		MinVersion: stdtls.VersionTLS12,
	}
	if cfg.MinVersion != nil {
		v, ok := parseTLSVersion(*cfg.MinVersion)
		if !ok {
			return nil, fmt.Errorf("invalid tls min version %q", *cfg.MinVersion)
		}
		tlsCfg.MinVersion = v
	}
	if client {
		if err := applyClientConfig(tlsCfg, cfg.Client); err != nil {
			return nil, err
		}
	} else {
		if err := applyServerConfig(tlsCfg, cfg.Server); err != nil {
			return nil, err
		}
	}
	return tlsCfg, nil
}

func applyClientConfig(tlsCfg *stdtls.Config, cfg SideConfig) error {
	if cfg.ServerName != "" {
		tlsCfg.ServerName = cfg.ServerName
	}
	if cfg.InsecureSkipVerify != nil {
		tlsCfg.InsecureSkipVerify = *cfg.InsecureSkipVerify
	}
	if cfg.CAFile != "" {
		pool, err := loadCertPool(cfg.CAFile)
		if err != nil {
			return err
		}
		tlsCfg.RootCAs = pool
	}
	if (cfg.CertFile == "") != (cfg.KeyFile == "") {
		return errors.New("tls client certificate and key must be provided together")
	}
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := stdtls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load tls client certificate: %w", err)
		}
		tlsCfg.Certificates = []stdtls.Certificate{cert}
	}
	applySPIFFEVerify(tlsCfg, cfg)
	return nil
}

func applyServerConfig(tlsCfg *stdtls.Config, cfg SideConfig) error {
	if (cfg.CertFile == "") != (cfg.KeyFile == "") {
		return errors.New("tls server certificate and key must be provided together")
	}
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := stdtls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load tls server certificate: %w", err)
		}
		tlsCfg.Certificates = []stdtls.Certificate{cert}
	}
	if cfg.ClientCAFile != "" {
		pool, err := loadCertPool(cfg.ClientCAFile)
		if err != nil {
			return err
		}
		tlsCfg.ClientCAs = pool
	}
	switch {
	case cfg.ClientAuth != nil:
		if ca, ok := parseClientAuth(*cfg.ClientAuth); ok {
			tlsCfg.ClientAuth = ca
		} else {
			return fmt.Errorf("invalid tls client auth mode %q", *cfg.ClientAuth)
		}
	case cfg.RequireClientCert != nil && *cfg.RequireClientCert:
		if tlsCfg.ClientCAs != nil {
			tlsCfg.ClientAuth = stdtls.RequireAndVerifyClientCert
		} else {
			tlsCfg.ClientAuth = stdtls.RequireAnyClientCert
		}
	case tlsCfg.ClientCAs != nil:
		tlsCfg.ClientAuth = stdtls.RequireAndVerifyClientCert
	}
	applySPIFFEVerify(tlsCfg, cfg)
	return nil
}

func applySPIFFEVerify(tlsCfg *stdtls.Config, cfg SideConfig) {
	if cfg.SPIFFEID == "" && cfg.SPIFFETrustDomain == "" {
		return
	}
	prev := tlsCfg.VerifyPeerCertificate
	expectedID := cfg.SPIFFEID
	expectedTrustDomain := cfg.SPIFFETrustDomain
	tlsCfg.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if prev != nil {
			if err := prev(rawCerts, verifiedChains); err != nil {
				return err
			}
		}
		leaf, err := leafFromVerifyArgs(rawCerts, verifiedChains)
		if err != nil {
			return err
		}
		uri := security.SPIFFEIDFromCert(leaf)
		if uri == nil {
			return errors.New("tls: spiffe id missing or invalid")
		}
		if expectedID != "" && uri.String() != expectedID {
			return fmt.Errorf("tls: spiffe id mismatch: got %q want %q", uri.String(), expectedID)
		}
		if expectedTrustDomain != "" && uri.Host != expectedTrustDomain {
			return fmt.Errorf(
				"tls: spiffe trust domain mismatch: got %q want %q",
				uri.Host,
				expectedTrustDomain,
			)
		}
		return nil
	}
}

func leafFromVerifyArgs(
	rawCerts [][]byte,
	verifiedChains [][]*x509.Certificate,
) (*x509.Certificate, error) {
	if len(verifiedChains) > 0 && len(verifiedChains[0]) > 0 && verifiedChains[0][0] != nil {
		return verifiedChains[0][0], nil
	}
	if len(rawCerts) == 0 {
		return nil, errors.New("tls: no peer certificates")
	}
	cert, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		return nil, err
	}
	return cert, nil
}

func parseTLSVersion(v string) (uint16, bool) {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "1.0", "tls1.0", "tls10":
		return stdtls.VersionTLS10, true
	case "1.1", "tls1.1", "tls11":
		return stdtls.VersionTLS11, true
	case "1.2", "tls1.2", "tls12":
		return stdtls.VersionTLS12, true
	case "1.3", "tls1.3", "tls13":
		return stdtls.VersionTLS13, true
	default:
		return 0, false
	}
}

func parseClientAuth(v string) (stdtls.ClientAuthType, bool) {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "none", "no_client_cert", "noclientcert":
		return stdtls.NoClientCert, true
	case "request", "request_client_cert", "requestclientcert":
		return stdtls.RequestClientCert, true
	case "require_any", "require_any_client_cert", "requireanyclientcert":
		return stdtls.RequireAnyClientCert, true
	case "verify_if_given", "verify_client_cert_if_given", "verifyclientcertifgiven":
		return stdtls.VerifyClientCertIfGiven, true
	case "require_and_verify", "require_and_verify_client_cert", "requireandverifyclientcert":
		return stdtls.RequireAndVerifyClientCert, true
	default:
		return stdtls.NoClientCert, false
	}
}

func loadCertPool(path string) (*x509.CertPool, error) {
	path = filepath.Clean(path)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read ca file %q: %w", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(b) {
		return nil, fmt.Errorf("failed to parse ca pem %q", path)
	}
	return pool, nil
}
