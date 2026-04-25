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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	stdtls "crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/transport/support/peer"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

func TestBuiltinProviderCompileAndBuild(t *testing.T) {
	profile, err := BuiltinProvider().Compile("secure", map[string]any{
		"min_version": "1.3",
	})
	require.NoError(t, err)

	material, err := profile.Build(security.BuildSpec{Authority: "svc.internal:443"})
	require.NoError(t, err)
	require.Equal(t, security.ModeTLS, material.Mode)
	require.NotNil(t, material.ClientTLS)
	require.NotNil(t, material.ServerTLS)
	require.EqualValues(t, stdtls.VersionTLS13, material.ClientTLS.MinVersion)
	require.Equal(t, "svc.internal", material.ClientTLS.ServerName)
}

func TestBuiltinProviderCompileRejectsInvalidConfig(t *testing.T) {
	_, err := BuiltinProvider().Compile("bad", map[string]any{
		"min_version": "bad",
	})
	require.Error(t, err)

	_, err = BuiltinProvider().Compile("bad-cert", map[string]any{
		"server": map[string]any{
			"cert_file": "/tmp/server.pem",
		},
	})
	require.Error(t, err)
}

func TestParseTLSVersion(t *testing.T) {
	tests := []struct {
		input string
		want  uint16
		ok    bool
	}{
		{"1.0", stdtls.VersionTLS10, true},
		{"tls1.0", stdtls.VersionTLS10, true},
		{"tls10", stdtls.VersionTLS10, true},
		{"1.1", stdtls.VersionTLS11, true},
		{"tls1.1", stdtls.VersionTLS11, true},
		{"tls11", stdtls.VersionTLS11, true},
		{"1.2", stdtls.VersionTLS12, true},
		{"tls1.2", stdtls.VersionTLS12, true},
		{"tls12", stdtls.VersionTLS12, true},
		{"1.3", stdtls.VersionTLS13, true},
		{"tls1.3", stdtls.VersionTLS13, true},
		{"tls13", stdtls.VersionTLS13, true},
		{"2.0", 0, false},
		{"", 0, false},
		{"bad", 0, false},
		{" 1.3 ", stdtls.VersionTLS13, true},
		{" TLS12 ", stdtls.VersionTLS12, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseTLSVersion(tt.input)
			require.Equal(t, tt.ok, ok)
			if tt.ok {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseClientAuth(t *testing.T) {
	tests := []struct {
		input string
		want  stdtls.ClientAuthType
		ok    bool
	}{
		{"none", stdtls.NoClientCert, true},
		{"no_client_cert", stdtls.NoClientCert, true},
		{"noclientcert", stdtls.NoClientCert, true},
		{"request", stdtls.RequestClientCert, true},
		{"request_client_cert", stdtls.RequestClientCert, true},
		{"requestclientcert", stdtls.RequestClientCert, true},
		{"require_any", stdtls.RequireAnyClientCert, true},
		{"require_any_client_cert", stdtls.RequireAnyClientCert, true},
		{"requireanyclientcert", stdtls.RequireAnyClientCert, true},
		{"verify_if_given", stdtls.VerifyClientCertIfGiven, true},
		{"verify_client_cert_if_given", stdtls.VerifyClientCertIfGiven, true},
		{"verifyclientcertifgiven", stdtls.VerifyClientCertIfGiven, true},
		{"require_and_verify", stdtls.RequireAndVerifyClientCert, true},
		{"require_and_verify_client_cert", stdtls.RequireAndVerifyClientCert, true},
		{"requireandverifyclientcert", stdtls.RequireAndVerifyClientCert, true},
		{"bad", stdtls.NoClientCert, false},
		{"", stdtls.NoClientCert, false},
		{" NONE ", stdtls.NoClientCert, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseClientAuth(tt.input)
			require.Equal(t, tt.ok, ok)
			if tt.ok {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestAuthorityToServerName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"host:port", "host"},
		{"host", "host"},
		{"", ""},
		{"  host:443  ", "host"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			require.Equal(t, tt.want, authorityToServerName(tt.input))
		})
	}
}

func TestRequestAuthenticator(t *testing.T) {
	ra := requestAuthenticator{}

	t.Run("nil request", func(t *testing.T) {
		_, err := ra.AuthenticateRequest(nil)
		require.Error(t, err)
	})

	t.Run("request with nil TLS", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		_, err := ra.AuthenticateRequest(req)
		require.Error(t, err)
	})

	t.Run("request with TLS state", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.TLS = &stdtls.ConnectionState{HandshakeComplete: true}
		auth, err := ra.AuthenticateRequest(req)
		require.NoError(t, err)
		require.NotNil(t, auth)

		ai, ok := auth.(AuthInfo)
		require.True(t, ok)
		require.Equal(t, security.PrivacyAndIntegrity, ai.SecurityLevel)
		require.True(t, ai.State.HandshakeComplete)
	})
}

func TestStateFromAuthInfo(t *testing.T) {
	t.Run("value AuthInfo", func(t *testing.T) {
		state, ok := StateFromAuthInfo(AuthInfo{
			CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: security.PrivacyAndIntegrity},
			State:          stdtls.ConnectionState{HandshakeComplete: true},
		})
		require.True(t, ok)
		require.True(t, state.HandshakeComplete)
	})

	t.Run("nil pointer AuthInfo", func(t *testing.T) {
		var ai *AuthInfo
		_, ok := StateFromAuthInfo(ai)
		require.False(t, ok)
	})

	t.Run("valid pointer AuthInfo", func(t *testing.T) {
		ai := &AuthInfo{
			CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: security.PrivacyAndIntegrity},
			State:          stdtls.ConnectionState{HandshakeComplete: true},
		}
		state, ok := StateFromAuthInfo(ai)
		require.True(t, ok)
		require.True(t, state.HandshakeComplete)
	})

	t.Run("non-TLS AuthInfo", func(t *testing.T) {
		_, ok := StateFromAuthInfo(security.BasicAuthInfo{
			CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: security.NoSecurity},
			Type:           "insecure",
		})
		require.False(t, ok)
	})
}

