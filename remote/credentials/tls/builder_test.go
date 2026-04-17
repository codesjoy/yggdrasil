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
	"crypto/rand"
	"crypto/rsa"
	stdtls "crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/remote/credentials"
)

func TestBuilderRegistered(t *testing.T) {
	if credentials.GetBuilder("tls") == nil {
		t.Fatalf("credentials.GetBuilder(tls) = nil, want non-nil")
	}
}

func TestBuilderFromConfig_Server(t *testing.T) {
	origKeyBase := config.KeyBase
	config.KeyBase = "yggdrasil_test_tls_builder_server"
	t.Cleanup(func() { config.KeyBase = origKeyBase })

	caCert, caKey, _ := newCA(t)
	serverCert := newLeafCert(t, caCert, caKey, x509.ExtKeyUsageServerAuth, []string{"example.com"})

	certFile := writeTempPEM(
		t,
		"server-cert-*.pem",
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCert.Certificate[0]}),
	)
	keyFile := writeTempPEM(
		t,
		"server-key-*.pem",
		pem.EncodeToMemory(
			&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(serverCert.PrivateKey.(*rsa.PrivateKey)),
			},
		),
	)

	key := config.Join(config.KeyBase, "remote", "credentials", "tls")
	if err := config.Set(key, map[string]any{
		"server": map[string]any{
			"cert_file": certFile,
			"key_file":  keyFile,
		},
	}); err != nil {
		t.Fatalf("config.Set: %v", err)
	}

	b := credentials.GetBuilder("tls")
	tc := b("svc", false)
	if tc == nil {
		t.Fatalf("builder returned nil TransportCredentials")
	}
	if got := tc.Info().SecurityProtocol; got != "tls" {
		t.Fatalf("tc.Info().SecurityProtocol=%q, want %q", got, "tls")
	}
}

func TestBuilderFromConfig_Client(t *testing.T) {
	origKeyBase := config.KeyBase
	config.KeyBase = "yggdrasil_test_tls_builder_client"
	t.Cleanup(func() { config.KeyBase = origKeyBase })

	caCert, caKey, _ := newCA(t)
	caPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})
	caFile := writeTempPEM(t, "ca-*.pem", caPem)

	key := config.Join(config.KeyBase, "remote", "credentials", "tls")
	if err := config.Set(key, map[string]any{
		"min_version": "1.2",
		"client": map[string]any{
			"server_name": "example.com",
			"ca_file":     caFile,
		},
	}); err != nil {
		t.Fatalf("config.Set: %v", err)
	}

	b := credentials.GetBuilder("tls")
	tc := b("svc", true)
	if tc == nil {
		t.Fatalf("builder returned nil TransportCredentials")
	}
	info := tc.Info()
	if info.SecurityProtocol != "tls" {
		t.Fatalf("SecurityProtocol=%q, want %q", info.SecurityProtocol, "tls")
	}
	if info.ServerName != "example.com" {
		t.Fatalf("ServerName=%q, want %q", info.ServerName, "example.com")
	}

	c, ok := tc.(*transportCredentials)
	if !ok {
		t.Fatalf("TransportCredentials type=%T, want *transportCredentials", tc)
	}
	if c.cfg.RootCAs == nil {
		t.Fatalf("RootCAs=nil, want non-nil")
	}
	verifyRootsCanVerify(t, c.cfg.RootCAs, caCert, caKey)
}

func TestBuilderSPIFFEIDMatch(t *testing.T) {
	origKeyBase := config.KeyBase
	config.KeyBase = "yggdrasil_test_tls_builder_spiffe_match"
	t.Cleanup(func() { config.KeyBase = origKeyBase })

	const serverName = "example.com"
	const spiffeID = "spiffe://foo.bar.com/server/workload/1"

	caCert, caKey, _ := newCA(t)
	serverCert := newLeafCertWithURI(
		t,
		caCert,
		caKey,
		x509.ExtKeyUsageServerAuth,
		[]string{serverName},
		spiffeID,
	)

	certFile := writeTempPEM(
		t,
		"server-cert-*.pem",
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCert.Certificate[0]}),
	)
	keyFile := writeTempPEM(
		t,
		"server-key-*.pem",
		pem.EncodeToMemory(
			&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(serverCert.PrivateKey.(*rsa.PrivateKey)),
			},
		),
	)
	caFile := writeTempPEM(
		t,
		"ca-*.pem",
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw}),
	)

	key := config.Join(config.KeyBase, "remote", "credentials", "tls")
	if err := config.Set(key, map[string]any{
		"client": map[string]any{
			"server_name": serverName,
			"ca_file":     caFile,
			"spiffe_id":   spiffeID,
		},
		"server": map[string]any{
			"cert_file": certFile,
			"key_file":  keyFile,
		},
	}); err != nil {
		t.Fatalf("config.Set: %v", err)
	}

	b := credentials.GetBuilder("tls")
	serverCreds := b("svc", false)
	clientCreds := b("svc", true)
	if serverCreds == nil || clientCreds == nil {
		t.Fatalf("builder returned nil creds: server=%v client=%v", serverCreds, clientCreds)
	}

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	serverErrCh := make(chan error, 1)
	go func() {
		_, _, err := serverCreds.ServerHandshake(c1)
		serverErrCh <- err
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, ai, clientErr := clientCreds.ClientHandshake(ctx, serverName+":443", c2)
	serverErr := <-serverErrCh

	if clientErr != nil || serverErr != nil {
		t.Fatalf("handshake errors: client=%v server=%v", clientErr, serverErr)
	}

	if got := SPIFFEIDFromAuthInfo(ai); got == nil || got.String() != spiffeID {
		t.Fatalf("SPIFFEIDFromAuthInfo got %v, want %q", got, spiffeID)
	}

	if c, ok := clientCreds.(*transportCredentials); ok {
		if c.cfg.RootCAs == nil {
			t.Fatalf("client RootCAs not set as expected")
		}
		verifyRootsCanVerify(t, c.cfg.RootCAs, caCert, caKey)
	}
}

