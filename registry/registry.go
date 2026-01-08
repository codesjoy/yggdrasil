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
	"sync"
)

const (
	// MDServerKind is the key for server kind metadata
	MDServerKind = "serverKind"
)

// Builder is the interface for registry builder
type Builder func() Registry

// Registry is the interface for registry
type Registry interface {
	// Register registers an instance
	Register(context.Context, Instance) error
	// Deregister deregister an instance
	Deregister(context.Context, Instance) error
	// Name returns the name of the registry
	Name() string
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
	builders = make(map[string]Builder)
	mu       sync.RWMutex
)

// RegisterBuilder registers a registry builder
func RegisterBuilder(name string, constructor Builder) {
	mu.Lock()
	defer mu.Unlock()
	builders[name] = constructor
}

// GetBuilder returns a registry builder
func GetBuilder(name string) Builder {
	mu.RLock()
	defer mu.RUnlock()
	constructor := builders[name]
	return constructor
}
