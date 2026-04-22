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

package server_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/interceptor"
	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/remote/marshaler"
	"github.com/codesjoy/yggdrasil/v3/server"
	"github.com/codesjoy/yggdrasil/v3/server/rest"
	restmiddleware "github.com/codesjoy/yggdrasil/v3/server/rest/middleware"
	"github.com/codesjoy/yggdrasil/v3/stats"
)

type externalServerRuntime struct{}

func (externalServerRuntime) ServerSettings() server.Settings {
	return server.Settings{Transports: []string{"test"}}
}

func (externalServerRuntime) ServerStatsHandler() stats.Handler { return stats.NoOpHandler }
func (externalServerRuntime) RESTConfig() *rest.Config          { return nil }
func (externalServerRuntime) RESTMiddlewareProviders() map[string]restmiddleware.Provider {
	return map[string]restmiddleware.Provider{}
}
func (externalServerRuntime) MarshalerBuilders() map[string]marshaler.MarshallerBuilder {
	return map[string]marshaler.MarshallerBuilder{}
}
func (externalServerRuntime) BuildUnaryServerInterceptor([]string) interceptor.UnaryServerInterceptor {
	return nil
}
func (externalServerRuntime) BuildStreamServerInterceptor([]string) interceptor.StreamServerInterceptor {
	return nil
}
func (externalServerRuntime) TransportServerProvider(string) remote.TransportServerProvider {
	return remote.NewTransportServerProvider("test", func(remote.MethodHandle) (remote.Server, error) {
		return &externalRemoteServer{
			info: remote.ServerInfo{
				Protocol: "test",
				Address:  "127.0.0.1:19090",
			},
		}, nil
	})
}

type externalRemoteServer struct{ info remote.ServerInfo }

func (s *externalRemoteServer) Start() error               { return nil }
func (s *externalRemoteServer) Handle() error              { return nil }
func (s *externalRemoteServer) Stop(context.Context) error { return nil }
func (s *externalRemoteServer) Info() remote.ServerInfo    { return s.info }

func TestNewServerRequiresRuntime(t *testing.T) {
	svr, err := server.New(nil)
	require.ErrorContains(t, err, "server runtime is required")
	require.Nil(t, svr)
}

func TestNewServerUsesExplicitRuntime(t *testing.T) {
	svr, err := server.New(externalServerRuntime{})
	require.NoError(t, err)
	require.NotNil(t, svr)

	started := make(chan struct{}, 1)
	require.NoError(t, svr.Serve(started))
	require.NotEmpty(t, svr.Endpoints())
	require.NoError(t, svr.Stop(context.Background()))
}
