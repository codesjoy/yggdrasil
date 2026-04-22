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

package client

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/pkg/utils/xsync"
	"github.com/codesjoy/yggdrasil/v3/balancer"
	"github.com/codesjoy/yggdrasil/v3/interceptor"
	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/resolver"
	"github.com/codesjoy/yggdrasil/v3/stream"
)

type testResolveNowResolver struct {
	resolveNowCalls int32
	addWatchErr     error
	delWatchErr     error
}

func (r *testResolveNowResolver) AddWatch(string, resolver.Client) error { return r.addWatchErr }
func (r *testResolveNowResolver) DelWatch(string, resolver.Client) error { return r.delWatchErr }
func (r *testResolveNowResolver) Type() string                           { return "test-resolver" }
func (r *testResolveNowResolver) ResolveNow()                            { atomic.AddInt32(&r.resolveNowCalls, 1) }

type testErrorBalancer struct {
	closeErr error
}

func (b *testErrorBalancer) UpdateState(resolver.State) {}
func (b *testErrorBalancer) Close() error               { return b.closeErr }
func (b *testErrorBalancer) Type() string               { return "test-error-balancer" }

type testCloseErrorClient struct {
	closeErr error
}

func (c *testCloseErrorClient) NewStream(context.Context, *stream.Desc, string) (stream.ClientStream, error) {
	return newMockClientStream(context.Background()), nil
}

func (c *testCloseErrorClient) Close() error        { return c.closeErr }
func (c *testCloseErrorClient) Scheme() string      { return "mock://close-err" }
func (c *testCloseErrorClient) State() remote.State { return remote.Ready }
func (c *testCloseErrorClient) Connect()            {}

func TestClientWaitForResolved(t *testing.T) {
	t.Run("fast path already resolved", func(t *testing.T) {
		cli := &client{resolvedEvent: xsync.NewEvent()}
		cli.resolvedEvent.Fire()
		require.NoError(t, cli.waitForResolved(context.Background()))
	})

	t.Run("ctx deadline exceeded", func(t *testing.T) {
		cli := &client{ctx: context.Background(), resolvedEvent: xsync.NewEvent()}
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()
		time.Sleep(5 * time.Millisecond)
		err := cli.waitForResolved(ctx)
		require.Error(t, err)
		require.Contains(t, strings.ToLower(err.Error()), "deadline")
	})

	t.Run("ctx canceled", func(t *testing.T) {
		cli := &client{ctx: context.Background(), resolvedEvent: xsync.NewEvent()}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := cli.waitForResolved(ctx)
		require.Error(t, err)
		require.Contains(t, strings.ToLower(err.Error()), "cancel")
	})

	t.Run("client closing", func(t *testing.T) {
		closedCtx, cancel := context.WithCancel(context.Background())
		cancel()
		cli := &client{ctx: closedCtx, resolvedEvent: xsync.NewEvent()}
		err := cli.waitForResolved(context.Background())
		require.ErrorIs(t, err, ErrClientClosing)
	})
}

func TestClientUpdateStateBufferReplacement(t *testing.T) {
	cli := &client{
		ctx:         context.Background(),
		stateChange: make(chan resolver.State, 1),
	}
	s1 := resolver.BaseState{Endpoints: []resolver.Endpoint{newMockEndpoint("old", "127.0.0.1:1001", "test")}}
	s2 := resolver.BaseState{Endpoints: []resolver.Endpoint{newMockEndpoint("new", "127.0.0.1:1002", "test")}}

	cli.UpdateState(s1)
	cli.UpdateState(s2)

	select {
	case got := <-cli.stateChange:
		require.Equal(t, "new", got.GetEndpoints()[0].Name())
	case <-time.After(time.Second):
		t.Fatal("timeout reading state change")
	}
}

