package yggdrasil

import (
	"testing"

	"github.com/codesjoy/yggdrasil/v2/config"
)

func TestValidateStartup_Strict_FailsOnMissingTracerBuilder(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.strict", true)
	_ = config.Set("yggdrasil.tracer", "missing-tracer")

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.strict", false)
		_ = config.Set("yggdrasil.tracer", "")
	})

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_NonStrict_WarnsOnly(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.enable", true)
	_ = config.Set("yggdrasil.startup.validate.strict", false)
	_ = config.Set("yggdrasil.tracer", "missing-tracer")

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.enable", false)
		_ = config.Set("yggdrasil.tracer", "")
	})

	if err := validateStartup(nil); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestValidateStartup_Strict_FailsOnMissingRestMarshalerBuilder(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.strict", true)
	_ = config.Set("yggdrasil.rest.enable", true)
	_ = config.Set("yggdrasil.rest.marshaler.support", []string{"nope"})

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.strict", false)
		_ = config.Set("yggdrasil.rest.enable", false)
		_ = config.Set("yggdrasil.rest.marshaler.support", []string{"jsonpb"})
	})

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsOnMissingClientInterceptor_Global(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.strict", true)
	_ = config.Set("yggdrasil.client.interceptor.unary", []string{"nope"})

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.strict", false)
		_ = config.Set("yggdrasil.client.interceptor.unary", []string{})
	})

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsOnMissingClientInterceptor_ByAppName(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.strict", true)
	_ = config.Set("yggdrasil.client.user.interceptor.unary", []string{"nope"})

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.strict", false)
		_ = config.Set("yggdrasil.client.user.interceptor.unary", []string{})
	})

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}
