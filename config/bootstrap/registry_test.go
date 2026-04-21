package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source"
)

func TestRegistryBuildAndRegister(t *testing.T) {
	registry := NewRegistry()

	_, _, err := registry.Build(SourceSpec{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "kind is required")

	_, _, err = registry.Build(SourceSpec{Kind: "unknown"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not supported")

	registry.Register("", func(spec SourceSpec) (source.Source, config.Priority, error) {
		return nil, 0, nil
	})
	registry.Register("x", nil)

	registry.Register("custom", func(spec SourceSpec) (src source.Source, p config.Priority, err error) {
		return nil, config.PriorityOverride, nil
	})
	_, priority, err := registry.Build(SourceSpec{Kind: "custom"})
	require.NoError(t, err)
	require.Equal(t, config.PriorityOverride, priority)
}

func TestParsePriority(t *testing.T) {
	cases := []struct {
		input string
		want  config.Priority
	}{
		{"", config.PriorityFile},
		{"defaults", config.PriorityDefaults},
		{"file", config.PriorityFile},
		{"remote", config.PriorityRemote},
		{"env", config.PriorityEnv},
		{"flag", config.PriorityFlag},
		{"override", config.PriorityOverride},
	}

	for _, tc := range cases {
		got, err := parsePriority(tc.input, config.PriorityFile)
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
	}

	_, err := parsePriority("invalid", config.PriorityFile)
	require.Error(t, err)
}

func TestBuildFileSourceAndEnvAndFlag(t *testing.T) {
	_, _, err := buildFileSource(SourceSpec{
		Kind: "file",
		Config: map[string]any{
			"path": "",
		},
	})
	require.Error(t, err)

	src, priority, err := buildFileSource(SourceSpec{
		Kind:     "file",
		Priority: "override",
		Config: map[string]any{
			"path": "/tmp/a.yaml",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "file", src.Kind())
	require.Equal(t, config.PriorityOverride, priority)

	_, _, err = buildFileSource(SourceSpec{
		Kind: "file",
		Config: map[string]any{
			"path":   "/tmp/a.yaml",
			"parser": "invalid",
		},
	})
	require.Error(t, err)

	envSrc, envPriority, err := buildEnvSource(SourceSpec{
		Kind:     "env",
		Priority: "env",
		Config: map[string]any{
			"prefixes":          []string{"APP"},
			"stripped_prefixes": []string{"RAW"},
			"delimiter":         "__",
			"parse_array":       true,
			"array_sep":         ",",
			"name":              "test-env",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "env", envSrc.Kind())
	require.Equal(t, "test-env", envSrc.Name())
	require.Equal(t, config.PriorityEnv, envPriority)

	flagSrc, flagPriority, err := buildFlagSource(SourceSpec{})
	require.NoError(t, err)
	require.Equal(t, "flag", flagSrc.Kind())
	require.Equal(t, config.PriorityFlag, flagPriority)
}
