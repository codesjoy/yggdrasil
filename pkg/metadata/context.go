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
// See the License for the specific language govecrning permissions and
// limitations under the License.

// Package metadata provides functions for attaching metadata to a context.
package metadata

import (
	"context"
	"fmt"
	"sync"
)

type (
	inKey     struct{}
	outKey    struct{}
	streamKey struct{}
)

type stream struct {
	mu      sync.Mutex
	header  MD
	trailer MD
}

// WithInContext returns a new context with the given metadata attached.
func WithInContext(ctx context.Context, md MD) context.Context {
	oldMd, ok := ctx.Value(inKey{}).(MD)
	if ok {
		return context.WithValue(ctx, inKey{}, Join(oldMd, md))
	}
	return context.WithValue(ctx, inKey{}, md)
}

// FromInContext returns the metadata attached to the given context.
func FromInContext(ctx context.Context) (md MD, ok bool) {
	md, ok = ctx.Value(inKey{}).(MD)
	if !ok {
		return MD{}, false
	}
	return md.Copy(), true
}

// WithOutContext returns a new context with the given metadata attached.
func WithOutContext(ctx context.Context, md MD) context.Context {
	oldMd, ok := ctx.Value(outKey{}).(MD)
	if ok {
		return context.WithValue(ctx, outKey{}, Join(oldMd, md))
	}
	return context.WithValue(ctx, outKey{}, md)
}

// FromOutContext returns the metadata attached to the given context.
func FromOutContext(ctx context.Context) (md MD, ok bool) {
	md, ok = ctx.Value(outKey{}).(MD)
	if !ok {
		return MD{}, false
	}
	return md.Copy(), true
}

// WithStreamContext returns a new context with the given metadata attached.
func WithStreamContext(ctx context.Context) context.Context {
	_, ok := ctx.Value(streamKey{}).(*stream)
	if !ok {
		return context.WithValue(ctx, streamKey{}, &stream{})
	}
	return ctx
}

// SetTrailer sets the trailer metadata attached to the given context.
func SetTrailer(ctx context.Context, md MD) error {
	h, ok := ctx.Value(streamKey{}).(*stream)
	if !ok {
		return fmt.Errorf("failed to fetch the stream from the context %v", ctx)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.trailer = Join(h.trailer, md)
	return nil
}

// FromTrailerCtx returns the trailer metadata attached to the given context.
func FromTrailerCtx(ctx context.Context) (md MD, ok bool) {
	h, ok := ctx.Value(streamKey{}).(*stream)
	if !ok {
		return MD{}, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.trailer == nil {
		return MD{}, false
	}
	return h.trailer.Copy(), true
}

// SetHeader sets the header metadata attached to the given context.
func SetHeader(ctx context.Context, md MD) error {
	h, ok := ctx.Value(streamKey{}).(*stream)
	if !ok {
		return fmt.Errorf("failed to fetch the stream from the context %v", ctx)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.header = Join(h.header, md)
	return nil
}

// FromHeaderCtx returns the header metadata attached to the given context.
func FromHeaderCtx(ctx context.Context) (md MD, ok bool) {
	h, ok := ctx.Value(streamKey{}).(*stream)
	if !ok {
		return MD{}, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.header == nil {
		return MD{}, false
	}
	return h.header.Copy(), true
}
