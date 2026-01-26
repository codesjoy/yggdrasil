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

package xds

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// RateLimiterConfig holds rate limiter configuration
type RateLimiterConfig struct {
	MaxTokens     uint32
	TokensPerFill uint32
	FillInterval  time.Duration
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	config *RateLimiterConfig

	tokens       uint32
	lastFillTime int64
	mu           sync.Mutex

	allowedCount  uint64
	rejectedCount uint64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewRateLimiter creates a new rate limiter with the given configuration
func NewRateLimiter(config *RateLimiterConfig) *RateLimiter {
	if config == nil {
		config = &RateLimiterConfig{
			MaxTokens:     1000,
			TokensPerFill: 100,
			FillInterval:  time.Second,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	rl := &RateLimiter{
		config:       config,
		tokens:       config.MaxTokens,
		lastFillTime: time.Now().UnixNano(),
		ctx:          ctx,
		cancel:       cancel,
	}

	rl.wg.Add(1)
	go rl.refillTokens()

	return rl
}

func (rl *RateLimiter) refillTokens() {
	defer rl.wg.Done()

	ticker := time.NewTicker(rl.config.FillInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.ctx.Done():
			return
		case <-ticker.C:
			rl.mu.Lock()
			current := atomic.LoadUint32(&rl.tokens)
			newTokens := current + rl.config.TokensPerFill
			if newTokens > rl.config.MaxTokens {
				newTokens = rl.config.MaxTokens
			}
			atomic.StoreUint32(&rl.tokens, newTokens)
			atomic.StoreInt64(&rl.lastFillTime, time.Now().UnixNano())
			rl.mu.Unlock()
		}
	}
}

// Allow checks if a request is allowed under the rate limit
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	current := atomic.LoadUint32(&rl.tokens)
	if current > 0 {
		atomic.StoreUint32(&rl.tokens, current-1)
		atomic.AddUint64(&rl.allowedCount, 1)
		return true
	}

	atomic.AddUint64(&rl.rejectedCount, 1)
	return false
}

// Wait blocks until a request is allowed or context is cancelled
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(rl.config.FillInterval / 10):
			continue
		}
	}
}

// Stop stops the rate limiter's token refill goroutine
func (rl *RateLimiter) Stop() {
	rl.cancel()
	rl.wg.Wait()
}

// RateLimiterStats holds rate limiter statistics
type RateLimiterStats struct {
	CurrentTokens uint32
	MaxTokens     uint32
	AllowedCount  uint64
	RejectedCount uint64
}

// GetStats returns current rate limiter statistics
func (rl *RateLimiter) GetStats() RateLimiterStats {
	return RateLimiterStats{
		CurrentTokens: atomic.LoadUint32(&rl.tokens),
		MaxTokens:     rl.config.MaxTokens,
		AllowedCount:  atomic.LoadUint64(&rl.allowedCount),
		RejectedCount: atomic.LoadUint64(&rl.rejectedCount),
	}
}