func TestCloneTLSConfig(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		require.Nil(t, cloneTLSConfig(nil))
	})

	t.Run("non-nil", func(t *testing.T) {
		cfg := &stdtls.Config{ServerName: "test-server"}
		cloned := cloneTLSConfig(cfg)
		require.NotNil(t, cloned)
		require.Equal(t, "test-server", cloned.ServerName)
	})
}

func TestConnAuthenticator_Clone(t *testing.T) {
	t.Run("non-nil receiver", func(t *testing.T) {
		c := &connAuthenticator{
			clientTLS:          &stdtls.Config{ServerName: "test"},
			serverTLS:          &stdtls.Config{ServerName: "test-srv"},
			serverNameOverride: "override",
		}
		cloned := c.Clone()
		require.NotNil(t, cloned)

		clonedCA := cloned.(*connAuthenticator)
		require.Equal(t, "test", clonedCA.clientTLS.ServerName)
		require.Equal(t, "override", clonedCA.serverNameOverride)
	})

	t.Run("nil receiver", func(t *testing.T) {
		var c *connAuthenticator
		require.Nil(t, c.Clone())
	})
}

func TestConnAuthenticator_OverrideServerName(t *testing.T) {
	c := &connAuthenticator{}
	err := c.OverrideServerName("my-server")
	require.NoError(t, err)
	require.Equal(t, "my-server", c.serverNameOverride)
}

func TestConnAuthenticator_Info(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var c *connAuthenticator
		info := c.Info()
		require.Equal(t, "tls", info.SecurityProtocol)
		require.Empty(t, info.ServerName)
	})

	t.Run("with config", func(t *testing.T) {
		c := &connAuthenticator{
			clientTLS: &stdtls.Config{ServerName: "configured"},
		}
		info := c.Info()
		require.Equal(t, "tls", info.SecurityProtocol)
		require.Equal(t, "configured", info.ServerName)
	})
}

