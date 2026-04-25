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

package peer

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithContextAndFromContext(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		p := &Peer{
			Addr:     &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
			Protocol: "grpc",
		}
		ctx := WithContext(context.Background(), p)
		got, ok := FromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, p, got)
	})

	t.Run("no peer in context", func(t *testing.T) {
		_, ok := FromContext(context.Background())
		assert.False(t, ok)
	})
}
