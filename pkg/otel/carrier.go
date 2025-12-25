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

// Package otel provides OpenTelemetry tracing and metrics
package otel

import (
	"strings"

	"github.com/codesjoy/yggdrasil/pkg/metadata"

	"go.opentelemetry.io/otel/propagation"
)

// MetadataReaderWriter is a TextMapCarrier that reads and writes to a metadata.MD
type MetadataReaderWriter struct {
	md *metadata.MD
}

// NewMetadataReaderWriter returns a new MetadataReaderWriter
func NewMetadataReaderWriter(md *metadata.MD) *MetadataReaderWriter {
	return &MetadataReaderWriter{md: md}
}

// assert that MetadataReaderWriter implements the TextMapCarrier interface
var _ propagation.TextMapCarrier = (*MetadataReaderWriter)(nil)

// Get returns the value for a given key
func (w MetadataReaderWriter) Get(key string) string {
	values := w.md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, ";")
}

// Set sets the value for a given key
func (w MetadataReaderWriter) Set(key, val string) {
	w.md.Set(key, val)
}

// Keys returns the keys for all values
func (w MetadataReaderWriter) Keys() []string {
	out := make([]string, 0, len(*w.md))
	for key := range *w.md {
		out = append(out, key)
	}
	return out
}
