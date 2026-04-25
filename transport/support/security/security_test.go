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

package security

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecurityLevel_String(t *testing.T) {
	tests := []struct {
		level SecurityLevel
		want  string
	}{
		{NoSecurity, "NoSecurity"},
		{IntegrityOnly, "IntegrityOnly"},
		{PrivacyAndIntegrity, "PrivacyAndIntegrity"},
		{InvalidSecurityLevel, "invalid SecurityLevel: 0"},
		{SecurityLevel(99), "invalid SecurityLevel: 99"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			require.Equal(t, tt.want, tt.level.String())
		})
	}
}

func TestCommonAuthInfo_GetCommonAuthInfo(t *testing.T) {
	original := CommonAuthInfo{SecurityLevel: PrivacyAndIntegrity}
	returned := original.GetCommonAuthInfo()
	require.Equal(t, original.SecurityLevel, returned.SecurityLevel)
}

func TestBasicAuthInfo_AuthType(t *testing.T) {
	info := BasicAuthInfo{
		CommonAuthInfo: CommonAuthInfo{SecurityLevel: NoSecurity},
		Type:           "test-type",
	}
	require.Equal(t, "test-type", info.AuthType())
}

func TestCheckSecurityLevel(t *testing.T) {
	err := CheckSecurityLevel(BasicAuthInfo{
		CommonAuthInfo: CommonAuthInfo{SecurityLevel: PrivacyAndIntegrity},
		Type:           "test",
	}, NoSecurity)
	require.NoError(t, err)

	err = CheckSecurityLevel(BasicAuthInfo{
		CommonAuthInfo: CommonAuthInfo{SecurityLevel: NoSecurity},
		Type:           "test",
	}, PrivacyAndIntegrity)
	require.Error(t, err)

	err = CheckSecurityLevel(nil, NoSecurity)
	require.Error(t, err)

	// InvalidSecurityLevel returns nil (early exit path)
	err = CheckSecurityLevel(BasicAuthInfo{
		CommonAuthInfo: CommonAuthInfo{SecurityLevel: InvalidSecurityLevel},
		Type:           "test",
	}, NoSecurity)
	require.NoError(t, err)

	// IntegrityOnly vs NoSecurity -> nil (integrity satisfies)
	err = CheckSecurityLevel(BasicAuthInfo{
		CommonAuthInfo: CommonAuthInfo{SecurityLevel: IntegrityOnly},
		Type:           "test",
	}, NoSecurity)
	require.NoError(t, err)

	// IntegrityOnly vs PrivacyAndIntegrity -> error
	err = CheckSecurityLevel(BasicAuthInfo{
		CommonAuthInfo: CommonAuthInfo{SecurityLevel: IntegrityOnly},
		Type:           "test",
	}, PrivacyAndIntegrity)
	require.Error(t, err)

	// AuthInfo that does NOT implement internalInfo -> nil
	err = CheckSecurityLevel(authInfoNoInternal{}, NoSecurity)
	require.NoError(t, err)
}

// authInfoNoInternal is an AuthInfo that does not implement GetCommonAuthInfo.
type authInfoNoInternal struct{}

func (authInfoNoInternal) AuthType() string { return "no-internal" }
