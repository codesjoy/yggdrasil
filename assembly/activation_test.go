// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package assembly

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/config"
	configchain "github.com/codesjoy/yggdrasil/v3/config/chain"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
)

type activationSourceModule struct {
	testCapabilityModule
	kinds []string
}

func (m activationSourceModule) ConfigSourceBuilders() map[string]configchain.ContextBuilder {
	builders := make(map[string]configchain.ContextBuilder, len(m.kinds))
	for _, kind := range m.kinds {
		builders[kind] = func(
			configchain.BuildContext,
			configchain.SourceSpec,
		) (source.Source, config.Priority, error) {
			return nil, config.PriorityDefaults, nil
		}
	}
	return builders
}

type countingCapabilityModule struct {
	name         string
	capabilities []module.Capability
	calls        *int
}

func (m *countingCapabilityModule) Name() string { return m.name }

func (m *countingCapabilityModule) Capabilities() []module.Capability {
	*m.calls++
	return append([]module.Capability(nil), m.capabilities...)
}

type activationDependentModule struct {
	testCapabilityModule
	dependencies []string
}

func (m activationDependentModule) DependsOn() []string {
	return append([]string(nil), m.dependencies...)
}

func activationCapability(
	capability string,
	provider string,
	cardinality module.CapabilityCardinality,
) module.Capability {
	return module.Capability{
		Spec: module.CapabilitySpec{
			Name:        capability,
			Cardinality: cardinality,
			Type:        reflect.TypeOf(struct{}{}),
		},
		Name:  provider,
		Value: struct{}{},
	}
}

func TestPlanExplicitReferencesActivateNonAutoModules(t *testing.T) {
	tests := []struct {
		name       string
		payload    map[string]any
		moduleName string
		capability module.Capability
	}{
		{
			name: "resolver",
			payload: map[string]any{"yggdrasil": map[string]any{
				"discovery": map[string]any{
					"resolvers": map[string]any{
						"xds": map[string]any{"type": "xds"},
					},
				},
			}},
			moduleName: "xds",
			capability: activationCapability(capResolver, "xds", module.NamedOne),
		},
		{
			name: "balancer",
			payload: map[string]any{"yggdrasil": map[string]any{
				"balancers": map[string]any{
					"defaults": map[string]any{
						"polaris": map[string]any{"type": "polaris"},
					},
				},
			}},
			moduleName: "polaris",
			capability: activationCapability(capBalancer, "polaris", module.NamedOne),
		},
		{
			name: "registry",
			payload: map[string]any{"yggdrasil": map[string]any{
				"discovery": map[string]any{
					"registry": map[string]any{"type": "etcd"},
				},
			}},
			moduleName: "etcd",
			capability: activationCapability(capRegistry, "etcd", module.NamedOne),
		},
		{
			name: "tracer",
			payload: map[string]any{"yggdrasil": map[string]any{
				"observability": map[string]any{
					"telemetry": map[string]any{"tracer": "trace-a"},
				},
			}},
			moduleName: "trace-a",
			capability: activationCapability(capTracer, "trace-a", module.NamedOne),
		},
		{
			name: "logger",
			payload: map[string]any{"yggdrasil": map[string]any{
				"observability": map[string]any{
					"logging": map[string]any{
						"handlers": map[string]any{
							"default": map[string]any{"type": "custom-log"},
						},
					},
				},
			}},
			moduleName: "custom-log",
			capability: activationCapability(capLoggerHandler, "custom-log", module.NamedOne),
		},
		{
			name: "interceptor",
			payload: map[string]any{"yggdrasil": map[string]any{
				"server": map[string]any{
					"interceptors": map[string]any{
						"unary": []any{"validator"},
					},
				},
			}},
			moduleName: "validator",
			capability: activationCapability(
				capUnaryServer,
				"validator",
				module.OrderedMany,
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := compileTestInput(t, test.payload)
			input.Modules = append(input.Modules, testCapabilityModule{
				name:         test.moduleName,
				capabilities: []module.Capability{test.capability},
			})

			result, err := Plan(context.Background(), input)
			require.NoError(t, err)
			require.True(t, specHasModule(result.Spec, test.moduleName))
			require.True(t, moduleHasReason(
				result.Spec,
				test.moduleName,
				"enabled by explicit_config",
			))
		})
	}
}

