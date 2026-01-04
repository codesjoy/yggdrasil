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
	"sync/atomic"
	"testing"
	"time"

	"github.com/codesjoy/yggdrasil/pkg/balancer"
	"github.com/codesjoy/yggdrasil/pkg/remote"
)

// mockPicker is a mock implementation of balancer.Picker
type mockPicker struct {
	results     []balancer.PickResult
	errors      []error
	callCount   int32
	returnIndex int32
}

func newMockPicker() *mockPicker {
	return &mockPicker{
		results: make([]balancer.PickResult, 0),
		errors:  make([]error, 0),
	}
}

func (m *mockPicker) AddResult(result balancer.PickResult, err error) {
	m.results = append(m.results, result)
	m.errors = append(m.errors, err)
}

func (m *mockPicker) Next(ri balancer.RPCInfo) (balancer.PickResult, error) {
	atomic.AddInt32(&m.callCount, 1)
	idx := atomic.AddInt32(&m.returnIndex, 1) - 1
	if int(idx) >= len(m.results) {
		idx = int32(len(m.results) - 1)
	}
	return m.results[idx], m.errors[idx]
}

func (m *mockPicker) GetCallCount() int32 {
	return atomic.LoadInt32(&m.callCount)
}

// mockPickResult is a mock implementation of balancer.PickResult
type mockPickResult struct {
	client     remote.Client
	reportFunc func(err error)
}

func newMockPickResult(client remote.Client) *mockPickResult {
	return &mockPickResult{
		client: client,
	}
}

func (m *mockPickResult) RemoteClient() remote.Client {
	return m.client
}

func (m *mockPickResult) Report(err error) {
	if m.reportFunc != nil {
		m.reportFunc(err)
	}
}

func TestPickerSnap_UpdatePicker(t *testing.T) {
	cli := &client{}
	cli.pickerSnap.Store(&pickerSnap{
		picker:     nil,
		blockingCh: make(chan struct{}),
	})

	picker1 := newMockPicker()
	mockClient := newMockRemoteClient("test", remote.Ready)
	picker1.AddResult(newMockPickResult(mockClient), nil)

	// Update picker
	cli.updatePicker(picker1)

	// Verify new picker is set
	snap := cli.pickerSnap.Load()
	if snap.picker != picker1 {
		t.Fatal("expected picker to be updated")
	}
}

func TestPickerSnap_UpdatePicker_ClosesOldChannel(t *testing.T) {
	cli := &client{}
	oldCh := make(chan struct{})
	cli.pickerSnap.Store(&pickerSnap{
		picker:     nil,
		blockingCh: oldCh,
	})

	picker1 := newMockPicker()
	mockClient := newMockRemoteClient("test", remote.Ready)
	picker1.AddResult(newMockPickResult(mockClient), nil)

	// Update picker
	cli.updatePicker(picker1)

	// Verify old channel is closed
	select {
	case <-oldCh:
		// Channel is closed, as expected
	default:
		t.Fatal("expected old blocking channel to be closed")
	}
}

func TestPick_Success(t *testing.T) {
	cli := &client{}
	cli.pickerSnap.Store(&pickerSnap{
		picker:     nil,
		blockingCh: make(chan struct{}),
	})

	mockClient := newMockRemoteClient("test", remote.Ready)
	picker := newMockPicker()
	picker.AddResult(newMockPickResult(mockClient), nil)

	cli.updatePicker(picker)

	ctx := context.Background()
	info := &balancer.RPCInfo{
		Ctx:    ctx,
		Method: "/test/method",
	}

	result, err := cli.pick(true, info)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result to be non-nil")
	}
	if result.RemoteClient() != mockClient {
		t.Fatal("expected correct remote client")
	}
}

func TestPick_NilPickerSnap(t *testing.T) {
	cli := &client{}
	// Don't initialize pickerSnap

	ctx := context.Background()
	info := &balancer.RPCInfo{
		Ctx:    ctx,
		Method: "/test/method",
	}

	_, err := cli.pick(true, info)
	if err != ErrClientClosing {
		t.Fatalf("expected ErrClientClosing, got %v", err)
	}
}

