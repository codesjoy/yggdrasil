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

package config

import "strings"

// View is an immutable, decode-capable config subsection.
type View interface {
	Path() string
	Decode(target any) error
	Sub(path string) View
	Exists() bool
}

type snapshotView struct {
	path     string
	snapshot Snapshot
}

// NewView creates a view from a snapshot and logical path.
func NewView(path string, snapshot Snapshot) View {
	return snapshotView{
		path:     strings.TrimSpace(path),
		snapshot: snapshot,
	}
}

func (v snapshotView) Path() string {
	return v.path
}

func (v snapshotView) Decode(target any) error {
	return v.snapshot.Decode(target)
}

func (v snapshotView) Sub(path string) View {
	path = strings.TrimSpace(path)
	if path == "" {
		return v
	}
	parts := splitDotPath(path)
	childPath := path
	if v.path != "" {
		childPath = v.path + "." + path
	}
	return snapshotView{
		path:     childPath,
		snapshot: v.snapshot.Section(parts...),
	}
}

func (v snapshotView) Exists() bool {
	return !v.snapshot.Empty()
}

func splitDotPath(path string) []string {
	raw := strings.Split(path, ".")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}
