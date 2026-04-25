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

package module

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStopOnceReturnsFirstError(t *testing.T) {
	var stop StopOnce
	calls := 0
	err := stop.Do(context.Background(), func(context.Context) error {
		calls++
		return errors.New("first")
	})
	require.EqualError(t, err, "first")

	err = stop.Do(context.Background(), func(context.Context) error {
		calls++
		return nil
	})
	require.EqualError(t, err, "first")
	require.Equal(t, 1, calls)
}

func TestStopOnceRecoversPanic(t *testing.T) {
	var stop StopOnce
	err := stop.Do(context.Background(), func(context.Context) error {
		panic("boom")
	})
	require.ErrorContains(t, err, "stop panic recovered")
	require.ErrorContains(t, err, "boom")
}
