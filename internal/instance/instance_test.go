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

func TestInstallProcessDefaultAndSnapshot(t *testing.T) {
	prev := ProcessDefaultSnapshot()
	t.Cleanup(func() {
		RestoreProcessDefault(prev)
	})

	InstallProcessDefault("demo-app", Config{
		Namespace: "ns-a",
		Version:   "1.2.3",
		Campus:    "campus-a",
		Region:    "region-a",
		Zone:      "zone-a",
		Metadata:  nil,
	})

	snapshot := ProcessDefaultSnapshot()
	require.Equal(t, "demo-app", snapshot.AppName)
	require.Equal(t, "ns-a", snapshot.Config.Namespace)
	require.Equal(t, "1.2.3", snapshot.Config.Version)
	require.Equal(t, "region-a", snapshot.Config.Region)
	require.Equal(t, "zone-a", snapshot.Config.Zone)
	require.Equal(t, "campus-a", snapshot.Config.Campus)
	require.Empty(t, snapshot.Config.Metadata)
}

func TestMetadataReturnsCopyAndAddMetadata(t *testing.T) {
	prev := ProcessDefaultSnapshot()
	t.Cleanup(func() {
		RestoreProcessDefault(prev)
	})

	InstallProcessDefault("demo-app", Config{
		Metadata: map[string]string{
			"env": "test",
		},
	})

	md := ProcessDefaultSnapshot().Config.Metadata
	md["env"] = "changed"

	require.Equal(t, "test", ProcessDefaultSnapshot().Config.Metadata["env"])
	require.True(t, global.AddMetadata("new", "1"))
	require.False(t, global.AddMetadata("new", "2"))
	require.Equal(t, "1", ProcessDefaultSnapshot().Config.Metadata["new"])
}
