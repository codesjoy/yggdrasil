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

	"github.com/codesjoy/yggdrasil/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetLifecycleStateForTest(t *testing.T) {
	t.Helper()

	initMu.Lock()
	oldInitialized := initialized.Load()
	oldAppRunning := appRunning.Load()
	oldOpts := opts
	initialized.Store(false)
	appRunning.Store(false)
	opts = &options{
		serviceDesc:     map[*server.ServiceDesc]interface{}{},
		restServiceDesc: map[*server.RestServiceDesc]restServiceDesc{},
	}
	initMu.Unlock()

	t.Cleanup(func() {
		initMu.Lock()
		initialized.Store(oldInitialized)
		appRunning.Store(oldAppRunning)
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
	assert.False(t, initialized.Load())

	err = Init("retry-app")
	require.NoError(t, err)
	assert.True(t, initialized.Load())
}

func TestServeRollbackAppRunningOnValidationError(t *testing.T) {
	resetLifecycleStateForTest(t)

	err := Serve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "please initialize")
	assert.False(t, appRunning.Load())
}

func TestRunRollbackAppRunningWhenInitFails(t *testing.T) {
	resetLifecycleStateForTest(t)

	err := Run("run-fail-app", func(*options) error {
		return errors.New("inject run init error")
	})
	require.Error(t, err)
	assert.False(t, appRunning.Load())
	assert.False(t, initialized.Load())
}
