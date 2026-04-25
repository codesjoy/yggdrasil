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

package insecure

import (
	"context"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

func TestBuiltinProvider(t *testing.T) {
	provider := BuiltinProvider()
	profile, err := provider.Compile("default", nil)
	require.NoError(t, err)

	material, err := profile.Build(security.BuildSpec{})
	require.NoError(t, err)
	require.Equal(t, security.ModeInsecure, material.Mode)
	require.NotNil(t, material.ConnAuth)
	require.NotNil(t, material.RequestAuth)
}

func TestCompileRejectsConfig(t *testing.T) {
	_, err := BuiltinProvider().Compile("default", map[string]any{"key": "value"})
	require.Error(t, err)
}

func TestProvider_Type(t *testing.T) {
	require.Equal(t, "insecure", BuiltinProvider().Type())
}

func TestProfile_Name_Type(t *testing.T) {
	profile, err := BuiltinProvider().Compile("my-profile", nil)
	require.NoError(t, err)
	require.Equal(t, "my-profile", profile.Name())
	require.Equal(t, "insecure", profile.Type())
}

func TestConnAuthenticator_ClientHandshake(t *testing.T) {
	ca := connAuthenticator{}
	client, server := net.Pipe()
	defer server.Close()

	conn, auth, err := ca.ClientHandshake(context.Background(), "localhost", client)
	require.NoError(t, err)
	require.Equal(t, client, conn)
	require.NotNil(t, auth)

	bai, ok := auth.(security.BasicAuthInfo)
	require.True(t, ok)
	require.Equal(t, security.NoSecurity, bai.SecurityLevel)
	require.Equal(t, "insecure", bai.Type)
	_ = conn.Close()
}

func TestConnAuthenticator_ServerHandshake(t *testing.T) {
	ca := connAuthenticator{}
	client, server := net.Pipe()
	defer client.Close()

	conn, auth, err := ca.ServerHandshake(server)
	require.NoError(t, err)
	require.Equal(t, server, conn)
	require.NotNil(t, auth)

	bai, ok := auth.(security.BasicAuthInfo)
	require.True(t, ok)
	require.Equal(t, security.NoSecurity, bai.SecurityLevel)
	_ = conn.Close()
}

func TestConnAuthenticator_Info(t *testing.T) {
	ca := connAuthenticator{}
	info := ca.Info()
	require.Equal(t, "insecure", info.SecurityProtocol)
}

func TestConnAuthenticator_Clone(t *testing.T) {
	ca := connAuthenticator{}
	cloned := ca.Clone()
	require.NotNil(t, cloned)
}

func TestConnAuthenticator_OverrideServerName(t *testing.T) {
	ca := connAuthenticator{}
	err := ca.OverrideServerName("new-server")
	require.NoError(t, err)
}

func TestRequestAuthenticator_AuthenticateRequest(t *testing.T) {
	ra := requestAuthenticator{}
	auth, err := ra.AuthenticateRequest(&http.Request{})
	require.NoError(t, err)
	require.NotNil(t, auth)

	bai, ok := auth.(security.BasicAuthInfo)
	require.True(t, ok)
	require.Equal(t, security.NoSecurity, bai.SecurityLevel)
}