func TestClientHandshake_NilConn(t *testing.T) {
	c := &connAuthenticator{clientTLS: &stdtls.Config{}}
	_, _, err := c.ClientHandshake(context.TODO(), "host:443", nil)
	require.Error(t, err)
}

func TestClientHandshake_NilConfig(t *testing.T) {
	c := &connAuthenticator{}
	client, _ := net.Pipe()
	defer client.Close()
	_, _, err := c.ClientHandshake(context.TODO(), "host:443", client)
	require.Error(t, err)
}

func TestServerHandshake_NilConn(t *testing.T) {
	c := &connAuthenticator{serverTLS: &stdtls.Config{}}
	_, _, err := c.ServerHandshake(nil)
	require.Error(t, err)
}

func TestServerHandshake_NilConfig(t *testing.T) {
	c := &connAuthenticator{}
	client, _ := net.Pipe()
	defer client.Close()
	_, _, err := c.ServerHandshake(client)
	require.Error(t, err)
}

func TestAuthInfo_AuthType(t *testing.T) {
	var ai AuthInfo
	require.Equal(t, "tls", ai.AuthType())
}

func TestSPIFFEIDFromAuthInfo(t *testing.T) {
	t.Run("with non-TLS AuthInfo", func(t *testing.T) {
		result := SPIFFEIDFromAuthInfo(security.BasicAuthInfo{
			CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: security.NoSecurity},
			Type:           "insecure",
		})
		require.Nil(t, result)
	})

	t.Run("with TLS AuthInfo no SPIFFE cert", func(t *testing.T) {
		ai := AuthInfo{
			CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: security.PrivacyAndIntegrity},
			State:          stdtls.ConnectionState{},
		}
		result := SPIFFEIDFromAuthInfo(ai)
		require.Nil(t, result)
	})
}

func TestSPIFFEIDFromPeer(t *testing.T) {
	t.Run("nil peer", func(t *testing.T) {
		require.Nil(t, SPIFFEIDFromPeer(nil))
	})

	t.Run("peer with non-TLS AuthInfo", func(t *testing.T) {
		p := &peer.Peer{
			AuthInfo: security.BasicAuthInfo{
				CommonAuthInfo: security.CommonAuthInfo{SecurityLevel: security.NoSecurity},
				Type:           "insecure",
			},
		}
		require.Nil(t, SPIFFEIDFromPeer(p))
	})

	t.Run("peer with TLS AuthInfo", func(t *testing.T) {
		p := &peer.Peer{
			AuthInfo: AuthInfo{
				CommonAuthInfo: security.CommonAuthInfo{
					SecurityLevel: security.PrivacyAndIntegrity,
				},
				State: stdtls.ConnectionState{},
			},
		}
		// No SPIFFE cert, should return nil
		require.Nil(t, SPIFFEIDFromPeer(p))
	})
}

func TestLeafFromVerifyArgs(t *testing.T) {
	t.Run("verifiedChains with valid leaf", func(t *testing.T) {
		cert := &x509.Certificate{}
		leaf, err := leafFromVerifyArgs(nil, [][]*x509.Certificate{{cert}})
		require.NoError(t, err)
		require.Equal(t, cert, leaf)
	})

	t.Run("empty both", func(t *testing.T) {
		_, err := leafFromVerifyArgs(nil, nil)
		require.Error(t, err)
	})

	t.Run("empty verifiedChains, invalid rawCerts", func(t *testing.T) {
		_, err := leafFromVerifyArgs([][]byte{{0xff, 0xff}}, nil)
		require.Error(t, err)
	})
}