func TestPlanExplicitActivationRejectsAmbiguousOwners(t *testing.T) {
	for _, test := range []struct {
		name      string
		overrides []Override
	}{
		{name: "both candidates inactive"},
		{
			name:      "one candidate already active",
			overrides: []Override{EnableModule("xds-a")},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := compileTestInput(t, map[string]any{
				"yggdrasil": map[string]any{
					"discovery": map[string]any{
						"resolvers": map[string]any{
							"xds": map[string]any{"type": "xds"},
						},
					},
				},
			}, test.overrides...)
			for _, name := range []string{"xds-a", "xds-b"} {
				input.Modules = append(input.Modules, testCapabilityModule{
					name: name,
					capabilities: []module.Capability{
						activationCapability(capResolver, "xds", module.NamedOne),
					},
				})
			}

			_, err := Plan(context.Background(), input)
			require.Error(t, err)
			var assemblyErr *Error
			require.ErrorAs(t, err, &assemblyErr)
			require.Equal(t, ErrAmbiguousModuleActivation, assemblyErr.Code)
			require.Equal(t, "xds-a,xds-b", assemblyErr.Context["modules"])
		})
	}
}

func TestPlanExplicitActivationRejectsDisabledOwner(t *testing.T) {
	for _, test := range []struct {
		name       string
		payload    map[string]any
		overrides  []Override
		wantSource string
	}{
		{
			name: "config override",
			payload: map[string]any{"yggdrasil": map[string]any{
				"overrides": map[string]any{
					"disable_modules": []any{"xds"},
				},
			}},
			wantSource: "config_override",
		},
		{
			name:       "code override",
			payload:    map[string]any{"yggdrasil": map[string]any{}},
			overrides:  []Override{DisableModule("xds")},
			wantSource: "code_override",
		},
		{
			name: "both overrides",
			payload: map[string]any{"yggdrasil": map[string]any{
				"overrides": map[string]any{
					"disable_modules": []any{"xds"},
				},
			}},
			overrides:  []Override{DisableModule("xds")},
			wantSource: "config_override,code_override",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload := test.payload
			payload["yggdrasil"].(map[string]any)["discovery"] = map[string]any{
				"resolvers": map[string]any{
					"xds": map[string]any{"type": "xds"},
				},
			}
			input := compileTestInput(t, payload, test.overrides...)
			input.Modules = append(input.Modules, testCapabilityModule{
				name: "xds",
				capabilities: []module.Capability{
					activationCapability(capResolver, "xds", module.NamedOne),
				},
			})

			_, err := Plan(context.Background(), input)
			require.Error(t, err)
			var assemblyErr *Error
			require.ErrorAs(t, err, &assemblyErr)
			require.Equal(t, ErrConflictingOverride, assemblyErr.Code)
			require.Equal(t, "xds", assemblyErr.Context["module"])
			require.Equal(t, "discovery.resolver.provider", assemblyErr.Context["capability"])
			require.Equal(t, "xds", assemblyErr.Context["provider"])
			require.Equal(t, test.wantSource, assemblyErr.Context["source"])
			require.Equal(t, "explicit_config", assemblyErr.Context["selected_by"])
		})
	}
}

func TestPlanForceDefaultActivatesModule(t *testing.T) {
	for _, test := range []struct {
		name       string
		payload    map[string]any
		overrides  []Override
		wantSource string
	}{
		{
			name:       "code override",
			payload:    map[string]any{"yggdrasil": map[string]any{}},
			overrides:  []Override{ForceDefault(capTracer, "trace-a")},
			wantSource: "code_override",
		},
		{
			name: "config override",
			payload: map[string]any{"yggdrasil": map[string]any{
				"overrides": map[string]any{
					"force_defaults": map[string]any{
						capTracer: "trace-a",
					},
				},
			}},
			wantSource: "config_override",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := compileTestInput(t, test.payload, test.overrides...)
			input.Modules = append(input.Modules, testCapabilityModule{
				name: "trace-a",
				capabilities: []module.Capability{
					activationCapability(capTracer, "trace-a", module.NamedOne),
				},
			})

			result, err := Plan(context.Background(), input)
			require.NoError(t, err)
			require.True(t, specHasModule(result.Spec, "trace-a"))
			require.True(t, moduleHasReason(
				result.Spec,
				"trace-a",
				"enabled by "+test.wantSource,
			))
		})
	}
}

func TestPlanRejectsInvalidForcedDefaultCapability(t *testing.T) {
	for _, test := range []struct {
		name      string
		payload   map[string]any
		overrides []Override
	}{
		{
			name: "config override",
			payload: map[string]any{"yggdrasil": map[string]any{
				"overrides": map[string]any{
					"force_defaults": map[string]any{
						"unknown.capability": "provider",
					},
				},
			}},
		},
		{
			name:      "code override",
			payload:   map[string]any{"yggdrasil": map[string]any{}},
			overrides: []Override{ForceDefault("unknown.capability", "provider")},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Plan(
				context.Background(),
				compileTestInput(t, test.payload, test.overrides...),
			)
			require.Error(t, err)
			var assemblyErr *Error
			require.ErrorAs(t, err, &assemblyErr)
			require.Equal(t, ErrUnknownExplicitBinding, assemblyErr.Code)
			require.Equal(t, "unknown.capability", assemblyErr.Context["capability"])
		})
	}
}

