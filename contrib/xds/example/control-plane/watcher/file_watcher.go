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

package watcher

import (
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher watches a file for changes
type FileWatcher struct {
	filePath string
	watcher  *fsnotify.Watcher
	callback func(string)
	debounce time.Duration
	mu       sync.Mutex
	timer    *time.Timer
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(
	filePath string,
	callback func(string),
	debounce time.Duration,
) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fw := &FileWatcher{
		filePath: filePath,
		watcher:  watcher,
		callback: callback,
		debounce: debounce,
	}

	dir := filepath.Dir(filePath)
	if err := watcher.Add(dir); err != nil {
		_ = watcher.Close()
		return nil, err
	}

	return fw, nil
}

// Start starts the file watcher
func (fw *FileWatcher) Start() {
	go fw.watch()
}

func (fw *FileWatcher) watch() {
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			if filepath.Clean(event.Name) != filepath.Clean(fw.filePath) {
				continue
			}

			if event.Op&fsnotify.Write == fsnotify.Write ||
				event.Op&fsnotify.Create == fsnotify.Create {
				slog.Info("File changed", "path", event.Name)
				fw.debounceCallback()
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("File watcher error", "error", err)
		}
	}
}

func (fw *FileWatcher) debounceCallback() {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.timer != nil {
		fw.timer.Stop()
	}

	fw.timer = time.AfterFunc(fw.debounce, func() {
		fw.callback(fw.filePath)
	})
}

// Close closes the file watcher
func (fw *FileWatcher) Close() error {
	return fw.watcher.Close()
}
