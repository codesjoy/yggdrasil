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
	"mime"
	"net/http"

	"github.com/codesjoy/yggdrasil/v2/utils/xarray"
	"google.golang.org/protobuf/encoding/protojson"
)

// BuildMarshalerRegistry builds a marshaler registry from a list of MIME types.
func BuildMarshalerRegistry(scheme ...string) Registry {
	scheme = xarray.DelDupStable(scheme)
	mr := &marshalerRegistry{mimeMap: make(map[string]Marshaler)}
	for _, item := range scheme {
		marshaler, err := buildMarshaller(item)
		if err != nil {
			slog.Warn(
				"failed to build marshaler",
				slog.String("scheme", item),
				slog.Any("error", err),
			)
		}
		_ = mr.add(item, marshaler)
	}
	return mr
}

var defaultMarshaler = &JSONPb{
	MarshalOptions: protojson.MarshalOptions{
		EmitUnpopulated: true,
	},
	UnmarshalOptions: protojson.UnmarshalOptions{
		DiscardUnknown: true,
	},
}

var (
	acceptHeader      = http.CanonicalHeaderKey("Accept")
	contentTypeHeader = http.CanonicalHeaderKey("Content-Type")
)

type marshalerRegistry struct {
	mimeMap map[string]Marshaler
}

// GetMarshaler returns the marshaler for the request.
func (mr *marshalerRegistry) GetMarshaler(r *http.Request) (inbound Marshaler, outbound Marshaler) {
	for _, acceptVal := range r.Header[acceptHeader] {
		if m, ok := mr.mimeMap[acceptVal]; ok {
			outbound = m
			break
		}
	}
	for _, contentTypeVal := range r.Header[contentTypeHeader] {
		contentType, _, err := mime.ParseMediaType(contentTypeVal)
		if err != nil {
			slog.Error("failed to parse Content-Type", slog.String("contentType", contentTypeVal))
			continue
		}
		if m, ok := mr.mimeMap[contentType]; ok {
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

// add adds a marshaler for a case-sensitive MIME type string ("*" to match any
// MIME type).
func (mr *marshalerRegistry) add(mime string, marshaler Marshaler) error {
	if len(mime) == 0 {
		return errors.New("empty MIME type")
	}
	mr.mimeMap[mime] = marshaler
	return nil
}
