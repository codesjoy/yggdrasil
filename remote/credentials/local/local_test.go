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

package local

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v3/remote/credentials"
)

type testConn struct {
	remote net.Addr
}

func (c testConn) Read([]byte) (int, error)         { return 0, nil }
func (c testConn) Write([]byte) (int, error)        { return 0, nil }
func (c testConn) Close() error                     { return nil }
func (c testConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (c testConn) RemoteAddr() net.Addr             { return c.remote }
func (c testConn) SetDeadline(time.Time) error      { return nil }
func (c testConn) SetReadDeadline(time.Time) error  { return nil }
func (c testConn) SetWriteDeadline(time.Time) error { return nil }

func TestLocalCredentialsClientHandshake(t *testing.T) {
	tc := newCredentials("", true)

	_, ai, err := tc.ClientHandshake(context.Background(), "", testConn{
		remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
	})
	if err != nil {
		t.Fatalf("ClientHandshake(loopback) error = %v", err)
	}
	if err := credentials.CheckSecurityLevel(ai, credentials.NoSecurity); err != nil {
		t.Fatalf("CheckSecurityLevel(loopback) error = %v", err)
	}

	_, ai, err = tc.ClientHandshake(context.Background(), "", testConn{
		remote: &net.UnixAddr{Name: "/tmp/test.sock", Net: "unix"},
	})
	if err != nil {
		t.Fatalf("ClientHandshake(unix) error = %v", err)
	}
	if err := credentials.CheckSecurityLevel(ai, credentials.PrivacyAndIntegrity); err != nil {
		t.Fatalf("CheckSecurityLevel(unix) error = %v", err)
	}

	if _, _, err = tc.ClientHandshake(context.Background(), "", testConn{
		remote: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 8080},
	}); err == nil {
		t.Fatal("ClientHandshake(non-local) error = nil, want non-nil")
	}
}