func TestClientUpdateState_RemoteStatesPrunedAndStaleIgnored(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bc := &balancerClient{
		serializer:   xsync.NewSerializer(ctx),
		remoteStates: map[string]remote.State{"a": remote.Ready, "b": remote.Ready, "gone": remote.Ready},
		activeNames:  map[string]struct{}{},
	}
	defer bc.serializer.Close()

	cli := &client{
		ctx:            ctx,
		balancer:       newMockBalancer(),
		balancerClient: bc,
		resolvedEvent:  xsync.NewEvent(),
	}
	bc.cli = cli

	endpointA := newMockEndpoint("a", "127.0.0.1:9001", "grpc")
	endpointB := newMockEndpoint("b", "127.0.0.1:9002", "grpc")
	endpointC := newMockEndpoint("c", "127.0.0.1:9003", "grpc")

	cli.updateState(resolver.BaseState{Endpoints: []resolver.Endpoint{endpointA, endpointB}})
	require.Equal(t, map[string]remote.State{"a": remote.Ready, "b": remote.Ready}, bc.remoteStates)

	cli.updateState(resolver.BaseState{Endpoints: []resolver.Endpoint{endpointB, endpointC}})
	require.Equal(t, map[string]remote.State{"b": remote.Ready}, bc.remoteStates)

	_, tracked := bc.rememberRemoteState(remote.ClientState{
		Endpoint: endpointA,
		State:    remote.TransientFailure,
	})
	require.False(t, tracked)
	require.Equal(t, map[string]remote.State{"b": remote.Ready}, bc.remoteStates)
}

func TestClientWatchUpdateStateAndStaticState(t *testing.T) {
	t.Run("watchUpdateState applies updates", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mb := newMockBalancer()
		cli := &client{
			ctx:           ctx,
			balancer:      mb,
			stateChange:   make(chan resolver.State, 1),
			resolvedEvent: xsync.NewEvent(),
		}

		done := make(chan struct{})
		go func() {
			cli.watchUpdateState()
			close(done)
		}()

		cli.UpdateState(resolver.BaseState{
			Endpoints: []resolver.Endpoint{newMockEndpoint("a", "127.0.0.1:1001", "test")},
		})

		require.Eventually(t, func() bool {
			mb.mu.Lock()
			defer mb.mu.Unlock()
			return mb.updateCount > 0
		}, time.Second, 10*time.Millisecond)
		require.True(t, cli.resolvedEvent.HasFired())

		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("watchUpdateState did not exit")
		}
	})

	t.Run("updateStaticState validates endpoints", func(t *testing.T) {
		mb := newMockBalancer()
		cli := &client{
			balancer:      mb,
			resolvedEvent: xsync.NewEvent(),
		}

		err := cli.updateStaticState(ServiceSettings{})
		require.ErrorContains(t, err, "no endpoints provided")

		err = cli.updateStaticState(ServiceSettings{
			Remote: RemoteSettings{
				Endpoints: []resolver.BaseEndpoint{
					{Address: "127.0.0.1:1001", Protocol: "test"},
				},
			},
		})
		require.NoError(t, err)
		require.True(t, cli.resolvedEvent.HasFired())
	})
}

func TestClientResolveNow(t *testing.T) {
	cli := &client{}
	cli.resolveNow()

	r := &testResolveNowResolver{}
	cli.resolver = r
	cli.resolveNow()
	require.Equal(t, int32(1), atomic.LoadInt32(&r.resolveNowCalls))
}

