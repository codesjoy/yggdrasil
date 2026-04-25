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
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestChanTagInfoBase tests ChanTagInfoBase methods
func TestChanTagInfoBase(t *testing.T) {
	t.Run("create and get fields", func(t *testing.T) {
		info := &ChanTagInfoBase{
			RemoteEndpoint: "127.0.0.1:8080",
			LocalEndpoint:  "127.0.0.1:3000",
			Protocol:       "grpc",
		}

		assert.Equal(t, "127.0.0.1:8080", info.GetRemoteEndpoint())
		assert.Equal(t, "127.0.0.1:3000", info.GetLocalEndpoint())
		assert.Equal(t, "grpc", info.GetProtocol())
	})

	t.Run("implements ChanTagInfo interface", func(t *testing.T) {
		var info ChanTagInfo = &ChanTagInfoBase{
			RemoteEndpoint: "remote",
			LocalEndpoint:  "local",
			Protocol:       "http",
		}

		assert.Equal(t, "remote", info.GetRemoteEndpoint())
		assert.Equal(t, "local", info.GetLocalEndpoint())
		assert.Equal(t, "http", info.GetProtocol())
	})
}

// TestChanBeginBase tests ChanBeginBase methods
func TestChanBeginBase(t *testing.T) {
	t.Run("client channel begin", func(t *testing.T) {
		begin := &ChanBeginBase{
			Client: true,
		}

		assert.True(t, begin.IsClient())
		assert.Implements(t, (*ChanStats)(nil), begin)
		assert.Implements(t, (*ChanBegin)(nil), begin)
	})

	t.Run("server channel begin", func(t *testing.T) {
		begin := &ChanBeginBase{
			Client: false,
		}

		assert.False(t, begin.IsClient())
	})
}

// TestChanEndBase tests ChanEndBase methods
func TestChanEndBase(t *testing.T) {
	t.Run("client channel end", func(t *testing.T) {
		end := &ChanEndBase{
			Client: true,
		}

		assert.True(t, end.IsClient())
		assert.Implements(t, (*ChanStats)(nil), end)
		assert.Implements(t, (*ChanEnd)(nil), end)
	})

	t.Run("server channel end", func(t *testing.T) {
		end := &ChanEndBase{
			Client: false,
		}

		assert.False(t, end.IsClient())
	})
}

// TestChanStatsInterface tests ChanStats interface implementations
func TestChanStatsInterface(t *testing.T) {
	t.Run("ChanBeginBase implements ChanStats", func(t *testing.T) {
		var stats ChanStats = &ChanBeginBase{Client: true}
		assert.True(t, stats.IsClient())
	})

	t.Run("ChanEndBase implements ChanStats", func(t *testing.T) {
		var stats ChanStats = &ChanEndBase{Client: false}
		assert.False(t, stats.IsClient())
	})
}

// TestChanTagInfoInterface tests ChanTagInfo interface implementation
func TestChanTagInfoInterface(t *testing.T) {
	t.Run("ChanTagInfoBase implements ChanTagInfo", func(t *testing.T) {
		var info ChanTagInfo = &ChanTagInfoBase{
			RemoteEndpoint: "example.com:443",
			LocalEndpoint:  "localhost:8080",
			Protocol:       "https",
		}

		assert.Equal(t, "example.com:443", info.GetRemoteEndpoint())
		assert.Equal(t, "localhost:8080", info.GetLocalEndpoint())
		assert.Equal(t, "https", info.GetProtocol())
	})
}
