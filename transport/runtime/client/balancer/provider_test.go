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
	"errors"
	"testing"
)

func TestConfigureProviders_Valid(t *testing.T) {
	p := NewProvider(
		"custom",
		func(serviceName, balancerName string, cli Client) (Balancer, error) {
			return nil, nil
		},
	)

	err := ConfigureProviders([]Provider{p})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, ok := GetProvider("custom")
	if !ok {
		t.Fatal("expected provider to be found")
	}
	if got.Type() != "custom" {
		t.Fatalf("expected type 'custom', got %q", got.Type())
	}

	// Restore default providers
	ConfigureProviders([]Provider{BuiltinProvider()})
}

func TestConfigureProviders_DuplicateType(t *testing.T) {
	p1 := NewProvider("dup", func(serviceName, balancerName string, cli Client) (Balancer, error) {
		return nil, nil
	})
	p2 := NewProvider("dup", func(serviceName, balancerName string, cli Client) (Balancer, error) {
		return nil, nil
	})

	err := ConfigureProviders([]Provider{p1, p2})
	if err == nil {
		t.Fatal("expected error for duplicate type")
	}

	// Restore
	ConfigureProviders([]Provider{BuiltinProvider()})
}

func TestConfigureProviders_EmptyType(t *testing.T) {
	p := NewProvider("", func(serviceName, balancerName string, cli Client) (Balancer, error) {
		return nil, nil
	})

	err := ConfigureProviders([]Provider{p})
	if err == nil {
		t.Fatal("expected error for empty type")
	}

	// Restore
	ConfigureProviders([]Provider{BuiltinProvider()})
}

func TestConfigureProviders_NilSkipped(t *testing.T) {
	err := ConfigureProviders([]Provider{nil, BuiltinProvider()})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, ok := GetProvider("round_robin")
	if !ok {
		t.Fatal("expected round_robin to still be available")
	}
	if got == nil {
		t.Fatal("expected non-nil provider")
	}

	// Restore
	ConfigureProviders([]Provider{BuiltinProvider()})
}

func TestGetProvider_Found(t *testing.T) {
	got, ok := GetProvider("round_robin")
	if !ok {
		t.Fatal("expected round_robin to be found")
	}
	if got.Type() != "round_robin" {
		t.Fatalf("expected type 'round_robin', got %q", got.Type())
	}
}

func TestGetProvider_NotFound(t *testing.T) {
	_, ok := GetProvider("nonexistent_type")
	if ok {
		t.Fatal("expected not found for unknown type")
	}
}

func TestProvider_Type(t *testing.T) {
	p := NewProvider("mytype", nil)
	if p.Type() != "mytype" {
		t.Fatalf("expected 'mytype', got %q", p.Type())
	}
}

func TestProvider_New(t *testing.T) {
	called := false
	p := NewProvider(
		"test_new",
		func(serviceName, balancerName string, cli Client) (Balancer, error) {
			called = true
			return nil, errors.New("test error")
		},
	)
	_, err := p.New("svc", "bal", nil)
	if !called {
		t.Fatal("expected builder to be called")
	}
	if err == nil || err.Error() != "test error" {
		t.Fatalf("expected test error, got %v", err)
	}
}
