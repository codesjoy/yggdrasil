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

	"github.com/codesjoy/yggdrasil/v2/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

func init() {
	RegisterWriterBuilder("file", newFileWriter)
	RegisterWriterBuilder("console", newConsoleWriter)
}

// WriterBuilder is the interface that wraps the Write method.
type WriterBuilder func(name string) (io.Writer, error)

var writerBuilder = make(map[string]WriterBuilder)

// RegisterWriterBuilder registers a WriterBuilder for the given type.
func RegisterWriterBuilder(typeName string, f WriterBuilder) {
	writerBuilder[typeName] = f
}

// GetWriterBuilder returns the WriterBuilder for the given type.
func GetWriterBuilder(typeName string) (WriterBuilder, error) {
	f, ok := writerBuilder[typeName]
	if !ok {
		return nil, fmt.Errorf("writer builder for type %s not found", typeName)
	}
	return f, nil
}

// GetWriter returns the Writer for the given name.
func GetWriter(name string) (io.Writer, error) {
	writerType := config.GetString(config.Join("yggdrasil", "logger", "writer", name, "type"))
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
	err := config.Get(config.Join(config.KeyBase, "logger", "writer", name)).Scan(w)
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
