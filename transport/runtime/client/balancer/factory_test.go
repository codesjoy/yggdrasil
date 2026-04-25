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

package balancer

import (
	"testing"
)

func TestResolveType_WithDefault(t *testing.T) {
	tests := []struct {
		name         string
		balancerName string
		setupConfig  func() error
		want         string
		wantErr      bool
	}{
		{
			name:         "default balancer with no config uses round_robin",
			balancerName: "default",
			setupConfig:  func() error { return nil },
			want:         "round_robin",
			wantErr:      false,
		},
		{
			name:         "custom balancer without config returns error",
			balancerName: "custom",
			setupConfig:  func() error { return nil },
			wantErr:      true,
		},
		{
			name:         "configured balancer returns configured type",
			balancerName: "my-balancer",
			setupConfig: func() error {
				Configure(map[string]Spec{"my-balancer": {Type: "weighted"}}, nil)
				return nil
			},
			want:    "weighted",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setupConfig(); err != nil {
				t.Fatalf("setupConfig failed: %v", err)
			}

			got, err := ResolveType(tt.balancerName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ResolveType() = %v, want %v", got, tt.want)
			}
		})
	}
}
