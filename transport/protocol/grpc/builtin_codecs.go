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

package grpc

import (
	_ "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc/encoding/gzip"
	"github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc/encoding/jsonraw"
	_ "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc/encoding/proto"
	"github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc/encoding/raw"
)

// ConfigureBuiltinCodecs registers framework-owned grpc codecs.
func ConfigureBuiltinCodecs() {
	raw.RegisterCodec()
	jsonraw.RegisterCodec()
}
