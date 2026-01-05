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

package stats

import (
	"time"

	"github.com/codesjoy/yggdrasil/v2/metadata"
)

// RPCTagInfo contains the information that can be attached to the context.
type RPCTagInfo interface {
	GetFullMethod() string
	isRPCTagInfo()
}

// RPCTagInfoBase contains the information that can be attached to the context.
type RPCTagInfoBase struct {
	// FullMethod is the full RPC method string, i.e., /package.service/method.
	FullMethod string
}

func (s *RPCTagInfoBase) isRPCTagInfo() {}

// GetFullMethod returns the full RPC method string, i.e., /package.service/method.
func (s *RPCTagInfoBase) GetFullMethod() string {
	return s.FullMethod
}

// RPCStats contains the stats of an RPC.
type RPCStats interface {
	isRPCStats()
}

// RPCBegin contains the stats of an RPC when it begins.
type RPCBegin interface {
	RPCStats
	// IsClient returns true if this RPCStats is from client side.
	IsClient() bool
	// GetBeginTime returns the time when the RPC attempt begins.
	GetBeginTime() time.Time

	//// IsFailFast returns true if this RPC is failfast.
	//IsFailFast() bool

	// IsClientStream returns true if the RPC is a client streaming RPC.
	IsClientStream() bool
	// IsServerStream returns true if the RPC is a server streaming RPC.
	IsServerStream() bool

	//// IsTransparentRetryAttempt returns true if this attempt was initiated due to transparently retrying a previous attempt.
	//IsTransparentRetryAttempt() bool

	// GetProtocol returns the protocol used for the RPC.
	GetProtocol() string
}

// RPCBeginBase contains the stats of an RPC when it begins.
type RPCBeginBase struct {
	// Client is true if this Begin is from client side.
	Client bool
	// BeginTime is the time when the RPC attempt begins.
	BeginTime time.Time
	// ClientStream indicates whether the RPC is a client streaming RPC.
	ClientStream bool
	// ServerStream indicates whether the RPC is a server streaming RPC.
	ServerStream bool
	// Protocol is the protocol used for the RPC.
	Protocol string
}

// IsClient returns true if this Begin is from client side.
func (s *RPCBeginBase) IsClient() bool { return s.Client }

// IsRPCStats returns true if this RPCStats is from client side.
func (s *RPCBeginBase) isRPCStats() {}

// GetBeginTime returns the time when the RPC attempt begins.
func (s *RPCBeginBase) GetBeginTime() time.Time {
	return s.BeginTime
}

// IsClientStream returns true if the RPC is a client streaming RPC.
func (s *RPCBeginBase) IsClientStream() bool {
	return s.ClientStream
}

// IsServerStream returns true if the RPC is a server streaming RPC.
func (s *RPCBeginBase) IsServerStream() bool {
	return s.ServerStream
}

// GetProtocol returns the protocol used for the RPC.
func (s *RPCBeginBase) GetProtocol() string {
	return s.Protocol
}

// RPCInPayload contains the stats of an inbound payload.
type RPCInPayload interface {
	RPCStats
	// IsClient returns true if this InPayload is from client side.
	IsClient() bool
	// GetPayload returns the payload with original type.
	GetPayload() any
	// GetData returns the serialized message payload.
	GetData() []byte
	// GetTransportSize returns the size of the  payload data on channel..
	GetTransportSize() int
	// GetRecvTime is the time when the payload is received.
	GetRecvTime() time.Time
	// GetProtocol returns the protocol used for the RPC.
	GetProtocol() string
}

// RPCInPayloadBase contains the stats of an inbound payload.
type RPCInPayloadBase struct {
	// Client is true if this InPayload is from client side.
	Client bool
	// Payload is the payload with original type.
	Payload any
	// Data is the serialized message payload.
	Data []byte
	// TransportSize is the size of the payload data on the wire.
	TransportSize int
	// RecvTime is the time when the payload is received.
	RecvTime time.Time
	// Protocol is the protocol used for the RPC.
	Protocol string
}

// IsClient returns true if this InPayload is from client side.
func (s *RPCInPayloadBase) IsClient() bool {
	return s.Client
}

// GetRPCStats returns true if this RPCStats is from client side.
func (s *RPCInPayloadBase) isRPCStats() {
}

// GetPayload returns the payload with original type.
func (s *RPCInPayloadBase) GetPayload() any {
	return s.Payload
}

