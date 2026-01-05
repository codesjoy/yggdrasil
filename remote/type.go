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

package remote

import (
	"context"

	"github.com/codesjoy/yggdrasil/v2/stream"
)

// MethodHandle defines the method handle function.
type MethodHandle func(ServerStream)

// Server defines the interface for a remote server.
type Server interface {
	Start() error
	Handle() error
	Stop() error
	Info() ServerInfo
}

// ServerStream defines the interface for a server stream.
type ServerStream interface {
	stream.ServerStream
	Method() string
	Start(isClientStream, isServerStream bool) error
	Finish(any, error)
}

// State indicates the state of connectivity.
// It can be the state of a ClientConn or SubConn.
type State int

const (
	// Idle indicates the ClientConn is idle.
	Idle State = iota
	// Connecting indicates the ClientConn is connecting.
	Connecting
	// Ready indicates the ClientConn is ready for work.
	Ready
	// TransientFailure indicates the ClientConn has seen a failure but expects to recover.
	TransientFailure
	// Shutdown indicates the ClientConn has started shutting down.
	Shutdown
)

// Client define the interface for a remote client.
type Client interface {
	NewStream(
		ctx context.Context,
		desc *stream.Desc,
		method string,
	) (stream.ClientStream, error)
	Close() error
	Scheme() string
	State() State
	Connect()
}
