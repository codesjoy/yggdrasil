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
	"mime"
	"strings"

	"google.golang.org/protobuf/proto"
)

// Marshaler scheme names and HTTP content types.
const (
	SchemeJSONPb = "jsonpb"
	SchemeProto  = "proto"

	ContentTypeJSON  = "application/json"
	ContentTypeProto = "application/octet-stream"
)

var defaultProtoMarshaler = &ProtoMarshaler{}

// NormalizeContentType canonicalizes an HTTP content-type for matching.
func NormalizeContentType(v string) string {
	if v == "" {
		return ""
	}
	ct, _, err := mime.ParseMediaType(v)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(v))
	}
	return strings.ToLower(ct)
}

// CanonicalContentTypeForScheme returns the canonical MIME type for a marshaler scheme.
func CanonicalContentTypeForScheme(scheme string) string {
	switch normalizeScheme(scheme) {
	case SchemeJSONPb:
		return ContentTypeJSON
	case SchemeProto:
		return ContentTypeProto
	default:
		return ""
	}
}

// MarshalerForContentType returns the preferred marshaler for the given HTTP content type.
func MarshalerForContentType( //nolint:revive // stutter is acceptable for clarity
	contentType string,
) Marshaler {
	switch ct := NormalizeContentType(contentType); {
	case ct == "":
		return nil
	case ct == ContentTypeJSON || strings.Contains(ct, "json"):
		return defaultMarshaler
	default:
		return defaultProtoMarshaler
	}
}

// MarshalerForValue returns the default marshaler for a value when no explicit config exists.
func MarshalerForValue(v any) Marshaler { //nolint:revive // stutter is acceptable for clarity
	if v == nil {
		return defaultProtoMarshaler
	}
	if _, ok := v.(proto.Message); ok {
		return defaultProtoMarshaler
	}
	return defaultMarshaler
}
