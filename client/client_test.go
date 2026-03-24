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
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/v2/balancer"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

func init() {
	// Register mock balancer builder
	balancer.RegisterBuilder(
		"mock_balancer",
		func(serviceName, balancerName string, client balancer.Client) (balancer.Balancer, error) {
			return newMockBalancer(), nil
		},
	)

	// Register mock resolver builder
	resolver.RegisterBuilder("mock_type", func(name string) (resolver.Resolver, error) {
		return newMockResolver(), nil
	})

	// Register mock remote client builder
	remote.RegisterClientBuilder(
		"mock_protocol",
		func(ctx context.Context, s string, e resolver.Endpoint, h stats.Handler, f remote.OnStateChange) (remote.Client, error) {
			return newMockRemoteClient(e.Name(), remote.Ready), nil
		},
	)
}

func setupConfig(appName string, conf map[string]interface{}) error {
	key := config.Join(config.KeyBase, "client", fmt.Sprintf("{%s}", appName))
	if v, ok := conf["balancer"].(string); ok && v != "" {
		_ = config.Set(config.Join(config.KeyBase, "balancer", v, "type"), v)
	}
	return config.Set(key, conf)
}

func TestNewClient_Static(t *testing.T) {
	appName := "test_static_app"
	endpoints := []resolver.BaseEndpoint{
		{Address: "127.0.0.1:8080", Protocol: "tcp"},
	}
	conf := map[string]interface{}{
		"balancer": "mock_balancer",
		"remote": map[string]interface{}{
			"endpoints": endpoints,
		},
	}

	if err := setupConfig(appName, conf); err != nil {
		t.Fatalf("setupConfig failed: %v", err)
	}

	cli, err := NewClient(context.Background(), appName)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer cli.Close() // nolint:errcheck

	c := cli.(*client)
	if c.resolver != nil {
		t.Error("expected resolver to be nil for static config")
	}

	// Wait for state update (async in NewClient -> updateState -> updatePicker)
	time.Sleep(50 * time.Millisecond)

	mb, ok := c.balancer.(*mockBalancer)
	if !ok {
		t.Fatalf("expected balancer to be *mockBalancer, got %T", c.balancer)
	}
	mb.mu.Lock()
	defer mb.mu.Unlock()
	if mb.state == nil {
		t.Error("expected balancer state to be updated")
	} else if len(mb.state.GetEndpoints()) != 1 {
		t.Errorf("expected 1 endpoint, got %d", len(mb.state.GetEndpoints()))
	}
}

func TestNewClient_Resolver(t *testing.T) {
	appName := "test_resolver_app"
	conf := map[string]interface{}{
		"balancer": "mock_balancer",
		"resolver": "test_resolver",
	}

	// Setup resolver config
	resolverKey := config.Join(config.KeyBase, "resolver", "test_resolver", "type")
	if err := config.Set(resolverKey, "mock_type"); err != nil {
		t.Fatalf("config.Set resolver type failed: %v", err)
	}

	if err := setupConfig(appName, conf); err != nil {
		t.Fatalf("setupConfig failed: %v", err)
	}

	cli, err := NewClient(context.Background(), appName)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer cli.Close() // nolint:errcheck

	c := cli.(*client)
	if c.resolver == nil {
		t.Error("expected resolver to be non-nil")
	}

	mr := c.resolver.(*mockResolver)
	mr.mu.Lock()
	if _, ok := mr.watchers[appName]; !ok {
		t.Error("expected watcher to be added")
	}
	mr.mu.Unlock()
}

