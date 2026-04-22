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

package rest

// MetadataHeaderPrefix is the http prefix that represents custom metadata
// parameters to or from an RPC call.
const MetadataHeaderPrefix = "Yggdrasil-Metadata-"

// MetadataTrailerPrefix is prepended to RPC metadata as it is converted to
// HTTP headers in a response handled by rest
const MetadataTrailerPrefix = "Yggdrasil-Trailer-"
