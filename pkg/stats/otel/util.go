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

package otel

import (
	"context"
	"strings"

	"github.com/codesjoy/yggdrasil/pkg/metadata"
	xtrace "github.com/codesjoy/yggdrasil/pkg/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.23.1"
)

func parseFullMethod(fullMethod string) (string, []attribute.KeyValue) {
	if !strings.HasPrefix(fullMethod, "/") {
		// Invalid format, does not follow `/package.service/method`.
		return fullMethod, nil
	}
	name := fullMethod[1:]
	pos := strings.LastIndex(name, "/")
	if pos < 0 {
		// Invalid format, does not follow `/package.service/method`.
		return name, nil
	}
	service, method := name[:pos], name[pos+1:]

	var attrs []attribute.KeyValue
	if service != "" {
		attrs = append(attrs, semconv.RPCService(service))
	}
	if method != "" {
		attrs = append(attrs, semconv.RPCMethod(method))
	}
	return name, attrs
}

func inject(ctx context.Context, propagators propagation.TextMapPropagator) context.Context {
	md, _ := metadata.FromOutContext(ctx)
	propagators.Inject(ctx, xtrace.NewMetadataReaderWriter(&md))
	return metadata.WithOutContext(ctx, md)
}

func extract(ctx context.Context, propagators propagation.TextMapPropagator) context.Context {
	md, _ := metadata.FromInContext(ctx)
	return propagators.Extract(ctx, xtrace.NewMetadataReaderWriter(&md))
}
