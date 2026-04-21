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

package file

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/codesjoy/yggdrasil/v2/config/source"
)

func TestWatchGuardBranches(t *testing.T) {
	src := NewSource("/tmp/a.yaml", false).(*file)
	change, err := src.Watch()
	require.NoError(t, err)
	require.Nil(t, change)

	stopped := &file{enableWatcher: true, stopped: true, exit: make(chan bool)}
	_, err = stopped.Watch()
	require.Error(t, err)
	require.Contains(t, err.Error(), "stopped")

	watched := &file{enableWatcher: true, watched: true, exit: make(chan bool)}
	_, err = watched.Watch()
	require.Error(t, err)
	require.Contains(t, err.Error(), "already enabled")
}

func TestWatchMethodErrorExitAndEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("app:\n  name: demo\n"), 0o600))

	f := &file{
		path:   path,
		parser: yaml.Unmarshal,
		exit:   make(chan bool),
		fw: &fsnotify.Watcher{
			Events: make(chan fsnotify.Event, 1),
			Errors: make(chan error, 1),
		},
	}

	f.fw.Events <- fsnotify.Event{Name: path, Op: fsnotify.Write}
	data, err := f.watch()
	require.NoError(t, err)
	require.NotNil(t, data)

	watchErr := errors.New("watch error")
	f.fw.Errors <- watchErr
	_, err = f.watch()
	require.ErrorIs(t, err, watchErr)

	close(f.exit)
	_, err = f.watch()
	require.ErrorIs(t, err, errSourceClosed)
}

func TestWatchMethodExitWhileWaiting(t *testing.T) {
	f := &file{
		path:   filepath.Join(t.TempDir(), "config.yaml"),
		parser: yaml.Unmarshal,
		exit:   make(chan bool),
		fw: &fsnotify.Watcher{
			Events: make(chan fsnotify.Event),
			Errors: make(chan error),
		},
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		close(f.exit)
	}()

	_, err := f.watch()
	require.ErrorIs(t, err, errSourceClosed)
}

func TestWatchMethodRenameBranch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("app:\n  name: demo\n"), 0o600))

	fw, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer fw.Close()

	f := &file{
		path:   path,
		parser: yaml.Unmarshal,
		exit:   make(chan bool),
		fw:     fw,
	}

	go func() {
		fw.Events <- fsnotify.Event{
			Name: filepath.Join(dir, "not-exists.yaml"),
			Op:   fsnotify.Rename,
		}
	}()

	data, err := f.watch()
	require.NoError(t, err)
	require.NotNil(t, data)
}

func TestWatchMethodEventPathMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	f := &file{
		path:   path,
		parser: yaml.Unmarshal,
		exit:   make(chan bool),
		fw: &fsnotify.Watcher{
			Events: make(chan fsnotify.Event, 1),
			Errors: make(chan error, 1),
		},
	}

	f.fw.Events <- fsnotify.Event{Name: path, Op: fsnotify.Write}
	_, err := f.watch()
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrNotExist))
}

func TestWatchSuccessAndCloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("app:\n  name: demo\n"), 0o600))

	src := NewSource(path, true).(*file)
	changeCh, err := src.Watch()
	require.NoError(t, err)
	require.NotNil(t, changeCh)

	require.NoError(t, os.WriteFile(path, []byte("app:\n  name: changed\n"), 0o600))

	var data source.Data
	select {
	case data = <-changeCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting file watch change")
	}
	require.NotNil(t, data)

	var out map[string]any
	require.NoError(t, data.Unmarshal(&out))
	require.Equal(t, "changed", out["app"].(map[string]any)["name"])

	require.NoError(t, src.Close())
	require.NoError(t, src.Close())
}