// GetData returns the serialized message payload.
func (s *RPCInPayloadBase) GetData() []byte {
	return s.Data
}

// GetTransportSize returns the size of the  payload data on channel..
func (s *RPCInPayloadBase) GetTransportSize() int {
	return s.TransportSize
}

// GetProtocol returns the protocol used for the RPC.
func (s *RPCInPayloadBase) GetProtocol() string {
	return s.Protocol
}

// GetRecvTime is the time when the payload is received.
func (s *RPCInPayloadBase) GetRecvTime() time.Time {
	return s.RecvTime
}

// RPCInHeader contains the stats of an inbound header.
type RPCInHeader interface {
	RPCStats
	// GetHeader contains the header metadata received.
	GetHeader() metadata.MD
	// GetProtocol is the protocol used for the RPC.
	GetProtocol() string
	// GetTransportSize returns the size of the  payload data on channel..
	GetTransportSize() int
}

// RPCClientInHeader contains the stats of an inbound header on the client side.
type RPCClientInHeader interface {
	RPCInHeader
	clientInHeader()
}

// RPCServerInHeader contains the stats of an inbound header on the server side.
type RPCServerInHeader interface {
	RPCInHeader
	serverInHeader()
	// GetFullMethod is the full RPC method string, i.e., /package.service/method.
	GetFullMethod() string
	// GetRemoteEndpoint is the remote endpoint of the corresponding channel.
	GetRemoteEndpoint() string
	// GetLocalEndpoint is the local endpoint of the corresponding channel.
	GetLocalEndpoint() string
}

// RPCInHeaderBase contains the stats of an inbound header.
type RPCInHeaderBase struct {
	// Header contains the header metadata received.
	Header metadata.MD
	// Protocol is the protocol used for the RPC.
	Protocol      string
	TransportSize int
}

// GetRPCStats returns true if this RPCStats is from client side.
func (s *RPCInHeaderBase) isRPCStats() {}

// GetHeader returns the header metadata received.
func (s *RPCInHeaderBase) GetHeader() metadata.MD {
	return s.Header
}

// GetProtocol returns the protocol used for the RPC.
func (s *RPCInHeaderBase) GetProtocol() string {
	return s.Protocol
}

// GetTransportSize returns the size of the  payload data on channel.
func (s *RPCInHeaderBase) GetTransportSize() int {
	return s.TransportSize
}

// RPCClientInHeaderBase contains the stats of an inbound header on the client side.
type RPCClientInHeaderBase struct {
	RPCInHeaderBase
}

func (s *RPCClientInHeaderBase) clientInHeader() {}

// RPCServerInHeaderBase contains the stats of an inbound header on the server side.
type RPCServerInHeaderBase struct {
	RPCInHeaderBase

	// The following fields are valid only if Client is false.
	// FullMethod is the full RPC method string, i.e., /package.service/method.
	FullMethod string
	// RemoteEndpoint is the remote address of the corresponding transport channel.
	RemoteEndpoint string
	// LocalAddr is the local address of the corresponding transport channel.
	LocalEndpoint string
}

func (s *RPCServerInHeaderBase) serverInHeader() {}

// GetFullMethod is the full RPC method string, i.e., /package.service/method.
func (s *RPCServerInHeaderBase) GetFullMethod() string {
	return s.FullMethod
}

// GetRemoteEndpoint is the remote endpoint of the corresponding channel.
func (s *RPCServerInHeaderBase) GetRemoteEndpoint() string {
	return s.RemoteEndpoint
}

// GetLocalEndpoint is the local endpoint of the corresponding channel.
func (s *RPCServerInHeaderBase) GetLocalEndpoint() string {
	return s.LocalEndpoint
}

// RPCInTrailer contains the stats of an inbound trailer.
type RPCInTrailer interface {
	RPCStats
	// IsClient returns true if this InTrailer is from client side.
	IsClient() bool
	// GetTrailer contains the trailer metadata received from the server. This
	// field is only valid if this InTrailer is from the client side.
	GetTrailer() metadata.MD
	// GetTransportSize returns the size of the  trailer data on channel.
	GetTransportSize() int
	// GetProtocol returns the protocol used for the RPC.
	GetProtocol() string
}

