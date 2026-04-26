package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/file"
	"github.com/codesjoy/yggdrasil/v3/examples/03-diagnostics-reload/business"
)

func TestDiagnosticsExposeReloadRequiresRestart(t *testing.T) {
	t.Helper()

	grpcPort := freeTCPPort(t)
	governorPort := freeTCPPort(t)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	reloadPath := filepath.Join(dir, "reload.yaml")

	writeFile(
		t,
		configPath,
		fmt.Sprintf(
			"yggdrasil:\n  mode: dev\n  server:\n    transports:\n      - \"grpc\"\n  transports:\n    grpc:\n      server:\n        address: \"127.0.0.1:%d\"\n  admin:\n    governor:\n      port: %d\napp:\n  diagnostics_reload:\n    greeting: \"hello from smoke test\"\n",
			grpcPort,
			governorPort,
		),
	)
	writeFile(t, reloadPath, "yggdrasil:\n  mode: dev\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := yapp.New(
		business.AppName,
		yapp.WithConfigPath(configPath),
		yapp.WithConfigSource(
			"example:reload",
			config.PriorityOverride,
			file.NewSource(reloadPath, true),
		),
	)
	if err != nil {
		t.Fatalf("New app: %v", err)
	}
	defer func() {
		_ = app.Stop(context.Background())
		_ = app.Wait()
	}()

	if err := app.Prepare(ctx); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if err := app.ComposeAndInstall(ctx, business.Compose); err != nil {
		t.Fatalf("ComposeAndInstall: %v", err)
	}
	if err := app.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	endpoint := fmt.Sprintf("http://127.0.0.1:%d/diagnostics", governorPort)
	initial := waitForDiagnostics(t, endpoint, func(resp diagnosticsResponse) bool {
		return resp.Assembly.CurrentSpecHash != ""
	})

	writeFile(t, reloadPath, "yggdrasil:\n  mode: prod-grpc\n")

	updated := waitForDiagnostics(t, endpoint, func(resp diagnosticsResponse) bool {
		return resp.Assembly.LastError != nil &&
			resp.Assembly.LastError.Code == "ReloadRequiresRestart" &&
			resp.Assembly.CurrentSpecHash != "" &&
			resp.Assembly.CurrentSpecHash != initial.Assembly.CurrentSpecHash
	})

	if updated.Assembly.LastError.Context["target"] != "business.bundle" {
		t.Fatalf(
			"reload target = %q, want business.bundle",
			updated.Assembly.LastError.Context["target"],
		)
	}
}

type diagnosticsResponse struct {
	Assembly struct {
		CurrentSpecHash string         `json:"current_spec_hash"`
		LastError       *assemblyError `json:"last_error"`
	} `json:"assembly"`
}

type assemblyError struct {
	Code    string            `json:"code"`
	Context map[string]string `json:"context"`
}

func waitForDiagnostics(
	t *testing.T,
	url string,
	ok func(diagnosticsResponse) bool,
) diagnosticsResponse {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec // local test endpoint
		if err == nil {
			var payload diagnosticsResponse
			decodeErr := json.NewDecoder(resp.Body).Decode(&payload)
			_ = resp.Body.Close()
			if decodeErr == nil && ok(payload) {
				return payload
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("diagnostics endpoint %s did not reach expected state", url)
	return diagnosticsResponse{}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp port: %v", err)
	}
	defer func() { _ = listener.Close() }()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("listener is not TCP")
	}
	return addr.Port
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

var _ = errors.New
