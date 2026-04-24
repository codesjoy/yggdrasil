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
	"github.com/codesjoy/yggdrasil/v3/module"
)

// Reload triggers one staged reload cycle.
func (a *App) Reload(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	a.reloadMu.Lock()
	defer a.reloadMu.Unlock()

	a.mu.Lock()
	if a.state != lifecycleStateRunning {
		a.mu.Unlock()
		return nil
	}
	opts := a.opts
	hub := a.hub
	prevPlan := a.lastPlanResult
	prevSpec := a.assemblySpec
	prevBindings := selectedCapabilityBindings(prevPlan, opts.resolvedSettings)
	businessInstalled := a.explicitBundleInstalled
	a.mu.Unlock()

	if err := refreshResolvedSettings(opts); err != nil {
		return a.recordReloadError(wrapAssemblyStageError("reload", err))
	}
	nextPlan, err := a.buildAssemblyResult(ctx)
	if err != nil {
		return a.recordReloadError(wrapAssemblyStageError("reload", err))
	}
	diff, err := yassembly.Diff(prevSpec, nextPlan.Spec)
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
		restartErr := yassembly.NewError(
			yassembly.ErrReloadRequiresRestart,
			"reload",
			"reload requires restart for the current assembly or business graph diff",
			nil,
			map[string]string{"target": target},
		)
		a.storeReloadPlan(nextPlan, diff, false, restartErr)
		return restartErr
	}

	if !internalruntime.CapabilityBindingsEqual(prevBindings, nextPlan.CapabilityBindings) {
		hub.MarkCapabilityBindingsChanged()
	}
	hub.SetCapabilityBindings(nextPlan.CapabilityBindings)
	if err := hub.Reload(ctx, opts.configManager.Snapshot()); err != nil {
		return a.recordReloadError(yassembly.NewError(
			yassembly.ErrRuntimeReconcileFailed,
			"reload",
			"module hub reload reconcile failed",
			err,
			nil,
		))
	}

	a.storeReloadPlan(nextPlan, diff, true, nil)
	return nil
}

func (a *App) recordReloadError(err error) error {
	a.mu.Lock()
	a.recordAssemblyErrorLocked(assemblyStageReload, err)
	a.mu.Unlock()
	return err
}

func (a *App) storeReloadPlan(result *yassembly.Result, diff *yassembly.SpecDiff, stable bool, reloadErr error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lastPlanResult = result
	a.lastSpecDiff = diff
	if result != nil {
		a.lastPlanHash = result.Hash
		a.assemblySpec = result.Spec
		if stable {
			a.lastStablePlanHash = result.Hash
		}
	}
	if a.runtimeAssembly != nil && result != nil {
		a.runtimeAssembly.Spec = result.Spec
		a.runtimeAssembly.Modules = append([]module.Module(nil), result.Modules...)
	}
	if reloadErr != nil {
		a.recordAssemblyErrorLocked(assemblyStageReload, reloadErr)
		return
	}
	a.clearAssemblyErrorLocked(assemblyStageReload)
}
