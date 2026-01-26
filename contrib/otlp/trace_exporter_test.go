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

func TestBuildResourceAttributes(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		customAttrs map[string]interface{}
		wantAttrs   map[string]any
	}{
		{
			name:        "service name only",
			serviceName: "test-service",
			customAttrs: nil,
			wantAttrs: map[string]any{
				"service.name": "test-service",
			},
		},
		{
			name:        "with custom attributes",
			serviceName: "test-service",
			customAttrs: map[string]interface{}{
				"deployment.environment": "production",
				"service.version":        "1.0.0",
			},
			wantAttrs: map[string]any{
				"service.name":           "test-service",
				"deployment.environment": "production",
				"service.version":        "1.0.0",
			},
		},
		{
			name:        "service name in custom attrs",
			serviceName: "test-service",
			customAttrs: map[string]interface{}{
				"service.name": "override-service",
			},
			wantAttrs: map[string]any{
				"service.name": "override-service",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildResourceAttributes(tt.serviceName, tt.customAttrs)

			// Check service name
			if got["service.name"] != tt.wantAttrs["service.name"] {
				t.Errorf(
					"service.name = %v, want %v",
					got["service.name"],
					tt.wantAttrs["service.name"],
				)
			}

			// Check all attributes
			for k, wantVal := range tt.wantAttrs {
				if gotVal, ok := got[k]; !ok {
					t.Errorf("missing attribute %s", k)
				} else if gotVal != wantVal {
					t.Errorf("attribute %s = %v, want %v", k, gotVal, wantVal)
				}
			}
		})
	}
}

func TestNewTracerProvider_InvalidProtocol(t *testing.T) {
	cfg := TraceExporterConfig{
		Protocol: "invalid",
	}

	_, err := NewTracerProvider("test-service", cfg)
	if err == nil {
		t.Error("expected error for invalid protocol")
	}
}

func TestDefaultConstants(t *testing.T) {
	tests := []struct {
		name  string
		value time.Duration
		min   time.Duration
		max   time.Duration
	}{
		{
			name:  "defaultBatchTimeout",
			value: defaultBatchTimeout,
			min:   1 * time.Second,
			max:   60 * time.Second,
		},
		{
			name:  "defaultTimeout",
			value: defaultTimeout,
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

func TestGetMetricTemporality(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "delta",
			in:   "delta",
			want: "delta",
		},
		{
			name: "cumulative",
			in:   "cumulative",
			want: "cumulative",
		},
		{
			name: "empty defaults to cumulative",
			in:   "",
			want: "cumulative",
		},
		{
			name: "unknown defaults to cumulative",
			in:   "unknown",
			want: "cumulative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMetricTemporality(tt.in)
			if got != tt.want {
				t.Errorf("getMetricTemporality(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
