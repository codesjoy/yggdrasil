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
	"reflect"
	"sort"
	"strings"
)

// CapabilityCardinality defines cardinality rules.
type CapabilityCardinality int

// Capability cardinality constraints.
const (
	ExactlyOne CapabilityCardinality = iota
	OptionalOne
	Many
	OrderedMany
	NamedOne
)

func (c CapabilityCardinality) String() string {
	switch c {
	case ExactlyOne:
		return "exactly_one"
	case OptionalOne:
		return "optional_one"
	case Many:
		return "many"
	case OrderedMany:
		return "ordered_many"
	case NamedOne:
		return "named_one"
	default:
		return "unknown"
	}
}

// CapabilitySpec is the capability contract declaration.
type CapabilitySpec struct {
	Name        string
	Cardinality CapabilityCardinality
	Type        reflect.Type
}

// Capability is one concrete capability value exposed by a module.
type Capability struct {
	Spec  CapabilitySpec
	Name  string
	Value any
}

// CapabilityProvider exposes one or more capabilities.
type CapabilityProvider interface {
	Capabilities() []Capability
}

type capabilityEntry struct {
	spec      CapabilitySpec
	module    Module
	name      string
	value     any
	valueType reflect.Type
}

type capabilityBinding struct {
	spec    CapabilitySpec
	entries []capabilityEntry
}

type capabilityCollectResult struct {
	index           map[string][]capabilityEntry
	bindings        map[string]capabilityBinding
	conflicts       []string
	moduleConflicts map[string][]string
}

type capabilityCollector struct {
	index           map[string][]capabilityEntry
	specs           map[string]CapabilitySpec
	moduleConflicts map[string][]string
	conflicts       []string
}

func newCapabilityCollector() *capabilityCollector {
	return &capabilityCollector{
		index:           map[string][]capabilityEntry{},
		specs:           map[string]CapabilitySpec{},
		moduleConflicts: map[string][]string{},
	}
}

func (c *capabilityCollector) addConflict(message string, mods ...Module) {
	c.conflicts = append(c.conflicts, message)
	for _, mod := range mods {
		if mod == nil {
			continue
		}
		name := mod.Name()
		c.moduleConflicts[name] = append(c.moduleConflicts[name], message)
	}
}

func (c *capabilityCollector) collect(modules []Module) (bool, error) {
	for _, mod := range modules {
		provider, ok := mod.(CapabilityProvider)
		if !ok {
			continue
		}
		for _, cap := range provider.Capabilities() {
			partial, err := c.addCapability(mod, cap)
			if err != nil {
				return partial, err
			}
		}
	}
	return false, nil
}

func (c *capabilityCollector) addCapability(mod Module, cap Capability) (bool, error) {
	if err := validateCapability(mod, cap); err != nil {
		return false, err
	}
	if cap.Name == "" {
		cap.Name = mod.Name()
	}
	if currentSpec, exists := c.specs[cap.Spec.Name]; exists {
		if message := capabilitySpecConflict(currentSpec, cap.Spec); message != "" {
			c.addConflict(message, mod)
			return true, errors.New(message)
		}
	}
	valueType := reflect.TypeOf(cap.Value)
	if !typeMatches(cap.Spec.Type, valueType) {
		message := fmt.Sprintf(
			"module %q capability %q value type %v does not match %v",
			mod.Name(),
			cap.Spec.Name,
			valueType,
			cap.Spec.Type,
		)
		c.addConflict(message, mod)
		return true, errors.New(message)
	}
	c.specs[cap.Spec.Name] = cap.Spec
	c.index[cap.Spec.Name] = append(c.index[cap.Spec.Name], capabilityEntry{
		spec:      cap.Spec,
		module:    mod,
		name:      cap.Name,
		value:     cap.Value,
		valueType: valueType,
	})
	return false, nil
}

func (c *capabilityCollector) result(withBindings bool) capabilityCollectResult {
	result := capabilityCollectResult{
		index:           c.index,
		conflicts:       slicesUniq(c.conflicts),
		moduleConflicts: normalizeModuleConflicts(c.moduleConflicts),
	}
	if withBindings {
		result.bindings = buildCapabilityBindings(c.index, c.specs)
	}
	return result
}

