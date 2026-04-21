package client

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/pkg/utils/xsync"
	"github.com/codesjoy/yggdrasil/v2/balancer"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

type countingBackoff struct {
	calls int32
}

func (b *countingBackoff) Backoff(_ int) time.Duration {
	atomic.AddInt32(&b.calls, 1)
	return time.Millisecond
}

func (b *countingBackoff) Count() int32 {
	return atomic.LoadInt32(&b.calls)
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
