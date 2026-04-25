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

package module

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/config"
)

// Init initializes modules in topological order.
func (h *Hub) Init(ctx context.Context, snap config.Snapshot) error {
	h.mu.Lock()
	if !h.sealed {
		h.mu.Unlock()
		return errHubNotSealed
	}
	topo := append([]Module(nil), h.topoOrder...)
	h.snapshot = snap
	h.mu.Unlock()

	for _, mod := range topo {
		item, ok := mod.(Initializable)
		if !ok {
			continue
		}
		if err := item.Init(ctx, moduleView(mod, snap)); err != nil {
			return fmt.Errorf("module %q init failed: %w", mod.Name(), err)
		}
	}
	return nil
}

// Start starts modules with compensation semantics.
func (h *Hub) Start(ctx context.Context) error {
	h.mu.Lock()
	if !h.sealed {
		h.mu.Unlock()
		return errHubNotSealed
	}
	topo := append([]Module(nil), h.topoOrder...)
	h.started = map[string]bool{}
	h.mu.Unlock()

	startedSeq := make([]Module, 0, len(topo))
	for _, mod := range topo {
		item, ok := mod.(Startable)
		if !ok {
			h.markStarted(mod.Name())
			startedSeq = append(startedSeq, mod)
			continue
		}
		if err := item.Start(ctx); err != nil {
			compensateErr := h.stopSequence(ctx, startedSeq)
			if compensateErr != nil {
				return errors.Join(
					fmt.Errorf("module %q start failed: %w", mod.Name(), err),
					fmt.Errorf("start compensation failed: %w", compensateErr),
				)
			}
			return fmt.Errorf("module %q start failed: %w", mod.Name(), err)
		}
		h.markStarted(mod.Name())
		startedSeq = append(startedSeq, mod)
	}
	return nil
}

// Stop stops modules in reverse topo order.
func (h *Hub) Stop(ctx context.Context) error {
	h.mu.RLock()
	if !h.sealed {
		h.mu.RUnlock()
		return errHubNotSealed
	}
	topo := append([]Module(nil), h.topoOrder...)
	h.mu.RUnlock()
	return h.stopSequence(ctx, topo)
}

func (h *Hub) stopSequence(ctx context.Context, startedSeq []Module) error {
	var multiErr error
	for i := len(startedSeq) - 1; i >= 0; i-- {
		mod := startedSeq[i]
		item, ok := mod.(Stoppable)
		if !ok {
			continue
		}
		if err := item.Stop(ctx); err != nil {
			multiErr = errors.Join(
				multiErr,
				fmt.Errorf("module %q stop failed: %w", mod.Name(), err),
			)
		}
	}
	return multiErr
}

// SetCapabilityBindings stores the latest requested capability bindings compiled from config.
func (h *Hub) SetCapabilityBindings(bindings map[string][]string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.requestedBindings = cloneRequestedBindings(bindings)
}

type preparedReload struct {
	module    Module
	committer ReloadCommitter
}

