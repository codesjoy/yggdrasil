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

package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	internalassembly "github.com/codesjoy/yggdrasil/v3/app/internal/assembly"
	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

type preparedAssembly struct {
	Spec      *yassembly.Spec
	Modules   []module.Module
	Runtime   any
	Server    server.Server
	CloseFunc func(context.Context) error
}

func (pa *preparedAssembly) Close(ctx context.Context) error {
	if pa == nil || pa.CloseFunc == nil {
		return nil
	}
	return pa.CloseFunc(ctx)
}

func (a *App) plannedModules() []module.Module {
	mods := make([]module.Module, 0, 4+len(a.opts.modules)+len(a.opts.capabilityRegistrations))
	mods = append(mods,
		foundationBuiltinCapabilityModule{},
		connectivityBuiltinCapabilityModule{},
		statsOtelCapabilityModule{},
		foundationRuntimeModule{app: a},
		connectivityRuntimeModule{app: a},
	)
	for _, reg := range a.opts.capabilityRegistrations {
		mods = append(mods, capabilityRegistrationModule{reg: reg})
	}
	mods = append(mods, a.opts.modules...)
	return mods
}

func (a *App) assemblyInputLocked() yassembly.Input {
	resolved := a.opts.resolvedSettings
	if a.opts.mode != "" {
		resolved.Mode = a.opts.mode
	}
	return yassembly.Input{
		Identity: yassembly.IdentitySpec{
			AppName: a.name,
		},
		Resolved:  resolved,
		Snapshot:  a.opts.configManager.Snapshot(),
		Modules:   a.plannedModules(),
		Overrides: append([]yassembly.Override(nil), a.opts.planOverrides...),
	}
}

func (a *App) buildAssemblyResult(ctx context.Context) (*yassembly.Result, error) {
	if a == nil || a.opts == nil {
		return nil, nil
	}
	result, err := yassembly.Plan(ctx, a.assemblyInputLocked())
	if err != nil {
		return nil, err
	}
	return result, nil
}

func effectiveResolved(result *yassembly.Result, fallback settings.Resolved) settings.Resolved {
	if result == nil {
		return fallback
	}
	return result.EffectiveResolved
}

func selectedCapabilityBindings(
	result *yassembly.Result,
	fallback settings.Resolved,
) map[string][]string {
	if result == nil {
		return cloneCapabilityBindings(fallback.CapabilityBindings)
	}
	return cloneCapabilityBindings(result.CapabilityBindings)
}

func cloneCapabilityBindings(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(in))
	for key, items := range in {
		out[key] = append([]string(nil), items...)
	}
	return out
}

type assemblyStage = internalassembly.Stage

const (
	assemblyStagePrepare assemblyStage = internalassembly.StagePrepare
	assemblyStageCompose assemblyStage = internalassembly.StageCompose
	assemblyStageInstall assemblyStage = internalassembly.StageInstall
	assemblyStageReload  assemblyStage = internalassembly.StageReload
)

type (
	assemblyStageErrors   = internalassembly.StageErrors
	assemblyErrorSnapshot = internalassembly.ErrorSnapshot
	assemblyErrorState    = internalassembly.ErrorState
)

func (a *App) recordAssemblyErrorLocked(stage assemblyStage, err error) {
	a.assemblyErrors.Record(stage, normalizeAssemblyError(stage, err))
}

func (a *App) clearAssemblyErrorLocked(stage assemblyStage) {
	a.assemblyErrors.Clear(stage)
}

func (a *App) assemblyErrorDiagnosticsLocked() assemblyErrorSnapshot {
	return a.assemblyErrors.Snapshot()
}

func (a *App) failBeforeStart(err error, stage string, code yassembly.ErrorCode) error {
	if err == nil {
		return nil
	}
	a.mu.Lock()
	a.stopConfigWatchLocked()
	a.setStoppedLocked()
	a.mu.Unlock()
	cleanupErr := a.stopResources()
	if cleanupErr == nil {
		return err
	}
	wrapped := yassembly.NewError(code, stage, stage+" rollback failed", cleanupErr, nil)
	return errors.Join(err, wrapped)
}

func wrapAssemblyStageError(stage string, err error) error {
	return internalassembly.WrapStageError(stage, err)
}

func normalizeAssemblyError(stage assemblyStage, err error) *yassembly.Error {
	return internalassembly.NormalizeError(stage, err)
}

type defaultSelectionDiag struct {
	Value  string `json:"value"`
	Source string `json:"source"`
}

