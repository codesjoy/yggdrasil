package tls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/remote/credentials"
)

func init() {
	credentials.RegisterBuilder(name, newCredentials)
}

type builderConfig struct {
	MinVersion *string `mapstructure:"min_version"`
	Client     sideCfg `mapstructure:"client"`
	Server     sideCfg `mapstructure:"server"`
}

type sideCfg struct {
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

func newCredentials(serviceName string, client bool) credentials.TransportCredentials {
	cfg := loadBuilderConfig(serviceName)

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if cfg.MinVersion != nil {
		if v, ok := parseTLSVersion(*cfg.MinVersion); ok {
			tlsCfg.MinVersion = v
		}
	}

	if client {
		applyClientConfig(tlsCfg, cfg.Client)
	} else {
		applyServerConfig(tlsCfg, cfg.Server)
	}

	tc, err := New(Config{Client: client, TLSConfig: tlsCfg})
	if err != nil {
		slog.Warn("failed to build tls TransportCredentials", slog.Any("error", err))
		return nil
	}
	return tc
}

func loadBuilderConfig(serviceName string) builderConfig {
	keyGlobal := config.Join(config.KeyBase, "remote", "credentials", name)
	var global builderConfig
	if err := config.Get(keyGlobal).Scan(&global); err != nil {
		slog.Warn(
			"failed to scan tls global config",
			slog.Any("error", err),
			slog.String("key", keyGlobal),
		)
	}
	if serviceName == "" {
		return global
	}

	keySvc := config.Join(keyGlobal, "{"+serviceName+"}")
	var svc builderConfig
	if err := config.Get(keySvc).Scan(&svc); err != nil {
		return global
	}
	mergeBuilderConfig(&global, svc)
	return global
}

func mergeBuilderConfig(dst *builderConfig, src builderConfig) {
	if src.MinVersion != nil {
		dst.MinVersion = src.MinVersion
	}
	mergeSideCfg(&dst.Client, src.Client)
	mergeSideCfg(&dst.Server, src.Server)
}

func mergeSideCfg(dst *sideCfg, src sideCfg) {
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

func applyClientConfig(tlsCfg *tls.Config, cfg sideCfg) {
	if cfg.ServerName != "" {
		tlsCfg.ServerName = cfg.ServerName
	}
	if cfg.InsecureSkipVerify != nil {
		tlsCfg.InsecureSkipVerify = *cfg.InsecureSkipVerify
	}
	if cfg.CAFile != "" {
		if pool, ok := loadCertPool(cfg.CAFile); ok {
			tlsCfg.RootCAs = pool
		}
	}
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		if cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile); err == nil {
			tlsCfg.Certificates = []tls.Certificate{cert}
		} else {
			slog.Warn("failed to load tls client certificate", slog.Any("error", err))
		}
	}
	applySPIFFEVerify(tlsCfg, cfg)
}

func applyServerConfig(tlsCfg *tls.Config, cfg sideCfg) {
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		if cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile); err == nil {
			tlsCfg.Certificates = []tls.Certificate{cert}
		} else {
			slog.Warn("failed to load tls server certificate", slog.Any("error", err))
		}
	}

	if cfg.ClientCAFile != "" {
		if pool, ok := loadCertPool(cfg.ClientCAFile); ok {
			tlsCfg.ClientCAs = pool
		}
	}

	if cfg.ClientAuth != nil {
		if ca, ok := parseClientAuth(*cfg.ClientAuth); ok {
			tlsCfg.ClientAuth = ca
		}
	} else if cfg.RequireClientCert != nil && *cfg.RequireClientCert {
		if tlsCfg.ClientCAs != nil {
			tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		} else {
			tlsCfg.ClientAuth = tls.RequireAnyClientCert
		}
	} else if tlsCfg.ClientCAs != nil {
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	applySPIFFEVerify(tlsCfg, cfg)
}

func applySPIFFEVerify(tlsCfg *tls.Config, cfg sideCfg) {
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
		uri := credentials.SPIFFEIDFromCert(leaf)
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

func parseClientAuth(v string) (tls.ClientAuthType, bool) {
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

func loadCertPool(path string) (*x509.CertPool, bool) {
	path = filepath.Clean(path)
	b, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("failed to read ca file", slog.Any("error", err), slog.String("path", path))
		return nil, false
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(b) {
		slog.Warn("failed to parse ca pem", slog.String("path", path))
		return nil, false
	}
	return pool, true
}
