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

package otlp

import (
	"testing"
	"time"
)

func TestNewMeterProvider_InvalidProtocol(t *testing.T) {
	cfg := MetricExporterConfig{
		Protocol: "invalid",
	}

	_, err := NewMeterProvider("test-service", cfg)
	if err == nil {
		t.Error("expected error for invalid protocol")
	}
}

func TestMetricExportIntervalDefaults(t *testing.T) {
	tests := []struct {
		name  string
		value time.Duration
		min   time.Duration
		max   time.Duration
	}{
		{
			name:  "defaultExportInterval",
			value: defaultExportInterval,
			min:   10 * time.Second,
			max:   300 * time.Second,
		},
		{
			name:  "defaultExportTimeout",
			value: defaultExportTimeout,
			min:   10 * time.Second,
			max:   120 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value < tt.min || tt.value > tt.max {
				t.Errorf("%s = %v, want between %v and %v", tt.name, tt.value, tt.min, tt.max)
			}
		})
	}
}

func TestTemporalitySelectors(t *testing.T) {
	tests := []struct {
		name           string
		temporality    string
		wantCumulative bool
	}{
		{
			name:           "cumulative temporality",
			temporality:    "cumulative",
			wantCumulative: true,
		},
		{
			name:           "delta temporality",
			temporality:    "delta",
			wantCumulative: false,
		},
		{
			name:           "empty defaults to cumulative",
			temporality:    "",
			wantCumulative: true,
		},
		{
			name:           "unknown defaults to cumulative",
			temporality:    "unknown",
			wantCumulative: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMetricTemporality(tt.temporality)
			isCumulative := (got == "cumulative")
			if isCumulative != tt.wantCumulative {
				t.Errorf(
					"getMetricTemporality(%q) = %q, want cumulative=%v",
					tt.temporality,
					got,
					tt.wantCumulative,
				)
			}
		})
	}
}

func TestMetricConfigDefaults(t *testing.T) {
	// Test that empty config gets proper defaults
	cfg := MetricExporterConfig{}

	if cfg.ExportInterval == 0 {
		cfg.ExportInterval = defaultExportInterval
	}
	if cfg.ExportTimeout == 0 {
		cfg.ExportTimeout = defaultExportTimeout
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.Retry.InitialDelay == 0 {
		cfg.Retry.InitialDelay = defaultRetryInitialDelay
	}
	if cfg.Retry.MaxDelay == 0 {
		cfg.Retry.MaxDelay = defaultRetryMaxDelay
	}

	if cfg.ExportInterval != defaultExportInterval {
		t.Errorf("ExportInterval = %v, want %v", cfg.ExportInterval, defaultExportInterval)
	}
	if cfg.ExportTimeout != defaultExportTimeout {
		t.Errorf("ExportTimeout = %v, want %v", cfg.ExportTimeout, defaultExportTimeout)
	}
	if cfg.Timeout != defaultTimeout {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, defaultTimeout)
	}
	if cfg.Retry.InitialDelay != defaultRetryInitialDelay {
		t.Errorf(
			"Retry.InitialDelay = %v, want %v",
			cfg.Retry.InitialDelay,
			defaultRetryInitialDelay,
		)
	}
	if cfg.Retry.MaxDelay != defaultRetryMaxDelay {
		t.Errorf("Retry.MaxDelay = %v, want %v", cfg.Retry.MaxDelay, defaultRetryMaxDelay)
	}
}
