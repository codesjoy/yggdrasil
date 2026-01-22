package polaris

import (
	"testing"

	"github.com/codesjoy/yggdrasil/v2/config"
)

func TestResolveSDKAddresses_ExplicitWins(t *testing.T) {
	got := resolveSDKAddresses("owner", "", []string{"a"})
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("addresses = %#v, want [a]", got)
	}
}

func TestResolveSDKAddresses_DefaultSDKNameIsOwnerName(t *testing.T) {
	origKeyBase := config.KeyBase
	config.KeyBase = "yggdrasil_test_polaris_sdk_cfg_owner"
	t.Cleanup(func() { config.KeyBase = origKeyBase })

	if err := config.Set(
		config.Join(config.KeyBase, "polaris", "owner"),
		map[string]any{"addresses": []string{"127.0.0.1:1"}},
	); err != nil {
		t.Fatalf("Set(polaris.owner) error = %v", err)
	}

	got := resolveSDKAddresses("owner", "", nil)
	if len(got) != 1 || got[0] != "127.0.0.1:1" {
		t.Fatalf("addresses = %#v, want [127.0.0.1:1]", got)
	}
}

func TestResolveSDKAddresses_UsesSDKFieldWhenProvided(t *testing.T) {
	origKeyBase := config.KeyBase
	config.KeyBase = "yggdrasil_test_polaris_sdk_cfg_ref"
	t.Cleanup(func() { config.KeyBase = origKeyBase })

	if err := config.Set(
		config.Join(config.KeyBase, "polaris", "shared"),
		map[string]any{"addresses": []string{"127.0.0.1:2"}},
	); err != nil {
		t.Fatalf("Set(polaris.shared) error = %v", err)
	}

	got := resolveSDKAddresses("owner", "shared", nil)
	if len(got) != 1 || got[0] != "127.0.0.1:2" {
		t.Fatalf("addresses = %#v, want [127.0.0.1:2]", got)
	}
}

func TestSDKHolder_ConfigFilePreferred(t *testing.T) {
	origKeyBase := config.KeyBase
	config.KeyBase = "yggdrasil_test_polaris_sdk_cfg_file"
	t.Cleanup(func() { config.KeyBase = origKeyBase })

	if err := config.Set(
		config.Join(config.KeyBase, "polaris", "sdkfile"),
		map[string]any{
			"config_file": "/path/not/exists/polaris.yaml",
			"addresses":   []string{"127.0.0.1:1"},
		},
	); err != nil {
		t.Fatalf("Set(polaris.sdkfile) error = %v", err)
	}

	_, err := getSDKHolder("sdkfile", nil, nil).getContext()
	if err == nil {
		t.Fatalf("expected error from invalid configFile, got nil")
	}
}
