package k8s

import (
	"math"
	"math/rand"
	"time"
)

type backoff struct {
	cfg backoffConfig
}

func newBackoff(cfg backoffConfig) *backoff {
	return &backoff{cfg: cfg}
}

func (b *backoff) Backoff(retry int) time.Duration {
	if retry == 0 {
		return b.cfg.BaseDelay
	}
	delay := float64(b.cfg.BaseDelay) * math.Pow(b.cfg.Multiplier, float64(retry))
	if b.cfg.Jitter > 0 {
		delay = delay * (1.0 + b.cfg.Jitter*(2*rand.Float64()-1.0))
	}
	d := time.Duration(delay)
	if d > b.cfg.MaxDelay {
		return b.cfg.MaxDelay
	}
	return d
}
