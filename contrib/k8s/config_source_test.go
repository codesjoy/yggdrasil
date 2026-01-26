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

package k8s

import (
	"testing"
)

func TestInferParser(t *testing.T) {
	tests := []struct {
		key     string
		wantNil bool
	}{
		{"config.yaml", false},
		{"config.yml", false},
		{"config.json", false},
		{"config.toml", false},
		{"config.txt", false},
	}
	for _, tt := range tests {
		p := inferParser(tt.key)
		if tt.wantNil && p == nil {
			t.Fatalf("expected non-nil parser for %s", tt.key)
		}
	}
}

func TestInferKeyFromData(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want string
	}{
		{
			name: "yaml key",
			data: map[string]any{
				"app.yaml": "foo: bar",
			},
			want: "app.yaml",
		},
		{
			name: "json key",
			data: map[string]any{
				"app.json": `{"foo":"bar"}`,
			},
			want: "app.json",
		},
		{
			name: "first key",
			data: map[string]any{
				"foo": "bar",
			},
			want: "foo",
		},
		{
			name: "empty",
			data: map[string]any{},
			want: "config",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferKeyFromData(tt.data)
			if got != tt.want {
				t.Fatalf("expected key %s, got %s", tt.want, got)
			}
		})
	}
}

func TestSourceConstruction(t *testing.T) {
	t.Run("ConfigMapSource", func(t *testing.T) {
		src, err := NewConfigMapSource(ConfigSourceConfig{Name: "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if src.Type() != "configmap" {
			t.Fatalf("expected type configmap, got %s", src.Type())
		}
		if src.Name() != "test" {
			t.Fatalf("expected name test, got %s", src.Name())
		}
	})

	t.Run("ConfigMapSource empty name", func(t *testing.T) {
		_, err := NewConfigMapSource(ConfigSourceConfig{})
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})

	t.Run("SecretSource", func(t *testing.T) {
		src, err := NewSecretSource(ConfigSourceConfig{Name: "secret"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if src.Type() != "secret" {
			t.Fatalf("expected type secret, got %s", src.Type())
		}
	})
}
