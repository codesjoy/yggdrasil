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

// Package registry defines the pluggable registry interface for the framework.
// It sets the standard contract for implementing custom registration and
// instance management backends.
package registry

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

const (
	// MDServerKind is the key for server kind metadata
	MDServerKind = "serverKind"
)

// Spec describes a registry extension envelope.
type Spec struct {
	Type   string         `mapstructure:"type"`
	Config map[string]any `mapstructure:"config"`
}

// Builder builds a registry from its config subsection.
type Builder func(cfg map[string]any) (Registry, error)

// Provider is the capability-oriented registry provider.
type Provider interface {
	Type() string
	New(cfg map[string]any) (Registry, error)
}

type provider struct {
	typeName string
	builder  Builder
}

func (p provider) Type() string { return p.typeName }

func (p provider) New(cfg map[string]any) (Registry, error) { return p.builder(cfg) }

// NewProvider wraps a registry builder as a capability provider.
func NewProvider(typeName string, builder Builder) Provider {
	return provider{
		typeName: typeName,
		builder:  builder,
	}
}

// Registry is the interface for registry
type Registry interface {
	// Register registers an instance
	Register(context.Context, Instance) error
	// Deregister deregister an instance
	Deregister(context.Context, Instance) error
	// Type returns the type of the registry.
	Type() string
}

// Endpoint is the interface for endpoint
type Endpoint interface {
	// Scheme returns the scheme of the endpoint
	Scheme() string
	// Address returns the address of the endpoint
	Address() string
	// Metadata returns the metadata of the endpoint
	Metadata() map[string]string
}

// Instance is the interface for instance
type Instance interface {
	// Region returns the region of the instance
	Region() string
	// Zone returns the zone of the instance
	Zone() string
	// Campus returns the campus of the instance
	Campus() string
	// Namespace returns the namespace of the instance
	Namespace() string
	// Name returns the name of the instance
	Name() string
	// Version returns the version of the instance
	Version() string
	// Metadata returns the metadata of the instance
	Metadata() map[string]string
	// Endpoints returns the endpoints of the instance
	Endpoints() []Endpoint
}

var (
	providers  = make(map[string]Provider)
	mu         sync.RWMutex
	defaultReg Registry
	specV      Spec
)

// ConfigureProviders replaces all registry providers.
func ConfigureProviders(next []Provider) error {
	target := map[string]Provider{}
	for _, item := range next {
		if item == nil {
			continue
		}
		typeName := item.Type()
		if typeName == "" {
			return errors.New("registry provider type is empty")
		}
		if _, exists := target[typeName]; exists {
			return fmt.Errorf("duplicate registry provider for type %q", typeName)
		}
		target[typeName] = item
	}
	mu.Lock()
	providers = target
	defaultReg = nil
	mu.Unlock()
	return nil
}

// GetProvider returns a registry provider by type.
func GetProvider(typeName string) Provider {
	mu.RLock()
	defer mu.RUnlock()
	return providers[typeName]
}

// New creates a registry instance by type and config value.
func New(typeName string, cfg map[string]any) (Registry, error) {
	mu.RLock()
	p, ok := providers[typeName]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not found registry provider, type: %s", typeName)
	}
	return p.New(cfg)
}

// Configure installs the default registry spec resolved by the assembly layer.
func Configure(spec Spec) {
	mu.Lock()
	defer mu.Unlock()
	specV = spec
	defaultReg = nil
}

// CurrentSpec returns the currently configured default registry spec.
func CurrentSpec() Spec {
	mu.RLock()
	defer mu.RUnlock()
	return specV
}

// Get returns the default registry defined by yggdrasil.discovery.registry.
func Get() (Registry, error) {
	mu.RLock()
	if defaultReg != nil {
		r := defaultReg
		mu.RUnlock()
		return r, nil
	}
	mu.RUnlock()

	spec := CurrentSpec()
	typeName := spec.Type
	if typeName == "" {
		return nil, fmt.Errorf("not found registry type")
	}
	r, err := New(typeName, spec.Config)
	if err != nil {
		return nil, err
	}

	mu.Lock()
	if defaultReg == nil {
		defaultReg = r
	}
	out := defaultReg
	mu.Unlock()
	return out, nil
}