func TestPick_ContextCanceled(t *testing.T) {
	cli := &client{}
	cli.pickerSnap.Store(&pickerSnap{
		picker:     nil, // nil picker will cause blocking
		blockingCh: make(chan struct{}),
	})

	ctx, cancel := context.WithCancel(context.Background())
	info := &balancer.RPCInfo{
		Ctx:    ctx,
		Method: "/test/method",
	}

	// Cancel context immediately
	cancel()

	_, err := cli.pick(true, info)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestPick_ContextDeadlineExceeded(t *testing.T) {
	cli := &client{}
	cli.pickerSnap.Store(&pickerSnap{
		picker:     nil, // nil picker will cause blocking
		blockingCh: make(chan struct{}),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	info := &balancer.RPCInfo{
		Ctx:    ctx,
		Method: "/test/method",
	}

	_, err := cli.pick(true, info)
	if err == nil {
		t.Fatal("expected error for deadline exceeded")
	}
}

func TestPick_NotReadyClient_Retry(t *testing.T) {
	cli := &client{}
	cli.pickerSnap.Store(&pickerSnap{
		picker:     nil,
		blockingCh: make(chan struct{}),
	})

	// First client is not ready, second is ready
	notReadyClient := newMockRemoteClient("not-ready", remote.Connecting)
	readyClient := newMockRemoteClient("ready", remote.Ready)

	// Initial picker returns NotReady
	picker1 := newMockPicker()
	picker1.AddResult(newMockPickResult(notReadyClient), nil)
	cli.updatePicker(picker1)

	ctx := context.Background()
	info := &balancer.RPCInfo{
		Ctx:    ctx,
		Method: "/test/method",
	}

	resultCh := make(chan balancer.PickResult)
	errCh := make(chan error)

	// Run pick in goroutine as it should block on NotReady
	go func() {
		result, err := cli.pick(true, info)
		resultCh <- result
		errCh <- err
	}()

	// Wait a bit to ensure pick is blocked
	time.Sleep(10 * time.Millisecond)

	// Update picker with one that returns Ready
	picker2 := newMockPicker()
	picker2.AddResult(newMockPickResult(readyClient), nil)
	cli.updatePicker(picker2)

	select {
	case result := <-resultCh:
		err := <-errCh
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result.RemoteClient() != readyClient {
			t.Fatal("expected ready client to be picked")
		}
	case <-time.After(time.Second):
		t.Fatal("pick did not complete in time")
	}
}

func TestPick_NoAvailableInstance_FailFast(t *testing.T) {
	cli := &client{}
	cli.pickerSnap.Store(&pickerSnap{
		picker:     nil,
		blockingCh: make(chan struct{}),
	})

	// Initial picker returns ErrNoAvailableInstance
	picker1 := newMockPicker()
	picker1.AddResult(nil, balancer.ErrNoAvailableInstance)
	cli.updatePicker(picker1)

	ctx := context.Background()
	info := &balancer.RPCInfo{
		Ctx:    ctx,
		Method: "/test/method",
	}

	resultCh := make(chan balancer.PickResult)
	errCh := make(chan error)

	// Start pick which should block
	go func() {
		result, err := cli.pick(true, info)
		resultCh <- result
		errCh <- err
	}()

	// Wait for pick to block
	time.Sleep(10 * time.Millisecond)

	// Update picker with valid result
	mockClient := newMockRemoteClient("test", remote.Ready)
	picker2 := newMockPicker()
	picker2.AddResult(newMockPickResult(mockClient), nil)
	cli.updatePicker(picker2)

	select {
	case result := <-resultCh:
		err := <-errCh
		if err != nil {
			t.Fatalf("expected no error after retry, got %v", err)
		}
		if result == nil {
			t.Fatal("expected result to be non-nil")
		}
		if result.RemoteClient() != mockClient {
			t.Fatal("expected correct remote client")
		}
	case <-time.After(time.Second):
		t.Fatal("pick did not complete in time")
	}
}

func TestPick_PickerUpdatedDuringPick(t *testing.T) {
	cli := &client{}
	blockingCh := make(chan struct{})
	cli.pickerSnap.Store(&pickerSnap{
		picker:     nil,
		blockingCh: blockingCh,
	})

	mockClient := newMockRemoteClient("test", remote.Ready)
	newPicker := newMockPicker()
	newPicker.AddResult(newMockPickResult(mockClient), nil)

	ctx := context.Background()
	info := &balancer.RPCInfo{
		Ctx:    ctx,
		Method: "/test/method",
	}

	// Start pick in goroutine
	resultCh := make(chan balancer.PickResult)
	errCh := make(chan error)
	go func() {
		result, err := cli.pick(true, info)
		resultCh <- result
		errCh <- err
	}()

	// Give pick time to start blocking
	time.Sleep(10 * time.Millisecond)

	// Update picker (this closes the old blocking channel)
	cli.updatePicker(newPicker)

	// Pick should complete now
	select {
	case result := <-resultCh:
		err := <-errCh
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result == nil {
			t.Fatal("expected result to be non-nil")
		}
	case <-time.After(time.Second):
		t.Fatal("pick did not complete in time")
	}
}
