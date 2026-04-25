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

package transport

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
)

func TestStateString(t *testing.T) {
	cases := []struct {
		state State
		want  string
	}{
		{Idle, "IDLE"},
		{Connecting, "CONNECTING"},
		{Ready, "READY"},
		{TransientFailure, "TRANSIENT_FAILURE"},
		{Shutdown, "SHUTDOWN"},
		{State(99), "INVALID_STATE"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.state.String())
	}
}

func TestNewTransportClientProvider(t *testing.T) {
	var called bool
	builder := func(ctx context.Context, svc string, ep resolver.Endpoint, sh stats.Handler, cb OnStateChange) (Client, error) {
		called = true
		return nil, nil
	}
	p := NewTransportClientProvider("test", builder)
	assert.Equal(t, "test", p.Protocol())

	_, err := p.NewClient(context.Background(), "svc", resolver.BaseEndpoint{}, nil, nil)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestNewTransportServerProvider(t *testing.T) {
	var called bool
	builder := func(handle MethodHandle) (Server, error) {
		called = true
		return nil, nil
	}
	p := NewTransportServerProvider("test", builder)
	assert.Equal(t, "test", p.Protocol())

	_, err := p.NewServer(func(ss ServerStream) {})
	require.NoError(t, err)
	assert.True(t, called)
}