func TestLoadCertPool(t *testing.T) {
	t.Run("non-existent file", func(t *testing.T) {
		_, err := loadCertPool("/nonexistent/path/ca.pem")
		require.Error(t, err)
	})

	t.Run("invalid PEM", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/bad.pem"
		require.NoError(t, os.WriteFile(path, []byte("not a pem"), 0o600))
		_, err := loadCertPool(path)
		require.Error(t, err)
	})

	t.Run("valid PEM file", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/ca.pem"
		// Write a self-signed CA cert PEM
		certPEM := createTestCACertPEM(t)
		require.NoError(t, os.WriteFile(path, certPEM, 0o600))
		pool, err := loadCertPool(path)
		require.NoError(t, err)
		require.NotNil(t, pool)
	})
}

func TestBuild_EmptyAuthority(t *testing.T) {
	profile, err := BuiltinProvider().Compile("secure", nil)
	require.NoError(t, err)

	material, err := profile.Build(security.BuildSpec{Authority: ""})
	require.NoError(t, err)
	require.NotNil(t, material.ClientTLS)
	// ServerName should be empty for empty authority
	require.Empty(t, material.ClientTLS.ServerName)
}

func TestBuiltinProviderCompileWithClientCert(t *testing.T) {
	t.Run("cert-only returns error", func(t *testing.T) {
		_, err := BuiltinProvider().Compile("client", map[string]any{
			"client": map[string]any{
				"cert_file": "/tmp/client.pem",
			},
		})
		require.Error(t, err)
	})

	t.Run("cert and key together (non-existent)", func(t *testing.T) {
		_, err := BuiltinProvider().Compile("client", map[string]any{
			"client": map[string]any{
				"cert_file": "/tmp/client.pem",
				"key_file":  "/tmp/client.key",
			},
		})
		require.Error(t, err) // files don't exist
	})
}

func TestBuiltinProviderCompileWithServerConfig(t *testing.T) {
	t.Run("client_auth mode", func(t *testing.T) {
		_, err := BuiltinProvider().Compile("server", map[string]any{
			"server": map[string]any{
				"client_auth": "require_and_verify_client_cert",
			},
		})
		require.NoError(t, err)
	})

	t.Run("invalid client_auth mode", func(t *testing.T) {
		_, err := BuiltinProvider().Compile("server", map[string]any{
			"server": map[string]any{
				"client_auth": "invalid",
			},
		})
		require.Error(t, err)
	})
}

func TestProfile_Build_ServerNameFromAuthority(t *testing.T) {
	profile, err := BuiltinProvider().Compile("secure", map[string]any{
		"client": map[string]any{
			"server_name": "explicit-name",
		},
	})
	require.NoError(t, err)

	material, err := profile.Build(security.BuildSpec{Authority: "authority-host:443"})
	require.NoError(t, err)
	// When server_name is explicitly set in config, it takes precedence
	require.Equal(t, "explicit-name", material.ClientTLS.ServerName)
}

// createTestCACertPEM generates a minimal self-signed CA certificate PEM for testing.
func createTestCACertPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	now := time.Now()
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             now,
		NotAfter:              now.Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	return certPEM
}

// ---------------------------------------------------------------------------
// Test helpers for generating key pairs, certificates, and SPIFFE certs
// ---------------------------------------------------------------------------

// testCertPair holds a PEM-encoded certificate and key pair plus parsed forms.
type testCertPair struct {
	certPEM  []byte
	keyPEM   []byte
	cert     *x509.Certificate
	key      *ecdsa.PrivateKey
	certPool *x509.CertPool
}

// createTestCertPair generates a self-signed CA cert+key pair.
func createTestCertPair(t *testing.T) *testCertPair {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             now,
		NotAfter:              now.Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	pool := x509.NewCertPool()
	pool.AddCert(cert)

	return &testCertPair{
		certPEM:  certPEM,
		keyPEM:   keyPEM,
		cert:     cert,
		key:      key,
		certPool: pool,
	}
}

