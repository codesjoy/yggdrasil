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
	"errors"
	"fmt"
	"sync"

	"github.com/codesjoy/yggdrasil/v3/config"
)

// Hub is the module runtime center.
type Hub struct {
	mu sync.RWMutex

	modules []Module
	index   map[string]Module
	sealed  bool

	topoOrder      []Module
	topoLayer      map[string]int
	lastStableTopo []Module
	started        map[string]bool

	restartFlag         map[string]bool
	restartRequired     bool
	reloadState         ReloadState
	capabilityIdx       map[string][]capabilityEntry
	capabilityBindings  map[string]capabilityBinding
	requestedBindings   map[string][]string
	capabilityConflicts []string
	moduleConflicts     map[string][]string
	dependencyErrors    []string
	reloadAll           bool

	snapshot config.Snapshot
}

// NewHub creates an empty Hub.
func NewHub() *Hub {
	return &Hub{
		index:             map[string]Module{},
		started:           map[string]bool{},
		restartFlag:       map[string]bool{},
		capabilityIdx:     map[string][]capabilityEntry{},
		requestedBindings: map[string][]string{},
		moduleConflicts:   map[string][]string{},
		reloadState: ReloadState{
			Phase: ReloadPhaseIdle,
		},
	}
}

// Use registers modules before sealing.
func (h *Hub) Use(modules ...Module) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sealed {
		return errHubSealed
	}
	for _, mod := range modules {
		if mod == nil {
			return errors.New("module is nil")
		}
		name := mod.Name()
		if name == "" {
			return errors.New("module name is empty")
		}
		if _, ok := h.index[name]; ok {
			return fmt.Errorf("module %q already exists", name)
		}
		if moduleScope(mod) == ScopeRuntimeFactory {
			return fmt.Errorf("module %q has unsupported scope runtime_factory", name)
		}
		h.modules = append(h.modules, mod)
		h.index[name] = mod
	}
	return nil
}

// Seal validates the graph and capability cardinality.
func (h *Hub) Seal() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sealed {
		return nil
	}
	h.dependencyErrors = nil
	h.capabilityConflicts = nil
	h.moduleConflicts = map[string][]string{}

	dag, depErrs, err := buildDAG(h.modules, h.index)
	h.dependencyErrors = append([]string(nil), depErrs...)
	if err != nil {
		return err
	}
	caps, err := collectCapabilities(h.modules)
	h.capabilityConflicts = append([]string(nil), caps.conflicts...)
	h.moduleConflicts = normalizeModuleConflicts(caps.moduleConflicts)
	if err != nil {
		return err
	}
	h.topoOrder = dag.order
	h.lastStableTopo = append([]Module(nil), dag.order...)
	h.topoLayer = dag.layers
	h.capabilityIdx = caps.index
	h.capabilityBindings = caps.bindings
	h.sealed = true
	return nil
}
