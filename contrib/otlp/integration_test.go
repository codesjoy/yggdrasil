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

package otlp_test

import (
	"testing"

	_ "github.com/codesjoy/yggdrasil/contrib/otlp/v2"
	xotel "github.com/codesjoy/yggdrasil/v2/otel"
)

// TestBuilderRegistration verifies that all OTLP builders are properly registered.
func TestBuilderRegistration(t *testing.T) {
	tests := []struct {
		name    string
		builder string
		isTrace bool
	}{
		{
			name:    "otlp-grpc trace builder",
			builder: "otlp-grpc",
			isTrace: true,
		},
		{
			name:    "otlp-http trace builder",
			builder: "otlp-http",
			isTrace: true,
		},
		{
			name:    "otlp-grpc meter builder",
			builder: "otlp-grpc",
			isTrace: false,
		},
		{
			name:    "otlp-http meter builder",
			builder: "otlp-http",
			isTrace: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isTrace {
				_, ok := xotel.GetTracerProviderBuilder(tt.builder)
				if !ok {
					t.Errorf("trace builder %q not registered", tt.builder)
				}
			} else {
				_, ok := xotel.GetMeterProviderBuilder(tt.builder)
				if !ok {
					t.Errorf("meter builder %q not registered", tt.builder)
				}
			}
		})
	}
}
