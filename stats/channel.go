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

// Package stats defines the stats of a transport connection.
package stats

// ChanTagInfo defines the relevant information needed by connection context tagger.
type ChanTagInfo interface {
	GetProtocol() string
	GetRemoteEndpoint() string
	GetLocalEndpoint() string
	isChanTagInfo()
}

// ChanTagInfoBase defines the relevant information needed by connection context tagger.
type ChanTagInfoBase struct {
	// RemoteEndpoint is the remote address of the corresponding transport channel.
	RemoteEndpoint string
	// LocalAddr is the local address of the corresponding transport channel.
	LocalEndpoint string
	// Protocol is the protocol used for the RPC.
	Protocol string
}

// GetRemoteEndpoint is the remote endpoint of the corresponding transport channel.
func (s *ChanTagInfoBase) GetRemoteEndpoint() string { return s.RemoteEndpoint }

// GetLocalEndpoint is the local endpoint of the corresponding transport channel.
func (s *ChanTagInfoBase) GetLocalEndpoint() string { return s.LocalEndpoint }

// GetProtocol is the protocol used for the RPC.
func (s *ChanTagInfoBase) GetProtocol() string { return s.Protocol }

func (s *ChanTagInfoBase) isChanTagInfo() {}

// ChanStats contains the stats of a transport connection.
type ChanStats interface {
	isChanStats()
	IsClient() bool
}

// ChanBegin defines the stats of a transport connection when it begins.
type ChanBegin interface {
	ChanStats
	isBegin()
}

// ChanBeginBase defines the stats of a transport connection when it begins.
type ChanBeginBase struct {
	// Client is true if this ConnBegin is from client side.
	Client bool
}

// IsClient indicates if this is from client side.
func (s *ChanBeginBase) IsClient() bool { return s.Client }
func (s *ChanBeginBase) isChanStats()   {}
func (s *ChanBeginBase) isBegin()       {}

// ChanEnd defines the stats of a transport connection when it ends.
type ChanEnd interface {
	ChanStats
	isEnd()
}

// ChanEndBase defines the stats of a transport connection when it ends.
type ChanEndBase struct {
	// Client is true if this ConnEnd is from client side.
	Client bool
}

// IsClient indicates if this is from client side.
func (s *ChanEndBase) IsClient() bool { return s.Client }
func (s *ChanEndBase) isChanStats()   {}
func (s *ChanEndBase) isEnd()         {}
