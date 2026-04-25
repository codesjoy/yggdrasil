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

package server

// EndpointKind identifies one advertised server endpoint kind.
type EndpointKind string

const (
	// EndpointKindRPC represents the RPC server.
	EndpointKindRPC EndpointKind = "rpc"
	// EndpointKindGovernor represents the governor server.
	EndpointKindGovernor EndpointKind = "governor"
	// EndpointKindRest represents the REST server.
	EndpointKindRest EndpointKind = "rest"
)
