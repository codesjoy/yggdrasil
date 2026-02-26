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

package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

const (
	serverReadyURL       = "http://127.0.0.1:55887/v1/shelves"
	serverReadyTimeout   = 10 * time.Second
	serverShutdownTimout = 15 * time.Second
	serverBinaryPath     = "./.tmp_rest_server_test_bin"
)

func buildServerBinary(t *testing.T) {
	t.Helper()

	buildCmd := exec.Command("go", "build", "-o", serverBinaryPath, ".")
	buildCmd.Dir = "."

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build server binary: %v\n%s", err, string(output))
	}

	t.Cleanup(func() {
		_ = os.Remove(serverBinaryPath)
	})
}

func startServerProcess(t *testing.T) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()

	buildServerBinary(t)
	cmd := exec.Command(serverBinaryPath)
	cmd.Dir = "."

	var logs bytes.Buffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	t.Cleanup(func() {
		if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})

	return cmd, &logs
}

func waitHTTPReady(t *testing.T, url string, timeout time.Duration) {
	t.Helper()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(100 * time.Millisecond)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return
		}
		lastErr = fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("server was not ready within %s: %v", timeout, lastErr)
}

func signalServerInterrupt(t *testing.T, cmd *exec.Cmd, logs *bytes.Buffer) {
	t.Helper()

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("failed to send SIGINT: %v\nlogs:\n%s", err, logs.String())
	}
}

func waitProcessExit(t *testing.T, cmd *exec.Cmd, logs *bytes.Buffer, timeout time.Duration) {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server exited with error: %v\nlogs:\n%s", err, logs.String())
		}
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("server did not exit within %s\nlogs:\n%s", timeout, logs.String())
	}
}

// TestServerGracefulShutdown tests that the server can shut down gracefully when receiving SIGINT.
func TestServerGracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cmd, logs := startServerProcess(t)
	waitHTTPReady(t, serverReadyURL, serverReadyTimeout)
	signalServerInterrupt(t, cmd, logs)
	waitProcessExit(t, cmd, logs, serverShutdownTimout)
}

// TestServerWithRequestDuringShutdown tests that active requests are handled during shutdown.
func TestServerWithRequestDuringShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cmd, logs := startServerProcess(t)
	waitHTTPReady(t, serverReadyURL, serverReadyTimeout)

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(serverReadyURL)
	if err != nil {
		t.Fatalf("failed to make request: %v\nlogs:\n%s", err, logs.String())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf(
			"expected status 200, got %d: %s\nlogs:\n%s",
			resp.StatusCode,
			string(body),
			logs.String(),
		)
	}

	signalServerInterrupt(t, cmd, logs)
	waitProcessExit(t, cmd, logs, serverShutdownTimout)
}
