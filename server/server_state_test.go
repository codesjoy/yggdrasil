package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/internal/constant"
	"github.com/codesjoy/yggdrasil/v3/remote"
)

func TestServerInfoAndEndpoints(t *testing.T) {
	info := &serverInfo{
		scheme:   "grpc",
		address:  "localhost:8080",
		svrKind:  constant.ServerKindRPC,
		metadata: map[string]string{"version": "1.0"},
	}
	require.Equal(t, "grpc", info.Scheme())
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
	require.Equal(t, "grpc", endpoints[0].Scheme())
	require.Equal(t, "127.0.0.1:9000", endpoints[0].Address())
	require.Equal(t, "v", endpoints[0].Metadata()["k"])
	require.Equal(t, constant.ServerKindRPC, endpoints[0].Kind())
	require.Equal(t, "http", endpoints[1].Scheme())
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
