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

package grpc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	stdtls "crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	ggrpc "google.golang.org/grpc"
	gcredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/interop/grpc_testing"

	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
	ytls "github.com/codesjoy/yggdrasil/v3/transport/support/security/tls"
)

type interopTLSCerts struct {
	caFile         string
	serverCertFile string
	serverKeyFile  string
	clientCertFile string
	clientKeyFile  string
	caPool         *x509.CertPool
	serverCert     stdtls.Certificate
	clientCert     stdtls.Certificate
}

func newInteropTLSCerts(t *testing.T) interopTLSCerts {
	t.Helper()
	dir := t.TempDir()
	caCert, caKey, caPEM := createInteropCA(t)

	serverCertPEM, serverKeyPEM := createInteropLeaf(
		t,
		caCert,
		caKey,
		2,
		"interop-server",
		[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		true,
	)
	clientCertPEM, clientKeyPEM := createInteropLeaf(
		t,
		caCert,
		caKey,
		3,
		"interop-client",
		[]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		false,
	)

	caFile := writeInteropPEM(t, dir, "ca.pem", caPEM)
	serverCertFile := writeInteropPEM(t, dir, "server-cert.pem", serverCertPEM)
	serverKeyFile := writeInteropPEM(t, dir, "server-key.pem", serverKeyPEM)
	clientCertFile := writeInteropPEM(t, dir, "client-cert.pem", clientCertPEM)
	clientKeyFile := writeInteropPEM(t, dir, "client-key.pem", clientKeyPEM)

	caPool := x509.NewCertPool()
	require.True(t, caPool.AppendCertsFromPEM(caPEM))
	serverCert, err := stdtls.X509KeyPair(serverCertPEM, serverKeyPEM)
	require.NoError(t, err)
	clientCert, err := stdtls.X509KeyPair(clientCertPEM, clientKeyPEM)
	require.NoError(t, err)

	return interopTLSCerts{
		caFile:         caFile,
		serverCertFile: serverCertFile,
		serverKeyFile:  serverKeyFile,
		clientCertFile: clientCertFile,
		clientKeyFile:  clientKeyFile,
		caPool:         caPool,
		serverCert:     serverCert,
		clientCert:     clientCert,
	}
}

func createInteropCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "yggdrasil grpc interop ca",
		},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign |
			x509.KeyUsageCRLSign |
			x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		&key.PublicKey,
		key,
	)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func createInteropLeaf(
	t *testing.T,
	caCert *x509.Certificate,
	caKey *ecdsa.PrivateKey,
	serial int64,
	commonName string,
	keyUsage []x509.ExtKeyUsage,
	includeServerAddresses bool,
) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:   now.Add(-time.Minute),
		NotAfter:    now.Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: keyUsage,
	}
	if includeServerAddresses {
		template.DNSNames = []string{"localhost"}
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	}

	der, err := x509.CreateCertificate(
		rand.Reader,
		template,
		caCert,
		&key.PublicKey,
		caKey,
	)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func writeInteropPEM(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func compileInteropTLSProfile(
	t *testing.T,
	certs interopTLSCerts,
	mtls bool,
) security.Profile {
	t.Helper()
	server := map[string]any{
		"cert_file": certs.serverCertFile,
		"key_file":  certs.serverKeyFile,
	}
	client := map[string]any{
		"ca_file":     certs.caFile,
		"server_name": "localhost",
	}
	if mtls {
		server["client_ca_file"] = certs.caFile
		client["cert_file"] = certs.clientCertFile
		client["key_file"] = certs.clientKeyFile
	}
	profile, err := ytls.BuiltinProvider().Compile("interop", map[string]any{
		"server": server,
		"client": client,
	})
	require.NoError(t, err)
	return profile
}

func dialInteropTLS(
	t *testing.T,
	address string,
	certs interopTLSCerts,
	mtls bool,
) *ggrpc.ClientConn {
	t.Helper()
	tlsConfig := &stdtls.Config{
		MinVersion: stdtls.VersionTLS12,
		RootCAs:    certs.caPool,
		ServerName: "localhost",
	}
	if mtls {
		tlsConfig.Certificates = []stdtls.Certificate{certs.clientCert}
	}
	return dialGRPCGo(
		t,
		address,
		ggrpc.WithTransportCredentials(gcredentials.NewTLS(tlsConfig)),
	)
}

func callYGGUnary(t *testing.T, client remote.Client, ctx context.Context) {
	t.Helper()
	call, err := client.NewStream(
		ctx,
		&stream.Desc{},
		grpc_testing.TestService_UnaryCall_FullMethodName,
	)
	require.NoError(t, err)
	require.NoError(t, call.SendMsg(&grpc_testing.SimpleRequest{}))
	require.NoError(t, call.RecvMsg(new(grpc_testing.SimpleResponse)))
}

func TestInteropTLSMatrix(t *testing.T) {
	certs := newInteropTLSCerts(t)

	t.Run("grpc-go client to yggdrasil TLS server", func(t *testing.T) {
		profile := compileInteropTLSProfile(t, certs, false)
		srv := startYGGServer(t, Settings{
			Server: ServerConfig{
				Network:         "tcp",
				Address:         "127.0.0.1:0",
				SecurityProfile: "interop",
			},
		}, handleYGGInterop, map[string]security.Profile{"interop": profile})
		conn := dialInteropTLS(t, srv.Info().Address, certs, false)

		_, err := grpc_testing.NewTestServiceClient(conn).UnaryCall(
			context.Background(),
			&grpc_testing.SimpleRequest{},
		)
		require.NoError(t, err)
	})

	t.Run("yggdrasil client to grpc-go TLS server", func(t *testing.T) {
		profile := compileInteropTLSProfile(t, certs, false)
		address := startGRPCGoServer(
			t,
			grpcGoInteropService{},
			ggrpc.Creds(gcredentials.NewTLS(&stdtls.Config{
				MinVersion:   stdtls.VersionTLS12,
				Certificates: []stdtls.Certificate{certs.serverCert},
			})),
		)
		client := newYGGClientWithConfig(t, address, ClientConfig{
			Transport: ClientTransportOptions{
				Authority:       "localhost",
				SecurityProfile: "interop",
			},
		}, map[string]security.Profile{"interop": profile})

		callYGGUnary(t, client, context.Background())
	})

	t.Run("grpc-go client to yggdrasil mTLS server", func(t *testing.T) {
		profile := compileInteropTLSProfile(t, certs, true)
		srv := startYGGServer(t, Settings{
			Server: ServerConfig{
				Network:         "tcp",
				Address:         "127.0.0.1:0",
				SecurityProfile: "interop",
			},
		}, handleYGGInterop, map[string]security.Profile{"interop": profile})
		conn := dialInteropTLS(t, srv.Info().Address, certs, true)

		_, err := grpc_testing.NewTestServiceClient(conn).UnaryCall(
			context.Background(),
			&grpc_testing.SimpleRequest{},
		)
		require.NoError(t, err)
	})

	t.Run("yggdrasil client to grpc-go mTLS server", func(t *testing.T) {
		profile := compileInteropTLSProfile(t, certs, true)
		address := startGRPCGoServer(
			t,
			grpcGoInteropService{},
			ggrpc.Creds(gcredentials.NewTLS(&stdtls.Config{
				MinVersion:   stdtls.VersionTLS12,
				Certificates: []stdtls.Certificate{certs.serverCert},
				ClientAuth:   stdtls.RequireAndVerifyClientCert,
				ClientCAs:    certs.caPool,
			})),
		)
		client := newYGGClientWithConfig(t, address, ClientConfig{
			Transport: ClientTransportOptions{
				Authority:       "localhost",
				SecurityProfile: "interop",
			},
		}, map[string]security.Profile{"interop": profile})

		callYGGUnary(t, client, context.Background())
	})
}
