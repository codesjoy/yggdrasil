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
	"github.com/codesjoy/yggdrasil/v2/internal/configtest"
	"github.com/codesjoy/yggdrasil/v2/internal/settings"
	"github.com/codesjoy/yggdrasil/v2/logger"
)

func TestInitLoggerUsesNewLoggingTree(t *testing.T) {
	configtest.Set(t, "yggdrasil.logging.writers.default.type", "console")
	configtest.Set(t, "yggdrasil.logging.handlers.default.type", "text")
	configtest.Set(t, "yggdrasil.logging.handlers.default.writer", "default")
	configtest.Set(t, "yggdrasil.logging.handlers.default.config.level", "info")
	configtest.Set(t, "yggdrasil.logging.remote_level", "error")
	root, err := settings.NewCatalog(config.Default()).Root().Current()
	require.NoError(t, err)
	resolved, err := settings.Compile(root)
	require.NoError(t, err)
	logger.Configure(resolved.Logging)

	require.NoError(t, initLogger())
}
