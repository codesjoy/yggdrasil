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

package flag

import (
	"encoding/json"
	flag2 "flag"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlagRead(t *testing.T) {
	fs := flag2.NewFlagSet("test", flag2.ContinueOnError)
	value := fs.String("app-server-port", "8080", "")
	require.NoError(t, fs.Parse([]string{"--app-server-port=8088"}))
	require.Equal(t, "8088", *value)

	src := NewSource(fs)
	data, err := src.Read()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data.Bytes(), &out))
	require.Equal(t, "8088", out["app"].(map[string]any)["server"].(map[string]any)["port"])
}

func TestFlagReadParsesArgsInMultiplePasses(t *testing.T) {
	fs := flag2.NewFlagSet("flag-test", flag2.ContinueOnError)
	_ = fs.String("app-server-port", "8080", "")
	_ = fs.String("service_name", "default", "")
	_ = fs.String("yggdrasil.server.transports", "", "")

	oldArgs := os.Args
	os.Args = []string{
		"flag-test",
		"--app-server-port=7001",
		"sub",
		"--service_name=demo",
		"--unknown=value",
		"--yggdrasil.server.transports=grpc",
	}
	t.Cleanup(func() { os.Args = oldArgs })

	src := NewSource(fs)
	data, err := src.Read()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data.Bytes(), &out))
	require.Equal(t, "7001", out["app"].(map[string]any)["server"].(map[string]any)["port"])
	require.Equal(t, "demo", out["service"].(map[string]any)["name"])
	require.Equal(
		t,
		"grpc",
		out["yggdrasil"].(map[string]any)["server"].(map[string]any)["transports"],
	)
	require.Equal(t, "flag-test", src.Name())
	require.Equal(t, "flag", src.Kind())
	require.NoError(t, src.Close())
}

func TestFlagReadIgnoresNames(t *testing.T) {
	fs := flag2.NewFlagSet("flag-test", flag2.ContinueOnError)
	_ = fs.String("yggdrasil-config", "", "")
	_ = fs.String("app-name", "demo", "")

	oldArgs := os.Args
	os.Args = []string{"flag-test", "--yggdrasil-config=./config.yaml", "--app-name=ok"}
	t.Cleanup(func() { os.Args = oldArgs })

	src := NewSourceWithOptions(fs, WithIgnoredNames("yggdrasil-config"))
	data, err := src.Read()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data.Bytes(), &out))
	require.Equal(t, "ok", out["app"].(map[string]any)["name"])
	require.NotContains(t, out, "yggdrasil")
}

func TestFlagNameWithNilFlagSet(t *testing.T) {
	src := &flag{}
	require.Equal(t, "", src.Name())
}

func TestNewSourceWithNilDefaultsToCommandLine(t *testing.T) {
	src := NewSource(nil)
	require.NotNil(t, src)
	require.Equal(t, "flag", src.Kind())
}
