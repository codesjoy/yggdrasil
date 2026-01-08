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

// Package file provides functionality for reading configuration from a file
package file

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config/source"
	"github.com/codesjoy/yggdrasil/v2/internal/backoff"
	"github.com/codesjoy/yggdrasil/v2/utils/xgo"
	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

var errSourceClosed = errors.New("the file source is closed")

type file struct {
	mu            sync.Mutex
	exit          chan bool
	path          string
	parser        source.Parser
	enableWatcher bool

	stopped bool
	watched bool
	fw      *fsnotify.Watcher
}

func (f *file) Read() (source.Data, error) {
	fh, err := os.Open(f.path)
	if err != nil {
		return nil, err
	}
	defer fh.Close() //nolint:errcheck
	b, err := io.ReadAll(fh)
	if err != nil {
		return nil, err
	}
	cs := source.NewBytesSourceData(source.PriorityFile, b, f.parser)
	return cs, nil
}

func (f *file) Name() string {
	return "file"
}

func (f *file) Changeable() bool {
	return f.enableWatcher
}

func (f *file) Watch() (<-chan source.Data, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped {
		return nil, fmt.Errorf("the file source is stopped")
	}
	if f.watched {
		return nil, fmt.Errorf("file watcher is already enabled")
	}
	f.watched = true
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	_ = fw.Add(f.path)
	f.fw = fw
	change := make(chan source.Data, 1)
	xgo.Go(func() {
		defer func() {
			close(change)
		}()
		for {
			chg, err := f.watch()
			if err != nil {
				if errors.Is(err, errSourceClosed) {
					return
				}
				slog.Error("fault to watch file", slog.Any("error", err))
				continue
			}
			if chg == nil {
				return
			}
			change <- chg
		}
	})
	return change, nil
}

func (f *file) watch() (source.Data, error) {
	select {
	case <-f.exit:
		return nil, errSourceClosed
	default:
	}
	// try to get the event
	select {
	case event := <-f.fw.Events:
		if event.Op == fsnotify.Rename {
			// check existence of file, and add watch again
			_, err := os.Stat(event.Name)
			if err == nil || !errors.Is(err, os.ErrNotExist) {
				if err = f.fw.Add(f.path); err != nil {
					return nil, err
				}
			}
			func() {
				bo := backoff.Exponential{Config: backoff.Config{
					BaseDelay:  time.Second,
					Multiplier: 1.6,
					Jitter:     0.2,
					MaxDelay:   time.Second * 30,
				}}
				retries := 0
				t := time.NewTimer(time.Millisecond)
				defer t.Stop()
				for {
					select {
					case <-f.exit:
						return
					case <-t.C:
						_, err := os.Stat(f.path)
						if err == nil || !errors.Is(err, os.ErrNotExist) {
							if err = f.fw.Add(f.path); err == nil {
								return
							}
						}
						slog.Error("add watch", slog.Any("error", err))
						retries++
						t.Reset(bo.Backoff(retries))
					}
				}
			}()
		}
		_, err := os.Stat(f.path)
		if err != nil {
			return nil, err
		}
		c, err := f.Read()
		if err != nil {
			return nil, err
		}
		return c, nil
	case err := <-f.fw.Errors:
		return nil, err
	case <-f.exit:
		return nil, errSourceClosed
	}
}

func (f *file) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped {
		return nil
	}
	f.stopped = true
	close(f.exit)
	return nil
}

// NewSource returns a new file source
func NewSource(path string, watchable bool, parser ...source.Parser) source.Source {
	var p source.Parser
	if len(parser) > 0 {
		p = parser[0]
	} else {
		p, _ = source.ParseParser(strings.Trim(filepath.Ext(path), "."))
		if p == nil {
			p = yaml.Unmarshal
		}
	}
	return &file{
		path:          path,
		enableWatcher: watchable,
		parser:        p,
		exit:          make(chan bool),
	}
}
