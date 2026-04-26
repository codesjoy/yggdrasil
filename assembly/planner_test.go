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

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
)

type testCapabilityModule struct {
	name         string
	capabilities []module.Capability
}

func (m testCapabilityModule) Name() string { return m.name }

func (m testCapabilityModule) Capabilities() []module.Capability {
	return append([]module.Capability(nil), m.capabilities...)
}

type testAutoRule struct {
	path        string
	description string
}

func (r testAutoRule) Match(ctx module.AutoRuleContext) bool {
	return !ctx.Snapshot.Section(splitDotPath(r.path)...).Empty()
}

func (r testAutoRule) Describe() string {
	return r.description
}

func (r testAutoRule) AffectedPaths() []string {
	return []string{r.path}
}

type testAutoModule struct {
	testCapabilityModule
	autoSpec module.AutoSpec
}

func (m testAutoModule) AutoSpec() module.AutoSpec {
	return m.autoSpec
}

func testPlannerModules() []module.Module {
	buildNamed := func(spec, name string, cardinality module.CapabilityCardinality) module.Capability {
		return module.Capability{
			Spec: module.CapabilitySpec{
				Name:        spec,
				Cardinality: cardinality,
				Type:        reflect.TypeOf(struct{}{}),
			},
			Name:  name,
			Value: struct{}{},
		}
	}
	return []module.Module{
		testCapabilityModule{
			name: "test.foundation",
			capabilities: []module.Capability{
				buildNamed(capLoggerHandler, "text", module.NamedOne),
				buildNamed(capLoggerHandler, "json", module.NamedOne),
				buildNamed(capLoggerWriter, "console", module.NamedOne),
				buildNamed(capLoggerWriter, "file", module.NamedOne),
				buildNamed(capStatsHandler, "otel", module.NamedOne),
				buildNamed(capRegistry, "multi_registry", module.NamedOne),
				buildNamed(capServerTrans, "grpc", module.NamedOne),
				buildNamed(capServerTrans, "http", module.NamedOne),
				buildNamed(capClientTrans, "grpc", module.NamedOne),
				buildNamed(capClientTrans, "http", module.NamedOne),
				buildNamed(capUnaryServer, "logging", module.OrderedMany),
				buildNamed(capStreamServer, "logging", module.OrderedMany),
				buildNamed(capUnaryClient, "logging", module.OrderedMany),
				buildNamed(capStreamClient, "logging", module.OrderedMany),
				buildNamed(capRESTMW, "logger", module.OrderedMany),
				buildNamed(capRESTMW, "marshaler", module.OrderedMany),
				buildNamed(capMarshaler, "jsonpb", module.NamedOne),
				buildNamed(capMarshaler, "proto", module.NamedOne),
				buildNamed(capBalancer, "round_robin", module.NamedOne),
			},
		},
	}
}

func compileTestInput(t *testing.T, payload map[string]any, overrides ...Override) Input {
	t.Helper()
	var root settings.Root
	require.NoError(t, config.NewSnapshot(payload).Decode(&root))
	resolved, err := settings.Compile(root)
	require.NoError(t, err)
	return Input{
		Identity:  IdentitySpec{AppName: "assembly-test"},
		Resolved:  resolved,
		Snapshot:  config.NewSnapshot(payload),
		Modules:   testPlannerModules(),
		Overrides: overrides,
	}
}

