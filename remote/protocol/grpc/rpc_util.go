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
	"fmt"
	"strings"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding"
	"github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding/proto"

	"google.golang.org/genproto/googleapis/rpc/code"
)

const (
	scheme = "grpc"
)

// callInfo contains all related configuration and information about an RPC.
type callInfo struct {
	maxReceiveMessageSize *int
	maxSendMessageSize    *int
	contentSubtype        string
	contentSubtypeSet     bool
	forceCodec            bool
	codec                 encoding.Codec
}

func defaultCallInfo() *callInfo {
	return &callInfo{}
}

// setCallInfoCodec should only be called after CallOptions have been applied.
func setCallInfoCodec(c *callInfo) error {
	if c.codec != nil {
		// codec was already set by a CallOption; use it, but set the content
		// subtype if it is not set.
		if c.contentSubtype == "" {
			// c.codec is a baseCodec to hide the difference between grpc.Codec and
			// encoding.Codec (Name vs. String method name).  We only support
			// setting content subtype from encoding.Codec to avoid a behavior
			// change with the deprecated version.
			c.contentSubtype = strings.ToLower(c.codec.Name())
		}
		if c.contentSubtype == "" {
			return xerror.New(
				code.Code_INTERNAL,
				"grpc: forced codec requires a non-empty content-subtype",
			)
		}
		return nil
	}

	if c.contentSubtype == "" {
		if c.contentSubtypeSet {
			return xerror.New(code.Code_INTERNAL, "grpc: content-subtype cannot be empty")
		}
		// No codec specified in CallOptions; use proto by default.
		c.codec = encoding.GetCodec(proto.Name)
		return nil
	}

	// c.contentSubtype is already lowercased in CallContentSubtype
	c.codec = encoding.GetCodec(c.contentSubtype)
	if c.codec == nil {
		return xerror.New(
			code.Code_INTERNAL,
			fmt.Sprintf("no codec registered for content-subtype %s", c.contentSubtype),
		)
	}
	return nil
}

// The SupportPackageIsVersion variables are referenced from generated protocol
// buffer files to ensure compatibility with the gRPC version used.  The latest
// support package version is 7.
//
// Older versions are kept for compatibility.
//
// These constants should not be referenced from any other code.
const (
	SupportPackageIsVersion3 = true
	SupportPackageIsVersion4 = true
	SupportPackageIsVersion5 = true
	SupportPackageIsVersion6 = true
	SupportPackageIsVersion7 = true
)
