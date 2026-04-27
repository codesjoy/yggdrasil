package marshaler

import (
	"mime"
	"strings"

	"google.golang.org/protobuf/proto"
)

const (
	// SchemeJSONPb identifies the JSON protobuf marshaler scheme.
	SchemeJSONPb = "jsonpb"
	// SchemeProto identifies the binary protobuf marshaler scheme.
	SchemeProto = "proto"

	// ContentTypeJSON is the canonical JSON HTTP content type.
	ContentTypeJSON = "application/json"
	// ContentTypeProto is the canonical binary protobuf HTTP content type.
	ContentTypeProto = "application/octet-stream"
)

var defaultProtoMarshaler = &ProtoMarshaller{}

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
//
//nolint:revive // Keep the exported name explicit for existing marshaler package callers.
func MarshalerForContentType(contentType string) Marshaler {
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
//
//nolint:revive // Keep the exported name explicit for existing marshaler package callers.
func MarshalerForValue(v any) Marshaler {
	if v == nil {
		return defaultProtoMarshaler
	}
	if _, ok := v.(proto.Message); ok {
		return defaultProtoMarshaler
	}
	return defaultMarshaler
}
