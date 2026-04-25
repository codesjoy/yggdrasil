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
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReloadState_WithNilError(t *testing.T) {
	s := ReloadState{}.withError(nil)
	require.Equal(t, "", s.LastErrorText)
	require.Nil(t, s.LastError)
}

func TestReloadState_WithNonNilError(t *testing.T) {
	err := errors.New("something broke")
	s := ReloadState{}.withError(err)
	require.Equal(t, "something broke", s.LastErrorText)
	require.Equal(t, err, s.LastError)
}
