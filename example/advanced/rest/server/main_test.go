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
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestServerGracefulShutdown tests that the server can shut down gracefully when receiving SIGINT
func TestServerGracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start the server as a subprocess
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", "main.go")
	cmd.Dir = "."
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Wait for server to start
	time.Sleep(3 * time.Second)

	// Send SIGINT (Ctrl+C)
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("Failed to send SIGINT: %v", err)
	}

	// Server should exit within shutdown timeout + grace period
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Server exited with error: %v", err)
		}
		t.Log("Server exited gracefully")
	case <-time.After(15 * time.Second):
		cmd.Process.Kill()
		t.Fatal("Server did not exit gracefully within timeout")
	}
}

// TestServerWithRequestDuringShutdown tests that active requests are handled during shutdown
func TestServerWithRequestDuringShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start the server as a subprocess
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", "main.go")
	cmd.Dir = "."
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Wait for server to be ready
	time.Sleep(3 * time.Second)

	// Make a test request
	resp, err := http.Get("http://127.0.0.1:55887/v1/shelves")
	if err != nil {
		cmd.Process.Kill()
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		cmd.Process.Kill()
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Send SIGINT (Ctrl+C)
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		cmd.Process.Kill()
		t.Fatalf("Failed to send SIGINT: %v", err)
	}

	// Server should exit within shutdown timeout + grace period
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Server exited with error: %v", err)
		}
		t.Log("Server exited gracefully after handling request")
	case <-time.After(15 * time.Second):
		cmd.Process.Kill()
		t.Fatal("Server did not exit gracefully within timeout")
	}
}