func TestClientInvokeAndNewStream(t *testing.T) {
	remoteCli := newMockRemoteClient("invoke", remote.Ready)
	remoteCli.newStreamFunc = func(ctx context.Context, _ *stream.Desc, _ string) (stream.ClientStream, error) {
		st := newMockClientStream(ctx)
		return st, nil
	}
	picker := newMockPicker()
	picker.AddResult(newMockPickResult(remoteCli), nil)

	cli := &client{
		ctx:           context.Background(),
		fastFail:      true,
		resolvedEvent: xsync.NewEvent(),
	}
	cli.resolvedEvent.Fire()
	cli.pickerSnap.Store(&pickerSnap{picker: nil, blockingCh: make(chan struct{})})
	cli.updatePicker(picker)

	var streamIntCalled bool
	cli.streamInterceptor = func(
		ctx context.Context,
		desc *stream.Desc,
		method string,
		streamer interceptor.Streamer,
	) (stream.ClientStream, error) {
		streamIntCalled = true
		return streamer(ctx, desc, method)
	}
	_, err := cli.NewStream(context.Background(), &stream.Desc{ServerStreams: true}, "/svc/stream")
	require.NoError(t, err)
	require.True(t, streamIntCalled)

	var unaryIntCalled bool
	cli.unaryInterceptor = func(
		ctx context.Context,
		method string,
		req, reply any,
		invoker interceptor.UnaryInvoker,
	) error {
		unaryIntCalled = true
		return invoker(ctx, method, req, reply)
	}
	var reply string
	require.NoError(t, cli.Invoke(context.Background(), "/svc/unary", "req", &reply))
	require.True(t, unaryIntCalled)
}

func TestNewStream_NoAvailableInstanceDoesNotBackoff(t *testing.T) {
	for _, failFast := range []bool{true, false} {
		t.Run("fail_fast_"+strconv.FormatBool(failFast), func(t *testing.T) {
			backoffCounter := &countingBackoff{}
			cli := &client{
				ctx:           context.Background(),
				fastFail:      failFast,
				streamBackoff: backoffCounter,
				resolvedEvent: xsync.NewEvent(),
			}
			cli.resolvedEvent.Fire()
			cli.pickerSnap.Store(&pickerSnap{
				picker:     nil,
				blockingCh: make(chan struct{}),
			})
			picker := newMockPicker()
			picker.AddResult(nil, balancer.ErrNoAvailableInstance)
			cli.updatePicker(picker)

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			_, err := cli.newStream(ctx, &stream.Desc{}, "/svc/method")
			require.Error(t, err)
			require.Equal(t, int32(0), backoffCounter.Count())
		})
	}
}

func TestNewClientStaticAndClose(t *testing.T) {
	runtime := newTestRuntime()
	runtime.configs["svc"] = ServiceSettings{
		Remote: RemoteSettings{
			Endpoints: []resolver.BaseEndpoint{
				{Address: "127.0.0.1:1001", Protocol: "test"},
			},
		},
	}

	cliRaw, err := New(context.Background(), "svc", runtime)
	require.NoError(t, err)
	require.NotNil(t, cliRaw)
	cli := cliRaw.(*client)
	require.True(t, cli.resolvedEvent.HasFired())

	require.NoError(t, cli.Close())
	require.ErrorIs(t, cli.Close(), ErrClientClosing)
}

func TestNewClientNoEndpoints(t *testing.T) {
	runtime := newTestRuntime()
	runtime.configs["svc"] = ServiceSettings{}

	cli, err := New(context.Background(), "svc", runtime)
	require.ErrorContains(t, err, "no endpoints provided")
	require.Nil(t, cli)
}

func TestInitResolverAndBalancerErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := newTestRuntime()
	runtime.newBalancer = func(string, string, balancer.Client) (balancer.Balancer, error) {
		return nil, errors.New("balancer not found")
	}
	cli := &client{ctx: ctx, appName: "svc", runtime: runtime}
	err := cli.initResolverAndBalancer(ServiceSettings{Balancer: "not-exist"})
	require.Error(t, err)

	runtime.newBalancer = func(string, string, balancer.Client) (balancer.Balancer, error) {
		return newMockBalancer(), nil
	}
	runtime.newResolver = func(string) (resolver.Resolver, error) {
		return nil, errors.New("resolver not found")
	}
	err = cli.initResolverAndBalancer(ServiceSettings{
		Balancer: "default",
		Resolver: "not-exist",
	})
	require.Error(t, err)
}