func TestPlannerExplicitChainActivationReferences(t *testing.T) {
	input := compileTestInput(t, map[string]any{"yggdrasil": map[string]any{}})
	p := &planner{
		input:           input,
		registry:        newDefaultPlannerRegistry(),
		codeOverrides:   newOverrideSet(),
		configOverrides: newOverrideSet(),
	}
	p.codeOverrides.ForcedTemplates[chainRESTAll] = templateSelection{
		Name:    "default-rest-observable",
		Version: "v1",
	}

	references, err := p.explicitChainActivationReferences()
	require.NoError(t, err)
	require.Contains(t, references, activationReference{
		capability: capRESTMW,
		provider:   "logger",
		source:     "code_override",
	})
}

func TestPlanModeDefaultsDoNotActivateModules(t *testing.T) {
	input := compileTestInput(t, map[string]any{"yggdrasil": map[string]any{
		"mode": "prod-grpc",
	}})
	input.Modules = append(input.Modules, testCapabilityModule{
		name: "trace-otel",
		capabilities: []module.Capability{
			activationCapability(capTracer, "otel", module.NamedOne),
		},
	})

	result, err := Plan(context.Background(), input)
	require.NoError(t, err)
	require.False(t, specHasModule(result.Spec, "trace-otel"))
	require.Empty(t, result.EffectiveResolved.Telemetry.Tracer)
}

func TestPlanFrameworkDefaultsDoNotActivateModules(t *testing.T) {
	input := compileTestInput(t, map[string]any{"yggdrasil": map[string]any{}})
	input.Modules = append(input.Modules, testCapabilityModule{
		name: "console-extra",
		capabilities: []module.Capability{
			activationCapability(capLoggerWriter, "console", module.NamedOne),
		},
	})

	result, err := Plan(context.Background(), input)
	require.NoError(t, err)
	require.False(t, specHasModule(result.Spec, "console-extra"))
}

func TestPlanConfigSourceKindActivatesModuleUnlessDirect(t *testing.T) {
	for _, test := range []struct {
		name   string
		direct bool
		want   bool
	}{
		{name: "module owner", want: true},
		{name: "direct builder", direct: true, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := compileTestInput(t, map[string]any{"yggdrasil": map[string]any{}})
			input.ConfigSourceKinds = []string{"etcd-source"}
			if test.direct {
				input.DirectConfigSourceKinds = []string{"etcd-source"}
			}
			input.Modules = append(input.Modules, activationSourceModule{
				testCapabilityModule: testCapabilityModule{name: "etcd-source"},
				kinds:                []string{"etcd-source"},
			})

			result, err := Plan(context.Background(), input)
			require.NoError(t, err)
			require.Equal(t, test.want, specHasModule(result.Spec, "etcd-source"))
		})
	}
}

func TestPlanConfigSourceKindRejectsAmbiguousOrDisabledOwner(t *testing.T) {
	for _, test := range []struct {
		name       string
		payload    map[string]any
		overrides  []Override
		wantCode   ErrorCode
		wantSource string
	}{
		{
			name:       "ambiguous owners",
			payload:    map[string]any{"yggdrasil": map[string]any{}},
			wantCode:   ErrAmbiguousModuleActivation,
			wantSource: "explicit_config",
		},
		{
			name: "disabled owner",
			payload: map[string]any{"yggdrasil": map[string]any{
				"overrides": map[string]any{
					"disable_modules": []any{"source-a"},
				},
			}},
			wantCode:   ErrConflictingOverride,
			wantSource: "config_override",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := compileTestInput(t, test.payload, test.overrides...)
			input.ConfigSourceKinds = []string{"custom-source"}
			ownerCount := 2
			if test.wantCode == ErrConflictingOverride {
				ownerCount = 1
			}
			for index := 0; index < ownerCount; index++ {
				name := fmt.Sprintf("source-%c", 'a'+index)
				input.Modules = append(input.Modules, activationSourceModule{
					testCapabilityModule: testCapabilityModule{name: name},
					kinds:                []string{"custom-source"},
				})
			}

			_, err := Plan(context.Background(), input)
			require.Error(t, err)
			var assemblyErr *Error
			require.ErrorAs(t, err, &assemblyErr)
			require.Equal(t, test.wantCode, assemblyErr.Code)
			require.Equal(t, test.wantSource, assemblyErr.Context["source"])
		})
	}
}