func TestDryRunAppliesModeDefaultsAndTemplates(t *testing.T) {
	input := compileTestInput(t, map[string]any{
		"yggdrasil": map[string]any{
			"mode": "prod-http-gateway",
			"server": map[string]any{
				"transports": []any{"grpc"},
			},
			"transports": map[string]any{
				"grpc": map[string]any{
					"server": map[string]any{},
					"client": map[string]any{},
				},
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
	require.Equal(t, "prod-http-gateway", result.Spec.Mode.Name)
	require.Equal(t, "json", result.Spec.Defaults[capLoggerHandler])
	require.Equal(t, "console", result.Spec.Defaults[capLoggerWriter])
	require.Equal(t, "multi_registry", result.Spec.Defaults[capRegistry])
	require.Equal(t, "logging", result.Spec.Chains[chainUnaryServer].Items[0])
	require.Equal(t, "default-observable", result.Spec.Chains[chainUnaryServer].Template)
	require.Equal(t, "logger", result.Spec.Chains[chainRESTAll].Items[0])
	require.Equal(t, []string{"logging"}, result.EffectiveResolved.Server.Interceptors.Unary)
	require.Equal(t, []string{"logger"}, result.EffectiveResolved.Transports.Rest.Middleware.All)
	require.Equal(t, []string{"multi_registry"}, result.CapabilityBindings[capRegistry])
	require.NotEmpty(t, result.Hash)
}

func TestDryRunHonorsOverridesAndDisableAuto(t *testing.T) {
	input := compileTestInput(t, map[string]any{
		"yggdrasil": map[string]any{
			"mode": "prod-http-gateway",
			"overrides": map[string]any{
				"force_defaults": map[string]any{
					capLoggerHandler: "text",
				},
				"force_templates": map[string]any{
					chainRESTAll: "default-rest-observable@v1",
				},
				"disable_auto": []any{chainUnaryServer},
			},
			"server": map[string]any{
				"transports": []any{"grpc"},
			},
			"transports": map[string]any{
				"grpc": map[string]any{
					"server": map[string]any{},
					"client": map[string]any{},
				},
				"http": map[string]any{
					"rest": map[string]any{
						"host": "127.0.0.1",
						"port": 0,
					},
				},
			},
		},
	}, ForceDefault(capRegistry, "multi_registry"))

	result, err := Plan(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "text", result.Spec.Defaults[capLoggerHandler])
	require.Equal(t, "multi_registry", result.Spec.Defaults[capRegistry])
	require.Empty(t, result.Spec.Chains[chainUnaryServer].Items)
	require.Equal(t, "logger", result.Spec.Chains[chainRESTAll].Items[0])
	require.Equal(t, "config_override", result.DefaultSources[capLoggerHandler])
	require.Equal(t, "code_override", result.DefaultSources[capRegistry])
}

func TestDryRunRejectsUnknownModeAndTemplate(t *testing.T) {
	_, err := Plan(context.Background(), compileTestInput(t, map[string]any{
		"yggdrasil": map[string]any{
			"mode": "unknown-mode",
		},
	}))
	var assemblyErr *Error
	require.ErrorAs(t, err, &assemblyErr)
	require.Equal(t, ErrInvalidMode, assemblyErr.Code)

	_, err = Plan(context.Background(), compileTestInput(t, map[string]any{
		"yggdrasil": map[string]any{},
	}, ForceTemplate(chainUnaryServer, "missing-template", "v1")))
	require.ErrorAs(t, err, &assemblyErr)
	require.Equal(t, ErrUnknownTemplate, assemblyErr.Code)
}

func TestDryRunRejectsAmbiguousDefault(t *testing.T) {
	input := compileTestInput(t, map[string]any{
		"yggdrasil": map[string]any{},
	})
	input.Modules = append(input.Modules, testCapabilityModule{
		name: "tracer.providers",
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
	var assemblyErr *Error
	require.ErrorAs(t, err, &assemblyErr)
	require.Equal(t, ErrAmbiguousDefault, assemblyErr.Code)
}

func TestDryRunSelectsAutoModulesAndDefaultPolicyDeterministically(t *testing.T) {
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
	input.Modules = append(input.Modules,
		testAutoModule{
			testCapabilityModule: testCapabilityModule{
				name: "trace.high",
				capabilities: []module.Capability{
					{
						Spec: module.CapabilitySpec{
							Name:        capTracer,
							Cardinality: module.NamedOne,
							Type:        reflect.TypeOf(struct{}{}),
						},
						Name:  "trace-high",
						Value: struct{}{},
					},
				},
			},
			autoSpec: module.AutoSpec{
				Provides: []module.CapabilitySpec{
					{
						Name:        capTracer,
						Cardinality: module.NamedOne,
						Type:        reflect.TypeOf(struct{}{}),
					},
				},
				AutoRules: []module.AutoRule{
					testAutoRule{
						path:        "yggdrasil.observability.telemetry.stats.server",
						description: "observability stats enabled",
					},
				},
				DefaultPolicy: &module.DefaultPolicy{Score: 20},
			},
		},
		testAutoModule{
			testCapabilityModule: testCapabilityModule{
				name: "trace.low",
				capabilities: []module.Capability{
					{
						Spec: module.CapabilitySpec{
							Name:        capTracer,
							Cardinality: module.NamedOne,
							Type:        reflect.TypeOf(struct{}{}),
						},
						Name:  "trace-low",
						Value: struct{}{},
					},
				},
			},
			autoSpec: module.AutoSpec{
				Provides: []module.CapabilitySpec{
					{
						Name:        capTracer,
						Cardinality: module.NamedOne,
						Type:        reflect.TypeOf(struct{}{}),
					},
				},
				AutoRules: []module.AutoRule{
					testAutoRule{
						path:        "yggdrasil.observability.telemetry.stats.server",
						description: "observability stats enabled",
					},
				},
				DefaultPolicy: &module.DefaultPolicy{Score: 10},
			},
		},
	)

	result, err := Plan(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "trace-high", result.Spec.Defaults[capTracer])
	require.Len(t, result.MatchedAutoRules, 2)
	require.Contains(
		t,
		result.AffectedPathsByDomain["modules"],
		"yggdrasil.observability.telemetry.stats.server",
	)
	require.Len(t, result.DefaultCandidates[capTracer], 2)
	require.True(t, result.DefaultCandidates[capTracer][0].Selected)
	require.Equal(t, "trace-high", result.DefaultCandidates[capTracer][0].Provider)
}

func TestDryRunRejectsInvalidAutoSpecAndAmbiguousAutoDefault(t *testing.T) {
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
	input.Modules = append(input.Modules, testAutoModule{
		testCapabilityModule: testCapabilityModule{
			name: "broken.auto",
			capabilities: []module.Capability{
				{
					Spec: module.CapabilitySpec{
						Name:        capMeter,
						Cardinality: module.NamedOne,
						Type:        reflect.TypeOf(struct{}{}),
					},
					Name:  "meter",
					Value: struct{}{},
				},
			},
		},
		autoSpec: module.AutoSpec{
			Provides: []module.CapabilitySpec{
				{Name: capTracer, Cardinality: module.NamedOne, Type: reflect.TypeOf(struct{}{})},
			},
			AutoRules: []module.AutoRule{
				testAutoRule{
					path:        "yggdrasil.observability.telemetry.stats.server",
					description: "observability stats enabled",
				},
			},
		},
	})

	_, err := Plan(context.Background(), input)
	requireAssemblyErr := func(code ErrorCode) {
		t.Helper()
		var assemblyErr *Error
		require.ErrorAs(t, err, &assemblyErr)
		require.Equal(t, code, assemblyErr.Code)
	}
	requireAssemblyErr(ErrInvalidAutoRule)

	ambiguous := compileTestInput(t, map[string]any{
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
	ambiguous.Modules = append(ambiguous.Modules,
		testAutoModule{
			testCapabilityModule: testCapabilityModule{
				name: "trace.a",
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
			},
			autoSpec: module.AutoSpec{
				Provides: []module.CapabilitySpec{
					{
						Name:        capTracer,
						Cardinality: module.NamedOne,
						Type:        reflect.TypeOf(struct{}{}),
					},
				},
				AutoRules: []module.AutoRule{
					testAutoRule{
						path:        "yggdrasil.observability.telemetry.stats.server",
						description: "observability stats enabled",
					},
				},
				DefaultPolicy: &module.DefaultPolicy{Score: 10},
			},
		},
		testAutoModule{
			testCapabilityModule: testCapabilityModule{
				name: "trace.b",
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
			},
			autoSpec: module.AutoSpec{
				Provides: []module.CapabilitySpec{
					{
						Name:        capTracer,
						Cardinality: module.NamedOne,
						Type:        reflect.TypeOf(struct{}{}),
					},
				},
				AutoRules: []module.AutoRule{
					testAutoRule{
						path:        "yggdrasil.observability.telemetry.stats.server",
						description: "observability stats enabled",
					},
				},
				DefaultPolicy: &module.DefaultPolicy{Score: 10},
			},
		},
	)
	_, err = Plan(context.Background(), ambiguous)
	var assemblyErr *Error
	require.ErrorAs(t, err, &assemblyErr)
	require.Equal(t, ErrAmbiguousDefault, assemblyErr.Code)
}

func TestPlanSupportsExplicitTemplateConfigShapesAndDryRunReturnsSpec(t *testing.T) {
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
					"unary_server": "default-observable@v1",
				},
				"middleware": map[string]any{
					"rest_all": map[string]any{
						"template": "default-rest-observable",
						"version":  "v1",
					},
				},
			},
		},
	})

	result, err := Plan(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "default-observable", result.Spec.Chains[chainUnaryServer].Template)
	require.Equal(t, "v1", result.Spec.Chains[chainUnaryServer].Version)
	require.Equal(t, []string{"logging"}, result.Spec.Chains[chainUnaryServer].Items)
	require.Equal(t, "default-rest-observable", result.Spec.Chains[chainRESTAll].Template)
	require.Equal(t, []string{"logger"}, result.Spec.Chains[chainRESTAll].Items)

	spec, err := DryRun(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, spec)
	require.Equal(t, result.Spec.Chains[chainUnaryServer], spec.Chains[chainUnaryServer])
	require.Equal(t, result.Spec.Chains[chainRESTAll], spec.Chains[chainRESTAll])
}
