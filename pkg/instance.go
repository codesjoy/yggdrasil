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

// Package pkg provides common functionality for yggdrasil.
package pkg

import (
	"sync"

	"github.com/codesjoy/yggdrasil/pkg/config"
)

// ServerKind represents the type of server.
type ServerKind string

const (
	// ServerKindRPC represents the rpc server.
	ServerKindRPC ServerKind = "rpc"
	// ServerKindGovernor represents the governor server.
	ServerKindGovernor ServerKind = "governor"
	// ServerKindRest represents the rest server.
	ServerKindRest ServerKind = "rest"
)

const (
	// FrameworkVersion is the version of the framework.
	FrameworkVersion = "1.3.1"
	// FrameworkName is the name of the framework.
	FrameworkName = "yggdrasil"
)

var (
	instanceInfo = &instance{}
	once         sync.Once
)

// InitInstanceInfo initializes the instance information.
func InitInstanceInfo() {
	once.Do(func() {
		info := struct {
			Name      string            `mapstructure:"name"`
			Namespace string            `mapstructure:"namespace"`
			Version   string            `mapstructure:"version"`
			Campus    string            `mapstructure:"campus"`
			Metadata  map[string]string `mapstructure:"metadata"`
			Region    string            `mapstructure:"region"`
			Zone      string            `mapstructure:"zone"`
		}{}
		_ = config.Get(config.Join(config.KeyBase, "application")).Scan(&info)
		if info.Metadata == nil {
			info.Metadata = make(map[string]string)
		}
		instanceInfo = &instance{
			name:      info.Name,
			region:    info.Region,
			zone:      info.Zone,
			campus:    info.Campus,
			namespace: info.Namespace,
			version:   info.Version,
			metadata:  info.Metadata,
		}
	})
}

// Namespace returns the namespace of the instance.
func Namespace() string {
	return instanceInfo.Namespace()
}

// Name returns the name of the instance.
func Name() string {
	return instanceInfo.Name()
}

// Version returns the version of the instance.
func Version() string {
	return instanceInfo.Version()
}

// Region returns the region of the instance.
func Region() string {
	return instanceInfo.Region()
}

// Zone returns the zone of the instance.
func Zone() string {
	return instanceInfo.Zone()
}

// Campus returns the campus of the instance.
func Campus() string {
	return instanceInfo.Campus()
}

// Metadata returns the metadata of the instance.
func Metadata() map[string]string {
	return instanceInfo.Metadata()
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

func (i *instance) Namespace() string {
	return i.namespace
}

func (i *instance) Name() string {
	return i.name
}

func (i *instance) Version() string {
	return i.version
}

func (i *instance) Region() string {
	return i.region
}

func (i *instance) Zone() string {
	return i.zone
}

func (i *instance) Campus() string {
	return i.campus
}

func (i *instance) Metadata() map[string]string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	md := make(map[string]string)
	for k, v := range i.metadata {
		md[k] = v
	}
	return md
}

func (i *instance) AddMetadata(key, val string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if _, ok := i.metadata[key]; ok {
		return false
	}
	i.metadata[key] = val
	return true
}
