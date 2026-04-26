package main

import "testing"

func TestComposeBundleInstallsGreeterBinding(t *testing.T) {
	bundle, err := composeBundle(nil)
	if err != nil {
		t.Fatalf("composeBundle() error = %v", err)
	}
	if len(bundle.RPCBindings) != 1 {
		t.Fatalf("RPCBindings = %d, want 1", len(bundle.RPCBindings))
	}
	if got := bundle.Diagnostics[0].Message; got != "grpcx" {
		t.Fatalf("Diagnostics[0].Message = %q, want %q", got, "grpcx")
	}
}
