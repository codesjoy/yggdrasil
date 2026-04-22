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

package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
)

// Builder defines a middleware builder
type Builder func() func(http.Handler) http.Handler

// Provider is the capability-oriented middleware provider.
type Provider interface {
	Name() string
	Build() func(http.Handler) http.Handler
}

type provider struct {
	name    string
	builder Builder
}

func (p provider) Name() string { return p.name }

func (p provider) Build() func(http.Handler) http.Handler { return p.builder() }

// NewProvider wraps a middleware builder as a capability provider.
func NewProvider(name string, builder Builder) Provider {
	return provider{
		name:    name,
		builder: builder,
	}
}

var (
	mu        sync.RWMutex
	providers = defaultProviders()
)

// ConfigureProviders replaces all middleware providers.
func ConfigureProviders(next []Provider) error {
	target := map[string]Provider{}
	for _, item := range next {
		if item == nil {
			continue
		}
		name := item.Name()
		if name == "" {
			return errors.New("rest middleware provider name is empty")
		}
		if _, exists := target[name]; exists {
			return fmt.Errorf("duplicate rest middleware provider %q", name)
		}
		target[name] = item
	}
	mu.Lock()
	providers = target
	mu.Unlock()
	return nil
}

// GetProvider returns a middleware provider by name.
func GetProvider(name string) Provider {
	mu.RLock()
	defer mu.RUnlock()
	return providers[name]
}

// Build resolves middleware providers by name and builds the chain.
func Build(names ...string) chi.Middlewares {
	mu.RLock()
	providerCopy := make(map[string]Provider, len(providers))
	for name, provider := range providers {
		providerCopy[name] = provider
	}
	mu.RUnlock()
	return BuildWithProviders(providerCopy, names...)
}

// BuildWithProviders resolves middleware providers from an explicit provider map.
func BuildWithProviders(providerMap map[string]Provider, names ...string) chi.Middlewares {
	handlers := make(chi.Middlewares, 0, len(names))
	for _, item := range names {
		provider := providerMap[item]
		if provider != nil {
			handlers = append(handlers, provider.Build())
		}
	}
	return handlers
}

func defaultProviders() map[string]Provider {
	return map[string]Provider{
		"logger":    NewProvider("logger", requestLogger),
		"marshaler": NewProvider("marshaler", newMarshalerMiddleware),
	}
}
