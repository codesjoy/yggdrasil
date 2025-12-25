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

package stats

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/codesjoy/yggdrasil/pkg/config"
)

var (
	mu             sync.RWMutex
	handlerBuilder = map[string]HandlerBuilder{}
	svrOnce        sync.Once
	svrHandler     Handler
	cliOnce        sync.Once
	cliHandler     Handler
)

// RegisterHandlerBuilder registers a HandlerBuilder.
func RegisterHandlerBuilder(name string, builder HandlerBuilder) {
	mu.Lock()
	defer mu.Unlock()
	handlerBuilder[name] = builder
}

// GetHandlerBuilder gets a HandlerBuilder by name.
func GetHandlerBuilder(name string) HandlerBuilder {
	mu.Lock()
	defer mu.Unlock()
	builder, ok := handlerBuilder[name]
	if !ok {
		return nil
	}
	return builder
}

// Handler defines the interface to handle stats.
type Handler interface {
	// TagRPC can attach some information to the given context.
	// The context used for the rest lifetime of the RPC will be derived from
	// the returned context.
	TagRPC(context.Context, RPCTagInfo) context.Context
	// HandleRPC processes the RPC stats.
	HandleRPC(context.Context, RPCStats)

	// TagChannel can attach some information to the given context.
	// The returned context will be used for stats handling.
	// For channel stats handling, the context used in HandleChannel for this
	// channel will be derived from the context returned.
	// For RPC stats handling,
	//  - On server side, the context used in HandleRPC for all RPCs on this
	// channel will be derived from the context returned.
	//  - On client side, the context is not derived from the context returned.
	TagChannel(context.Context, ChanTagInfo) context.Context
	// HandleChannel processes the Channel stats.
	HandleChannel(context.Context, ChanStats)
}

// HandlerBuilder builds a Handler.
type HandlerBuilder func(isServer bool) Handler

type handlerChain struct {
	handlers []Handler
}

// TagRPC attaches some information to the given context.
func (h *handlerChain) TagRPC(ctx context.Context, info RPCTagInfo) context.Context {
	for _, handler := range h.handlers {
		ctx = handler.TagRPC(ctx, info)
	}
	return ctx
}

// HandleRPC processes the RPC stats.
func (h *handlerChain) HandleRPC(ctx context.Context, rs RPCStats) {
	for _, handler := range h.handlers {
		handler.HandleRPC(ctx, rs)
	}
}

// TagChannel attaches some information to the given context.
func (h *handlerChain) TagChannel(ctx context.Context, info ChanTagInfo) context.Context {
	for _, handler := range h.handlers {
		ctx = handler.TagChannel(ctx, info)
	}
	return ctx
}

// HandleChannel processes the Channel stats.
func (h *handlerChain) HandleChannel(ctx context.Context, cs ChanStats) {
	for _, handler := range h.handlers {
		handler.HandleChannel(ctx, cs)
	}
}

// GetServerHandler gets the server side stats handler.
func GetServerHandler() Handler {
	svrOnce.Do(func() {
		names := config.Get(config.Join(config.KeyBase, "stats", "server")).String("")
		h := &handlerChain{handlers: make([]Handler, 0)}
		for _, name := range strings.Split(names, ",") {
			if name == "" {
				continue
			}
			builder := GetHandlerBuilder(name)
			if builder == nil {
				slog.Warn("fault to get stats handler builder", slog.String("name", name))
				continue
			}
			h.handlers = append(h.handlers, builder(true))
		}
		svrHandler = h
	})
	return svrHandler
}

// GetClientHandler gets the client side stats handler.
func GetClientHandler() Handler {
	cliOnce.Do(func() {
		names := config.Get(config.Join(config.KeyBase, "stats", "client")).String("")
		h := &handlerChain{handlers: make([]Handler, 0)}
		for _, name := range strings.Split(names, ",") {
			if name == "" {
				continue
			}
			builder := GetHandlerBuilder(name)
			if builder == nil {
				slog.Warn("fault to get stats handler builder", slog.String("name", name))
				continue
			}
			h.handlers = append(h.handlers, builder(false))
		}
		cliHandler = h
	})
	return cliHandler
}
