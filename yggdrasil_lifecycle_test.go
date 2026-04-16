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

package yggdrasil

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/application"
	"github.com/codesjoy/yggdrasil/v2/server"
)

func resetLifecycleStateForTest(t *testing.T) {
	t.Helper()

	initMu.Lock()
	oldState := state
	oldOpts := opts
	oldApp := app
	state = lifecycleStateNew
	app, _ = application.New()
	opts = &options{
		serviceDesc:     map[*server.ServiceDesc]interface{}{},
		restServiceDesc: map[*server.RestServiceDesc]restServiceDesc{},
	}
	initMu.Unlock()

	t.Cleanup(func() {
		initMu.Lock()
		state = oldState
		app = oldApp
		opts = oldOpts
		initMu.Unlock()
	})
}

func TestInitFailureAllowsRetry(t *testing.T) {
	resetLifecycleStateForTest(t)

	err := Init("retry-app", func(*options) error {
		return errors.New("inject init error")
	})
	require.Error(t, err)
	assert.Equal(t, lifecycleStateNew, state)

	err = Init("retry-app")
	require.NoError(t, err)
	assert.Equal(t, lifecycleStateInitialized, state)
}

func TestServeRequiresInitialization(t *testing.T) {
	resetLifecycleStateForTest(t)

	err := Serve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "please initialize")
	assert.Equal(t, lifecycleStateNew, state)
}

func TestRunKeepsNewStateWhenInitFails(t *testing.T) {
	resetLifecycleStateForTest(t)

	err := Run("run-fail-app", func(*options) error {
		return errors.New("inject run init error")
	})
	require.Error(t, err)
	assert.Equal(t, lifecycleStateNew, state)
}

func TestRestartUnsupportedAfterStop(t *testing.T) {
	resetLifecycleStateForTest(t)

	require.NoError(t, Init("stop-app"))
	require.NoError(t, Stop())
	assert.Equal(t, lifecycleStateStopped, state)

	err := Init("stop-app")
	require.ErrorIs(t, err, errRestartUnsupported)

	err = Run("stop-app")
	require.ErrorIs(t, err, errRestartUnsupported)

	err = Serve()
	require.ErrorIs(t, err, errRestartUnsupported)
}