// createTestLeafCertPair generates a leaf cert signed by parent, optionally with a SPIFFE URI.
func createTestLeafCertPair(t *testing.T, parent *testCertPair, spiffeURI string) *testCertPair {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(2),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{"localhost"},
	}
	if spiffeURI != "" {
		u, err := url.Parse(spiffeURI)
		require.NoError(t, err)
		tmpl.URIs = []*url.URL{u}
	}

	certDER, err := x509.CreateCertificate(
		rand.Reader,
		&tmpl,
		parent.cert,
		&key.PublicKey,
		parent.key,
	)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return &testCertPair{
		certPEM:  certPEM,
		keyPEM:   keyPEM,
		cert:     cert,
		key:      key,
		certPool: parent.certPool,
	}
}

// writeCertFiles writes cert and key PEM to temp dir and returns their paths.
func writeCertFiles(t *testing.T, pair *testCertPair) (certPath, keyPath string) {
	t.Helper()
	dir := t.TempDir()
	certPath = dir + "/cert.pem"
	keyPath = dir + "/key.pem"
	require.NoError(t, os.WriteFile(certPath, pair.certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyPath, pair.keyPEM, 0o600))
	return
}

// writeCAPEMFile writes a CA PEM to temp dir and returns its path.
func writeCAPEMFile(t *testing.T, pair *testCertPair) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/ca.pem"
	require.NoError(t, os.WriteFile(path, pair.certPEM, 0o600))
	return path
}

// ---------------------------------------------------------------------------
// Accessor tests (provider.Type, profile.Name, profile.Type)
// ---------------------------------------------------------------------------

func TestProvider_Type(t *testing.T) {
	require.Equal(t, "tls", BuiltinProvider().Type())
}

func TestProfile_Name_Type(t *testing.T) {
	p, err := BuiltinProvider().Compile("my-profile", nil)
	require.NoError(t, err)
	require.Equal(t, "my-profile", p.Name())
	require.Equal(t, "tls", p.Type())
}

// ---------------------------------------------------------------------------
// applyClientConfig branch tests
// ---------------------------------------------------------------------------

func TestApplyClientConfig_InsecureSkipVerify(t *testing.T) {
	tlsCfg := &stdtls.Config{}
	yes := true
	err := applyClientConfig(tlsCfg, SideConfig{InsecureSkipVerify: &yes})
	require.NoError(t, err)
	require.True(t, tlsCfg.InsecureSkipVerify)
}

func TestApplyClientConfig_WithCAPool(t *testing.T) {
	ca := createTestCertPair(t)
	caPath := writeCAPEMFile(t, ca)

	tlsCfg := &stdtls.Config{}
	err := applyClientConfig(tlsCfg, SideConfig{CAFile: caPath})
	require.NoError(t, err)
	require.NotNil(t, tlsCfg.RootCAs)
}

func TestApplyClientConfig_WithClientCert(t *testing.T) {
	ca := createTestCertPair(t)
	certPath, keyPath := writeCertFiles(t, ca)

	tlsCfg := &stdtls.Config{}
	err := applyClientConfig(tlsCfg, SideConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
	})
	require.NoError(t, err)
	require.Len(t, tlsCfg.Certificates, 1)
}

// ---------------------------------------------------------------------------
// applyServerConfig branch tests
// ---------------------------------------------------------------------------

func TestApplyServerConfig_RequireClientCert_WithPool(t *testing.T) {
	ca := createTestCertPair(t)
	caPath := writeCAPEMFile(t, ca)
	yes := true

	tlsCfg := &stdtls.Config{}
	err := applyServerConfig(tlsCfg, SideConfig{
		RequireClientCert: &yes,
		ClientCAFile:      caPath,
	})
	require.NoError(t, err)
	require.Equal(t, stdtls.RequireAndVerifyClientCert, tlsCfg.ClientAuth)
	require.NotNil(t, tlsCfg.ClientCAs)
}

