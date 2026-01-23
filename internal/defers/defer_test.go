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

package defers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefer_Order(t *testing.T) {
	d := NewDefer()
	var str string
	d.Register(
		func(context.Context) error { str += "1,"; return nil },
		func(context.Context) error { str += "2,"; return nil },
		func(context.Context) error { str += "3,"; return nil },
		func(context.Context) error { str += "4,"; return nil },
	)
	err := d.Done(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "4,3,2,1,", str)
}

func TestDefer_ErrorJoin(t *testing.T) {
	d := NewDefer()
	err1 := errors.New("err1")
	err2 := errors.New("err2")
	d.Register(
		func(context.Context) error { return err1 },
		func(context.Context) error { return nil },
		func(context.Context) error { return err2 },
	)

	err := d.Done(context.Background())
	assert.Error(t, err)
	assert.True(t, errors.Is(err, err1))
	assert.True(t, errors.Is(err, err2))
}