func TestInvoke_Success(t *testing.T) {
	appName := "test_invoke_app"
	conf := map[string]interface{}{
		"balancer": "mock_balancer",
		"remote": map[string]interface{}{
			"endpoints": []resolver.BaseEndpoint{{Address: "addr"}},
		},
	}
	_ = setupConfig(appName, conf)

	cli, err := NewClient(context.Background(), appName)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer cli.Close()

	c := cli.(*client)

	// Mock the picker to return a valid result
	mockClient := newMockRemoteClient("test_remote", remote.Ready)
	mockClient.newStreamFunc = func(ctx context.Context, desc *stream.Desc, method string) (stream.ClientStream, error) {
		cs := newMockClientStream(ctx)
		return cs, nil
	}
	reported := make([]error, 0, 1)
	pickResult := newMockPickResult(mockClient)
	pickResult.reportFunc = func(err error) {
		reported = append(reported, err)
	}

	picker := newMockPicker()
	picker.AddResult(pickResult, nil)
	c.updatePicker(picker)

	err = cli.Invoke(context.Background(), "/test/method", "args", "reply")
	if err != nil {
		t.Errorf("Invoke failed: %v", err)
	}
	if len(reported) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reported))
	}
	if reported[0] != nil {
		t.Fatalf("expected success report, got %v", reported[0])
	}
}

func TestInvoke_PickerError(t *testing.T) {
	appName := "test_invoke_err_app"
	conf := map[string]interface{}{
		"balancer": "mock_balancer",
		"remote": map[string]interface{}{
			"endpoints": []resolver.BaseEndpoint{{Address: "addr"}},
		},
	}
	_ = setupConfig(appName, conf)

	cli, err := NewClient(context.Background(), appName)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer cli.Close()

	c := cli.(*client)

	// Mock picker to return error
	picker := newMockPicker()
	picker.AddResult(nil, errors.New("picker error"))
	c.updatePicker(picker)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = cli.Invoke(ctx, "/test/method", "args", "reply")
	if err == nil {
		t.Error("expected Invoke to fail with picker error")
	}
}

func TestNewStream_Success(t *testing.T) {
	appName := "test_stream_app"
	conf := map[string]interface{}{
		"balancer": "mock_balancer",
		"remote": map[string]interface{}{
			"endpoints": []resolver.BaseEndpoint{{Address: "addr"}},
		},
	}
	_ = setupConfig(appName, conf)

	cli, err := NewClient(context.Background(), appName)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer cli.Close()

	c := cli.(*client)

	mockClient := newMockRemoteClient("test_remote", remote.Ready)
	reported := make([]error, 0, 1)
	pickResult := newMockPickResult(mockClient)
	pickResult.reportFunc = func(err error) {
		reported = append(reported, err)
	}
	picker := newMockPicker()
	picker.AddResult(pickResult, nil)
	c.updatePicker(picker)

	s, err := cli.NewStream(context.Background(), &stream.Desc{}, "/test/method")
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}
	if s == nil {
		t.Error("expected stream to be non-nil")
	}
	if len(reported) != 0 {
		t.Fatalf("expected no report before RPC completion, got %d", len(reported))
	}
}

func TestNewStream_RemoteErrorReportsFailure(t *testing.T) {
	appName := "test_stream_report_err_app"
	conf := map[string]interface{}{
		"balancer": "mock_balancer",
		"remote": map[string]interface{}{
			"endpoints": []resolver.BaseEndpoint{{Address: "addr"}},
		},
	}
	_ = setupConfig(appName, conf)

	cli, err := NewClient(context.Background(), appName)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer cli.Close()

	c := cli.(*client)
	mockClient := newMockRemoteClient("test_remote", remote.Ready)
	wantErr := errors.New("new stream failed")
	mockClient.newStreamFunc = func(ctx context.Context, desc *stream.Desc, method string) (stream.ClientStream, error) {
		return nil, wantErr
	}

	reported := make([]error, 0, 1)
	pickResult := newMockPickResult(mockClient)
	pickResult.reportFunc = func(err error) {
		reported = append(reported, err)
	}

	picker := newMockPicker()
	picker.AddResult(pickResult, nil)
	c.updatePicker(picker)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err = cli.NewStream(ctx, &stream.Desc{}, "/test/method")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if len(reported) == 0 {
		t.Fatal("expected at least 1 report")
	}
	for i, reportedErr := range reported {
		if !errors.Is(reportedErr, wantErr) {
			t.Fatalf("expected reported error %v at index %d, got %v", wantErr, i, reportedErr)
		}
	}
}

