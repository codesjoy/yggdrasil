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

package assembly

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/module"
)

func TestRequireProvider_NotAvailable(t *testing.T) {
	// Use capRegistry which has only "multi_registry" as a provider.
	// ForceDefault with a nonexistent provider triggers requireProvider.
	input := compileTestInput(t, map[string]any{
		"yggdrasil": map[string]any{},
	}, ForceDefault(capRegistry, "nonexistent_registry"))
	_, err := Plan(context.Background(), input)
	require.Error(t, err)
	var assemblyErr *Error
	require.ErrorAs(t, err, &assemblyErr)
	require.Equal(t, ErrUnknownExplicitBinding, assemblyErr.Code)
}

func TestRequireProvider_AmbiguousMultiple(t *testing.T) {
	input := compileTestInput(t, map[string]any{
		"yggdrasil": map[string]any{},
	}, ForceDefault(capTracer, "nonexistent_provider"))
	// Add two providers for capTracer so requireProvider takes the ambiguous path
	input.Modules = append(input.Modules, testCapabilityModule{
		name: "tracer.a",
		capabilities: []module.Capability{
			{
				Spec: module.CapabilitySpec{
					Name:        capTracer,
					Cardinality: module.NamedOne,
					Type:        reflect.TypeOf(struct{}{}),
				},
				Name:  "trace-a",
				Value: struct{}{},
			},
		},
	}, testCapabilityModule{
		name: "tracer.b",
		capabilities: []module.Capability{
			{
				Spec: module.CapabilitySpec{
					Name:        capTracer,
					Cardinality: module.NamedOne,
					Type:        reflect.TypeOf(struct{}{}),
				},
				Name:  "trace-b",
				Value: struct{}{},
			},
		},
	})

	_, err := Plan(context.Background(), input)
	require.Error(t, err)
	var assemblyErr *Error
	require.ErrorAs(t, err, &assemblyErr)
	require.Equal(t, ErrAmbiguousDefault, assemblyErr.Code)
}

func TestSelectChain_DisableAuto(t *testing.T) {
	input := compileTestInput(t, map[string]any{
		"yggdrasil": map[string]any{
			"mode": "dev",
			"server": map[string]any{
				"transports": []any{"grpc"},
			},
			"transports": map[string]any{
				"http": map[string]any{
					"rest": map[string]any{
						"host": "127.0.0.1",
						"port": 0,
					},
				},
			},
		},
	}, DisableAuto(chainUnaryServer))

	result, err := Plan(context.Background(), input)
	require.NoError(t, err)
	// With DisableAuto on the chain path, mode template should not apply
	chain, ok := result.Spec.Chains[chainUnaryServer]
	if ok {
		require.Empty(t, chain.Items)
	}
}

func TestExpandTemplate_VersionNotFound(t *testing.T) {
	// Use config-based template override with a template name but no version.
	// ForceTemplate(path, name, "") is a no-op in the override set.
	// Instead use explicit config that specifies a template without version.
	input := compileTestInput(t, map[string]any{
		"yggdrasil": map[string]any{
			"server": map[string]any{
				"transports": []any{"grpc"},
			},
			"transports": map[string]any{
				"http": map[string]any{
					"rest": map[string]any{
						"host": "127.0.0.1",
						"port": 0,
					},
				},
			},
			"extensions": map[string]any{
				"interceptors": map[string]any{
					"unary_server": "default-observable",
				},
			},
		},
	})
	_, err := Plan(context.Background(), input)
	require.Error(t, err)
	var assemblyErr *Error
	require.ErrorAs(t, err, &assemblyErr)
	require.Equal(t, ErrTemplateVersionNotFound, assemblyErr.Code)
}

func TestPlan_EnableModuleOverride(t *testing.T) {
	input := compileTestInput(t, map[string]any{
		"yggdrasil": map[string]any{
			"observability": map[string]any{
				"telemetry": map[string]any{
					"stats": map[string]any{
						"server": "otel",
					},
				},
			},
		},
	})
	enabledModule := testAutoModule{
		testCapabilityModule: testCapabilityModule{
			name: "auto.trace",
			capabilities: []module.Capability{
				{
					Spec: module.CapabilitySpec{
						Name:        capTracer,
						Cardinality: module.NamedOne,
						Type:        reflect.TypeOf(struct{}{}),
					},
					Name:  "auto-tracer",
					Value: struct{}{},
				},
			},
		},
		autoSpec: module.AutoSpec{
			Provides: []module.CapabilitySpec{
				{Name: capTracer, Cardinality: module.NamedOne, Type: reflect.TypeOf(struct{}{})},
			},
			AutoRules:     []module.AutoRule{}, // no rules -> won't match on its own
			DefaultPolicy: &module.DefaultPolicy{Score: 5},
		},
	}
	input.Modules = append(input.Modules, enabledModule)

	// Without EnableModule, the auto module has no matching rules so it won't be enabled.
	result, err := Plan(context.Background(), input)
	require.NoError(t, err)
	found := false
	for _, m := range result.Spec.Modules {
		if m.Name == "auto.trace" {
			found = true
		}
	}
	require.False(t, found, "auto module without matching rules should not be enabled")

	// With EnableModule override, it should be force-enabled.
	input2 := compileTestInput(t, map[string]any{
		"yggdrasil": map[string]any{
			"observability": map[string]any{
				"telemetry": map[string]any{
					"stats": map[string]any{
						"server": "otel",
					},
				},
			},
		},
	}, EnableModule("auto.trace"))
	input2.Modules = append(input2.Modules, enabledModule)

	result2, err := Plan(context.Background(), input2)
	require.NoError(t, err)
	found2 := false
	for _, m := range result2.Spec.Modules {
		if m.Name == "auto.trace" {
			found2 = true
		}
	}
	require.True(t, found2, "EnableModule should force-include the auto module")
}

func TestPlan_ProdGRPCMode(t *testing.T) {
	input := compileTestInput(t, map[string]any{
		"yggdrasil": map[string]any{
			"mode": "prod-grpc",
			"server": map[string]any{
				"transports": []any{"grpc"},
			},
			"transports": map[string]any{
				"grpc": map[string]any{
					"server": map[string]any{},
					"client": map[string]any{},
				},
			},
		},
	})
	result, err := Plan(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "prod-grpc", result.Spec.Mode.Name)
	require.Equal(t, "json", result.Spec.Defaults[capLoggerHandler])
	require.Equal(t, "console", result.Spec.Defaults[capLoggerWriter])

	// Verify server chains are resolved
	chain, ok := result.Spec.Chains[chainUnaryServer]
	require.True(t, ok)
	require.Equal(t, "default-observable", chain.Template)

	// Verify client chains are resolved
	clientChain, ok := result.Spec.Chains[chainUnaryClient]
	require.True(t, ok)
	require.Equal(t, "default-client-safe", clientChain.Template)
}

func TestPlan_DevMode(t *testing.T) {
	input := compileTestInput(t, map[string]any{
		"yggdrasil": map[string]any{
			"mode": "dev",
			"server": map[string]any{
				"transports": []any{"grpc"},
			},
			"transports": map[string]any{
				"http": map[string]any{
					"rest": map[string]any{
						"host": "127.0.0.1",
						"port": 0,
					},
				},
			},
		},
	})
	result, err := Plan(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "dev", result.Spec.Mode.Name)
	require.Equal(t, "text", result.Spec.Defaults[capLoggerHandler])
	require.Equal(t, "console", result.Spec.Defaults[capLoggerWriter])
}
