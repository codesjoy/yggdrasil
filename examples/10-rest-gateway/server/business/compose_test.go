package business

import "testing"

func TestComposeInstallsRPCAndRESTBindings(t *testing.T) {
	bundle, err := Compose(nil)
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	if len(bundle.RPCBindings) != 1 {
		t.Fatalf("RPCBindings = %d, want 1", len(bundle.RPCBindings))
	}
	if len(bundle.RESTBindings) != 1 {
		t.Fatalf("RESTBindings = %d, want 1", len(bundle.RESTBindings))
	}
	if got := bundle.RESTBindings[0].Name; got != "library-rest" {
		t.Fatalf("RESTBindings[0].Name = %q, want %q", got, "library-rest")
	}
}
