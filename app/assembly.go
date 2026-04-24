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

	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
	"github.com/codesjoy/yggdrasil/v3/server"
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
	mods := make([]module.Module, 0, 4+len(a.opts.modules))
	mods = append(mods,
		foundationBuiltinCapabilityModule{},
		connectivityBuiltinCapabilityModule{},
		statsOtelCapabilityModule{},
		foundationRuntimeModule{app: a},
		connectivityRuntimeModule{app: a},
	)
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

func selectedCapabilityBindings(result *yassembly.Result, fallback settings.Resolved) map[string][]string {
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

type assemblyStage string

const (
	assemblyStagePrepare assemblyStage = "prepare"
	assemblyStageCompose assemblyStage = "compose"
	assemblyStageInstall assemblyStage = "install"
	assemblyStageReload  assemblyStage = "reload"
)

type assemblyErrorRecord struct {
	err *yassembly.Error
	seq uint64
}

type assemblyStageErrors struct {
	Prepare *yassembly.Error `json:"prepare"`
	Compose *yassembly.Error `json:"compose"`
	Install *yassembly.Error `json:"install"`
	Reload  *yassembly.Error `json:"reload"`
}

type assemblyErrorSnapshot struct {
	lastError      *yassembly.Error
	lastErrorStage string
	errors         assemblyStageErrors
}

type assemblyErrorState struct {
	prepare assemblyErrorRecord
	compose assemblyErrorRecord
	install assemblyErrorRecord
	reload  assemblyErrorRecord

	latest      *yassembly.Error
	latestStage assemblyStage
	nextSeq     uint64
}

func (a *App) recordAssemblyErrorLocked(stage assemblyStage, err error) {
	a.assemblyErrors.record(stage, normalizeAssemblyError(stage, err))
}

func (a *App) clearAssemblyErrorLocked(stage assemblyStage) {
	a.assemblyErrors.clear(stage)
}

func (a *App) assemblyErrorDiagnosticsLocked() assemblyErrorSnapshot {
	return a.assemblyErrors.snapshot()
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
	if err == nil {
		return nil
	}
	if structured, ok := err.(*yassembly.Error); ok {
		if structured.Stage == "" {
			structured.Stage = stage
		}
		return structured
	}
	code := yassembly.ErrRuntimeSurfaceUnavailable
	switch stage {
	case "prepare":
		code = yassembly.ErrRuntimeSurfaceUnavailable
	case "compose":
		code = yassembly.ErrComposeFailed
	case "install":
		code = yassembly.ErrInstallValidationFailed
	case "reload":
		code = yassembly.ErrRuntimeReconcileFailed
	}
	return yassembly.NewError(code, stage, err.Error(), err, nil)
}

func normalizeAssemblyError(stage assemblyStage, err error) *yassembly.Error {
	if err == nil {
		return nil
	}

	var structured *yassembly.Error
	if errors.As(err, &structured) {
		clone := cloneAssemblyError(structured)
		clone.Stage = string(stage)
		return clone
	}

	return yassembly.NewError(defaultAssemblyErrorCode(stage), string(stage), err.Error(), err, nil)
}

func defaultAssemblyErrorCode(stage assemblyStage) yassembly.ErrorCode {
	switch stage {
	case assemblyStageCompose:
		return yassembly.ErrComposeFailed
	case assemblyStageInstall:
		return yassembly.ErrInstallValidationFailed
	case assemblyStageReload:
		return yassembly.ErrRuntimeReconcileFailed
	default:
		return yassembly.ErrRuntimeSurfaceUnavailable
	}
}

func cloneAssemblyError(err *yassembly.Error) *yassembly.Error {
	if err == nil {
		return nil
	}

	clone := *err
	if err.Context != nil {
		clone.Context = make(map[string]string, len(err.Context))
		for key, value := range err.Context {
			clone.Context[key] = value
		}
	}
	return &clone
}

func (s *assemblyErrorState) record(stage assemblyStage, err *yassembly.Error) {
	if err == nil {
		s.clear(stage)
		return
	}

	s.nextSeq++
	record := assemblyErrorRecord{err: err, seq: s.nextSeq}
	switch stage {
	case assemblyStagePrepare:
		s.prepare = record
	case assemblyStageCompose:
		s.compose = record
	case assemblyStageInstall:
		s.install = record
	case assemblyStageReload:
		s.reload = record
	default:
		return
	}
	s.recalculateLatest()
}

func (s *assemblyErrorState) clear(stage assemblyStage) {
	switch stage {
	case assemblyStagePrepare:
		s.prepare = assemblyErrorRecord{}
	case assemblyStageCompose:
		s.compose = assemblyErrorRecord{}
	case assemblyStageInstall:
		s.install = assemblyErrorRecord{}
	case assemblyStageReload:
		s.reload = assemblyErrorRecord{}
	default:
		return
	}
	s.recalculateLatest()
}

func (s *assemblyErrorState) snapshot() assemblyErrorSnapshot {
	return assemblyErrorSnapshot{
		lastError:      cloneAssemblyError(s.latest),
		lastErrorStage: string(s.latestStage),
		errors: assemblyStageErrors{
			Prepare: cloneAssemblyError(s.prepare.err),
			Compose: cloneAssemblyError(s.compose.err),
			Install: cloneAssemblyError(s.install.err),
			Reload:  cloneAssemblyError(s.reload.err),
		},
	}
}

func (s *assemblyErrorState) recalculateLatest() {
	s.latest = nil
	s.latestStage = ""

	bestStage := assemblyStage("")
	bestSeq := uint64(0)
	bestErr := (*yassembly.Error)(nil)
	for _, item := range []struct {
		stage  assemblyStage
		record assemblyErrorRecord
	}{
		{stage: assemblyStagePrepare, record: s.prepare},
		{stage: assemblyStageCompose, record: s.compose},
		{stage: assemblyStageInstall, record: s.install},
		{stage: assemblyStageReload, record: s.reload},
	} {
		if item.record.err == nil || item.record.seq < bestSeq {
			continue
		}
		bestSeq = item.record.seq
		bestStage = item.stage
		bestErr = item.record.err
	}

	s.latest = bestErr
	s.latestStage = bestStage
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
		LastError:             errorDiag.lastError,
		LastErrorStage:        errorDiag.lastErrorStage,
		Errors:                errorDiag.errors,
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
		diag.MatchedAutoRules = append([]yassembly.MatchedAutoRule(nil), a.lastPlanResult.MatchedAutoRules...)
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
