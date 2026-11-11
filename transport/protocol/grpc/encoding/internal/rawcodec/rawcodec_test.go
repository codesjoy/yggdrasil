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

package rawcodec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gmem "google.golang.org/grpc/mem"
)

func TestCodec(t *testing.T) {
	for _, name := range []string{"raw", "jsonraw"} {
		t.Run(name, func(t *testing.T) {
			codec := New(name)
			payload := []byte(`{"message":"hello"}`)

			t.Run("marshal", func(t *testing.T) {
				got, err := codec.Marshal(payload)
				require.NoError(t, err)
				assert.Equal(t, payload, got)
			})

			t.Run("marshal rejects non bytes", func(t *testing.T) {
				_, err := codec.Marshal("hello")
				require.Error(t, err)
				assert.ErrorContains(t, err, "want []byte")
			})

			t.Run("unmarshal", func(t *testing.T) {
				var got []byte
				require.NoError(t, codec.Unmarshal(payload, &got))
				assert.Equal(t, payload, got)
			})

			t.Run("unmarshal rejects non byte pointer", func(t *testing.T) {
				var got string
				err := codec.Unmarshal(payload, &got)
				require.Error(t, err)
				assert.ErrorContains(t, err, "want *[]byte")
			})

			t.Run("marshal v2", func(t *testing.T) {
				t.Run("success", func(t *testing.T) {
					got, err := codec.MarshalV2(payload)
					require.NoError(t, err)
					assert.Equal(t, payload, got.Materialize())
				})

				t.Run("empty bytes returns nil", func(t *testing.T) {
					got, err := codec.MarshalV2([]byte{})
					require.NoError(t, err)
					assert.Nil(t, got)
				})

				t.Run("non bytes error", func(t *testing.T) {
					_, err := codec.MarshalV2("not bytes")
					require.Error(t, err)
					assert.ErrorContains(t, err, "want []byte")
				})
			})

			t.Run("unmarshal v2", func(t *testing.T) {
				t.Run("success", func(t *testing.T) {
					buffer := gmem.NewBuffer(&payload, nil)
					var got []byte
					require.NoError(t, codec.UnmarshalV2(gmem.BufferSlice{buffer}, &got))
					assert.Equal(t, payload, got)
				})

				t.Run("non pointer error", func(t *testing.T) {
					var got string
					err := codec.UnmarshalV2(nil, &got)
					require.Error(t, err)
					assert.ErrorContains(t, err, "want *[]byte")
				})
			})

			t.Run("name", func(t *testing.T) {
				assert.Equal(t, name, codec.Name())
			})
		})
	}
}