// RPCInTrailerBase contains the stats of an inbound trailer.
type RPCInTrailerBase struct {
	// Client is true if this InTrailer is from client side.
	Client bool
	// Trailer contains the trailer metadata received from the server. This
	// field is only valid if this InTrailer is from the client side.
	Trailer metadata.MD
	// TransportSize is the size of the trailer data on the wire.
	TransportSize int
	// Protocol is the protocol used for the RPC.
	Protocol string
}

// IsClient returns true if this InTrailer is from client side.
func (s *RPCInTrailerBase) IsClient() bool { return s.Client }

// GetRPCStats returns true if this RPCStats is from client side.
func (s *RPCInTrailerBase) isRPCStats() {}

// GetTrailer contains the trailer metadata received from the server.
func (s *RPCInTrailerBase) GetTrailer() metadata.MD {
	return s.Trailer
}

// GetTransportSize returns the size of the  trailer data on channel.
func (s *RPCInTrailerBase) GetTransportSize() int {
	return s.TransportSize
}

// GetProtocol returns the protocol used for the RPC.
func (s *RPCInTrailerBase) GetProtocol() string {
	return s.Protocol
}

// RPCOutPayload contains the stats of an outbound payload.
type RPCOutPayload interface {
	RPCStats
	// IsClient returns true if this OutPayload is from client side.
	IsClient() bool
	// GetPayload returns the payload with original type.
	GetPayload() any
	// GetData returns the serialized message payload.
	GetData() []byte
	// GetTransportSize returns the size of the  payload data on channel..
	GetTransportSize() int
	// GetSendTime is the time when the payload is received.
	GetSendTime() time.Time
	// GetProtocol returns the protocol used for the RPC.
	GetProtocol() string
}

// RPCOutPayloadBase contains the stats of an outbound payload.
type RPCOutPayloadBase struct {
	// Client is true if this OutPayload is from client side.
	Client bool
	// Payload is the payload with original type.
	Payload any
	// Data is the serialized message payload.
	Data []byte
	// TransportSize is the size of the trailer data on the wire.
	TransportSize int
	// SendTime is the time when the payload is received.
	SendTime time.Time
	// Protocol is the protocol used for the RPC.
	Protocol string
}

// IsClient returns true if this OutPayload is from client side.
func (s *RPCOutPayloadBase) IsClient() bool { return s.Client }

func (s *RPCOutPayloadBase) isRPCStats() {}

// GetPayload returns the payload with original type.
func (s *RPCOutPayloadBase) GetPayload() any {
	return s.Payload
}

// GetData returns the serialized message payload.
func (s *RPCOutPayloadBase) GetData() []byte {
	return s.Data
}

// GetTransportSize returns the size of the  payload data on channel.
func (s *RPCOutPayloadBase) GetTransportSize() int {
	return s.TransportSize
}

// GetSendTime is the time when the payload is received.
func (s *RPCOutPayloadBase) GetSendTime() time.Time {
	return s.SendTime
}

// GetProtocol returns the protocol used for the RPC.
func (s *RPCOutPayloadBase) GetProtocol() string {
	return s.Protocol
}

// RPCOutHeader contains the stats of an outbound header.
type RPCOutHeader interface {
	RPCStats
	// IsClient returns true if this OutHeader is from client side.
	IsClient() bool
	// GetHeader contains the header metadata sent.
	GetHeader() metadata.MD
	// GetFullMethod is the full RPC method string, i.e., /package.service/method.
	GetFullMethod() string
	// GetRemoteEndpoint is the remote endpoint of the corresponding channel.
	GetRemoteEndpoint() string
	// GetLocalEndpoint is the local endpoint of the corresponding channel.
	GetLocalEndpoint() string
	// GetProtocol is the protocol used for the RPC.
	GetProtocol() string
	// GetTransportSize returns the size of the  payload data on channel..
	GetTransportSize() int
}

// OutHeaderBase contains the stats of an outbound header.
type OutHeaderBase struct {
	// Client is true if this OutHeader is from client side.
	Client bool
	// Header contains the header metadata sent.
	Header metadata.MD
	// TransportSize is the size of the payload data on the wire.
	TransportSize int
	// The following fields are valid only if Client is true.
	// FullMethod is the full RPC method string, i.e., /package.service/method.
	FullMethod string
	// RemoteEndpoint is the remote address of the corresponding transport channel.
	RemoteEndpoint string
	// LocalAddr is the local address of the corresponding transport channel.
	LocalEndpoint string
	// Protocol is the protocol used for the RPC.
	Protocol string
}

