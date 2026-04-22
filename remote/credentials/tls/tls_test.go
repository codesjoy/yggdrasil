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
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v3/remote/credentials"
)

func TestTLSHandshake_ServerAndClient(t *testing.T) {
	caCert, caKey, caPool := newCA(t)
	serverCert := newLeafCert(t, caCert, caKey, x509.ExtKeyUsageServerAuth, []string{"example.com"})

	serverTLS := &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCert},
		MinVersion:   stdtls.VersionTLS12,
	}
	clientTLS := &stdtls.Config{
		RootCAs:    caPool,
		ServerName: "example.com",
		MinVersion: stdtls.VersionTLS12,
	}

	serverCreds, err := New(Config{Client: false, TLSConfig: serverTLS})
	if err != nil {
		t.Fatalf("New(server) = %v", err)
	}
	clientCreds, err := New(Config{Client: true, TLSConfig: clientTLS})
	if err != nil {
		t.Fatalf("New(client) = %v", err)
	}

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	serverErrCh := make(chan error, 1)
	serverAICh := make(chan credentials.AuthInfo, 1)
	go func() {
		_, ai, err := serverCreds.ServerHandshake(c1)
		if err == nil {
			serverAICh <- ai
		}
		serverErrCh <- err
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, clientAI, clientErr := clientCreds.ClientHandshake(ctx, "example.com:443", c2)

	if clientErr != nil {
		t.Fatalf("ClientHandshake error: %v", clientErr)
	}
	if err := <-serverErrCh; err != nil {
		t.Fatalf("ServerHandshake error: %v", err)
	}

	if clientAI.AuthType() != "tls" {
		t.Fatalf("client AuthType=%q, want %q", clientAI.AuthType(), "tls")
	}
	state, ok := StateFromAuthInfo(clientAI)
	if !ok {
		t.Fatalf("StateFromAuthInfo(clientAI) = false")
	}
	if got := state.ServerName; got != "example.com" {
		t.Fatalf("client state.ServerName=%q, want %q", got, "example.com")
	}
	if len(state.VerifiedChains) == 0 {
		t.Fatalf("client state.VerifiedChains empty, want non-empty")
	}

	serverAI := <-serverAICh
	if serverAI.AuthType() != "tls" {
		t.Fatalf("server AuthType=%q, want %q", serverAI.AuthType(), "tls")
	}
	serverState, ok := StateFromAuthInfo(serverAI)
	if !ok {
		t.Fatalf("StateFromAuthInfo(serverAI) = false")
	}
	if !serverState.HandshakeComplete {
		t.Fatalf("server state.HandshakeComplete=false, want true")
	}
}

func TestTLSHandshake_ServerNameMismatchFails(t *testing.T) {
	caCert, caKey, caPool := newCA(t)
	serverCert := newLeafCert(t, caCert, caKey, x509.ExtKeyUsageServerAuth, []string{"example.com"})

	serverTLS := &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCert},
		MinVersion:   stdtls.VersionTLS12,
	}
	clientTLS := &stdtls.Config{
		RootCAs:    caPool,
		ServerName: "bad.example",
		MinVersion: stdtls.VersionTLS12,
	}

	serverCreds, err := New(Config{Client: false, TLSConfig: serverTLS})
	if err != nil {
		t.Fatalf("New(server) = %v", err)
	}
	clientCreds, err := New(Config{Client: true, TLSConfig: clientTLS})
	if err != nil {
		t.Fatalf("New(client) = %v", err)
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
	_, _, clientErr := clientCreds.ClientHandshake(ctx, "example.com:443", c2)

	if clientErr == nil {
		t.Fatalf("ClientHandshake error = nil, want non-nil")
	}
	if err := <-serverErrCh; err == nil {
		t.Fatalf("ServerHandshake error = nil, want non-nil")
	}
}

func TestTLSHandshake_mTLS(t *testing.T) {
	caCert, caKey, caPool := newCA(t)
	serverCert := newLeafCert(t, caCert, caKey, x509.ExtKeyUsageServerAuth, []string{"example.com"})
	clientCert := newLeafCert(t, caCert, caKey, x509.ExtKeyUsageClientAuth, nil)

	serverTLS := &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCert},
		ClientCAs:    caPool,
		ClientAuth:   stdtls.RequireAndVerifyClientCert,
		MinVersion:   stdtls.VersionTLS12,
	}
	clientTLS := &stdtls.Config{
		RootCAs:      caPool,
		Certificates: []stdtls.Certificate{clientCert},
		ServerName:   "example.com",
		MinVersion:   stdtls.VersionTLS12,
	}

	serverCreds, err := New(Config{Client: false, TLSConfig: serverTLS})
	if err != nil {
		t.Fatalf("New(server) = %v", err)
	}
	clientCreds, err := New(Config{Client: true, TLSConfig: clientTLS})
	if err != nil {
		t.Fatalf("New(client) = %v", err)
	}

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	serverErrCh := make(chan error, 1)
	serverAICh := make(chan credentials.AuthInfo, 1)
	go func() {
		_, ai, err := serverCreds.ServerHandshake(c1)
		if err == nil {
			serverAICh <- ai
		}
		serverErrCh <- err
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _, clientErr := clientCreds.ClientHandshake(ctx, "example.com:443", c2)

	if clientErr != nil {
		t.Fatalf("ClientHandshake error: %v", clientErr)
	}
	if err := <-serverErrCh; err != nil {
		t.Fatalf("ServerHandshake error: %v", err)
	}

	serverAI := <-serverAICh
	serverState, ok := StateFromAuthInfo(serverAI)
	if !ok {
		t.Fatalf("StateFromAuthInfo(serverAI) = false")
	}
	if len(serverState.PeerCertificates) == 0 {
		t.Fatalf("server state.PeerCertificates empty, want non-empty")
	}
}

func newCA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatalf("rand.Int: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(CA): %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("x509.ParseCertificate(CA): %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return cert, key, pool
}

func newLeafCert(
	t *testing.T,
	caCert *x509.Certificate,
	caKey *rsa.PrivateKey,
	eku x509.ExtKeyUsage,
	dnsNames []string,
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
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{eku},
		DNSNames:     dnsNames,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(leaf): %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("x509.ParseCertificate(leaf): %v", err)
	}
	return stdtls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
		Leaf:        cert,
	}
}
