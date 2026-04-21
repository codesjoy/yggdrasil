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

package yggdrasil

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v2/config"
)

type loggerLevelHolder struct {
	Level string `mapstructure:"level"`
}

func TestGetLoggerHandlerConfigValue_PreferNestedConfig(t *testing.T) {
	cfg := config.NewConfig(".")
	require.NoError(t, cfg.Set("type", "console"))
	require.NoError(t, cfg.Set("level", "debug"))
	require.NoError(t, cfg.Set("config.level", "info"))

	vals := cfg.ValueToValues(cfg.Get(""))
	selected := getLoggerHandlerConfigValue(vals)

	var got loggerLevelHolder
	require.NoError(t, selected.Scan(&got))
	require.Equal(t, "info", got.Level)
}

func TestGetLoggerHandlerConfigValue_FallbackToLegacyRoot(t *testing.T) {
	cfg := config.NewConfig(".")
	require.NoError(t, cfg.Set("type", "console"))
	require.NoError(t, cfg.Set("level", "warn"))

	vals := cfg.ValueToValues(cfg.Get(""))
	selected := getLoggerHandlerConfigValue(vals)

	var got loggerLevelHolder
	require.NoError(t, selected.Scan(&got))
	require.Equal(t, "warn", got.Level)
}
