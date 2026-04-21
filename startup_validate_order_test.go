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

	"github.com/codesjoy/yggdrasil/v2/client"
	"github.com/codesjoy/yggdrasil/v2/internal/configtest"
	"github.com/codesjoy/yggdrasil/v2/logger"
	"github.com/codesjoy/yggdrasil/v2/registry"
	"github.com/codesjoy/yggdrasil/v2/remote/rest"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/server"
	"github.com/codesjoy/yggdrasil/v2/stats"
)

func TestRefreshResolvedSettings_DoesNotApplyGlobalStores(t *testing.T) {
	configtest.Set(t, "yggdrasil.telemetry.tracer", "missing-tracer")

	sentinel := installSentinelStores()
	t.Cleanup(resetSentinelStores)

	opts := &options{}
	require.NoError(t, refreshResolvedSettings(opts))
	require.Equal(t, "missing-tracer", opts.resolvedSettings.Telemetry.Tracer)
	assertSentinelStores(t, sentinel)
}

func TestValidateStartup_FailedValidationDoesNotApplyGlobalStores(t *testing.T) {
	configtest.Set(t, "yggdrasil.admin.validation.strict", true)
	configtest.Set(t, "yggdrasil.telemetry.tracer", "missing-tracer")

	sentinel := installSentinelStores()
	t.Cleanup(resetSentinelStores)

	err := validateStartup(nil)
	require.Error(t, err)
	assertSentinelStores(t, sentinel)
}

type sentinelStores struct {
	logger   logger.Settings
	client   client.ServiceConfig
	server   server.Settings
	stats    stats.Settings
	registry registry.Spec
	resolver resolver.Spec
	rest     *rest.Config
}

func installSentinelStores() sentinelStores {
	loggerState := logger.Settings{
		Handlers: map[string]logger.HandlerSpec{
			"sentinel": {Type: "json", Writer: "sentinel"},
		},
		Writers: map[string]logger.WriterSpec{
			"sentinel": {Type: "console"},
		},
		Interceptors: map[string]map[string]any{
			"sentinel": {"enabled": true},
		},
		RemoteLevel: "warn",
	}
	clientState := client.ServiceConfig{Resolver: "sentinel"}
	serverState := server.Settings{Transports: []string{"sentinel"}, RestEnabled: true}
	statsState := stats.Settings{Server: "sentinel", Client: "sentinel"}
	registryState := registry.Spec{Type: "sentinel", Config: map[string]any{"enabled": true}}
	resolverState := resolver.Spec{Type: "sentinel", Config: map[string]any{"enabled": true}}
	restState := &rest.Config{Port: 4321}

	logger.Configure(loggerState)
	client.Configure(client.Settings{Services: map[string]client.ServiceConfig{"sentinel": clientState}})
	server.Configure(serverState)
	stats.Configure(statsState)
	registry.Configure(registryState)
	resolver.Configure(map[string]resolver.Spec{"sentinel": resolverState})
	rest.Configure(restState)

	return sentinelStores{
		logger:   loggerState,
		client:   clientState,
		server:   serverState,
		stats:    statsState,
		registry: registryState,
		resolver: resolverState,
		rest:     restState,
	}
}

func resetSentinelStores() {
	logger.Configure(logger.Settings{})
	client.Configure(client.Settings{})
	server.Configure(server.Settings{})
	stats.Configure(stats.Settings{})
	registry.Configure(registry.Spec{})
	resolver.Configure(nil)
	rest.Configure(nil)
}

func assertSentinelStores(t *testing.T, sentinel sentinelStores) {
	t.Helper()
	require.Equal(t, sentinel.logger, logger.CurrentSettings())
	require.Equal(t, sentinel.client, client.CurrentConfig("sentinel"))
	require.Equal(t, sentinel.server, server.CurrentSettings())
	require.Equal(t, sentinel.stats, stats.CurrentSettings())
	require.Equal(t, sentinel.registry, registry.CurrentSpec())
	require.Equal(t, sentinel.resolver, resolver.CurrentSpec("sentinel"))
	require.Same(t, sentinel.rest, rest.CurrentConfig())
}
