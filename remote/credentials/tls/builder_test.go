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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadBuilderConfigMergesServiceOverrides(t *testing.T) {
	minVersion := "1.3"
	Configure(
		BuilderConfig{
			MinVersion: &minVersion,
			Client:     sideCfg{ServerName: "global"},
		},
		map[string]BuilderConfig{
			"demo": {
				Client: sideCfg{ServerName: "svc"},
			},
		},
	)

	cfg := loadBuilderConfig("demo")
	require.NotNil(t, cfg.MinVersion)
	require.Equal(t, "1.3", *cfg.MinVersion)
	require.Equal(t, "svc", cfg.Client.ServerName)
}

func TestNewCredentialsUsesConfiguredServerName(t *testing.T) {
	Configure(BuilderConfig{Client: sideCfg{ServerName: "demo.internal"}}, nil)

	creds := newCredentials("", true)
	require.NotNil(t, creds)
}
