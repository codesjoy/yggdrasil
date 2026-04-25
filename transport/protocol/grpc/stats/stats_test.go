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

package stats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientInHeader_GetCompression(t *testing.T) {
	s := &ClientInHeader{Compression: "gzip"}
	assert.Equal(t, "gzip", s.GetCompression())
}

func TestServerInHeader_GetCompression(t *testing.T) {
	s := &ServerInHeader{Compression: "snappy"}
	assert.Equal(t, "snappy", s.GetCompression())
}

func TestOutHeader_GetCompression(t *testing.T) {
	s := &OutHeader{Compression: "deflate"}
	assert.Equal(t, "deflate", s.GetCompression())
}

func TestInPayload_Getters(t *testing.T) {
	s := &InPayload{Compression: "gzip", CompressedLength: 42}
	assert.Equal(t, "gzip", s.GetCompression())
	assert.Equal(t, 42, s.GetCompressedLength())
}

func TestOutPayload_Getters(t *testing.T) {
	s := &OutPayload{Compression: "zstd", CompressedLength: 99}
	assert.Equal(t, "zstd", s.GetCompression())
	assert.Equal(t, 99, s.GetCompressedLength())
}