func collectCapabilities(modules []Module) (capabilityCollectResult, error) {
	collector := newCapabilityCollector()
	partial, err := collector.collect(modules)
	if err != nil {
		if partial {
			return collector.result(false), err
		}
		return capabilityCollectResult{}, err
	}
	addCapabilityConflicts(collector.index, collector.specs, collector.addConflict)
	result := collector.result(true)
	if len(result.conflicts) > 0 {
		return result, errors.New(strings.Join(result.conflicts, "; "))
	}
	return result, nil
}

func validateCapability(mod Module, cap Capability) error {
	if cap.Spec.Name == "" {
		return fmt.Errorf("module %q has capability with empty spec name", mod.Name())
	}
	if cap.Value == nil {
		return fmt.Errorf(
			"module %q capability %q has nil value",
			mod.Name(),
			cap.Spec.Name,
		)
	}
	if cap.Spec.Type == nil {
		return fmt.Errorf(
			"module %q capability %q has empty type",
			mod.Name(),
			cap.Spec.Name,
		)
	}
	return nil
}

func capabilitySpecConflict(current, next CapabilitySpec) string {
	if current.Cardinality != next.Cardinality {
		return fmt.Sprintf(
			"capability %q cardinality mismatch: %s vs %s",
			next.Name,
			current.Cardinality,
			next.Cardinality,
		)
	}
	if current.Type != next.Type {
		return fmt.Sprintf(
			"capability %q type mismatch: %v vs %v",
			next.Name,
			current.Type,
			next.Type,
		)
	}
	return ""
}

func addCapabilityConflicts(
	index map[string][]capabilityEntry,
	specs map[string]CapabilitySpec,
	addConflict func(string, ...Module),
) {
	for specName, entries := range index {
		spec := specs[specName]
		switch spec.Cardinality {
		case ExactlyOne:
			if len(entries) != 1 {
				message := fmt.Sprintf(
					"capability %q requires exactly one provider, got %d",
					specName,
					len(entries),
				)
				addConflict(message, modulesOfEntries(entries)...)
			}
		case OptionalOne:
			if len(entries) > 1 {
				message := fmt.Sprintf(
					"capability %q allows at most one provider, got %d",
					specName,
					len(entries),
				)
				addConflict(message, modulesOfEntries(entries)...)
			}
		case NamedOne, OrderedMany:
			seen := map[string]struct{}{}
			for _, item := range entries {
				if _, ok := seen[item.name]; ok {
					message := fmt.Sprintf(
						"capability %q has duplicate provider name %q",
						specName,
						item.name,
					)
					addConflict(message, modulesOfEntries(entries)...)
					continue
				}
				seen[item.name] = struct{}{}
			}
		}
	}
}

func buildCapabilityBindings(
	index map[string][]capabilityEntry,
	specs map[string]CapabilitySpec,
) map[string]capabilityBinding {
	specNames := make([]string, 0, len(index))
	for specName := range index {
		specNames = append(specNames, specName)
	}
	sort.Strings(specNames)
	bindings := make(map[string]capabilityBinding, len(specNames))
	for _, specName := range specNames {
		spec := specs[specName]
		entries := append([]capabilityEntry(nil), index[specName]...)
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].name != entries[j].name {
				return entries[i].name < entries[j].name
			}
			return entries[i].module.Name() < entries[j].module.Name()
		})
		bindings[specName] = capabilityBinding{
			spec:    spec,
			entries: entries,
		}
	}
	return bindings
}

func typeMatches(expect reflect.Type, actual reflect.Type) bool {
	if expect == nil {
		return true
	}
	if actual == nil {
		return false
	}
	if expect.Kind() == reflect.Interface {
		return actual.Implements(expect)
	}
	return actual.AssignableTo(expect)
}

func castValue[T any](value any) (T, error) {
	var zero T
	out, ok := value.(T)
	if !ok {
		return zero, fmt.Errorf("capability value type mismatch: %T", value)
	}
	return out, nil
}

func (h *Hub) capabilityEntries(spec CapabilitySpec) ([]capabilityEntry, error) {
	if spec.Name == "" {
		return nil, errors.New("capability spec name is required")
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if !h.sealed {
		return nil, errHubNotSealed
	}
	entries := append([]capabilityEntry(nil), h.capabilityIdx[spec.Name]...)
	for _, item := range entries {
		if !typeMatches(spec.Type, item.valueType) {
			return nil, fmt.Errorf("capability %q provider %q type mismatch", spec.Name, item.name)
		}
	}
	return entries, nil
}

func modulesOfEntries(entries []capabilityEntry) []Module {
	seen := map[string]struct{}{}
	out := make([]Module, 0, len(entries))
	for _, entry := range entries {
		name := entry.module.Name()
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, entry.module)
	}
	return out
}

