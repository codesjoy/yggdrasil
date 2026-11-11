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
	"maps"
	"reflect"
	"slices"
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

type diagnosticsState struct {
	modules             []Module
	lastStableTopology  []Module
	topologyLayer       map[string]int
	started             map[string]bool
	restartRequired     map[string]bool
	reloadState         ReloadState
	dependencyErrors    []string
	capabilityConflicts []string
	moduleConflicts     map[string][]string
	capabilityBindings  map[string]capabilityBinding
	requestedBindings   map[string][]string
}

// Modules returns a diagnostics snapshot of all modules.
func (h *Hub) Modules() []ModuleDiag {
	return modulesFromDiagnostics(h.diagnosticsState())
}

func (h *Hub) diagnosticsState() diagnosticsState {
	h.mu.RLock()
	defer h.mu.RUnlock()

	moduleConflicts := make(map[string][]string, len(h.moduleConflicts))
	for name, items := range h.moduleConflicts {
		moduleConflicts[name] = append([]string(nil), items...)
	}
	capabilityBindings := make(map[string]capabilityBinding, len(h.capabilityBindings))
	for name, binding := range h.capabilityBindings {
		cloned := binding
		cloned.entries = append([]capabilityEntry(nil), binding.entries...)
		capabilityBindings[name] = cloned
	}
	requestedBindings := make(map[string][]string, len(h.requestedBindings))
	for name, items := range h.requestedBindings {
		requestedBindings[name] = append([]string(nil), items...)
	}

	return diagnosticsState{
		modules:             append([]Module(nil), h.topoOrder...),
		lastStableTopology:  append([]Module(nil), h.lastStableTopo...),
		topologyLayer:       cloneIntMap(h.topoLayer),
		started:             cloneBoolMap(h.started),
		restartRequired:     cloneBoolMap(h.restartFlag),
		reloadState:         h.reloadState,
		dependencyErrors:    append([]string(nil), h.dependencyErrors...),
		capabilityConflicts: append([]string(nil), h.capabilityConflicts...),
		moduleConflicts:     moduleConflicts,
		capabilityBindings:  capabilityBindings,
		requestedBindings:   requestedBindings,
	}
}

func modulesFromDiagnostics(state diagnosticsState) []ModuleDiag {
	result := make([]ModuleDiag, 0, len(state.modules))
	for index, mod := range state.modules {
		name := mod.Name()
		item := ModuleDiag{
			Name:                name,
			DependsOn:           dependsOn(mod),
			TopoIndex:           index,
			TopoLayer:           state.topologyLayer[name],
			Started:             state.started[name],
			RestartRequired:     state.restartRequired[name],
			ReloadPhase:         string(state.reloadState.Phase),
			CapabilityConflicts: append([]string(nil), state.moduleConflicts[name]...),
		}
		item.DependencyErrors = moduleDependencyErrors(name, state.dependencyErrors)
		if state.reloadState.FailedModule == name {
			item.LastReloadError = state.reloadState.LastErrorText
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
	state := h.diagnosticsState()
	return Diagnostics{
		Modules:             modulesFromDiagnostics(state),
		Topology:            moduleNames(state.modules),
		LastStableTopology:  moduleNames(state.lastStableTopology),
		StartedModules:      startedModuleNames(state),
		ReloadState:         state.reloadState,
		DependencyErrors:    append([]string(nil), state.dependencyErrors...),
		CapabilityConflicts: append([]string(nil), state.capabilityConflicts...),
		Capabilities:        capabilityDiagnostics(state),
		Bindings:            capabilityBindingDiagnostics(state),
	}
}

func startedModuleNames(state diagnosticsState) []string {
	startedModules := make([]string, 0, len(state.started))
	for _, mod := range state.modules {
		name := mod.Name()
		if state.started[name] {
			startedModules = append(startedModules, name)
		}
	}
	slices.Sort(startedModules)
	return startedModules
}

func capabilityDiagnostics(state diagnosticsState) []CapabilityDiag {
	specNames := slices.Sorted(maps.Keys(state.capabilityBindings))
	capabilityItems := make([]CapabilityDiag, 0, len(specNames))
	for _, specName := range specNames {
		binding := state.capabilityBindings[specName]
		providers := make([]CapabilityProviderDiag, 0, len(binding.entries))
		for _, entry := range binding.entries {
			typeName := ""
			if entry.valueType != nil {
				typeName = entry.valueType.String()
			} else {
				valueType := reflect.TypeOf(entry.value)
				if valueType != nil {
					typeName = valueType.String()
				}
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
			Conflicts:   capabilityConflictsForSpec(binding.spec.Name, state.capabilityConflicts),
		})
	}
	return capabilityItems
}

func capabilityBindingDiagnostics(state diagnosticsState) []CapabilityBindingDiag {
	bindingNames := make(
		[]string,
		0,
		len(state.requestedBindings)+len(state.capabilityBindings),
	)
	seenBindingName := map[string]struct{}{}
	for specName := range state.requestedBindings {
		if _, ok := seenBindingName[specName]; ok {
			continue
		}
		seenBindingName[specName] = struct{}{}
		bindingNames = append(bindingNames, specName)
	}
	for specName := range state.capabilityBindings {
		if _, ok := seenBindingName[specName]; ok {
			continue
		}
		seenBindingName[specName] = struct{}{}
		bindingNames = append(bindingNames, specName)
	}
	slices.Sort(bindingNames)
	bindingItems := make([]CapabilityBindingDiag, 0, len(bindingNames))
	for _, specName := range bindingNames {
		requested := append([]string(nil), state.requestedBindings[specName]...)
		binding := state.capabilityBindings[specName]
		available := make([]string, 0, len(binding.entries))
		availableSet := make(map[string]struct{}, len(binding.entries))
		for _, entry := range binding.entries {
			if _, ok := availableSet[entry.name]; ok {
				continue
			}
			availableSet[entry.name] = struct{}{}
			available = append(available, entry.name)
		}
		slices.Sort(available)
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
			Conflicts: capabilityConflictsForSpec(specName, state.capabilityConflicts),
		})
	}
	return bindingItems
}

func moduleNames(modules []Module) []string {
	names := make([]string, 0, len(modules))
	for _, mod := range modules {
		names = append(names, mod.Name())
	}
	return names
}

func cloneIntMap(in map[string]int) map[string]int {
	if in == nil {
		return map[string]int{}
	}
	return maps.Clone(in)
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return map[string]bool{}
	}
	return maps.Clone(in)
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