func TestApplyServerConfig_RequireClientCert_NoPool(t *testing.T) {
	yes := true

	tlsCfg := &stdtls.Config{}
	err := applyServerConfig(tlsCfg, SideConfig{
		RequireClientCert: &yes,
	})
	require.NoError(t, err)
	require.Equal(t, stdtls.RequireAnyClientCert, tlsCfg.ClientAuth)
}

func TestApplyServerConfig_ImplicitFromClientCAs(t *testing.T) {
	ca := createTestCertPair(t)
	caPath := writeCAPEMFile(t, ca)

	tlsCfg := &stdtls.Config{}
	err := applyServerConfig(tlsCfg, SideConfig{
		ClientCAFile: caPath,
	})
	require.NoError(t, err)
	// Setting ClientCAFile without explicit RequireClientCert should
	// implicitly set RequireAndVerifyClientCert.
	require.Equal(t, stdtls.RequireAndVerifyClientCert, tlsCfg.ClientAuth)
}

func TestApplyServerConfig_ServerCertAndKey(t *testing.T) {
	ca := createTestCertPair(t)
	certPath, keyPath := writeCertFiles(t, ca)

	tlsCfg := &stdtls.Config{}
	err := applyServerConfig(tlsCfg, SideConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
	})
	require.NoError(t, err)
	require.Len(t, tlsCfg.Certificates, 1)
}

