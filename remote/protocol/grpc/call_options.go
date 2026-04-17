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

package grpc

import (
	"context"
	"strings"

	"github.com/codesjoy/pkg/basic/xerror"
	"google.golang.org/genproto/googleapis/rpc/code"

	"github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding"
)

// CallOption configures a single gRPC call.
type CallOption interface {
	apply(*callInfo) error
}

type callOptionsContextKey struct{}

type contentSubtypeCallOption struct {
	contentSubtype string
}

type forceCodecCallOption struct {
	codec encoding.Codec
}

// WithCallOptions attaches grpc call options to the context.
func WithCallOptions(ctx context.Context, opts ...CallOption) context.Context {
	if len(opts) == 0 {
		return ctx
	}
	prev := callOptionsFromContext(ctx)
	merged := make([]CallOption, 0, len(prev)+len(opts))
	merged = append(merged, prev...)
	merged = append(merged, opts...)
	return context.WithValue(ctx, callOptionsContextKey{}, merged)
}

// CallContentSubtype sets the request content-subtype for a call.
func CallContentSubtype(contentSubtype string) CallOption {
	return contentSubtypeCallOption{contentSubtype: strings.ToLower(contentSubtype)}
}

// ForceCodec forces a codec for a call.
func ForceCodec(codec encoding.Codec) CallOption {
	return forceCodecCallOption{codec: codec}
}

func (o contentSubtypeCallOption) apply(c *callInfo) error {
	c.contentSubtype = o.contentSubtype
	c.contentSubtypeSet = true
	return nil
}

func (o forceCodecCallOption) apply(c *callInfo) error {
	if o.codec == nil {
		return xerror.New(code.Code_INTERNAL, "grpc: forced codec cannot be nil")
	}
	c.forceCodec = true
	c.codec = o.codec
	return nil
}

func callOptionsFromContext(ctx context.Context) []CallOption {
	if ctx == nil {
		return nil
	}
	opts, _ := ctx.Value(callOptionsContextKey{}).([]CallOption)
	return opts
}

func applyCallOptions(c *callInfo, opts []CallOption) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt.apply(c); err != nil {
			return err
		}
	}
	return nil
}
