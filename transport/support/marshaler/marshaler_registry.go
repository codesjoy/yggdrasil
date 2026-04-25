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

package marshaler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

// BuildMarshalerRegistry builds a marshaler registry from a list of scheme names.
func BuildMarshalerRegistry(scheme ...string) Registry {
	scheme = dedupStableStrings(scheme)
	mr := NewRegistry()
	for _, item := range scheme {
		marshaler, err := BuildMarshaler(item)
		if err != nil {
			slog.Warn(
				"failed to build marshaler",
				slog.String("scheme", item),
				slog.Any("error", err),
			)
			continue
		}
		_ = mr.Register(item, marshaler)
	}
	return mr
}

// BuildMarshalerRegistryWithBuilders builds a marshaler registry from explicit builders.
func BuildMarshalerRegistryWithBuilders(
	builders map[string]MarshalerBuilder,
	jsonpbCfg *JSONPbConfig,
	scheme ...string,
) Registry {
	scheme = dedupStableStrings(scheme)
	mr := NewRegistry()
	for _, item := range scheme {
		m, err := BuildMarshalerWithBuilders(builders, item, jsonpbCfg)
		if err != nil {
			slog.Warn(
				"failed to build marshaler",
				slog.String("scheme", item),
				slog.Any("error", err),
			)
			continue
		}
		_ = mr.Register(item, m)
	}
	return mr
}

var defaultMarshaler = NewJSONPbMarshalerWithConfig(nil)

var (
	acceptHeader      = http.CanonicalHeaderKey("Accept")
	contentTypeHeader = http.CanonicalHeaderKey("Content-Type")
)

// MarshalerRegistry is a registry for marshaler.
// revive:disable:exported
type MarshalerRegistry struct {
	mimeMap map[string]Marshaler
}

// NewRegistry returns a new marshaler registry.
func NewRegistry() *MarshalerRegistry {
	return &MarshalerRegistry{mimeMap: make(map[string]Marshaler)}
}

// GetMarshaler returns the marshaler for the request.
func (mr *MarshalerRegistry) GetMarshaler(r *http.Request) (inbound Marshaler, outbound Marshaler) {
	for _, acceptVal := range headerValues(r.Header[acceptHeader]) {
		if m := mr.lookup(acceptVal); m != nil {
			outbound = m
			break
		}
	}
	for _, contentTypeVal := range headerValues(r.Header[contentTypeHeader]) {
		if m := mr.lookup(contentTypeVal); m != nil {
			inbound = m
			break
		}
	}
	if inbound == nil {
		inbound = defaultMarshaler
	}
	if outbound == nil {
		outbound = inbound
	}
	return inbound, outbound
}

// Register adds a marshaler for a case-sensitive MIME type string ("*" to match any
// MIME type).
func (mr *MarshalerRegistry) Register(mime string, marshaler Marshaler) error {
	if marshaler == nil {
		return errors.New("nil marshaler")
	}
	scheme := normalizeScheme(mime)
	if scheme == "" {
		return errors.New("empty MIME type")
	}
	if canonical := CanonicalContentTypeForScheme(scheme); canonical != "" {
		mr.mimeMap[canonical] = marshaler
	}
	mr.mimeMap[scheme] = marshaler
	return nil
}

func (mr *MarshalerRegistry) lookup(value string) Marshaler {
	if value == "" {
		return nil
	}
	normalized := NormalizeContentType(value)
	if normalized != "" {
		if m, ok := mr.mimeMap[normalized]; ok {
			return m
		}
	}
	alias := normalizeScheme(value)
	if alias == "" {
		return nil
	}
	return mr.mimeMap[alias]
}

func headerValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func dedupStableStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	i := 0
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values[i] = value
		i++
	}
	return values[:i]
}
