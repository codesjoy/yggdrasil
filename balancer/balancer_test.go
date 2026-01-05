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

// Package balancer implements load balancing algorithms for client requests.
package balancer

import (
	"errors"
	"testing"
)

func TestRegisterBuilder(t *testing.T) {
	testBuilder := func(name string, cli Client) (Balancer, error) {
		return nil, nil
	}

	// Register a new builder
	RegisterBuilder("test_balancer", testBuilder)

	// Verify it can be retrieved
	builder, err := GetBuilder("test_balancer")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if builder == nil {
		t.Fatal("expected builder to be non-nil")
	}
}

func TestGetBuilder_NotFound(t *testing.T) {
	_, err := GetBuilder("non_existent_balancer")
	if err == nil {
		t.Fatal("expected error for non-existent balancer")
	}
	expectedErr := "not found balancer builder, name: non_existent_balancer"
	if err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestGetBuilder_RoundRobin(t *testing.T) {
	// round_robin is registered in init()
	builder, err := GetBuilder("round_robin")
	if err != nil {
		t.Fatalf("expected no error for round_robin, got %v", err)
	}
	if builder == nil {
		t.Fatal("expected round_robin builder to be non-nil")
	}
}

func TestRegisterBuilder_Override(t *testing.T) {
	called := false
	testBuilder1 := func(name string, cli Client) (Balancer, error) {
		return nil, errors.New("builder1")
	}
	testBuilder2 := func(name string, cli Client) (Balancer, error) {
		called = true
		return nil, errors.New("builder2")
	}

	RegisterBuilder("override_test", testBuilder1)
	RegisterBuilder("override_test", testBuilder2)

	builder, err := GetBuilder("override_test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err = builder("test", nil)
	if !called {
		t.Fatal("expected second builder to be called")
	}
	if err == nil || err.Error() != "builder2" {
		t.Fatalf("expected error 'builder2', got %v", err)
	}
}
