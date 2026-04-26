package main

import "testing"

func TestComposeBundleInstallsRPCBinding(t *testing.T) {
	bundle, err := composeBundle(NewLibraryServer())(nil)
	if err != nil {
		t.Fatalf("composeBundle() error = %v", err)
	}
	if len(bundle.RPCBindings) != 1 {
		t.Fatalf("RPCBindings = %d, want 1", len(bundle.RPCBindings))
	}
	if got := bundle.Diagnostics[0].Code; got != "error.reason.binding" {
		t.Fatalf("Diagnostics[0].Code = %q, want %q", got, "error.reason.binding")
	}
}
