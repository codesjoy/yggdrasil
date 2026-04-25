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

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/internal/constant"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
)

func TestServerInfoAndEndpoints(t *testing.T) {
	info := &serverInfo{
		protocol: "grpc",
		address:  "localhost:8080",
		svrKind:  constant.ServerKindRPC,
		metadata: map[string]string{"version": "1.0"},
	}
	require.Equal(t, "grpc", info.Protocol())
	require.Equal(t, "localhost:8080", info.Address())
	require.Equal(t, constant.ServerKindRPC, info.Kind())
	require.Equal(t, "1.0", info.Metadata()["version"])

	s := newTestServer()
	s.servers = []remote.Server{
		&testRemoteServer{
			info: remote.ServerInfo{
				Protocol:   "grpc",
				Address:    "127.0.0.1:9000",
				Attributes: map[string]string{"k": "v"},
			},
		},
	}
	s.restEnable = true
	s.restSvr = &mockRestServer{
		address: "127.0.0.1:8080",
		attr:    map[string]string{"kind": "rest"},
	}

	endpoints := s.Endpoints()
	require.Len(t, endpoints, 2)
	require.Equal(t, "grpc", endpoints[0].Protocol())
	require.Equal(t, "127.0.0.1:9000", endpoints[0].Address())
	require.Equal(t, "v", endpoints[0].Metadata()["k"])
	require.Equal(t, constant.ServerKindRPC, endpoints[0].Kind())
	require.Equal(t, "http", endpoints[1].Protocol())
	require.Equal(t, "127.0.0.1:8080", endpoints[1].Address())
	require.Equal(t, "rest", endpoints[1].Metadata()["kind"])
	require.Equal(t, constant.ServerKindRest, endpoints[1].Kind())
}

func TestStateNameAndRuntimeErrorReporting(t *testing.T) {
	s := newTestServer()

	states := map[int]string{
		serverStateInit:    "init",
		serverStateRunning: "running",
		serverStateClosing: "closing",
		42:                 "unknown",
	}
	for state, want := range states {
		s.state = state
		require.Equal(t, want, s.stateNameLocked())
	}

	errCh := make(chan error, 1)
	s.reportServeRuntimeError(errCh, nil)
	select {
	case err := <-errCh:
		t.Fatalf("unexpected error written: %v", err)
	default:
	}

	s.reportServeRuntimeError(errCh, errors.New("first"))
	s.reportServeRuntimeError(errCh, errors.New("second"))

	err := <-errCh
	assert.EqualError(t, err, "first")
}
