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

// Package identity contains the App-local runtime identity shared across
// internal packages that cannot import app without creating cycles.
package identity

import "github.com/codesjoy/yggdrasil/v3/internal/instance"

// Identity contains the resolved runtime identity for one App.
type Identity struct {
	AppName   string            `json:"app_name"  yaml:"app_name"`
	Namespace string            `json:"namespace" yaml:"namespace"`
	Version   string            `json:"version"   yaml:"version"`
	Region    string            `json:"region"    yaml:"region"`
	Zone      string            `json:"zone"      yaml:"zone"`
	Campus    string            `json:"campus"    yaml:"campus"`
	Metadata  map[string]string `json:"metadata"  yaml:"metadata"`
}

// FromInstanceConfig builds an Identity from the resolved app name and
// instance-compatible configuration.
func FromInstanceConfig(appName string, cfg instance.Config) Identity {
	return Identity{
		AppName:   appName,
		Namespace: cfg.Namespace,
		Version:   cfg.Version,
		Region:    cfg.Region,
		Zone:      cfg.Zone,
		Campus:    cfg.Campus,
		Metadata:  cloneMetadata(cfg.Metadata),
	}
}

// InstanceConfig returns the identity in the legacy instance.Config shape.
func (id Identity) InstanceConfig() instance.Config {
	return instance.Config{
		Namespace: id.Namespace,
		Version:   id.Version,
		Campus:    id.Campus,
		Metadata:  cloneMetadata(id.Metadata),
		Region:    id.Region,
		Zone:      id.Zone,
	}
}

// MetadataCopy returns detached identity metadata.
func (id Identity) MetadataCopy() map[string]string {
	return cloneMetadata(id.Metadata)
}

// IsZero reports whether the identity has not been resolved.
func (id Identity) IsZero() bool {
	return id.AppName == "" &&
		id.Namespace == "" &&
		id.Version == "" &&
		id.Region == "" &&
		id.Zone == "" &&
		id.Campus == "" &&
		len(id.Metadata) == 0
}

func cloneMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
