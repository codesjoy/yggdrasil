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

package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
	"github.com/codesjoy/yggdrasil/v3/module"
)

func TestReloadWithInstalledBusinessMarksRestartRequired(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, assemblyTestConfig(false))

	app, err := Open(
		WithConfigManager(manager),
		WithAppName("reload-installed-business"),
		WithModules(testTransportModule{recorder: recorder}),
	)
	require.NoError(t, err)
	require.NoError(t, app.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
		return &BusinessBundle{
			RPCBindings: []RPCBinding{
				{
					ServiceName: testAssemblyServiceName,
					Desc:        &testAssemblyRPCServiceDesc,
					Impl:        &testAssemblyServiceImpl{},
				},
			},
		}, nil
	}))
	require.NoError(t, app.Start(context.Background()))
	waitForChannel(t, recorder.started, 2*time.Second, "reload server did not start")
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.NoError(t, manager.LoadLayer("override", config.PriorityOverride, memory.NewSource("override", map[string]any{
		"yggdrasil": map[string]any{
			"logging": map[string]any{
				"remote_level": "warn",
			},
		},
	})))
	err = app.Reload(context.Background())
	requireAssemblyErrorCode(t, err, yassembly.ErrReloadRequiresRestart)
	require.True(t, app.hub.ReloadState().RestartRequired)
}

func TestReloadRuntimeOnlyChangeHotReloadsWithoutBusinessBundle(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, assemblyTestConfig(false))

	app, err := Open(
		WithConfigManager(manager),
		WithAppName("reload-runtime-only"),
		WithModules(testTransportModule{recorder: recorder}),
	)
	require.NoError(t, err)
	require.NoError(t, app.Start(context.Background()))
	waitForChannel(t, recorder.started, 2*time.Second, "runtime-only reload server did not start")
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.NoError(t, manager.LoadLayer("override", config.PriorityOverride, memory.NewSource("override", map[string]any{
		"yggdrasil": map[string]any{
			"logging": map[string]any{
				"remote_level": "warn",
			},
		},
	})))
	require.NoError(t, app.Reload(context.Background()))
	require.Nil(t, app.assemblyErrors.reload.err)
	require.Equal(t, module.ReloadPhaseIdle, app.hub.ReloadState().Phase)
}

func TestReloadUpdatesPlanHashesAndDiffDiagnostics(t *testing.T) {
	recorder := newTransportRecorder()
	manager := newTestManager(t, map[string]any{
		"yggdrasil": map[string]any{
			"mode": "dev",
			"admin": map[string]any{
				"governor": map[string]any{"port": 0},
			},
			"server": map[string]any{
				"transports": []any{"test"},
			},
		},
	})
	app, err := Open(
		WithConfigManager(manager),
		WithAppName("reload-plan-diff"),
		WithModules(testTransportModule{recorder: recorder}),
	)
	require.NoError(t, err)
	require.NoError(t, app.Start(context.Background()))
	waitForChannel(t, recorder.started, 2*time.Second, "reload plan server did not start")
	initialHash := app.lastPlanHash
	require.NotEmpty(t, initialHash)
	t.Cleanup(func() { _ = app.Stop(context.Background()) })

	require.NoError(t, manager.LoadLayer("mode", config.PriorityOverride, memory.NewSource("mode", map[string]any{
		"yggdrasil": map[string]any{
			"mode": "prod-grpc",
		},
	})))
	err = app.Reload(context.Background())
	requireAssemblyErrorCode(t, err, yassembly.ErrReloadRequiresRestart)
	require.True(t, app.hub.ReloadState().RestartRequired)
	require.NotNil(t, app.lastSpecDiff)
	require.True(t, app.lastSpecDiff.HasChanges)
	require.Contains(t, app.lastSpecDiff.AffectedDomains, "mode")
	require.NotEqual(t, initialHash, app.lastPlanHash)
	require.Equal(t, initialHash, app.lastStablePlanHash)
}
