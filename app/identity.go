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

package app

import internalidentity "github.com/codesjoy/yggdrasil/v3/internal/identity"

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

// Identity returns the resolved App identity.
func (a *App) Identity() (Identity, bool) {
	if a == nil {
		return Identity{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.identityResolved {
		return Identity{}, false
	}
	return publicIdentity(a.identity), true
}

func publicIdentity(identity internalidentity.Identity) Identity {
	return Identity{
		AppName:   identity.AppName,
		Namespace: identity.Namespace,
		Version:   identity.Version,
		Region:    identity.Region,
		Zone:      identity.Zone,
		Campus:    identity.Campus,
		Metadata:  cloneIdentityMetadata(identity.Metadata),
	}
}

func (identity Identity) internal() internalidentity.Identity {
	return internalidentity.Identity{
		AppName:   identity.AppName,
		Namespace: identity.Namespace,
		Version:   identity.Version,
		Region:    identity.Region,
		Zone:      identity.Zone,
		Campus:    identity.Campus,
		Metadata:  cloneIdentityMetadata(identity.Metadata),
	}
}

func (identity Identity) metadataCopy() map[string]string {
	return cloneIdentityMetadata(identity.Metadata)
}

func (identity Identity) isZero() bool {
	return identity.AppName == "" &&
		identity.Namespace == "" &&
		identity.Version == "" &&
		identity.Region == "" &&
		identity.Zone == "" &&
		identity.Campus == "" &&
		len(identity.Metadata) == 0
}

func cloneIdentityMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