// IsClient returns true if this OutHeader is from client side.
func (s *OutHeaderBase) IsClient() bool {
	return s.Client
}

func (s *OutHeaderBase) isRPCStats() {}

// GetHeader contains the header metadata sent.
func (s *OutHeaderBase) GetHeader() metadata.MD {
	return s.Header
}

// GetFullMethod is the full RPC method string, i.e., /package.service/method.
func (s *OutHeaderBase) GetFullMethod() string {
	return s.FullMethod
}

// GetRemoteEndpoint is the remote endpoint of the corresponding channel.
func (s *OutHeaderBase) GetRemoteEndpoint() string {
	return s.RemoteEndpoint
}

// GetLocalEndpoint is the local endpoint of the corresponding channel.
func (s *OutHeaderBase) GetLocalEndpoint() string {
	return s.LocalEndpoint
}

// GetProtocol is the protocol used for the RPC.
func (s *OutHeaderBase) GetProtocol() string {
	return s.Protocol
}

// GetTransportSize returns the size of the  payload data on channel..
func (s *OutHeaderBase) GetTransportSize() int {
	return s.TransportSize
}

// RPCOutTrailer contains the stats of an outbound trailer.
type RPCOutTrailer interface {
	RPCStats
	// IsClient returns true if this OutTrailer is from client side.
	IsClient() bool
	// GetTrailer contains the trailer metadata sent to the client. This
	// field is only valid if this OutTrailer is from the server side.
	GetTrailer() metadata.MD
	// GetTransportSize returns the size of the  trailer data on channel.
	GetTransportSize() int
}

// OutTrailerBase contains stats when an RPC ends.
type OutTrailerBase struct {
	// Client is true if this OutTrailer is from client side.
	Client bool
	// TransportSize is the size of the trailer data on the wire.
	TransportSize int
	// Trailer contains the trailer metadata sent to the client. This
	// field is only valid if this OutTrailer is from the server side.
	Trailer metadata.MD
}

// IsClient returns true if this OutTrailer is from client side.
func (s *OutTrailerBase) IsClient() bool {
	return s.Client
}

func (s *OutTrailerBase) isRPCStats() {}

// GetTrailer returns the trailer metadata sent to the client. This
func (s *OutTrailerBase) GetTrailer() metadata.MD {
	return s.Trailer
}

// GetTransportSize returns the size of the  trailer data on channel.
func (s *OutTrailerBase) GetTransportSize() int {
	return s.TransportSize
}

// RPCEnd contains the stats of an RPC when it ends.
type RPCEnd interface {
	RPCStats
	// IsClient returns true if this End is from client side.
	IsClient() bool
	// GetBeginTime returns the time when the RPC began.
	GetBeginTime() time.Time
	// GetEndTime returns the time when the RPC ends.
	GetEndTime() time.Time
	// Error returns the error the RPC ended with. It is an error generated from
	// status.Status and can be converted back to status.Status using
	// status.FromError if non-nil.
	Error() error
	// GetProtocol returns the protocol used for the RPC.
	GetProtocol() string
}

// RPCEndBase contains stats when an RPC ends.
type RPCEndBase struct {
	// Client is true if this End is from client side.
	Client bool
	// BeginTime is the time when the RPC began.
	BeginTime time.Time
	// EndTime is the time when the RPC ends.
	EndTime time.Time
	// Err is the error the RPC ended with. It is an error generated from
	// status.Status and can be converted back to status.Status using
	// status.FromError if non-nil.
	Err      error
	Protocol string
}

// IsClient returns true if this End is from client side.
func (s *RPCEndBase) IsClient() bool { return s.Client }

func (s *RPCEndBase) isRPCStats() {}

// GetBeginTime returns the time when the RPC began.
func (s *RPCEndBase) GetBeginTime() time.Time {
	return s.BeginTime
}

// GetEndTime returns the time when the RPC ends.
func (s *RPCEndBase) GetEndTime() time.Time {
	return s.EndTime
}

// Error returns the error the RPC ended with. It is an error generated from
// status.Status and can be converted back to status.Status using
// status.FromError if non-nil.
func (s *RPCEndBase) Error() error {
	return s.Err
}

// GetProtocol returns the protocol used for the RPC.
func (s *RPCEndBase) GetProtocol() string {
	return s.Protocol
}
