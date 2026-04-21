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
