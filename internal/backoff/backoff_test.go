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

package backoff

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExponentialBackoff_RetriesZeroReturnsBaseDelay(t *testing.T) {
	exp := Exponential{
		Config: Config{
			BaseDelay:  250 * time.Millisecond,
			Multiplier: 2,
			Jitter:     0,
			MaxDelay:   time.Second,
		},
	}

	require.Equal(t, 250*time.Millisecond, exp.Backoff(0))
}

func TestExponentialBackoff_GrowsAndCapsAtMaxDelay(t *testing.T) {
	exp := Exponential{
		Config: Config{
			BaseDelay:  100 * time.Millisecond,
			Multiplier: 2,
			Jitter:     0,
			MaxDelay:   450 * time.Millisecond,
		},
	}

	require.Equal(t, 200*time.Millisecond, exp.Backoff(1))
	require.Equal(t, 450*time.Millisecond, exp.Backoff(3))
}

func TestExponentialBackoff_NegativeDurationFallsBackToZero(t *testing.T) {
	exp := Exponential{
		Config: Config{
			BaseDelay:  -time.Second,
			Multiplier: 1,
			Jitter:     0,
			MaxDelay:   time.Second,
		},
	}

	require.Zero(t, exp.Backoff(1))
}
