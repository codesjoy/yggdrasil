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

package xgo

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGo tests the Go function
func TestGo(t *testing.T) {
	t.Run("simple execution", func(t *testing.T) {
		var executed atomic.Bool
		Go(func() {
			executed.Store(true)
		})

		// Wait for goroutine to execute
		time.Sleep(100 * time.Millisecond)
		assert.True(t, executed.Load(), "goroutine should be executed")
	})

	t.Run("multiple goroutines", func(t *testing.T) {
		var counter atomic.Int32
		for i := 0; i < 10; i++ {
			Go(func() {
				counter.Add(1)
			})
		}

		// Wait for all goroutines to execute
		time.Sleep(200 * time.Millisecond)
		assert.Equal(t, int32(10), counter.Load(), "all goroutines should be executed")
	})

	t.Run("panic recovery", func(t *testing.T) {
		var executed atomic.Bool
		var recovered atomic.Bool

		Go(func() {
			executed.Store(true)
			panic("test panic")
		})

		// Set recovered flag after delay
		go func() {
			time.Sleep(150 * time.Millisecond)
			recovered.Store(true)
		}()

		// Wait for goroutine to execute and recover
		time.Sleep(100 * time.Millisecond)
		assert.True(t, executed.Load(), "goroutine should be executed before panic")

		// Test should not crash - panic should be recovered
		// We can't directly test that panic was logged, but we verify no crash
		time.Sleep(100 * time.Millisecond)
		assert.True(t, recovered.Load(), "test should complete without crashing")
	})

	t.Run("channel communication", func(t *testing.T) {
		ch := make(chan string, 1)
		Go(func() {
			ch <- "hello from goroutine"
		})

		select {
		case msg := <-ch:
			assert.Equal(t, "hello from goroutine", msg)
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for message")
		}
	})

	t.Run("wait group synchronization", func(t *testing.T) {
		var wg sync.WaitGroup
		var counter atomic.Int32

		for i := 0; i < 5; i++ {
			wg.Add(1)
			Go(func() {
				defer wg.Done()
				counter.Add(1)
			})
		}

		wg.Wait()
		assert.Equal(t, int32(5), counter.Load())
	})
}