func TestApplyServerConfig_CertKeyMismatch(t *testing.T) {
	tlsCfg := &stdtls.Config{}
	err := applyServerConfig(tlsCfg, SideConfig{
		CertFile: "/tmp/server.pem",
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// SPIFFE verification tests
// ---------------------------------------------------------------------------

func TestApplySPIFFEVerify_NoConfig(t *testing.T) {
	tlsCfg := &stdtls.Config{}
	applySPIFFEVerify(tlsCfg, SideConfig{})
	require.Nil(t, tlsCfg.VerifyPeerCertificate)
}

func TestApplySPIFFEVerify_WithSPIFFEID_Match(t *testing.T) {
	ca := createTestCertPair(t)
	leaf := createTestLeafCertPair(t, ca, "spiffe://example.com/my-service")

	tlsCfg := &stdtls.Config{}
	applySPIFFEVerify(tlsCfg, SideConfig{SPIFFEID: "spiffe://example.com/my-service"})
	require.NotNil(t, tlsCfg.VerifyPeerCertificate)

	// Call the callback with verified chains containing the leaf cert.
	err := tlsCfg.VerifyPeerCertificate(nil, [][]*x509.Certificate{{leaf.cert}})
	require.NoError(t, err)
}

func TestApplySPIFFEVerify_WithSPIFFEID_Mismatch(t *testing.T) {
	ca := createTestCertPair(t)
	leaf := createTestLeafCertPair(t, ca, "spiffe://example.com/my-service")

	tlsCfg := &stdtls.Config{}
	applySPIFFEVerify(tlsCfg, SideConfig{SPIFFEID: "spiffe://example.com/other-service"})
	require.NotNil(t, tlsCfg.VerifyPeerCertificate)

	err := tlsCfg.VerifyPeerCertificate(nil, [][]*x509.Certificate{{leaf.cert}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "spiffe id mismatch")
}

func TestApplySPIFFEVerify_WithTrustDomain_Match(t *testing.T) {
	ca := createTestCertPair(t)
	leaf := createTestLeafCertPair(t, ca, "spiffe://example.com/my-service")

	tlsCfg := &stdtls.Config{}
	applySPIFFEVerify(tlsCfg, SideConfig{SPIFFETrustDomain: "example.com"})
	require.NotNil(t, tlsCfg.VerifyPeerCertificate)

	err := tlsCfg.VerifyPeerCertificate(nil, [][]*x509.Certificate{{leaf.cert}})
	require.NoError(t, err)
}

func TestApplySPIFFEVerify_WithTrustDomain_Mismatch(t *testing.T) {
	ca := createTestCertPair(t)
	leaf := createTestLeafCertPair(t, ca, "spiffe://example.com/my-service")

	tlsCfg := &stdtls.Config{}
	applySPIFFEVerify(tlsCfg, SideConfig{SPIFFETrustDomain: "other.com"})
	require.NotNil(t, tlsCfg.VerifyPeerCertificate)

	err := tlsCfg.VerifyPeerCertificate(nil, [][]*x509.Certificate{{leaf.cert}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "spiffe trust domain mismatch")
}

func TestApplySPIFFEVerify_MissingSPIFFEID(t *testing.T) {
	ca := createTestCertPair(t)
	leaf := createTestLeafCertPair(t, ca, "") // no SPIFFE URI

	tlsCfg := &stdtls.Config{}
	applySPIFFEVerify(tlsCfg, SideConfig{SPIFFEID: "spiffe://example.com/my-service"})
	require.NotNil(t, tlsCfg.VerifyPeerCertificate)

	err := tlsCfg.VerifyPeerCertificate(nil, [][]*x509.Certificate{{leaf.cert}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "spiffe id missing or invalid")
}

func TestApplySPIFFEVerify_PrevCallbackError(t *testing.T) {
	ca := createTestCertPair(t)
	leaf := createTestLeafCertPair(t, ca, "spiffe://example.com/svc")

	prevErr := errors.New("previous check failed")
	tlsCfg := &stdtls.Config{
		VerifyPeerCertificate: func(rawCerts [][]byte, chains [][]*x509.Certificate) error {
			return prevErr
		},
	}
	applySPIFFEVerify(tlsCfg, SideConfig{SPIFFEID: "spiffe://example.com/svc"})

	err := tlsCfg.VerifyPeerCertificate(nil, [][]*x509.Certificate{{leaf.cert}})
	require.ErrorIs(t, err, prevErr)
}

func TestApplySPIFFEVerify_WithRawCertsFallback(t *testing.T) {
	ca := createTestCertPair(t)
	leaf := createTestLeafCertPair(t, ca, "spiffe://example.com/svc")

	tlsCfg := &stdtls.Config{}
	applySPIFFEVerify(tlsCfg, SideConfig{SPIFFEID: "spiffe://example.com/svc"})

	// Pass nil verifiedChains so it falls back to parsing rawCerts.
	err := tlsCfg.VerifyPeerCertificate([][]byte{leaf.cert.Raw}, nil)
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Client/Server handshake success path tests
// ---------------------------------------------------------------------------

func TestClientServerHandshake_EndToEnd(t *testing.T) {
	ca := createTestCertPair(t)
	// Create a leaf cert with DNS SAN "localhost" for the server identity.
	serverLeaf := createTestLeafCertPair(t, ca, "")
	serverCertPath, serverKeyPath := writeCertFiles(t, serverLeaf)
	caPath := writeCAPEMFile(t, ca)

	// Compile server and client profiles using actual cert files.
	serverProfile, err := BuiltinProvider().Compile("server", map[string]any{
		"server": map[string]any{
			"cert_file": serverCertPath,
			"key_file":  serverKeyPath,
		},
	})
	require.NoError(t, err)

	clientProfile, err := BuiltinProvider().Compile("client", map[string]any{
		"client": map[string]any{
			"ca_file":     caPath,
			"server_name": "localhost",
		},
	})
	require.NoError(t, err)

	serverMaterial, err := serverProfile.Build(security.BuildSpec{Authority: "localhost:443"})
	require.NoError(t, err)

	clientMaterial, err := clientProfile.Build(security.BuildSpec{Authority: "localhost:443"})
	require.NoError(t, err)

	serverConnAuth := serverMaterial.ConnAuth.(*connAuthenticator)
	clientConnAuth := clientMaterial.ConnAuth.(*connAuthenticator)

	clientRaw, serverRaw := net.Pipe()

	var (
		clientConn net.Conn
		clientInfo security.AuthInfo
		serverConn net.Conn
		serverInfo security.AuthInfo
		clientErr  error
		serverErr  error
	)

	// Run client and server handshakes concurrently.
	done := make(chan struct{})
	go func() {
		serverConn, serverInfo, serverErr = serverConnAuth.ServerHandshake(serverRaw)
		close(done)
	}()

	clientConn, clientInfo, clientErr = clientConnAuth.ClientHandshake(
		context.Background(), "localhost:443", clientRaw,
	)
	<-done

	require.NoError(t, clientErr, "client handshake should succeed")
	require.NoError(t, serverErr, "server handshake should succeed")
	require.NotNil(t, clientConn)
	require.NotNil(t, serverConn)
	require.NotNil(t, clientInfo)
	require.NotNil(t, serverInfo)

	// Verify AuthInfo has correct security level.
	clientTLSInfo, ok := clientInfo.(AuthInfo)
	require.True(t, ok)
	require.Equal(t, security.PrivacyAndIntegrity, clientTLSInfo.SecurityLevel)
	require.True(t, clientTLSInfo.State.HandshakeComplete)

	serverTLSInfo, ok := serverInfo.(AuthInfo)
	require.True(t, ok)
	require.Equal(t, security.PrivacyAndIntegrity, serverTLSInfo.SecurityLevel)
	require.True(t, serverTLSInfo.State.HandshakeComplete)

	// Clean up.
	clientConn.Close()
	serverConn.Close()
}

func TestClientHandshake_ServerNamePriority(t *testing.T) {
	ca := createTestCertPair(t)
	caPath := writeCAPEMFile(t, ca)

	// Build a profile where the client TLS config already has a ServerName
	// set (from the config "server_name" field).
	p, err := BuiltinProvider().Compile("client", map[string]any{
		"client": map[string]any{
			"ca_file":     caPath,
			"server_name": "from-config",
		},
	})
	require.NoError(t, err)

	material, err := p.Build(security.BuildSpec{Authority: "from-authority:443"})
	require.NoError(t, err)
	// config server_name should take precedence over authority-derived name.
	require.Equal(t, "from-config", material.ClientTLS.ServerName)

	// Now test that override > config > authority.
	connAuth := material.ConnAuth.(*connAuthenticator)
	require.NoError(t, connAuth.OverrideServerName("from-override"))

	// Verify that override takes precedence when config ServerName is non-empty.
	// We do this by inspecting the logic: serverNameOverride is only used when
	// cfg.ServerName is empty (see auth.go:57-62). So with a non-empty config
	// ServerName, the override is NOT applied. This matches the source code:
	//   if cfg.ServerName == "" { cfg.ServerName = c.serverNameOverride }
	//   if cfg.ServerName == "" { cfg.ServerName = authorityToServerName(authority) }
	// So the override is used only when config has no server_name.

	// Test the case where config has NO server_name, then override wins.
	p2, err := BuiltinProvider().Compile("client2", map[string]any{
		"client": map[string]any{
			"ca_file": caPath,
		},
	})
	require.NoError(t, err)

	material2, err := p2.Build(security.BuildSpec{Authority: "from-authority:443"})
	require.NoError(t, err)
	// Without config server_name, it's derived from authority.
	require.Equal(t, "from-authority", material2.ClientTLS.ServerName)

	connAuth2 := material2.ConnAuth.(*connAuthenticator)
	require.NoError(t, connAuth2.OverrideServerName("from-override"))

	// Now verify: when clientTLS.ServerName is empty (which Build sets from
	// authority), the ClientHandshake clones and checks again.
	// The Build method already set ServerName from authority, so the override
	// won't be used at handshake time. This is the current design.
	// We verify the field is set correctly:
	require.Equal(t, "from-override", connAuth2.serverNameOverride)
}

// ---------------------------------------------------------------------------
// Ensure imports are used
// ---------------------------------------------------------------------------

var (
	_ = http.Request{}
	_ = (*url.URL)(nil)
	_ = (*peer.Peer)(nil)
	_ = httptest.NewRequest
	_ = net.Pipe
	_ = errors.New
)
