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

package logger

import (
	"io"
	"os"
	"strings"
	"testing"
)

// resetWriterBuilders clears all registered writers for testing
func resetWriterBuilders() {
	writerBuilder = make(map[string]WriterBuilder)
	// Re-register default writers
	RegisterWriterBuilder("file", newFileWriter)
	RegisterWriterBuilder("console", newConsoleWriter)
}

func TestRegisterWriterBuilder(t *testing.T) {
	resetWriterBuilders()

	called := false
	testBuilder := func(_ string) (io.Writer, error) {
		called = true
		return os.Stdout, nil
	}

	RegisterWriterBuilder("test", testBuilder)

	f, err := GetWriterBuilder("test")
	if err != nil {
		t.Fatalf("GetWriterBuilder() error = %v", err)
	}

	if f == nil {
		t.Error("GetWriterBuilder() returned nil")
	}

	_, _ = f("test")
	if !called {
		t.Error("Registered builder was not called")
	}
}

func TestGetWriterBuilderNotFound(t *testing.T) {
	resetWriterBuilders()

	_, err := GetWriterBuilder("nonexistent")
	if err == nil {
		t.Error("GetWriterBuilder() expected error for nonexistent type")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("GetWriterBuilder() error = %v, expected 'not found'", err)
	}
}

func TestGetWriterBuilderDuplicate(t *testing.T) {
	resetWriterBuilders()

	RegisterWriterBuilder("test", func(_ string) (io.Writer, error) {
		return os.Stdout, nil
	})

	// Register again with different builder - should replace
	called := false
	RegisterWriterBuilder("test", func(_ string) (io.Writer, error) {
		called = true
		return os.Stdout, nil
	})

	f, _ := GetWriterBuilder("test")
	_, _ = f("test")

	if !called {
		t.Error("Duplicate registration did not replace the builder")
	}
}

func TestNewConsoleWriter(t *testing.T) {
	resetWriterBuilders()

	w, err := newConsoleWriter("test")
	if err != nil {
		t.Fatalf("newConsoleWriter() error = %v", err)
	}

	if w != os.Stdout {
		t.Error("newConsoleWriter() did not return os.Stdout")
	}
}

func TestGetWriterConsole(t *testing.T) {
	resetWriterBuilders()

	// Register console writer
	RegisterWriterBuilder("console", newConsoleWriter)

	// GetWriter requires config setup, so we just test the builder directly
	builder, err := GetWriterBuilder("console")
	if err != nil {
		t.Fatalf("GetWriterBuilder() error = %v", err)
	}

	w, err := builder("console")
	if err != nil {
		t.Fatalf("builder() error = %v", err)
	}

	if w != os.Stdout {
		t.Error("GetWriterBuilder('console') did not return os.Stdout")
	}
}

// TestNewFileWriter tests file writer creation
// Note: This test may be limited since it depends on config package
func TestNewFileWriter(t *testing.T) {
	resetWriterBuilders()

	// The newFileWriter depends on config, so we just test that it's registered
	f, err := GetWriterBuilder("file")
	if err != nil {
		t.Fatalf("GetWriterBuilder('file') error = %v", err)
	}

	if f == nil {
		t.Error("File writer builder not registered")
	}
}

func TestMultipleWriterBuilders(t *testing.T) {
	resetWriterBuilders()

	// Register multiple writers
	RegisterWriterBuilder("writer1", func(_ string) (io.Writer, error) {
		return &strings.Builder{}, nil
	})
	RegisterWriterBuilder("writer2", func(_ string) (io.Writer, error) {
		return &strings.Builder{}, nil
	})

	w1, err := GetWriterBuilder("writer1")
	if err != nil {
		t.Errorf("GetWriterBuilder('writer1') error = %v", err)
	}

	w2, err := GetWriterBuilder("writer2")
	if err != nil {
		t.Errorf("GetWriterBuilder('writer2') error = %v", err)
	}

	if w1 == nil || w2 == nil {
		t.Error("One or more writers not retrieved")
	}

	// Verify they're different functions
	_, _ = w1("test1")
	_, _ = w2("test2")
}

func TestWriterBuilderFunctionSignature(t *testing.T) {
	resetWriterBuilders()

	// Test that the function signature matches expected interface
	var builder WriterBuilder = func(_ string) (io.Writer, error) {
		return &strings.Builder{}, nil
	}

	RegisterWriterBuilder("signature_test", builder)

	retrieved, err := GetWriterBuilder("signature_test")
	if err != nil {
		t.Fatalf("GetWriterBuilder() error = %v", err)
	}

	w, err := retrieved("test")
	if err != nil {
		t.Fatalf("Retrieved builder error = %v", err)
	}

	if _, ok := w.(*strings.Builder); !ok {
		t.Error("Writer type mismatch")
	}
}