// Reload executes staged reload and updates snapshot on success.
func (h *Hub) Reload(ctx context.Context, snap config.Snapshot) error {
	h.mu.Lock()
	if !h.sealed {
		h.mu.Unlock()
		return errHubNotSealed
	}
	if h.reloadState.Phase == ReloadPhaseDegraded {
		h.mu.Unlock()
		return errors.New("module hub is degraded; restart required before next reload")
	}
	oldSnap := h.snapshot
	topo := append([]Module(nil), h.topoOrder...)
	forceAll := h.reloadAll
	restartRequired := h.restartRequired
	h.reloadAll = false
	h.reloadState = ReloadState{
		Phase:           ReloadPhasePreparing,
		RestartRequired: restartRequired,
	}
	h.mu.Unlock()

	affected := make([]Module, 0, len(topo))
	if forceAll {
		affected = append(affected, topo...)
	} else {
		for _, mod := range topo {
			if !configChanged(mod, oldSnap, snap) {
				continue
			}
			affected = append(affected, mod)
		}
	}
	if len(affected) == 0 {
		h.mu.Lock()
		h.snapshot = snap
		h.reloadState = ReloadState{
			Phase:           ReloadPhaseIdle,
			RestartRequired: restartRequired,
		}
		h.mu.Unlock()
		return nil
	}

	prepared := make([]preparedReload, 0, len(affected))
	for _, mod := range affected {
		item, ok := mod.(Reloadable)
		if !ok {
			h.mu.Lock()
			h.restartFlag[mod.Name()] = true
			h.restartRequired = true
			restartRequired = true
			h.reloadState.RestartRequired = true
			h.mu.Unlock()
			continue
		}
		committer, err := item.PrepareReload(ctx, moduleView(mod, snap))
		if err != nil {
			rollbackModule, rollbackErr := rollbackPrepared(ctx, prepared)
			state := ReloadState{
				Phase:           ReloadPhaseRollback,
				FailedModule:    mod.Name(),
				FailedStage:     ReloadFailedStagePrepare,
				RestartRequired: restartRequired,
			}.withError(err)
			if rollbackErr != nil {
				state.Phase = ReloadPhaseDegraded
				state.RestartRequired = true
				state.Diverged = true
				state.FailedStage = ReloadFailedStageRollback
				if rollbackModule != "" {
					state.FailedModule = rollbackModule
				}
				state = state.withError(errors.Join(err, rollbackErr))
			}
			h.mu.Lock()
			if state.RestartRequired {
				h.restartRequired = true
			}
			h.restartFlag[mod.Name()] = true
			h.reloadState = state
			h.mu.Unlock()
			return fmt.Errorf("module %q prepare reload failed: %w", mod.Name(), err)
		}
		prepared = append(prepared, preparedReload{module: mod, committer: committer})
	}

	h.mu.Lock()
	h.reloadState = ReloadState{
		Phase:           ReloadPhaseCommitting,
		RestartRequired: restartRequired,
	}
	h.mu.Unlock()

	for i := range prepared {
		item := prepared[i]
		if err := item.committer.Commit(ctx); err != nil {
			rollbackModule, rollbackErr := rollbackPrepared(ctx, prepared[i+1:])
			state := ReloadState{
				Phase:           ReloadPhaseRollback,
				RestartRequired: true,
				Diverged:        true,
				FailedModule:    item.module.Name(),
				FailedStage:     ReloadFailedStageCommit,
			}.withError(err)
			if rollbackErr != nil {
				state.Phase = ReloadPhaseDegraded
				state.FailedStage = ReloadFailedStageRollback
				if rollbackModule != "" {
					state.FailedModule = rollbackModule
				}
				state = state.withError(errors.Join(err, rollbackErr))
			}
			h.mu.Lock()
			h.restartRequired = true
			h.reloadState = state
			h.restartFlag[item.module.Name()] = true
			h.mu.Unlock()
			return fmt.Errorf("module %q commit reload failed: %w", item.module.Name(), err)
		}
	}

	h.mu.Lock()
	h.snapshot = snap
	h.reloadState = ReloadState{
		Phase:           ReloadPhaseIdle,
		RestartRequired: restartRequired,
	}
	h.mu.Unlock()
	return nil
}

func rollbackPrepared(ctx context.Context, prepared []preparedReload) (string, error) {
	var multiErr error
	var failedModule string
	for i := len(prepared) - 1; i >= 0; i-- {
		if err := prepared[i].committer.Rollback(ctx); err != nil {
			if failedModule == "" {
				failedModule = prepared[i].module.Name()
			}
			multiErr = errors.Join(
				multiErr,
				fmt.Errorf("module %q rollback failed: %w", prepared[i].module.Name(), err),
			)
		}
	}
	return failedModule, multiErr
}

func cloneRequestedBindings(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(in))
	for key, items := range in {
		next := make([]string, len(items))
		copy(next, items)
		out[key] = next
	}
	return out
}

// MarkCapabilityBindingsChanged marks the next reload to re-evaluate all modules.
func (h *Hub) MarkCapabilityBindingsChanged() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reloadAll = true
}

// MarkRestartRequired marks one module and the hub as requiring restart after reload.
func (h *Hub) MarkRestartRequired(moduleName string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.restartRequired = true
	h.reloadState.RestartRequired = true
	if moduleName != "" {
		h.restartFlag[moduleName] = true
	}
}

func moduleView(mod Module, snap config.Snapshot) config.View {
	path := ""
	if item, ok := mod.(Configurable); ok {
		path = item.ConfigPath()
	}
	if path == "" {
		return config.NewView("", snap)
	}
	return config.NewView(path, snap.Section(splitDotPath(path)...))
}

func splitDotPath(path string) []string {
	raw := strings.Split(path, ".")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func configChanged(mod Module, oldSnap, newSnap config.Snapshot) bool {
	path := ""
	if item, ok := mod.(Configurable); ok {
		path = item.ConfigPath()
	}
	if path == "" {
		return false
	}
	parts := splitDotPath(path)
	return string(oldSnap.Section(parts...).Bytes()) != string(newSnap.Section(parts...).Bytes())
}

func (h *Hub) markStarted(name string) {
	h.mu.Lock()
	h.started[name] = true
	h.mu.Unlock()
}
