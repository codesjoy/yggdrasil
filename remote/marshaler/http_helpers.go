package marshaler

import (
	"mime"
	"strings"

	"google.golang.org/protobuf/proto"
)

const (
	SchemeJSONPb = "jsonpb"
	SchemeProto  = "proto"

	ContentTypeJSON  = "application/json"
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
func MarshalerForValue(v any) Marshaler {
	if v == nil {
		return defaultProtoMarshaler
	}
	if _, ok := v.(proto.Message); ok {
		return defaultProtoMarshaler
	}
	return defaultMarshaler
}
