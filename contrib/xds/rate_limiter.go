package xds

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type RateLimiterConfig struct {
	MaxTokens     uint32
	TokensPerFill uint32
	FillInterval  time.Duration
}

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

func (rl *RateLimiter) Stop() {
	rl.cancel()
	rl.wg.Wait()
}

type RateLimiterStats struct {
	CurrentTokens uint32
	MaxTokens     uint32
	AllowedCount  uint64
	RejectedCount uint64
}

func (rl *RateLimiter) GetStats() RateLimiterStats {
	return RateLimiterStats{
		CurrentTokens: atomic.LoadUint32(&rl.tokens),
		MaxTokens:     rl.config.MaxTokens,
		AllowedCount:  atomic.LoadUint64(&rl.allowedCount),
		RejectedCount: atomic.LoadUint64(&rl.rejectedCount),
	}
}