func verifyRootsCanVerify(
	t *testing.T,
	roots *x509.CertPool,
	caCert *x509.Certificate,
	caKey *rsa.PrivateKey,
) {
	t.Helper()
	if roots == nil {
		t.Fatalf("roots=nil")
	}
	leafTLS := newLeafCert(t, caCert, caKey, x509.ExtKeyUsageServerAuth, []string{"example.com"})
	leaf, err := x509.ParseCertificate(leafTLS.Certificate[0])
	if err != nil {
		t.Fatalf("x509.ParseCertificate: %v", err)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:     roots,
		DNSName:   "example.com",
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Fatalf("leaf.Verify failed: %v", err)
	}
}

func TestBuilderSPIFFEIDMismatchFails(t *testing.T) {
	origKeyBase := config.KeyBase
	config.KeyBase = "yggdrasil_test_tls_builder_spiffe_mismatch"
	t.Cleanup(func() { config.KeyBase = origKeyBase })

	const serverName = "example.com"
	const serverSPIFFE = "spiffe://foo.bar.com/server/workload/1"
	const wantSPIFFE = "spiffe://foo.bar.com/server/workload/2"

	caCert, caKey, _ := newCA(t)
	serverCert := newLeafCertWithURI(
		t,
		caCert,
		caKey,
		x509.ExtKeyUsageServerAuth,
		[]string{serverName},
		serverSPIFFE,
	)

	certFile := writeTempPEM(
		t,
		"server-cert-*.pem",
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCert.Certificate[0]}),
	)
	keyFile := writeTempPEM(
		t,
		"server-key-*.pem",
		pem.EncodeToMemory(
			&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(serverCert.PrivateKey.(*rsa.PrivateKey)),
			},
		),
	)
	caFile := writeTempPEM(
		t,
		"ca-*.pem",
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw}),
	)

	key := config.Join(config.KeyBase, "remote", "credentials", "tls")
	if err := config.Set(key, map[string]any{
		"client": map[string]any{
			"server_name": serverName,
			"ca_file":     caFile,
			"spiffe_id":   wantSPIFFE,
		},
		"server": map[string]any{
			"cert_file": certFile,
			"key_file":  keyFile,
		},
	}); err != nil {
		t.Fatalf("config.Set: %v", err)
	}

	b := credentials.GetBuilder("tls")
	serverCreds := b("svc", false)
	clientCreds := b("svc", true)
	if serverCreds == nil || clientCreds == nil {
		t.Fatalf("builder returned nil creds: server=%v client=%v", serverCreds, clientCreds)
	}

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	serverErrCh := make(chan error, 1)
	go func() {
		_, _, err := serverCreds.ServerHandshake(c1)
		serverErrCh <- err
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _, clientErr := clientCreds.ClientHandshake(ctx, serverName+":443", c2)
	serverErr := <-serverErrCh

	if clientErr == nil && serverErr == nil {
		t.Fatalf("handshake error = nil, want non-nil due to spiffe mismatch")
	}
}

func TestBuilderReturnsNilOnInvalidFiles(t *testing.T) {
	origKeyBase := config.KeyBase
	config.KeyBase = "yggdrasil_test_tls_builder_invalid_files"
	t.Cleanup(func() { config.KeyBase = origKeyBase })

	key := config.Join(config.KeyBase, "remote", "credentials", "tls")
	if err := config.Set(key, map[string]any{
		"server": map[string]any{
			"cert_file": "/tmp/missing-server-cert.pem",
			"key_file":  "/tmp/missing-server-key.pem",
		},
		"client": map[string]any{
			"ca_file": "/tmp/missing-client-ca.pem",
		},
	}); err != nil {
		t.Fatalf("config.Set: %v", err)
	}

	b := credentials.GetBuilder("tls")
	if got := b("svc", false); got != nil {
		t.Fatalf("server builder returned %T, want nil", got)
	}
	if got := b("svc", true); got != nil {
		t.Fatalf("client builder returned %T, want nil", got)
	}
}

func newLeafCertWithURI(
	t *testing.T,
	caCert *x509.Certificate,
	caKey *rsa.PrivateKey,
	eku x509.ExtKeyUsage,
	dnsNames []string,
	spiffe string,
) stdtls.Certificate {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatalf("rand.Int: %v", err)
	}
	u, err := url.Parse(spiffe)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{eku},
		DNSNames:     dnsNames,
		URIs:         []*url.URL{u},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("x509.ParseCertificate: %v", err)
	}
	return stdtls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
		Leaf:        cert,
	}
}

func writeTempPEM(t *testing.T, pattern string, b []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatalf("os.CreateTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		t.Fatalf("write temp pem: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close temp pem: %v", err)
	}
	return f.Name()
}