func TestPlanExplicitActivationExpandsDependencyClosure(t *testing.T) {
	input := compileTestInput(t, map[string]any{"yggdrasil": map[string]any{
		"discovery": map[string]any{
			"resolvers": map[string]any{
				"xds": map[string]any{"type": "xds"},
			},
		},
	}})
	input.Modules = append(
		input.Modules,
		activationDependentModule{
			testCapabilityModule: testCapabilityModule{
				name: "xds",
				capabilities: []module.Capability{
					activationCapability(capResolver, "xds", module.NamedOne),
				},
			},
			dependencies: []string{"xds.dependency"},
		},
		testCapabilityModule{name: "xds.dependency"},
		testCapabilityModule{name: "not.selected"},
	)

	result, err := Plan(context.Background(), input)
	require.NoError(t, err)
	require.True(t, specHasModule(result.Spec, "xds"))
	require.True(t, specHasModule(result.Spec, "xds.dependency"))
	require.False(t, specHasModule(result.Spec, "not.selected"))
	require.True(t, moduleHasReason(result.Spec, "xds.dependency", "dependency of xds"))
}

func TestPlanActivationDiagnosticsAreDeterministic(t *testing.T) {
	payload := map[string]any{"yggdrasil": map[string]any{
		"discovery": map[string]any{
			"resolvers": map[string]any{
				"xds": map[string]any{"type": "xds"},
			},
		},
	}}
	input := compileTestInput(t, payload)
	input.Modules = append(input.Modules, testCapabilityModule{
		name: "xds",
		capabilities: []module.Capability{
			activationCapability(capResolver, "xds", module.NamedOne),
		},
	})

	first, err := Plan(context.Background(), input)
	require.NoError(t, err)
	decision := findActivationDecision(t, first.Spec, "xds")
	require.Equal(t, activationKindCapability, decision.Kind)
	require.Equal(t, "discovery.resolver.provider=xds", decision.Value)
	require.Equal(t, "explicit_config", decision.Source)
	require.True(t, moduleHasReason(
		first.Spec,
		"xds",
		"enabled by explicit_config",
	))

	for index := 0; index < 20; index++ {
		next, err := Plan(context.Background(), input)
		require.NoError(t, err)
		require.Equal(t, first.Hash, next.Hash)
	}
}

func TestPlanCapabilitiesIndexedOnce(t *testing.T) {
	input := compileTestInput(t, map[string]any{"yggdrasil": map[string]any{
		"discovery": map[string]any{
			"resolvers": map[string]any{
				"xds": map[string]any{"type": "xds"},
			},
		},
	}})
	calls := 0
	input.Modules = append(input.Modules, &countingCapabilityModule{
		name: "xds",
		capabilities: []module.Capability{
			activationCapability(capResolver, "xds", module.NamedOne),
		},
		calls: &calls,
	})

	_, err := Plan(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func findActivationDecision(t *testing.T, spec *Spec, moduleName string) Decision {
	t.Helper()
	for _, decision := range spec.Decisions {
		if decision.Kind == activationKindCapability && decision.Target == moduleName {
			return decision
		}
	}
	t.Fatalf("activation decision for module %q not found", moduleName)
	return Decision{}
}

func moduleHasReason(spec *Spec, name string, prefix string) bool {
	for _, item := range spec.Modules {
		if item.Name != name {
			continue
		}
		for _, reason := range item.Reasons {
			if strings.HasPrefix(reason, prefix) {
				return true
			}
		}
	}
	return false
}

func BenchmarkPlanExplicitActivation(b *testing.B) {
	const moduleCount = 100
	payload := map[string]any{"yggdrasil": map[string]any{
		"discovery": map[string]any{
			"resolvers": map[string]any{
				"selected": map[string]any{"type": "resolver-050"},
			},
		},
	}}
	var root settings.Root
	if err := config.NewSnapshot(payload).Decode(&root); err != nil {
		b.Fatal(err)
	}
	resolved, err := settings.Compile(root)
	if err != nil {
		b.Fatal(err)
	}
	modules := testPlannerModules()
	for index := 0; index < moduleCount; index++ {
		provider := fmt.Sprintf("resolver-%03d", index)
		modules = append(modules, testCapabilityModule{
			name: "resolver.module." + provider,
			capabilities: []module.Capability{
				activationCapability(capResolver, provider, module.NamedOne),
			},
		})
	}
	input := Input{
		Identity:  IdentitySpec{AppName: "assembly-benchmark"},
		Resolved:  resolved,
		Snapshot:  config.NewSnapshot(payload),
		Modules:   modules,
		Overrides: []Override{EnableModule("test.foundation")},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := Plan(context.Background(), input); err != nil {
			b.Fatal(err)
		}
	}
}