func normalizeModuleConflicts(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(in))
	for key, items := range in {
		out[key] = slicesUniq(items)
	}
	return out
}

func slicesUniq(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func ensureResolveCardinality(spec CapabilitySpec, want CapabilityCardinality) error {
	if spec.Cardinality != want {
		return fmt.Errorf(
			"capability %q resolve helper requires %s cardinality, got %s",
			spec.Name,
			want,
			spec.Cardinality,
		)
	}
	return nil
}

// ResolveExactlyOne resolves an exactly-one capability.
func ResolveExactlyOne[T any](h *Hub, spec CapabilitySpec) (T, error) {
	var zero T
	if err := ensureResolveCardinality(spec, ExactlyOne); err != nil {
		return zero, err
	}
	entries, err := h.capabilityEntries(spec)
	if err != nil {
		return zero, err
	}
	if len(entries) != 1 {
		return zero, fmt.Errorf(
			"capability %q requires exactly one provider, got %d",
			spec.Name,
			len(entries),
		)
	}
	return castValue[T](entries[0].value)
}

// ResolveOptionalOne resolves an optional-one capability.
func ResolveOptionalOne[T any](h *Hub, spec CapabilitySpec) (T, bool, error) {
	var zero T
	if err := ensureResolveCardinality(spec, OptionalOne); err != nil {
		return zero, false, err
	}
	entries, err := h.capabilityEntries(spec)
	if err != nil {
		return zero, false, err
	}
	if len(entries) == 0 {
		return zero, false, nil
	}
	if len(entries) > 1 {
		return zero, false, fmt.Errorf(
			"capability %q allows at most one provider, got %d",
			spec.Name,
			len(entries),
		)
	}
	out, err := castValue[T](entries[0].value)
	if err != nil {
		return zero, false, err
	}
	return out, true, nil
}

// ResolveMany resolves many capability providers.
func ResolveMany[T any](h *Hub, spec CapabilitySpec) ([]T, error) {
	if err := ensureResolveCardinality(spec, Many); err != nil {
		return nil, err
	}
	entries, err := h.capabilityEntries(spec)
	if err != nil {
		return nil, err
	}
	out := make([]T, 0, len(entries))
	for _, item := range entries {
		v, castErr := castValue[T](item.value)
		if castErr != nil {
			return nil, castErr
		}
		out = append(out, v)
	}
	return out, nil
}

// ResolveNamed resolves one provider by name.
func ResolveNamed[T any](h *Hub, spec CapabilitySpec, name string) (T, error) {
	var zero T
	if err := ensureResolveCardinality(spec, NamedOne); err != nil {
		return zero, err
	}
	entries, err := h.capabilityEntries(spec)
	if err != nil {
		return zero, err
	}
	for _, item := range entries {
		if item.name != name {
			continue
		}
		return castValue[T](item.value)
	}
	return zero, fmt.Errorf("capability %q provider %q not found", spec.Name, name)
}

// ResolveOrdered resolves providers in explicit config order.
func ResolveOrdered[T any](h *Hub, spec CapabilitySpec, names []string) ([]T, error) {
	if err := ensureResolveCardinality(spec, OrderedMany); err != nil {
		return nil, err
	}
	entries, err := h.capabilityEntries(spec)
	if err != nil {
		return nil, err
	}
	index := map[string]capabilityEntry{}
	for _, item := range entries {
		if _, exists := index[item.name]; exists {
			return nil, fmt.Errorf(
				"capability %q has duplicate provider name %q",
				spec.Name,
				item.name,
			)
		}
		index[item.name] = item
	}
	out := make([]T, 0, len(names))
	seen := map[string]struct{}{}
	for _, item := range names {
		if _, ok := seen[item]; ok {
			return nil, fmt.Errorf(
				"capability %q provider %q duplicated in ordered list",
				spec.Name,
				item,
			)
		}
		seen[item] = struct{}{}
		entry, ok := index[item]
		if !ok {
			return nil, fmt.Errorf("capability %q provider %q not found", spec.Name, item)
		}
		v, castErr := castValue[T](entry.value)
		if castErr != nil {
			return nil, castErr
		}
		out = append(out, v)
	}
	return out, nil
}
