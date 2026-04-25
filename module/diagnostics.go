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
	"reflect"
	"sort"
)

// ModuleDiag is one module diagnostic snapshot item.
type ModuleDiag struct { //nolint:revive // stutter is acceptable for clarity
	Name                string   `json:"name"`
	DependsOn           []string `json:"depends_on"`
	TopoIndex           int      `json:"topo_index"`
	TopoLayer           int      `json:"topo_layer"`
	Started             bool     `json:"started"`
	RestartRequired     bool     `json:"restart_required"`
	ReloadPhase         string   `json:"reload_phase"`
	LastReloadError     string   `json:"last_reload_error"`
	CapabilityConflicts []string `json:"capability_conflicts"`
	DependencyErrors    []string `json:"dependency_errors"`
}

// CapabilityProviderDiag is one capability provider snapshot item.
type CapabilityProviderDiag struct {
	Module string `json:"module"`
	Name   string `json:"name"`
	Type   string `json:"type"`
}

// CapabilityDiag is one capability assembly snapshot item.
type CapabilityDiag struct {
	Spec        string                   `json:"spec"`
	Cardinality string                   `json:"cardinality"`
	Providers   []CapabilityProviderDiag `json:"providers"`
	Conflicts   []string                 `json:"conflicts"`
}

// CapabilityBindingDiag is one requested capability binding snapshot item.
type CapabilityBindingDiag struct {
	Spec      string   `json:"spec"`
	Requested []string `json:"requested"`
	Resolved  []string `json:"resolved"`
	Missing   []string `json:"missing"`
	Conflicts []string `json:"conflicts"`
}

// Diagnostics is the full hub diagnostics snapshot.
type Diagnostics struct {
	Modules             []ModuleDiag            `json:"modules"`
	Topology            []string                `json:"topology"`
	LastStableTopology  []string                `json:"last_stable_topology"`
	StartedModules      []string                `json:"started_modules"`
	ReloadState         ReloadState             `json:"reload_state"`
	DependencyErrors    []string                `json:"dependency_errors"`
	CapabilityConflicts []string                `json:"capability_conflicts"`
	Capabilities        []CapabilityDiag        `json:"capabilities"`
	Bindings            []CapabilityBindingDiag `json:"bindings"`
}

// Modules returns a diagnostics snapshot of all modules.
func (h *Hub) Modules() []ModuleDiag {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]ModuleDiag, 0, len(h.topoOrder))
	for i, mod := range h.topoOrder {
		item := ModuleDiag{
			Name:                mod.Name(),
			DependsOn:           dependsOn(mod),
			TopoIndex:           i,
			TopoLayer:           h.topoLayer[mod.Name()],
			Started:             h.started[mod.Name()],
			RestartRequired:     h.restartFlag[mod.Name()],
			ReloadPhase:         string(h.reloadState.Phase),
			CapabilityConflicts: append([]string(nil), h.moduleConflicts[mod.Name()]...),
		}
		item.DependencyErrors = moduleDependencyErrors(mod.Name(), h.dependencyErrors)
		if h.reloadState.FailedModule == mod.Name() {
			item.LastReloadError = h.reloadState.LastErrorText
		}
		if reporter, ok := mod.(ReloadReporter); ok {
			item.ReloadPhase = string(reporter.ReloadState().Phase)
		}
		result = append(result, item)
	}
	return result
}

