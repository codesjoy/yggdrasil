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

package client_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client/balancer"
)

type externalClientRuntime struct{}

func (externalClientRuntime) ClientSettings(string) client.ServiceSettings {
	return client.ServiceSettings{
		Remote: client.RemoteSettings{
			Endpoints: []resolver.BaseEndpoint{
				{Address: "127.0.0.1:18080", Protocol: "test"},
			},
		},
	}
}

func (externalClientRuntime) ClientStatsHandler() stats.Handler { return stats.NoOpHandler }

func (externalClientRuntime) TransportClientProvider(string) remote.TransportClientProvider {
	return remote.NewTransportClientProvider("test", func(
		context.Context,
		string,
		resolver.Endpoint,
		stats.Handler,
		remote.OnStateChange,
	) (remote.Client, error) {
		return externalRemoteClient{}, nil
	})
}

func (externalClientRuntime) NewResolver(string) (resolver.Resolver, error) { return nil, nil }

func (externalClientRuntime) NewBalancer(
	string,
	string,
	balancer.Client,
) (balancer.Balancer, error) {
	return externalBalancer{}, nil
}

func (externalClientRuntime) BuildUnaryClientInterceptor(
	string,
	[]string,
) interceptor.UnaryClientInterceptor {
	return nil
}

func (externalClientRuntime) BuildStreamClientInterceptor(
	string,
	[]string,
) interceptor.StreamClientInterceptor {
	return nil
}

type externalRemoteClient struct{}

func (externalRemoteClient) NewStream(
	context.Context,
	*stream.Desc,
	string,
) (stream.ClientStream, error) {
	return nil, nil
}
func (externalRemoteClient) Close() error        { return nil }
func (externalRemoteClient) Protocol() string    { return "test" }
func (externalRemoteClient) State() remote.State { return remote.Ready }
func (externalRemoteClient) Connect()            {}

type externalBalancer struct{}

func (externalBalancer) UpdateState(resolver.State) {}
func (externalBalancer) Close() error               { return nil }
func (externalBalancer) Type() string               { return "external" }

func TestNewClientRequiresRuntime(t *testing.T) {
	cli, err := client.New(context.Background(), "svc", nil)
	require.ErrorContains(t, err, "client runtime is required")
	require.Nil(t, cli)
}

func TestNewClientUsesExplicitRuntime(t *testing.T) {
	cli, err := client.New(context.Background(), "svc", externalClientRuntime{})
	require.NoError(t, err)
	require.NotNil(t, cli)
	require.NoError(t, cli.Close())
}