type assemblyDiagnostics struct {
	Spec                  *yassembly.Spec                         `json:"spec,omitempty"`
	Mode                  yassembly.Mode                          `json:"mode"`
	EnabledModules        []string                                `json:"enabled_modules,omitempty"`
	SelectedDefaults      map[string]defaultSelectionDiag         `json:"selected_defaults,omitempty"`
	DefaultCandidates     map[string][]yassembly.DefaultCandidate `json:"default_candidates,omitempty"`
	Templates             map[string]yassembly.Chain              `json:"templates,omitempty"`
	MatchedAutoRules      []yassembly.MatchedAutoRule             `json:"matched_auto_rules,omitempty"`
	AffectedPathsByDomain map[string][]string                     `json:"affected_paths_by_domain,omitempty"`
	BusinessInputPaths    []string                                `json:"business_input_paths,omitempty"`
	BundleDiagnostics     []BundleDiag                            `json:"bundle_diagnostics,omitempty"`
	CurrentSpecHash       string                                  `json:"current_spec_hash,omitempty"`
	LastStableSpecHash    string                                  `json:"last_stable_spec_hash,omitempty"`
	LastSpecDiff          *yassembly.SpecDiff                     `json:"last_spec_diff,omitempty"`
	LastError             *yassembly.Error                        `json:"last_error,omitempty"`
	LastErrorStage        string                                  `json:"last_error_stage,omitempty"`
	Errors                assemblyStageErrors                     `json:"errors"`
}

func (a *App) assemblyDiagnostics() assemblyDiagnostics {
	a.mu.Lock()
	defer a.mu.Unlock()

	errorDiag := a.assemblyErrorDiagnosticsLocked()

	diag := assemblyDiagnostics{
		Spec:                  a.assemblySpec,
		CurrentSpecHash:       a.lastPlanHash,
		LastStableSpecHash:    a.lastStablePlanHash,
		LastSpecDiff:          a.lastSpecDiff,
		LastError:             errorDiag.LastError,
		LastErrorStage:        errorDiag.LastErrorStage,
		Errors:                errorDiag.Errors,
		SelectedDefaults:      map[string]defaultSelectionDiag{},
		DefaultCandidates:     map[string][]yassembly.DefaultCandidate{},
		Templates:             map[string]yassembly.Chain{},
		AffectedPathsByDomain: map[string][]string{},
		BusinessInputPaths:    append([]string(nil), a.bundleDiagnosticsBusinessInputsLocked()...),
		BundleDiagnostics:     append([]BundleDiag(nil), a.bundleDiagnostics...),
	}
	if a.assemblySpec != nil {
		diag.Mode = a.assemblySpec.Mode
		for _, mod := range a.assemblySpec.Modules {
			diag.EnabledModules = append(diag.EnabledModules, mod.Name)
		}
		for key, value := range a.assemblySpec.Chains {
			diag.Templates[key] = value
		}
	}
	if a.lastPlanResult != nil {
		for key, value := range a.lastPlanResult.Spec.Defaults {
			diag.SelectedDefaults[key] = defaultSelectionDiag{
				Value:  value,
				Source: a.lastPlanResult.DefaultSources[key],
			}
		}
		for key, items := range a.lastPlanResult.DefaultCandidates {
			diag.DefaultCandidates[key] = append([]yassembly.DefaultCandidate(nil), items...)
		}
		diag.MatchedAutoRules = append(
			[]yassembly.MatchedAutoRule(nil),
			a.lastPlanResult.MatchedAutoRules...)
		diag.BusinessInputPaths = append([]string(nil), a.lastPlanResult.BusinessInputPaths...)
		for key, value := range a.lastPlanResult.AffectedPathsByDomain {
			diag.AffectedPathsByDomain[key] = append([]string(nil), value...)
		}
	}
	return diag
}

func (a *App) bundleDiagnosticsBusinessInputsLocked() []string {
	if a == nil || a.lastPlanResult == nil {
		return nil
	}
	return a.lastPlanResult.BusinessInputPaths
}

func (a *App) installDiagnosticsRoutes() {
	if a.opts == nil || a.opts.governor == nil {
		return
	}
	a.opts.governor.HandleFunc("/module-hub", func(w http.ResponseWriter, r *http.Request) {
		writeDiagnosticsJSON(w, r, a.hub.Diagnostics())
	})
	a.opts.governor.HandleFunc("/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		writeDiagnosticsJSON(w, r, map[string]any{
			"module_hub": a.hub.Diagnostics(),
			"assembly":   a.assemblyDiagnostics(),
		})
	})
}

func writeDiagnosticsJSON(w http.ResponseWriter, r *http.Request, resp any) {
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	if r.URL.Query().Get("pretty") == "true" {
		encoder.SetIndent("", "    ")
	}
	_ = encoder.Encode(resp)
}