func TestBalancerClientMethods(t *testing.T) {
	t.Run("UpdateState updates picker and channel state", func(t *testing.T) {
		cli := &client{}
		cli.pickerSnap.Store(&pickerSnap{picker: nil, blockingCh: make(chan struct{})})
		bc := &balancerClient{cli: cli}

		p := newMockPicker()
		bc.UpdateState(balancer.State{Picker: p, ConnectivityState: remote.Ready})
		require.Equal(t, int32(remote.Ready), cli.channelState.Load())
		require.Equal(t, p, cli.pickerSnap.Load().picker)
	})

	t.Run("NewRemoteClient delegates to manager", func(t *testing.T) {
		ctx := context.Background()
		manager := newRemoteClientManager(ctx, "svc", newMockStatsHandler(), newTestRuntime())
		cli := &client{
			ctx:                 ctx,
			appName:             "svc",
			remoteClientManager: manager,
		}
		bc := &balancerClient{cli: cli}
		rc, err := bc.NewRemoteClient(
			newMockEndpoint("e1", "127.0.0.1:1001", "test"),
			balancer.NewRemoteClientOptions{},
		)
		require.NoError(t, err)
		require.NotNil(t, rc)
	})

	t.Run("createStateListener nil listener", func(t *testing.T) {
		bc := &balancerClient{}
		bc.createStateListener(nil)(remote.ClientState{})
	})

	t.Run("createStateListener without serializer invokes resolve now", func(t *testing.T) {
		r := &testResolveNowResolver{}
		cli := &client{resolver: r}
		bc := &balancerClient{
			cli:          cli,
			remoteStates: map[string]remote.State{},
			activeNames:  map[string]struct{}{"e1": {}},
		}

		var called int32
		l := bc.createStateListener(func(remote.ClientState) {
			atomic.AddInt32(&called, 1)
		})
		l(remote.ClientState{
			Endpoint:        newMockEndpoint("e1", "127.0.0.1:1001", "test"),
			State:           remote.TransientFailure,
			ConnectionError: errors.New("dial failed"),
		})

		require.Equal(t, int32(1), atomic.LoadInt32(&called))
		require.Equal(t, int32(1), atomic.LoadInt32(&r.resolveNowCalls))
	})

	t.Run("createStateListener with serializer", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		r := &testResolveNowResolver{}
		cli := &client{resolver: r}
		bc := &balancerClient{
			cli:          cli,
			serializer:   xsync.NewSerializer(ctx),
			remoteStates: map[string]remote.State{"e1": remote.Ready},
			activeNames:  map[string]struct{}{"e1": {}},
		}
		defer bc.serializer.Close()

		done := make(chan struct{})
		l := bc.createStateListener(func(remote.ClientState) { close(done) })
		l(remote.ClientState{
			Endpoint: newMockEndpoint("e1", "127.0.0.1:1001", "test"),
			State:    remote.Idle,
		})

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("listener not called through serializer")
		}
		require.Equal(t, int32(1), atomic.LoadInt32(&r.resolveNowCalls))
	})
}

func TestClientCloseAggregatesErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := newRemoteClientManager(ctx, "svc", newMockStatsHandler(), newTestRuntime())
	manager.remoteClients["close-err"] = &rcWrapper{
		name:                "close-err",
		remoteClientManager: manager,
		Client:              &testCloseErrorClient{closeErr: errors.New("remote close error")},
	}

	resolverWithErr := &testResolveNowResolver{delWatchErr: errors.New("del watch error")}
	cli := &client{
		ctx:                 ctx,
		cancel:              cancel,
		appName:             "svc",
		resolver:            resolverWithErr,
		balancer:            &testErrorBalancer{closeErr: errors.New("balancer close error")},
		remoteClientManager: manager,
	}

	err := cli.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), "del watch error")
	require.Contains(t, err.Error(), "balancer close error")
	require.Contains(t, err.Error(), "remote close error")
	require.ErrorIs(t, cli.Close(), ErrClientClosing)
}
