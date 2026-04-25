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

func TestLoadConfig_DefaultsOnly(t *testing.T) {
	Configure(map[string]Spec{
		"mybal": {Type: "round_robin", Config: map[string]any{"key1": "val1"}},
	}, nil)

	cfg := LoadConfig("svc", "mybal")
	if cfg["key1"] != "val1" {
		t.Fatalf("expected key1=val1, got %v", cfg["key1"])
	}

	// Restore
	Configure(nil, nil)
}

func TestLoadConfig_ServiceOverrideMerges(t *testing.T) {
	Configure(
		map[string]Spec{
			"mybal": {Type: "round_robin", Config: map[string]any{"key1": "val1"}},
		},
		map[string]map[string]Spec{
			"svc": {
				"mybal": {Config: map[string]any{"key2": "val2"}},
			},
		},
	)

	cfg := LoadConfig("svc", "mybal")
	if cfg["key1"] != "val1" {
		t.Fatalf("expected key1=val1, got %v", cfg["key1"])
	}
	if cfg["key2"] != "val2" {
		t.Fatalf("expected key2=val2, got %v", cfg["key2"])
	}

	// Restore
	Configure(nil, nil)
}

func TestLoadConfig_NoConfig(t *testing.T) {
	Configure(nil, nil)
	cfg := LoadConfig("svc", "mybal")
	if len(cfg) != 0 {
		t.Fatalf("expected empty config, got %v", cfg)
	}
}

func TestNew_DefaultBalancer(t *testing.T) {
	cli := newMockBalancerClient()
	b, err := New("svc", DefaultBalancerName, cli)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil balancer")
	}
	if b.Type() != "round_robin" {
		t.Fatalf("expected round_robin, got %q", b.Type())
	}
}

func TestNew_UnknownType(t *testing.T) {
	cli := newMockBalancerClient()
	// Configure an unknown balancer name that has no configured type
	Configure(nil, nil)
	_, err := New("svc", "unknown_balancer", cli)
	if err == nil {
		t.Fatal("expected error for unknown balancer name")
	}
}

func TestNew_NoProviderForType(t *testing.T) {
	// Configure a balancer name that maps to a type with no provider
	Configure(map[string]Spec{
		"mybal": {Type: "nonexistent_type"},
	}, nil)
	defer Configure(nil, nil)

	cli := newMockBalancerClient()
	_, err := New("svc", "mybal", cli)
	if err == nil {
		t.Fatal("expected error when provider not found for type")
	}
}
