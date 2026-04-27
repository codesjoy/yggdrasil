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

package yggdrasil

import "github.com/codesjoy/yggdrasil/v3/internal/instance"

// InstanceNamespace returns the namespace of the process-default instance.
//
// Deprecated: use App.Identity or Runtime.Identity for App-local identity.
func InstanceNamespace() string {
	return instance.Namespace()
}

// InstanceName returns the name of the process-default instance.
//
// Deprecated: use App.Identity or Runtime.Identity for App-local identity.
func InstanceName() string {
	return instance.Name()
}

// InstanceVersion returns the version of the process-default instance.
//
// Deprecated: use App.Identity or Runtime.Identity for App-local identity.
func InstanceVersion() string {
	return instance.Version()
}

// InstanceRegion returns the region of the process-default instance.
//
// Deprecated: use App.Identity or Runtime.Identity for App-local identity.
func InstanceRegion() string {
	return instance.Region()
}

// InstanceZone returns the zone of the process-default instance.
//
// Deprecated: use App.Identity or Runtime.Identity for App-local identity.
func InstanceZone() string {
	return instance.Zone()
}

// InstanceCampus returns the campus of the process-default instance.
//
// Deprecated: use App.Identity or Runtime.Identity for App-local identity.
func InstanceCampus() string {
	return instance.Campus()
}

// InstanceMetadata returns the metadata of the process-default instance.
//
// Deprecated: use App.Identity or Runtime.Identity for App-local identity.
func InstanceMetadata() map[string]string {
	return instance.Metadata()
}
