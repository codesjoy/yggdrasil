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
	"context"
	"errors"
	"sync"
)

// Defer is a defer mechanism.
type Defer struct {
	sync.Mutex
	fns []func(context.Context) error
}

// NewDefer creates a new defer mechanism.
func NewDefer() *Defer {
	return &Defer{
		fns: make([]func(context.Context) error, 0),
	}
}

// Register registers the functions to be executed.
func (d *Defer) Register(fns ...func(context.Context) error) {
	d.Lock()
	defer d.Unlock()
	d.fns = append(d.fns, fns...)
}

// Done executes the registered functions.
func (d *Defer) Done(ctx context.Context) error {
	d.Lock()
	defer d.Unlock()
	var multiErr error
	for i := len(d.fns) - 1; i >= 0; i-- {
		if err := d.fns[i](ctx); err != nil {
			multiErr = errors.Join(multiErr, err)
		}
	}
	return multiErr
}
