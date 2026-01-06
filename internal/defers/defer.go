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

// Package defers provides a defer mechanism.
package defers

import (
	"sync"
)

// Defer is a defer mechanism.
type Defer struct {
	sync.Mutex
	fns []func() error
}

// NewDefer creates a new defer mechanism.
func NewDefer() *Defer {
	return &Defer{
		fns: make([]func() error, 0),
	}
}

// Register registers the functions to be executed.
func (d *Defer) Register(fns ...func() error) {
	d.Lock()
	defer d.Unlock()
	d.fns = append(d.fns, fns...)
}

// Done executes the registered functions.
func (d *Defer) Done() {
	d.Lock()
	defer d.Unlock()
	for i := len(d.fns) - 1; i >= 0; i-- {
		_ = d.fns[i]()
	}
}
