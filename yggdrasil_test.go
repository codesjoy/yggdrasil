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
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
)

type blockingTask struct {
	started chan struct{}
	stopCh  chan struct{}
	once    sync.Once
	err     error
}

func newBlockingTask(err error) *blockingTask {
	return &blockingTask{
		started: make(chan struct{}),
		stopCh:  make(chan struct{}),
		err:     err,
	}
}

func (t *blockingTask) Serve() error {
	close(t.started)
	if t.err != nil {
		return t.err
	}
	<-t.stopCh
	return nil
}

func (t *blockingTask) Stop(context.Context) error {
	t.once.Do(func() {
		close(t.stopCh)
	})
	return nil
}

func rootConfigSource() source.Source {
	return memory.NewSource("root", map[string]any{
		"yggdrasil": map[string]any{
			"admin": map[string]any{
				"governor": map[string]any{
					"host": "127.0.0.1",
					"port": 0,
				},
			},
		},
	})
}

func useTestManager() {
	config.SetDefault(config.NewManager())
}

func TestOpenComposeAndInstallStartStop(t *testing.T) {
	useTestManager()

	app, err := Open(
		WithAppName("root-open"),
		WithConfigSource("root", config.PriorityOverride, rootConfigSource()),
	)
	require.NoError(t, err)

	task := newBlockingTask(nil)
	require.NoError(
		t,
		app.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
			return &BusinessBundle{
				Tasks: []BackgroundTask{task},
			}, nil
		}),
	)

	require.NoError(t, app.Start(context.Background()))
	select {
	case <-task.started:
	case <-time.After(2 * time.Second):
		t.Fatal("task did not start")
	}

	require.NoError(t, app.Stop(context.Background()))
	require.NoError(t, app.Wait())
	require.NoError(t, app.Stop(context.Background()))
}

func TestComposeAndInstallFailureStopsFacade(t *testing.T) {
	useTestManager()

	app, err := Open(
		WithAppName("root-compose-failure"),
		WithConfigSource("root", config.PriorityOverride, rootConfigSource()),
	)
	require.NoError(t, err)

	composeErr := errors.New("compose failed")
	err = app.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
		return nil, composeErr
	})
	require.ErrorIs(t, err, composeErr)
	require.Error(t, app.Start(context.Background()))
}

func TestRunHappyPathStopsOnContextCancel(t *testing.T) {
	useTestManager()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task := newBlockingTask(nil)
	go func() {
		<-task.started
		cancel()
	}()

	err := Run(
		ctx,
		func(Runtime) (*BusinessBundle, error) {
			return &BusinessBundle{
				Tasks: []BackgroundTask{task},
			}, nil
		},
		WithAppName("root-run"),
		WithConfigSource("root", config.PriorityOverride, rootConfigSource()),
	)
	require.NoError(t, err)
}

func TestWaitPropagatesServeFailure(t *testing.T) {
	useTestManager()

	app, err := Open(
		WithAppName("root-wait-error"),
		WithConfigSource("root", config.PriorityOverride, rootConfigSource()),
	)
	require.NoError(t, err)

	taskErr := errors.New("serve failed")
	require.NoError(
		t,
		app.ComposeAndInstall(context.Background(), func(Runtime) (*BusinessBundle, error) {
			return &BusinessBundle{
				Tasks: []BackgroundTask{newBlockingTask(taskErr)},
			}, nil
		}),
	)
	require.NoError(t, app.Start(context.Background()))
	require.ErrorIs(t, app.Wait(), taskErr)
}