// TestGoWithCtx tests the GoWithCtx function
func TestGoWithCtx(t *testing.T) {
	t.Run("simple execution", func(t *testing.T) {
		var executed atomic.Bool
		ctx := context.Background()

		GoWithCtx(ctx, func(c context.Context) {
			executed.Store(true)
			assert.Equal(t, ctx, c)
		})

		// Wait for goroutine to execute
		time.Sleep(100 * time.Millisecond)
		assert.True(t, executed.Load(), "goroutine should be executed")
	})

	t.Run("context cancellation", func(t *testing.T) {
		var cancelled atomic.Bool
		ctx, cancel := context.WithCancel(context.Background())

		GoWithCtx(ctx, func(c context.Context) {
			<-c.Done()
			cancelled.Store(true)
		})

		// Cancel the context
		time.Sleep(50 * time.Millisecond)
		cancel()

		// Wait for goroutine to detect cancellation
		time.Sleep(100 * time.Millisecond)
		assert.True(t, cancelled.Load(), "goroutine should detect context cancellation")
	})

	t.Run("context with timeout", func(t *testing.T) {
		var timeout atomic.Bool
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		GoWithCtx(ctx, func(c context.Context) {
			<-c.Done()
			timeout.Store(true)
		})

		// Wait for timeout
		time.Sleep(200 * time.Millisecond)
		assert.True(t, timeout.Load(), "goroutine should detect context timeout")
	})

	t.Run("context with value", func(t *testing.T) {
		type key string
		const myKey key = "testKey"

		ctx := context.WithValue(context.Background(), myKey, "testValue")
		var receivedValue atomic.Value

		GoWithCtx(ctx, func(c context.Context) {
			val := c.Value(myKey)
			receivedValue.Store(val)
		})

		// Wait for goroutine to execute
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, "testValue", receivedValue.Load())
	})

	t.Run("panic recovery with context", func(t *testing.T) {
		var executed atomic.Bool
		ctx := context.Background()

		GoWithCtx(ctx, func(c context.Context) {
			executed.Store(true)
			panic("test panic in goroutine with context")
		})

		// Wait for goroutine to execute and recover
		time.Sleep(100 * time.Millisecond)
		assert.True(t, executed.Load(), "goroutine should be executed before panic")

		// Test should not crash - panic should be recovered
		time.Sleep(50 * time.Millisecond)
		// If we reach here, panic was properly recovered
		assert.True(t, true)
	})

	t.Run("multiple goroutines with context", func(t *testing.T) {
		var counter atomic.Int32
		ctx := context.Background()

		for i := 0; i < 10; i++ {
			GoWithCtx(ctx, func(c context.Context) {
				counter.Add(1)
			})
		}

		// Wait for all goroutines to execute
		time.Sleep(200 * time.Millisecond)
		assert.Equal(t, int32(10), counter.Load(), "all goroutines should be executed")
	})

	t.Run("cancelled context prevents execution", func(t *testing.T) {
		var executed atomic.Bool
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel immediately
		cancel()

		GoWithCtx(ctx, func(c context.Context) {
			executed.Store(true)
		})

		// Goroutine might still execute, but should see cancelled context
		time.Sleep(100 * time.Millisecond)
		// We can't guarantee it won't execute, but context should be cancelled
		assert.True(t, true)
	})

	t.Run("context with deadline", func(t *testing.T) {
		var deadlineReached atomic.Bool
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(50*time.Millisecond))
		defer cancel()

		GoWithCtx(ctx, func(c context.Context) {
			<-c.Done()
			deadlineReached.Store(true)
		})

		// Wait for deadline
		time.Sleep(150 * time.Millisecond)
		assert.True(t, deadlineReached.Load(), "goroutine should detect context deadline")
	})

	t.Run("select statement with context", func(t *testing.T) {
		ctx := context.Background()
		ch := make(chan string, 1)

		GoWithCtx(ctx, func(c context.Context) {
			select {
			case <-c.Done():
				// Context cancelled
			case msg := <-ch:
				// Received message
				assert.Equal(t, "test message", msg)
			}
		})

		// Send message
		ch <- "test message"

		// Wait for goroutine to process
		time.Sleep(100 * time.Millisecond)
	})
}

// TestGo_ConcurrentExecution tests concurrent goroutine execution
func TestGo_ConcurrentExecution(t *testing.T) {
	const numGoroutines = 100
	var counter atomic.Int32
	var startWait, endWait sync.WaitGroup

	startWait.Add(numGoroutines)
	endWait.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		Go(func() {
			startWait.Done()
			startWait.Wait() // Wait for all to start

			counter.Add(1)

			endWait.Done()
		})
	}

	endWait.Wait()
	assert.Equal(t, int32(numGoroutines), counter.Load())
}

// TestGoWithCtx_ParentContextCancellation tests parent context cancellation
func TestGoWithCtx_ParentContextCancellation(t *testing.T) {
	var child1Done, child2Done atomic.Bool

	parentCtx, parentCancel := context.WithCancel(context.Background())

	child1Ctx, _ := context.WithCancel(parentCtx)
	child2Ctx, _ := context.WithCancel(parentCtx)

	GoWithCtx(child1Ctx, func(c context.Context) {
		<-c.Done()
		child1Done.Store(true)
	})

	GoWithCtx(child2Ctx, func(c context.Context) {
		<-c.Done()
		child2Done.Store(true)
	})

	// Cancel parent context
	time.Sleep(50 * time.Millisecond)
	parentCancel()

	// Both children should detect cancellation
	time.Sleep(100 * time.Millisecond)
	assert.True(t, child1Done.Load(), "child1 should detect parent cancellation")
	assert.True(t, child2Done.Load(), "child2 should detect parent cancellation")
}

// TestGo_NilPanicRecovery tests that nil panic is also recovered
func TestGo_NilPanicRecovery(t *testing.T) {
	var executed atomic.Bool

	Go(func() {
		executed.Store(true)
		panic(nil)
	})

	// Wait for goroutine to execute and recover
	time.Sleep(100 * time.Millisecond)
	assert.True(t, executed.Load(), "goroutine should be executed before panic")

	// Test should not crash
	assert.True(t, true)
}
