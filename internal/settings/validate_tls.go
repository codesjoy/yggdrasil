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

package settings

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type tlsBuilderValidationConfig struct {
	MinVersion *string                 `mapstructure:"min_version"`
	Client     tlsSideValidationConfig `mapstructure:"client"`
	Server     tlsSideValidationConfig `mapstructure:"server"`
}

type tlsSideValidationConfig struct {
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

func validateTLSCredentialConfig(resolved Resolved, serviceName string, client bool) error {
	cfg := tlsBuilderValidationConfig{}
	if raw, ok := resolved.Transports.GRPCCredentials["tls"]; ok {
		if err := DecodePayload(&cfg, raw); err != nil {
			return err
		}
	}
	if serviceName != "" {
		if serviceCfgs, ok := resolved.Transports.GRPCServiceCredentials[serviceName]; ok {
			if raw, ok := serviceCfgs["tls"]; ok {
				override := tlsBuilderValidationConfig{}
				if err := DecodePayload(&override, raw); err != nil {
					return err
				}
				mergeTLSBuilderValidationConfig(&cfg, override)
			}
		}
	}

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.MinVersion != nil {
		version, ok := parseTLSVersion(*cfg.MinVersion)
		if !ok {
			return fmt.Errorf("invalid tls min version %q", *cfg.MinVersion)
		}
		tlsCfg.MinVersion = version
	}
	if client {
		return applyTLSClientValidation(tlsCfg, cfg.Client)
	}
	return applyTLSServerValidation(tlsCfg, cfg.Server)
}

func mergeTLSBuilderValidationConfig(dst *tlsBuilderValidationConfig, src tlsBuilderValidationConfig) {
	if src.MinVersion != nil {
		dst.MinVersion = src.MinVersion
	}
	mergeTLSSideValidationConfig(&dst.Client, src.Client)
	mergeTLSSideValidationConfig(&dst.Server, src.Server)
}

func mergeTLSSideValidationConfig(dst *tlsSideValidationConfig, src tlsSideValidationConfig) {
	if src.ServerName != "" {
		dst.ServerName = src.ServerName
	}
	if src.InsecureSkipVerify != nil {
		dst.InsecureSkipVerify = src.InsecureSkipVerify
	}
	if src.CAFile != "" {
		dst.CAFile = src.CAFile
	}
	if src.ClientCAFile != "" {
		dst.ClientCAFile = src.ClientCAFile
	}
	if src.CertFile != "" {
		dst.CertFile = src.CertFile
	}
	if src.KeyFile != "" {
		dst.KeyFile = src.KeyFile
	}
	if src.ClientAuth != nil {
		dst.ClientAuth = src.ClientAuth
	}
	if src.RequireClientCert != nil {
		dst.RequireClientCert = src.RequireClientCert
	}
	if src.SPIFFEID != "" {
		dst.SPIFFEID = src.SPIFFEID
	}
	if src.SPIFFETrustDomain != "" {
		dst.SPIFFETrustDomain = src.SPIFFETrustDomain
	}
}

func applyTLSClientValidation(tlsCfg *tls.Config, cfg tlsSideValidationConfig) error {
	if cfg.ServerName != "" {
		tlsCfg.ServerName = cfg.ServerName
	}
	if cfg.InsecureSkipVerify != nil {
		tlsCfg.InsecureSkipVerify = *cfg.InsecureSkipVerify
	}
	if cfg.CAFile != "" {
		pool, err := loadTLSCertPool(cfg.CAFile)
		if err != nil {
			return err
		}
		tlsCfg.RootCAs = pool
	}
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load tls client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return nil
}

func applyTLSServerValidation(tlsCfg *tls.Config, cfg tlsSideValidationConfig) error {
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load tls server certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	if cfg.ClientCAFile != "" {
		pool, err := loadTLSCertPool(cfg.ClientCAFile)
		if err != nil {
			return err
		}
		tlsCfg.ClientCAs = pool
	}
	switch {
	case cfg.ClientAuth != nil:
		if _, ok := parseTLSClientAuth(*cfg.ClientAuth); !ok {
			return fmt.Errorf("invalid tls client auth mode %q", *cfg.ClientAuth)
		}
	case cfg.RequireClientCert != nil && *cfg.RequireClientCert:
		if tlsCfg.ClientCAs != nil {
			tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		} else {
			tlsCfg.ClientAuth = tls.RequireAnyClientCert
		}
	case tlsCfg.ClientCAs != nil:
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return nil
}

func parseTLSVersion(v string) (uint16, bool) {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "1.0", "tls1.0", "tls10":
		return tls.VersionTLS10, true
	case "1.1", "tls1.1", "tls11":
		return tls.VersionTLS11, true
	case "1.2", "tls1.2", "tls12":
		return tls.VersionTLS12, true
	case "1.3", "tls1.3", "tls13":
		return tls.VersionTLS13, true
	default:
		return 0, false
	}
}

func parseTLSClientAuth(v string) (tls.ClientAuthType, bool) {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "none", "no_client_cert", "noclientcert":
		return tls.NoClientCert, true
	case "request", "request_client_cert", "requestclientcert":
		return tls.RequestClientCert, true
	case "require_any", "require_any_client_cert", "requireanyclientcert":
		return tls.RequireAnyClientCert, true
	case "verify_if_given", "verify_client_cert_if_given", "verifyclientcertifgiven":
		return tls.VerifyClientCertIfGiven, true
	case "require_and_verify", "require_and_verify_client_cert", "requireandverifyclientcert":
		return tls.RequireAndVerifyClientCert, true
	default:
		return tls.NoClientCert, false
	}
}

func loadTLSCertPool(path string) (*x509.CertPool, error) {
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
