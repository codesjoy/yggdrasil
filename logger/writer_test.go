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
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/codesjoy/yggdrasil/v2/config"
)

// resetWriterBuilders clears all registered writers for testing
func resetWriterBuilders() {
	writerBuilderMu.Lock()
	defer writerBuilderMu.Unlock()
	writerBuilder = make(map[string]WriterBuilder)
	// Re-register default writers
	writerBuilder["file"] = newFileWriter
	writerBuilder["console"] = newConsoleWriter
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

	name := "writer_file_success"
	if err := config.Set(config.Join(config.KeyBase, "logger", "writer", name), map[string]any{
		"filename":   "/tmp/yggdrasil-logger-test.log",
		"max_size":   1,
		"max_backups": 1,
		"max_age":    1,
		"compress":   false,
	}); err != nil {
		t.Fatalf("config.Set(file writer config) error = %v", err)
	}

	w, err := newFileWriter(name)
	if err != nil {
		t.Fatalf("newFileWriter() error = %v", err)
	}
	if _, ok := w.(*lumberjack.Logger); !ok {
		t.Fatalf("newFileWriter() type = %T, want *lumberjack.Logger", w)
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

func TestWriterBuilderConcurrentRegisterAndGet(t *testing.T) {
	resetWriterBuilders()

	const total = 64
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("writer_%d", i)
			RegisterWriterBuilder(name, func(_ string) (io.Writer, error) {
				return &strings.Builder{}, nil
			})
			_, _ = GetWriterBuilder(name)
			_, _ = GetWriterBuilder("console")
		}(i)
	}
	wg.Wait()

	for i := 0; i < total; i++ {
		name := fmt.Sprintf("writer_%d", i)
		if _, err := GetWriterBuilder(name); err != nil {
			t.Fatalf("GetWriterBuilder(%q) error = %v", name, err)
		}
	}
}

func TestGetWriterSuccess(t *testing.T) {
	resetWriterBuilders()

	writerType := "writer_get_success_type"
	writerName := "writer_get_success_name"
	target := &strings.Builder{}
	RegisterWriterBuilder(writerType, func(string) (io.Writer, error) {
		return target, nil
	})
	if err := config.Set(config.Join(config.KeyBase, "logger", "writer", writerName, "type"), writerType); err != nil {
		t.Fatalf("config.Set(writer type) error = %v", err)
	}

	got, err := GetWriter(writerName)
	if err != nil {
		t.Fatalf("GetWriter() error = %v", err)
	}
	if got != target {
		t.Fatalf("GetWriter() = %v, want %v", got, target)
	}
}

func TestGetWriterBuilderNotRegistered(t *testing.T) {
	resetWriterBuilders()

	writerName := "writer_get_missing_builder"
	if err := config.Set(config.Join(config.KeyBase, "logger", "writer", writerName, "type"), "writer_type_not_exist"); err != nil {
		t.Fatalf("config.Set(writer type) error = %v", err)
	}

	_, err := GetWriter(writerName)
	if err == nil {
		t.Fatal("GetWriter() expected error for missing builder")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("GetWriter() err = %v, want contains not found", err)
	}
}

func TestGetWriterBuilderError(t *testing.T) {
	resetWriterBuilders()

	writerType := "writer_get_error_type"
	writerName := "writer_get_error_name"
	wantErr := errors.New("writer builder boom")

	RegisterWriterBuilder(writerType, func(string) (io.Writer, error) {
		return nil, wantErr
	})
	if err := config.Set(config.Join(config.KeyBase, "logger", "writer", writerName, "type"), writerType); err != nil {
		t.Fatalf("config.Set(writer type) error = %v", err)
	}

	_, err := GetWriter(writerName)
	if !errors.Is(err, wantErr) {
		t.Fatalf("GetWriter() err = %v, want %v", err, wantErr)
	}
}

func TestNewFileWriterScanError(t *testing.T) {
	resetWriterBuilders()

	name := "writer_file_scan_error"
	if err := config.Set(config.Join(config.KeyBase, "logger", "writer", name), []any{"invalid"}); err != nil {
		t.Fatalf("config.Set(invalid file writer config) error = %v", err)
	}

	if _, err := newFileWriter(name); err == nil {
		t.Fatal("newFileWriter() expected error for invalid config shape")
	}
}

func TestEmptyWriterWrite(t *testing.T) {
	w := emptyWriter{}
	n, err := w.Write([]byte("anything"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != 0 {
		t.Fatalf("Write() n = %d, want 0", n)
	}
}
