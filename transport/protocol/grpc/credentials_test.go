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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

func TestBuildTransportCredentialsWithProfilesFallsBackToInsecureWhenProfileEmpty(t *testing.T) {
	creds, err := buildTransportCredentialsWithProfiles(nil, "", "", true, "")
	require.NoError(t, err)
	require.NotNil(t, creds)
	require.Equal(t, "insecure", creds.Info().SecurityProtocol)
}

func TestBuildTransportCredentialsWithProfilesUsesTLSMaterial(t *testing.T) {
	creds, err := buildTransportCredentialsWithProfiles(map[string]security.Profile{
		"tls": mockProfile{
			name: "tls",
			material: security.Material{
				Mode:      security.ModeTLS,
				ClientTLS: nil,
			},
		},
	}, "tls", "", true, "")
	require.Error(t, err)
	require.Nil(t, creds)
}
