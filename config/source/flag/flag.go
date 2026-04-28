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

// Package flag provides a source for loading configuration from flags.
package flag

import (
	flag2 "flag"
	"os"
	"strings"

	"github.com/codesjoy/pkg/utils/xmap"

	"github.com/codesjoy/yggdrasil/v3/config/source"
)

type flag struct {
	fs           *flag2.FlagSet
	ignoredNames map[string]struct{}
}

func (fs *flag) Read() (source.Data, error) {
	if !fs.fs.Parsed() {
		if err := fs.parseKnownArgs(os.Args[1:]); err != nil {
			return nil, err
		}
	}
	result := make(map[string]any)
	visitFn := func(f *flag2.Flag) {
		if fs.ignored(f.Name) {
			return
		}
		n := strings.ToLower(f.Name)
		keys := strings.FieldsFunc(n, split)
		reverse(keys)

		tmp := make(map[string]any)
		for i, k := range keys {
			if i == 0 {
				if v, ok := f.Value.(flag2.Getter); ok {
					tmp[k] = v.Get()
				} else {
					tmp[k] = f.Value
				}
				continue
			}

			tmp = map[string]any{k: tmp}
		}
		xmap.MergeStringMap(result, tmp)
	}
	fs.fs.VisitAll(visitFn)
	return source.NewMapData(result), nil
}

func (fs *flag) parseKnownArgs(args []string) error {
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" || arg == "--" {
			continue
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			continue
		}
		nameValue := strings.TrimLeft(arg, "-")
		if nameValue == "" {
			continue
		}
		name, value, hasValue := strings.Cut(nameValue, "=")
		name = strings.TrimSpace(name)
		if name == "" || fs.ignored(name) {
			continue
		}
		item := fs.fs.Lookup(name)
		if item == nil {
			continue
		}
		if !hasValue {
			if boolValue, ok := item.Value.(boolFlag); ok && boolValue.IsBoolFlag() {
				value = "true"
				hasValue = true
			} else if i+1 < len(args) {
				i++
				value = args[i]
				hasValue = true
			}
		}
		if !hasValue {
			continue
		}
		if err := fs.fs.Set(name, value); err != nil {
			return err
		}
	}
	return nil
}

func (fs *flag) ignored(name string) bool {
	if fs == nil || len(fs.ignoredNames) == 0 {
		return false
	}
	_, ok := fs.ignoredNames[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

type boolFlag interface {
	IsBoolFlag() bool
}

func split(r rune) bool {
	return r == '-' || r == '_' || r == '.'
}

func reverse(ss []string) {
	for i := len(ss)/2 - 1; i >= 0; i-- {
		opp := len(ss) - 1 - i
		ss[i], ss[opp] = ss[opp], ss[i]
	}
}

func (fs *flag) Name() string {
	if fs.fs == nil {
		return ""
	}
	return fs.fs.Name()
}

func (fs *flag) Kind() string {
	return "flag"
}

func (fs *flag) Close() error {
	return nil
}

// NewSource creates a new flag source.
func NewSource(fs ...*flag2.FlagSet) source.Source {
	if len(fs) == 0 || fs[0] == nil {
		return NewSourceWithOptions(flag2.CommandLine)
	}
	return NewSourceWithOptions(fs[0])
}

// Option customizes a flag source.
type Option func(*flag)

// WithIgnoredNames skips exact flag names.
func WithIgnoredNames(names ...string) Option {
	return func(f *flag) {
		if f.ignoredNames == nil {
			f.ignoredNames = map[string]struct{}{}
		}
		for _, name := range names {
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" {
				continue
			}
			f.ignoredNames[name] = struct{}{}
		}
	}
}

// NewSourceWithOptions creates a new flag source with custom options.
func NewSourceWithOptions(fs *flag2.FlagSet, opts ...Option) source.Source {
	if fs == nil {
		fs = flag2.CommandLine
	}
	src := &flag{fs: fs}
	for _, opt := range opts {
		opt(src)
	}
	return src
}
