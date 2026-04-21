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

package instance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitInstanceInfoAndGetters(t *testing.T) {
	InitInstanceInfo("demo-app", Config{
		Namespace: "ns-a",
		Version:   "1.2.3",
		Campus:    "campus-a",
		Region:    "region-a",
		Zone:      "zone-a",
		Metadata:  nil,
	})

	require.Equal(t, "ns-a", Namespace())
	require.Equal(t, "demo-app", Name())
	require.Equal(t, "1.2.3", Version())
	require.Equal(t, "region-a", Region())
	require.Equal(t, "zone-a", Zone())
	require.Equal(t, "campus-a", Campus())
	require.Empty(t, Metadata())
}

func TestMetadataReturnsCopyAndAddMetadata(t *testing.T) {
	InitInstanceInfo("demo-app", Config{
		Metadata: map[string]string{
			"env": "test",
		},
	})

	md := Metadata()
	md["env"] = "changed"

	require.Equal(t, "test", Metadata()["env"])
	require.True(t, global.AddMetadata("new", "1"))
	require.False(t, global.AddMetadata("new", "2"))
	require.Equal(t, "1", Metadata()["new"])
}
