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

// Package instance provides instance information for yggdrasil.
package instance

import (
	"sync"
)

var (
	globalMu sync.RWMutex
	global   = &instance{metadata: map[string]string{}}
)

// Config contains instance metadata resolved by the assembly layer.
type Config struct {
	Namespace string            `mapstructure:"namespace" default:"default"`
	Version   string            `mapstructure:"version"   default:"0.0.1"`
	Campus    string            `mapstructure:"campus"    default:"default"`
	Metadata  map[string]string `mapstructure:"metadata"`
	Region    string            `mapstructure:"region"    default:"default"`
	Zone      string            `mapstructure:"zone"      default:"default"`
}

// Snapshot contains a restorable process-default instance facade state.
type Snapshot struct {
	AppName string
	Config  Config
}

// InitInstanceInfo initializes the Instance information.
//
// Deprecated: this updates the process-default instance facade. Core runtime
// code should use explicit App identity instead.
func InitInstanceInfo(appName string, info Config) {
	if info.Metadata == nil {
		info.Metadata = make(map[string]string)
	}
	globalMu.Lock()
	defer globalMu.Unlock()
	global = &instance{
		name:      appName,
		region:    info.Region,
		zone:      info.Zone,
		campus:    info.Campus,
		namespace: info.Namespace,
		version:   info.Version,
		metadata:  info.Metadata,
	}
}

// Current returns a detached snapshot of the process-default instance facade.
//
// Deprecated: use explicit App identity in core paths.
func Current() Snapshot {
	i := current()
	return Snapshot{
		AppName: i.Name(),
		Config: Config{
			Namespace: i.Namespace(),
			Version:   i.Version(),
			Campus:    i.Campus(),
			Metadata:  i.Metadata(),
			Region:    i.Region(),
			Zone:      i.Zone(),
		},
	}
}

// Restore replaces the process-default instance facade with a previous snapshot.
//
// Deprecated: use explicit App identity in core paths.
func Restore(snapshot Snapshot) {
	InitInstanceInfo(snapshot.AppName, snapshot.Config)
}

// Namespace returns the namespace of the instance.
//
// Deprecated: this reads the process-default instance facade. Core runtime code
// should use explicit App identity instead.
func Namespace() string {
	return current().Namespace()
}

// Name returns the name of the instance.
//
// Deprecated: this reads the process-default instance facade. Core runtime code
// should use explicit App identity instead.
func Name() string {
	return current().Name()
}

// Version returns the version of the instance.
//
// Deprecated: this reads the process-default instance facade. Core runtime code
// should use explicit App identity instead.
func Version() string {
	return current().Version()
}

// Region returns the region of the instance.
//
// Deprecated: this reads the process-default instance facade. Core runtime code
// should use explicit App identity instead.
func Region() string {
	return current().Region()
}

// Zone returns the zone of the instance.
//
// Deprecated: this reads the process-default instance facade. Core runtime code
// should use explicit App identity instead.
func Zone() string {
	return current().Zone()
}

// Campus returns the campus of the instance.
//
// Deprecated: this reads the process-default instance facade. Core runtime code
// should use explicit App identity instead.
func Campus() string {
	return current().Campus()
}

// Metadata returns the metadata of the instance.
//
// Deprecated: this reads the process-default instance facade. Core runtime code
// should use explicit App identity instead.
func Metadata() map[string]string {
	return current().Metadata()
}

type instance struct {
	namespace string
	name      string
	version   string
	region    string
	zone      string
	campus    string
	mu        sync.RWMutex
	metadata  map[string]string
}

var _ = (*instance)(nil)

func current() *instance {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return global
}

func (i *instance) Namespace() string {
	if i == nil {
		return ""
	}
	return i.namespace
}

func (i *instance) Name() string {
	if i == nil {
		return ""
	}
	return i.name
}

func (i *instance) Version() string {
	if i == nil {
		return ""
	}
	return i.version
}

func (i *instance) Region() string {
	if i == nil {
		return ""
	}
	return i.region
}

func (i *instance) Zone() string {
	if i == nil {
		return ""
	}
	return i.zone
}

func (i *instance) Campus() string {
	if i == nil {
		return ""
	}
	return i.campus
}

func (i *instance) Metadata() map[string]string {
	if i == nil {
		return map[string]string{}
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	md := make(map[string]string)
	for k, v := range i.metadata {
		md[k] = v
	}
	return md
}

func (i *instance) AddMetadata(key, val string) bool {
	if i == nil {
		return false
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if _, ok := i.metadata[key]; ok {
		return false
	}
	i.metadata[key] = val
	return true
}
