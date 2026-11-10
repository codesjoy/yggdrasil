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

	internalruntime "github.com/codesjoy/yggdrasil/v3/app/internal/runtime"
	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	internalsettings "github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
)

// Reload triggers one staged reload cycle.
func (a *App) Reload(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	a.reloadMu.Lock()
	defer a.reloadMu.Unlock()

	if a.stopping.Load() {
		return nil
	}
	a.mu.Lock()
	if a.state != lifecycleStateRunning {
		a.mu.Unlock()
		return nil
	}
	opts := a.opts
	hub := a.hub
	businessInstalled := a.explicitBundleInstalled
	a.mu.Unlock()

	prevPlan := a.stablePlan()
	if prevPlan == nil {
		a.mu.Lock()
		prevPlan = a.lastPlanResult
		a.mu.Unlock()
	}
	if prevPlan == nil || prevPlan.Spec == nil {
		return a.recordReloadError(yassembly.NewError(
			yassembly.ErrRuntimeNotReady,
			"reload",
			"stable assembly plan is not available",
			nil,
			nil,
		))
	}
	prevBindings := selectedCapabilityBindings(prevPlan, opts.resolvedSettings)
	alreadyRestartRequired := hub.ReloadState().RestartRequired
	if alreadyRestartRequired {
		target := "module.hub"
		if businessInstalled {
			target = "business.bundle"
		}
		return a.recordReloadError(reloadRequiresRestartError(target))
	}

	nextResolved, err := resolveResolvedSettings(opts)
	if err != nil {
		return a.recordReloadError(wrapAssemblyStageError("reload", err))
	}
	nextPlan, err := a.buildAssemblyResultWithResolved(ctx, nextResolved)
	if err != nil {
		return a.recordReloadError(wrapAssemblyStageError("reload", err))
	}
	diff, err := yassembly.Diff(prevPlan.Spec, nextPlan.Spec)
	if err != nil {
		return a.recordReloadError(wrapAssemblyStageError("reload", err))
	}

	changedPaths := internalruntime.ChangedConfigPaths(prevPlan, nextPlan)
	restartRequired := internalruntime.ReloadRequiresRestart(
		diff,
		changedPaths,
		nextPlan.BusinessInputPaths,
		businessInstalled,
	)
	if restartRequired {
		target := "assembly.plan"
		if businessInstalled {
			target = "business.bundle"
		}
		hub.MarkRestartRequired(target)
		restartErr := reloadRequiresRestartError(target)
		a.recordReloadAttempt(nextPlan, diff, restartErr)
		return restartErr
	}

	bindingsChanged := !internalruntime.CapabilityBindingsEqual(
		prevBindings,
		nextPlan.CapabilityBindings,
	)
	a.stageReloadCandidate(nextPlan)
	defer a.clearReloadCandidate()
	if bindingsChanged {
		hub.MarkCapabilityBindingsChanged()
		hub.SetCapabilityBindings(nextPlan.CapabilityBindings)
	}
	if err := hub.Reload(ctx, opts.configManager.Snapshot()); err != nil {
		if bindingsChanged {
			hub.SetCapabilityBindings(prevBindings)
			hub.MarkCapabilityBindingsChanged()
		}
		reloadErr := yassembly.NewError(
			yassembly.ErrRuntimeReconcileFailed,
			"reload",
			"module hub reload reconcile failed",
			err,
			nil,
		)
		a.recordReloadAttempt(nextPlan, diff, reloadErr)
		return reloadErr
	}

	if hub.ReloadState().RestartRequired {
		restartErr := reloadRequiresRestartError("module.hub")
		a.recordReloadAttempt(nextPlan, diff, restartErr)
		return restartErr
	}

	a.commitReloadPlan(nextPlan, nextResolved, diff)
	return nil
}

func reloadRequiresRestartError(target string) error {
	return yassembly.NewError(
		yassembly.ErrReloadRequiresRestart,
		"reload",
		"reload requires restart for the current assembly or business graph diff",
		nil,
		map[string]string{"target": target},
	)
}

func (a *App) recordReloadError(err error) error {
	a.mu.Lock()
	a.recordAssemblyErrorLocked(assemblyStageReload, err)
	a.mu.Unlock()
	return err
}

func (a *App) recordReloadAttempt(
	result *yassembly.Result,
	diff *yassembly.SpecDiff,
	reloadErr error,
) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lastPlanResult = result
	a.lastSpecDiff = diff
	if result != nil {
		a.lastPlanHash = result.Hash
	}
	if reloadErr != nil {
		a.recordAssemblyErrorLocked(assemblyStageReload, reloadErr)
		return
	}
	a.clearAssemblyErrorLocked(assemblyStageReload)
}

func (a *App) commitReloadPlan(
	result *yassembly.Result,
	resolved internalsettings.Resolved,
	diff *yassembly.SpecDiff,
) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lastPlanResult = result
	a.lastSpecDiff = diff
	if result == nil {
		return
	}
	a.lastPlanHash = result.Hash
	a.lastStablePlanHash = result.Hash
	a.assemblySpec = result.Spec
	a.opts.resolvedSettings = resolved
	if a.runtimeAssembly != nil {
		a.runtimeAssembly.Spec = result.Spec
		a.runtimeAssembly.Modules = append([]module.Module(nil), result.Modules...)
	}
	a.planMu.Lock()
	a.stablePlanResult = result
	a.planMu.Unlock()
	a.clearAssemblyErrorLocked(assemblyStageReload)
}
