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

package raw

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodecMarshal(t *testing.T) {
	c := codec{}
	want := []byte("hello")

	got, err := c.Marshal(want)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestCodecMarshalRejectsNonBytes(t *testing.T) {
	c := codec{}

	_, err := c.Marshal("hello")
	require.Error(t, err)
	assert.ErrorContains(t, err, "want []byte")
}

func TestCodecUnmarshal(t *testing.T) {
	c := codec{}
	data := []byte("hello")

	var got []byte
	err := c.Unmarshal(data, &got)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestCodecUnmarshalRejectsNonBytePointer(t *testing.T) {
	c := codec{}
	data := []byte("hello")

	var got string
	err := c.Unmarshal(data, &got)
	require.Error(t, err)
	assert.ErrorContains(t, err, "want *[]byte")
}