// Diagnostics returns the full diagnostics snapshot.
func (h *Hub) Diagnostics() Diagnostics {
	h.mu.RLock()
	defer h.mu.RUnlock()

	startedModules := make([]string, 0, len(h.started))
	for _, mod := range h.topoOrder {
		if h.started[mod.Name()] {
			startedModules = append(startedModules, mod.Name())
		}
	}
	sort.Strings(startedModules)

	specNames := make([]string, 0, len(h.capabilityBindings))
	for specName := range h.capabilityBindings {
		specNames = append(specNames, specName)
	}
	sort.Strings(specNames)
	capabilityItems := make([]CapabilityDiag, 0, len(specNames))
	for _, specName := range specNames {
		binding := h.capabilityBindings[specName]
		providers := make([]CapabilityProviderDiag, 0, len(binding.entries))
		for _, entry := range binding.entries {
			typeName := ""
			if entry.valueType != nil {
				typeName = entry.valueType.String()
			} else {
				typeName = reflect.TypeOf(entry.value).String()
			}
			providers = append(providers, CapabilityProviderDiag{
				Module: entry.module.Name(),
				Name:   entry.name,
				Type:   typeName,
			})
		}
		capabilityItems = append(capabilityItems, CapabilityDiag{
			Spec:        binding.spec.Name,
			Cardinality: binding.spec.Cardinality.String(),
			Providers:   providers,
			Conflicts:   capabilityConflictsForSpec(binding.spec.Name, h.capabilityConflicts),
		})
	}
	bindingNames := make([]string, 0, len(h.requestedBindings)+len(h.capabilityBindings))
	seenBindingName := map[string]struct{}{}
	for specName := range h.requestedBindings {
		if _, ok := seenBindingName[specName]; ok {
			continue
		}
		seenBindingName[specName] = struct{}{}
		bindingNames = append(bindingNames, specName)
	}
	for specName := range h.capabilityBindings {
		if _, ok := seenBindingName[specName]; ok {
			continue
		}
		seenBindingName[specName] = struct{}{}
		bindingNames = append(bindingNames, specName)
	}
	sort.Strings(bindingNames)
	bindingItems := make([]CapabilityBindingDiag, 0, len(bindingNames))
	for _, specName := range bindingNames {
		requested := append([]string(nil), h.requestedBindings[specName]...)
		binding := h.capabilityBindings[specName]
		available := make([]string, 0, len(binding.entries))
		availableSet := make(map[string]struct{}, len(binding.entries))
		for _, entry := range binding.entries {
			if _, ok := availableSet[entry.name]; ok {
				continue
			}
			availableSet[entry.name] = struct{}{}
			available = append(available, entry.name)
		}
		sort.Strings(available)
		resolved := make([]string, 0)
		missing := make([]string, 0)
		if len(requested) == 0 {
			resolved = append(resolved, available...)
		} else {
			for _, name := range requested {
				if _, ok := availableSet[name]; ok {
					resolved = append(resolved, name)
					continue
				}
				missing = append(missing, name)
			}
		}
		bindingItems = append(bindingItems, CapabilityBindingDiag{
			Spec:      specName,
			Requested: requested,
			Resolved:  resolved,
			Missing:   missing,
			Conflicts: capabilityConflictsForSpec(specName, h.capabilityConflicts),
		})
	}

	topology := make([]string, 0, len(h.topoOrder))
	for _, mod := range h.topoOrder {
		topology = append(topology, mod.Name())
	}
	lastStable := make([]string, 0, len(h.lastStableTopo))
	for _, mod := range h.lastStableTopo {
		lastStable = append(lastStable, mod.Name())
	}

	return Diagnostics{
		Modules:             h.Modules(),
		Topology:            topology,
		LastStableTopology:  lastStable,
		StartedModules:      startedModules,
		ReloadState:         h.reloadState,
		DependencyErrors:    append([]string(nil), h.dependencyErrors...),
		CapabilityConflicts: append([]string(nil), h.capabilityConflicts...),
		Capabilities:        capabilityItems,
		Bindings:            bindingItems,
	}
}

// ReloadState returns current hub reload state.
func (h *Hub) ReloadState() ReloadState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.reloadState
}

func moduleDependencyErrors(moduleName string, all []string) []string {
	out := make([]string, 0)
	prefix := "module \"" + moduleName + "\" "
	for _, item := range all {
		if len(item) >= len(prefix) && item[:len(prefix)] == prefix {
			out = append(out, item)
		}
	}
	return out
}

func capabilityConflictsForSpec(specName string, all []string) []string {
	out := make([]string, 0)
	prefix := "capability \"" + specName + "\" "
	for _, item := range all {
		if len(item) >= len(prefix) && item[:len(prefix)] == prefix {
			out = append(out, item)
		}
	}
	return out
}
