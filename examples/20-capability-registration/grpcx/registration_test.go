package grpcx

import (
	"context"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/module"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
)

func TestRegistrationInitDecodesConfig(t *testing.T) {
	st := &state{}
	view := config.NewView(configPath, config.NewSnapshot(map[string]any{
		"client": map[string]any{
			"connect_timeout": "5s",
		},
		"server": map[string]any{
			"address": "127.0.0.1:56081",
		},
	}))

	if err := st.init(context.Background(), view); err != nil {
		t.Fatalf("init() error = %v", err)
	}
	settings := st.settingsSnapshot()
	if got := settings.Server.Address; got != "127.0.0.1:56081" {
		t.Fatalf("settings.Server.Address = %q, want %q", got, "127.0.0.1:56081")
	}
	if got := settings.Client.ConnectTimeout; got != 5*time.Second {
		t.Fatalf("settings.Client.ConnectTimeout = %v, want %v", got, 5*time.Second)
	}

	caps := st.capabilities()
	if len(caps) != 2 {
		t.Fatalf("capabilities() len = %d, want 2", len(caps))
	}

	clientProvider := findClientProvider(t, caps)
	client, err := clientProvider.NewClient(
		context.Background(),
		"svc",
		testEndpoint{address: "127.0.0.1:56081", protocol: Protocol},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("clientProvider.NewClient() error = %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("client.Close() error = %v", err)
	}
}

func TestRegistrationCapabilitiesExposeGrpcxProviders(t *testing.T) {
	reg := NewRegistration()
	caps := reg.Capabilities()
	if len(caps) != 2 {
		t.Fatalf("Capabilities() len = %d, want 2", len(caps))
	}

	serverProvider := findServerProvider(t, caps)
	if serverProvider.Protocol() != Protocol {
		t.Fatalf("server provider protocol = %q, want %q", serverProvider.Protocol(), Protocol)
	}

	clientProvider := findClientProvider(t, caps)
	if clientProvider.Protocol() != Protocol {
		t.Fatalf("client provider protocol = %q, want %q", clientProvider.Protocol(), Protocol)
	}
}

func findServerProvider(
	t *testing.T,
	caps []module.Capability,
) remote.TransportServerProvider {
	t.Helper()
	for _, cap := range caps {
		if cap.Spec.Name != "transport.server.provider" {
			continue
		}
		value, ok := cap.Value.(remote.TransportServerProvider)
		if !ok {
			t.Fatalf("server capability has type %T", cap.Value)
		}
		if cap.Name != Protocol {
			t.Fatalf("server capability name = %q, want %q", cap.Name, Protocol)
		}
		return value
	}
	t.Fatal("server capability not found")
	return nil
}

func findClientProvider(
	t *testing.T,
	caps []module.Capability,
) remote.TransportClientProvider {
	t.Helper()
	for _, cap := range caps {
		if cap.Spec.Name != "transport.client.provider" {
			continue
		}
		value, ok := cap.Value.(remote.TransportClientProvider)
		if !ok {
			t.Fatalf("client capability has type %T", cap.Value)
		}
		if cap.Name != Protocol {
			t.Fatalf("client capability name = %q, want %q", cap.Name, Protocol)
		}
		return value
	}
	t.Fatal("client capability not found")
	return nil
}

type testEndpoint struct {
	address  string
	protocol string
}

func (e testEndpoint) Name() string                  { return e.protocol + "/" + e.address }
func (e testEndpoint) GetAddress() string            { return e.address }
func (e testEndpoint) GetProtocol() string           { return e.protocol }
func (e testEndpoint) GetAttributes() map[string]any { return nil }
