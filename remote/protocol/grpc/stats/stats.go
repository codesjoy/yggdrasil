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

// Package stats provides stats objects for gRPC.
package stats

import "github.com/codesjoy/yggdrasil/v2/stats"

// ClientInHeader is the stats object for incoming headers.
type ClientInHeader struct {
	stats.RPCClientInHeaderBase
	Compression string
}

// GetCompression returns the compression algorithm used for the RPC.
func (s *ClientInHeader) GetCompression() string {
	return s.Compression
}

// ServerInHeader is the stats object for incoming headers.
type ServerInHeader struct {
	stats.RPCServerInHeaderBase
	Compression string
}

// GetCompression returns the compression algorithm used for the RPC.
func (s *ServerInHeader) GetCompression() string {
	return s.Compression
}

// OutHeader is the stats object for outgoing headers.
type OutHeader struct {
	stats.OutHeaderBase
	Compression string
}

// GetCompression returns the compression algorithm used for the RPC.
func (s *OutHeader) GetCompression() string {
	return s.Compression
}

// InPayload is the stats object for incoming payloads.
type InPayload struct {
	stats.RPCInPayloadBase
	Compression      string
	CompressedLength int
}

// GetCompression returns the compression algorithm used for the RPC.
func (s *InPayload) GetCompression() string {
	return s.Compression
}

// GetCompressedLength returns the compressed length of the payload.
func (s *InPayload) GetCompressedLength() int {
	return s.CompressedLength
}

// OutPayload is the stats object for outgoing payloads.
type OutPayload struct {
	stats.RPCOutPayloadBase
	Compression      string
	CompressedLength int
}

// GetCompression returns the compression algorithm used for the RPC.
func (s *OutPayload) GetCompression() string {
	return s.Compression
}

// GetCompressedLength returns the compressed length of the payload.
func (s *OutPayload) GetCompressedLength() int {
	return s.CompressedLength
}
