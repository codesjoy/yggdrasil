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

// Package backoff provides strategies for retrying operations with exponential backoff.
package backoff

import (
	"math/rand/v2"
	"time"
)

// Strategy defines the methodology for backing off after a otlpgrpc connection
// failure.
type Strategy interface {
	// Backoff returns the amount of time to wait before the next retry given
	// the number of consecutive failures.
	Backoff(retries int) time.Duration
}

// Config defines the configuration options for backoff.
type Config struct {
	// BaseDelay is the amount of time to backoff after the first failure.
	BaseDelay time.Duration `mapstructure:"baseDelay"  default:"1s"`
	// Multiplier is the factor with which to multiply backoffs after a
	// failed retry. Should ideally be greater than 1.
	Multiplier float64 `mapstructure:"multiplier" default:"1.6"`
	// Jitter is the factor with which backoffs are randomized.
	Jitter float64 `mapstructure:"jitter"     default:"0.2"`
	// MaxDelay is the upper bound of backoff delay.
	MaxDelay time.Duration `mapstructure:"maxDelay"   default:"2m"`
}

// Exponential implements an exponential backoff strategy.
type Exponential struct {
	// Config contains all options to configure the backoff algorithm.
	Config Config
}

// Backoff returns the amount of time to wait before the next retry given the
// number of retries.
func (bc Exponential) Backoff(retries int) time.Duration {
	if retries == 0 {
		return bc.Config.BaseDelay
	}
	backoff, maxDelay := float64(bc.Config.BaseDelay), float64(bc.Config.MaxDelay)
	for backoff < maxDelay && retries > 0 {
		backoff *= bc.Config.Multiplier
		retries--
	}
	if backoff > maxDelay {
		backoff = maxDelay
	}
	// Randomize backoff delays so that if a cluster of requests start at
	// the same time, they won't operate in lockstep.
	// #nosec G404 - The random number here is only used for backoff jitter and does not require cryptographic security
	backoff *= 1 + bc.Config.Jitter*(rand.Float64()*2-1)
	if backoff < 0 {
		return 0
	}
	return time.Duration(backoff)
}