func TestClientStream_ReportLifecycle(t *testing.T) {
	t.Run("send failure reports once", func(t *testing.T) {
		mockStream := newMockClientStream(context.Background())
		wantErr := errors.New("send failed")
		mockStream.SetSendErr(wantErr)

		reported := make([]error, 0, 1)
		cs := &clientStream{
			desc:         &stream.Desc{},
			ClientStream: mockStream,
			report: func(err error) {
				reported = append(reported, err)
			},
		}

		err := cs.SendMsg("payload")
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected %v, got %v", wantErr, err)
		}

		mockStream.SetRecvErr(nil)
		if err := cs.RecvMsg(new(string)); err != nil {
			t.Fatalf("expected recv success, got %v", err)
		}

		if len(reported) != 1 {
			t.Fatalf("expected 1 report, got %d", len(reported))
		}
		if !errors.Is(reported[0], wantErr) {
			t.Fatalf("expected reported error %v, got %v", wantErr, reported[0])
		}
	})

	t.Run("unary recv success reports nil once", func(t *testing.T) {
		mockStream := newMockClientStream(context.Background())
		reported := make([]error, 0, 1)
		cs := &clientStream{
			desc:         &stream.Desc{},
			ClientStream: mockStream,
			report: func(err error) {
				reported = append(reported, err)
			},
		}

		if err := cs.RecvMsg(new(string)); err != nil {
			t.Fatalf("expected recv success, got %v", err)
		}

		mockStream.SetRecvErr(errors.New("late recv failure"))
		err := cs.RecvMsg(new(string))
		if err == nil {
			t.Fatal("expected second recv to fail")
		}

		if len(reported) != 1 {
			t.Fatalf("expected 1 report, got %d", len(reported))
		}
		if reported[0] != nil {
			t.Fatalf("expected success report, got %v", reported[0])
		}
	})

	t.Run("unary recv failure reports error", func(t *testing.T) {
		mockStream := newMockClientStream(context.Background())
		wantErr := errors.New("recv failed")
		mockStream.SetRecvErr(wantErr)

		reported := make([]error, 0, 1)
		cs := &clientStream{
			desc:         &stream.Desc{},
			ClientStream: mockStream,
			report: func(err error) {
				reported = append(reported, err)
			},
		}

		err := cs.RecvMsg(new(string))
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected %v, got %v", wantErr, err)
		}
		if len(reported) != 1 {
			t.Fatalf("expected 1 report, got %d", len(reported))
		}
		if !errors.Is(reported[0], wantErr) {
			t.Fatalf("expected reported error %v, got %v", wantErr, reported[0])
		}
	})

	t.Run("server stream reports success on eof only", func(t *testing.T) {
		mockStream := newMockClientStream(context.Background())
		reported := make([]error, 0, 1)
		cs := &clientStream{
			desc:         &stream.Desc{ServerStreams: true},
			ClientStream: mockStream,
			report: func(err error) {
				reported = append(reported, err)
			},
		}

		if err := cs.RecvMsg(new(string)); err != nil {
			t.Fatalf("expected first recv success, got %v", err)
		}
		if len(reported) != 0 {
			t.Fatalf("expected no report before stream completion, got %d", len(reported))
		}

		mockStream.SetRecvErr(io.EOF)
		err := cs.RecvMsg(new(string))
		if !errors.Is(err, io.EOF) {
			t.Fatalf("expected EOF, got %v", err)
		}
		if len(reported) != 1 {
			t.Fatalf("expected 1 report, got %d", len(reported))
		}
		if reported[0] != nil {
			t.Fatalf("expected success report, got %v", reported[0])
		}
	})
}

