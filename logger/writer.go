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
	"fmt"
	"io"
	"os"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/codesjoy/yggdrasil/v3/config"
)

func init() {
	RegisterWriterBuilder("file", newFileWriter)
	RegisterWriterBuilder("console", newConsoleWriter)
}

// WriterBuilder is the interface that wraps the Write method.
type WriterBuilder func(name string) (io.Writer, error)

var writerBuilder = make(map[string]WriterBuilder)
var writerBuilderMu sync.RWMutex

// RegisterWriterBuilder registers a WriterBuilder for the given type.
func RegisterWriterBuilder(typeName string, f WriterBuilder) {
	writerBuilderMu.Lock()
	defer writerBuilderMu.Unlock()
	writerBuilder[typeName] = f
}

// GetWriterBuilder returns the WriterBuilder for the given type.
func GetWriterBuilder(typeName string) (WriterBuilder, error) {
	writerBuilderMu.RLock()
	defer writerBuilderMu.RUnlock()
	f, ok := writerBuilder[typeName]
	if !ok {
		return nil, fmt.Errorf("writer builder for type %s not found", typeName)
	}
	return f, nil
}

// GetWriter returns the Writer for the given name.
func GetWriter(name string) (io.Writer, error) {
	spec := CurrentSettings().Writers[name]
	writerType := spec.Type
	if writerType == "" && name == "default" {
		writerType = "console"
	}
	f, err := GetWriterBuilder(writerType)
	if err != nil {
		return nil, err
	}
	w, err := f(name)
	if err != nil {
		return nil, err
	}
	return w, err
}

func newFileWriter(name string) (io.Writer, error) {
	w := &lumberjack.Logger{}
	spec := CurrentSettings().Writers[name]
	err := config.NewSnapshot(spec.Config).Decode(w)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func newConsoleWriter(string) (io.Writer, error) {
	return os.Stdout, nil
}

type emptyWriter struct{}

func (emptyWriter) Write([]byte) (n int, err error) {
	return
}
