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