func TestClose(t *testing.T) {
	appName := "test_close_app"
	// Use resolver so we can check if watcher is removed
	conf := map[string]interface{}{
		"balancer": "mock_balancer",
		"resolver": "test_resolver_close",
	}
	resolverKey := config.Join(config.KeyBase, "resolver", "test_resolver_close", "type")
	config.Set(resolverKey, "mock_type")
	setupConfig(appName, conf)

	cli, err := NewClient(context.Background(), appName)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if err := cli.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	c := cli.(*client)
	mr := c.resolver.(*mockResolver)
	mr.mu.Lock()
	if _, ok := mr.watchers[appName]; ok {
		t.Error("expected watcher to be removed")
	}
	mr.mu.Unlock()

	mb := c.balancer.(*mockBalancer)
	mb.mu.Lock()
	if !mb.closed {
		t.Error("expected balancer to be closed")
	}
	mb.mu.Unlock()
}

func TestBalancerClient_UpdateState(t *testing.T) {
	cli := &client{}
	cli.pickerSnap.Store(&pickerSnap{
		picker:     nil,
		blockingCh: make(chan struct{}),
	})

	bc := &balancerClient{cli: cli}

	picker := newMockPicker()
	state := balancer.State{Picker: picker}

	bc.UpdateState(state)

	snap := cli.pickerSnap.Load()
	if snap.picker != picker {
		t.Error("expected picker to be updated")
	}
}

func TestBalancerClient_NewRemoteClient(t *testing.T) {
	appName := "test_balancer_client_app"
	conf := map[string]interface{}{
		"balancer": "mock_balancer",
		"remote": map[string]interface{}{
			"endpoints": []resolver.BaseEndpoint{{Address: "addr"}},
		},
	}
	setupConfig(appName, conf)

	cli, err := NewClient(context.Background(), appName)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer cli.Close()

	c := cli.(*client)
	bc := &balancerClient{cli: c}

	// Use protocol "mock_protocol" which is registered in init
	ep := newMockEndpoint("ep1", "address", "mock_protocol")

	// Create client
	rc, err := bc.NewRemoteClient(ep, balancer.NewRemoteClientOptions{})
	if err != nil {
		t.Errorf("NewRemoteClient failed: %v", err)
	}
	if rc == nil {
		t.Error("expected remote client")
	}

	// Check if cached
	rc2, err := bc.NewRemoteClient(ep, balancer.NewRemoteClientOptions{})
	if err != nil {
		t.Errorf("NewRemoteClient 2 failed: %v", err)
	}
	if rc != rc2 {
		t.Error("expected remote client to be cached and same instance")
	}
}

func TestNewClient_WithDefaults(t *testing.T) {
	appName := "test_defaults_app"
	endpoints := []resolver.BaseEndpoint{
		{Address: "127.0.0.1:8080", Protocol: "mock_protocol"},
	}
	conf := map[string]interface{}{
		"remote": map[string]interface{}{
			"endpoints": endpoints,
		},
		// Note: NOT specifying balancer - should default to "default" -> round_robin
		// Note: NOT specifying resolver - should use static endpoints
	}

	if err := setupConfig(appName, conf); err != nil {
		t.Fatalf("setupConfig failed: %v", err)
	}

	cli, err := NewClient(context.Background(), appName)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer cli.Close()

	c := cli.(*client)

	// Verify resolver is nil (using static endpoints)
	if c.resolver != nil {
		t.Error("expected resolver to be nil for static config")
	}

	// Verify balancer is round_robin (default)
	if c.balancer.Type() != "round_robin" {
		t.Errorf("expected balancer type round_robin, got %s", c.balancer.Type())
	}

	// Wait for state update (async in NewClient -> updateState -> updatePicker)
	time.Sleep(50 * time.Millisecond)

	// The balancer should have received the static endpoint
	// We can't check the concrete type since it's not exported, but we verified the Type()
}
