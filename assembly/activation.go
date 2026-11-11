// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package assembly

import (
	"fmt"
	"slices"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/module"
)

const (
	activationKindCapability   = "activation.explicit_binding"
	activationKindConfigSource = "activation.config_source"
)

type activationReference struct {
	capability string
	provider   string
	source     string
}

type candidateActivationIndex struct {
	capabilityOwners   map[string]map[string][]module.Module
	capabilitySpecs    map[string]map[string]module.CapabilitySpec
	capabilityNames    map[string]map[string][]string
	configSourceOwners map[string][]module.Module
}

func newCandidateActivationIndex(modules []module.Module) candidateActivationIndex {
	index := candidateActivationIndex{
		capabilityOwners:   map[string]map[string][]module.Module{},
		capabilitySpecs:    map[string]map[string]module.CapabilitySpec{},
		capabilityNames:    map[string]map[string][]string{},
		configSourceOwners: map[string][]module.Module{},
	}
	for _, mod := range modules {
		if mod == nil {
			continue
		}
		moduleName := mod.Name()
		if provider, ok := mod.(module.CapabilityProvider); ok {
			for _, capability := range provider.Capabilities() {
				specName := strings.TrimSpace(capability.Spec.Name)
				if specName == "" {
					continue
				}
				providerName := strings.TrimSpace(capability.Name)
				if providerName == "" {
					providerName = moduleName
				}
				if index.capabilityOwners[specName] == nil {
					index.capabilityOwners[specName] = map[string][]module.Module{}
				}
				index.capabilityOwners[specName][providerName] = appendUniqueModule(
					index.capabilityOwners[specName][providerName],
					mod,
				)
				spec := capability.Spec
				spec.Name = specName
				if index.capabilitySpecs[moduleName] == nil {
					index.capabilitySpecs[moduleName] = map[string]module.CapabilitySpec{}
				}
				index.capabilitySpecs[moduleName][specName] = spec
				if index.capabilityNames[moduleName] == nil {
					index.capabilityNames[moduleName] = map[string][]string{}
				}
				names := index.capabilityNames[moduleName][specName]
				if !slices.Contains(names, providerName) {
					index.capabilityNames[moduleName][specName] = append(
						names,
						providerName,
					)
				}
			}
		}
		if provider, ok := mod.(module.ConfigSourceProvider); ok {
			for kind := range provider.ConfigSourceBuilders() {
				kind = strings.ToLower(strings.TrimSpace(kind))
				if kind == "" {
					continue
				}
				index.configSourceOwners[kind] = appendUniqueModule(
					index.configSourceOwners[kind],
					mod,
				)
			}
		}
	}
	for capability := range index.capabilityOwners {
		for provider := range index.capabilityOwners[capability] {
			sortModules(index.capabilityOwners[capability][provider])
		}
	}
	for kind := range index.configSourceOwners {
		sortModules(index.configSourceOwners[kind])
	}
	for _, capabilities := range index.capabilityNames {
		for capability := range capabilities {
			slices.Sort(capabilities[capability])
		}
	}
	return index
}

func appendUniqueModule(items []module.Module, next module.Module) []module.Module {
	for _, item := range items {
		if item.Name() == next.Name() {
			return items
		}
	}
	return append(items, next)
}

func sortModules(modules []module.Module) {
	slices.SortFunc(modules, func(left, right module.Module) int {
		return strings.Compare(left.Name(), right.Name())
	})
}

func (p *planner) activateExplicitReferences(
	enabled map[string]module.Module,
	index candidateActivationIndex,
) error {
	references, err := p.explicitActivationReferences()
	if err != nil {
		return err
	}
	for _, reference := range references {
		if err := p.activateCapabilityReference(
			enabled,
			index,
			reference,
		); err != nil {
			return err
		}
	}
	return p.activateConfigSourceReferences(enabled, index)
}

func (p *planner) explicitActivationReferences() ([]activationReference, error) {
	if err := p.validateForcedDefaultCapabilities(); err != nil {
		return nil, err
	}
	forced := map[string]activationReference{}
	for capability, provider := range p.configOverrides.ForcedDefaults {
		if isDefaultSelectable(capability) {
			forced[capability] = activationReference{
				capability: capability,
				provider:   provider,
				source:     "config_override",
			}
		}
	}
	for capability, provider := range p.codeOverrides.ForcedDefaults {
		if isDefaultSelectable(capability) {
			forced[capability] = activationReference{
				capability: capability,
				provider:   provider,
				source:     "code_override",
			}
		}
	}

	references := make([]activationReference, 0)
	for capability, providers := range p.input.Resolved.DeclaredCapabilityBindings {
		if _, overridden := forced[capability]; overridden {
			continue
		}
		for _, provider := range providers {
			references = append(references, activationReference{
				capability: capability,
				provider:   provider,
				source:     "explicit_config",
			})
		}
	}
	for _, reference := range forced {
		references = append(references, reference)
	}
	chainReferences, err := p.explicitChainActivationReferences()
	if err != nil {
		return nil, err
	}
	references = append(references, chainReferences...)
	return canonicalActivationReferences(references), nil
}

func (p *planner) validateForcedDefaultCapabilities() error {
	for _, overrides := range []struct {
		source   string
		defaults map[string]string
	}{
		{source: "config_override", defaults: p.configOverrides.ForcedDefaults},
		{source: "code_override", defaults: p.codeOverrides.ForcedDefaults},
	} {
		for _, capability := range sortedKeys(overrides.defaults) {
			if isDefaultSelectable(capability) {
				continue
			}
			return newError(
				ErrUnknownExplicitBinding,
				stagePlan,
				fmt.Sprintf(
					"forced default capability %q is not default-selectable",
					capability,
				),
				nil,
				map[string]string{
					"capability": capability,
					"source":     overrides.source,
				},
			)
		}
	}
	return nil
}

func (p *planner) explicitChainActivationReferences() ([]activationReference, error) {
	paths := []string{
		chainUnaryServer,
		chainStreamServer,
		chainUnaryClient,
		chainStreamClient,
		chainRESTAll,
		chainRESTRPC,
		chainRESTWeb,
	}
	references := make([]activationReference, 0)
	for _, path := range paths {
		if selection, ok := p.codeOverrides.ForcedTemplates[path]; ok {
			items, err := p.explicitTemplateItems(path, selection)
			if err != nil {
				return nil, err
			}
			references = appendActivationItems(
				references,
				capabilityForChain(path),
				items,
				"code_override",
			)
			continue
		}
		if selection, ok := p.configOverrides.ForcedTemplates[path]; ok {
			items, err := p.explicitTemplateItems(path, selection)
			if err != nil {
				return nil, err
			}
			references = appendActivationItems(
				references,
				capabilityForChain(path),
				items,
				"config_override",
			)
			continue
		}
		if selection, ok, err := explicitTemplateValue(p.input.Snapshot, path); err != nil {
			return nil, err
		} else if ok {
			items, err := p.explicitTemplateItems(path, selection)
			if err != nil {
				return nil, err
			}
			references = appendActivationItems(
				references,
				capabilityForChain(path),
				items,
				"explicit_config",
			)
			continue
		}
		if items, ok := explicitChainValue(p.input.Resolved, p.input.Snapshot, path); ok {
			references = appendActivationItems(
				references,
				capabilityForChain(path),
				items,
				"explicit_config",
			)
		}
	}
	return references, nil
}

func (p *planner) explicitTemplateItems(
	path string,
	selection templateSelection,
) ([]string, error) {
	template, err := p.lookupTemplate(path, selection)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), template.Items...), nil
}

func appendActivationItems(
	references []activationReference,
	capability string,
	items []string,
	source string,
) []activationReference {
	for _, item := range items {
		references = append(references, activationReference{
			capability: capability,
			provider:   item,
			source:     source,
		})
	}
	return references
}

func canonicalActivationReferences(
	references []activationReference,
) []activationReference {
	out := make([]activationReference, 0, len(references))
	seen := map[activationReference]struct{}{}
	for _, reference := range references {
		reference.capability = strings.TrimSpace(reference.capability)
		reference.provider = strings.TrimSpace(reference.provider)
		reference.source = strings.TrimSpace(reference.source)
		if reference.capability == "" || reference.provider == "" {
			continue
		}
		if _, ok := seen[reference]; ok {
			continue
		}
		seen[reference] = struct{}{}
		out = append(out, reference)
	}
	slices.SortFunc(out, func(left, right activationReference) int {
		if result := strings.Compare(left.capability, right.capability); result != 0 {
			return result
		}
		if result := strings.Compare(left.provider, right.provider); result != 0 {
			return result
		}
		return strings.Compare(left.source, right.source)
	})
	return out
}

func (p *planner) activateCapabilityReference(
	enabled map[string]module.Module,
	index candidateActivationIndex,
	reference activationReference,
) error {
	owners := index.capabilityOwners[reference.capability][reference.provider]
	if len(owners) == 0 {
		return newError(
			ErrUnknownExplicitBinding,
			stagePlan,
			fmt.Sprintf(
				"capability %q provider %q has no registered module candidate",
				reference.capability,
				reference.provider,
			),
			nil,
			map[string]string{
				"capability": reference.capability,
				"provider":   reference.provider,
				"source":     reference.source,
			},
		)
	}

	if disabled := p.disabledOwners(owners); len(disabled) > 0 {
		return p.activationDisabledError(disabled, reference)
	}
	eligible := p.eligibleOwners(owners)
	switch len(eligible) {
	case 0:
		return p.activationDisabledError(owners, reference)
	case 1:
		mod := eligible[0]
		if _, active := enabled[mod.Name()]; active {
			return nil
		}
		reason := fmt.Sprintf(
			"enabled by %s capability %q provider %q",
			reference.source,
			reference.capability,
			reference.provider,
		)
		p.enableActivatedModule(
			enabled,
			mod,
			activationKindCapability,
			reference.capability+"="+reference.provider,
			reference.source,
			reason,
		)
		return nil
	default:
		return p.ambiguousActivationError(
			eligible,
			reference.capability,
			reference.provider,
			reference.source,
		)
	}
}

func (p *planner) activateConfigSourceReferences(
	enabled map[string]module.Module,
	index candidateActivationIndex,
) error {
	direct := map[string]struct{}{}
	for _, kind := range p.input.DirectConfigSourceKinds {
		kind = strings.ToLower(strings.TrimSpace(kind))
		if kind != "" {
			direct[kind] = struct{}{}
		}
	}
	kinds := append([]string(nil), p.input.ConfigSourceKinds...)
	for index, kind := range kinds {
		kinds[index] = strings.ToLower(strings.TrimSpace(kind))
	}
	slices.Sort(kinds)
	kinds = slices.Compact(kinds)
	for _, kind := range kinds {
		if kind == "" {
			continue
		}
		if _, ok := direct[kind]; ok {
			continue
		}
		if isBuiltinConfigSourceKind(kind) {
			continue
		}
		owners := index.configSourceOwners[kind]
		if len(owners) == 0 {
			continue
		}
		if disabled := p.disabledOwners(owners); len(disabled) > 0 {
			return p.activationDisabledError(disabled, activationReference{
				capability: "config.source",
				provider:   kind,
				source:     "explicit_config",
			})
		}
		eligible := p.eligibleOwners(owners)
		switch len(eligible) {
		case 0:
			return p.activationDisabledError(owners, activationReference{
				capability: "config.source",
				provider:   kind,
				source:     "explicit_config",
			})
		case 1:
			mod := eligible[0]
			if _, active := enabled[mod.Name()]; active {
				continue
			}
			p.enableActivatedModule(
				enabled,
				mod,
				activationKindConfigSource,
				kind,
				"explicit_config",
				fmt.Sprintf(
					"enabled by explicit_config config source kind %q",
					kind,
				),
			)
		default:
			return p.ambiguousActivationError(
				eligible,
				"config.source",
				kind,
				"explicit_config",
			)
		}
	}
	return nil
}

func (p *planner) disabledOwners(owners []module.Module) []module.Module {
	disabled := make([]module.Module, 0, len(owners))
	for _, mod := range owners {
		if p.moduleDisabled(mod.Name()) {
			disabled = append(disabled, mod)
		}
	}
	return disabled
}

func (p *planner) eligibleOwners(owners []module.Module) []module.Module {
	eligible := make([]module.Module, 0, len(owners))
	for _, mod := range owners {
		if p.moduleDisabled(mod.Name()) {
			continue
		}
		eligible = append(eligible, mod)
	}
	return eligible
}

func (p *planner) moduleDisabled(name string) bool {
	return isDisabledModule(name, p.configOverrides, p.codeOverrides)
}

func (p *planner) activationDisabledError(
	owners []module.Module,
	reference activationReference,
) error {
	names := moduleNames(owners)
	moduleName := ""
	if len(names) > 0 {
		moduleName = names[0]
	}
	disabledSources := p.disabledModuleSources(names)
	return newError(
		ErrConflictingOverride,
		stagePlan,
		fmt.Sprintf(
			"module %q is disabled but explicitly selected by %s",
			moduleName,
			activationReferenceLabel(reference),
		),
		nil,
		map[string]string{
			"module":      moduleName,
			"modules":     strings.Join(names, ","),
			"capability":  reference.capability,
			"provider":    reference.provider,
			"source":      strings.Join(disabledSources, ","),
			"selected_by": reference.source,
		},
	)
}

func (p *planner) disabledModuleSources(names []string) []string {
	configDisabled := false
	codeDisabled := false
	for _, name := range names {
		if _, ok := p.configOverrides.DisabledModules[name]; ok {
			configDisabled = true
		}
		if _, ok := p.codeOverrides.DisabledModules[name]; ok {
			codeDisabled = true
		}
	}
	sources := make([]string, 0, 2)
	if configDisabled {
		sources = append(sources, "config_override")
	}
	if codeDisabled {
		sources = append(sources, "code_override")
	}
	return sources
}

func (p *planner) ambiguousActivationError(
	owners []module.Module,
	capability string,
	provider string,
	source string,
) error {
	names := moduleNames(owners)
	return newError(
		ErrAmbiguousModuleActivation,
		stagePlan,
		fmt.Sprintf(
			"%s %q is provided by multiple module candidates %v",
			capability,
			provider,
			names,
		),
		nil,
		map[string]string{
			"capability": capability,
			"provider":   provider,
			"modules":    strings.Join(names, ","),
			"source":     source,
		},
	)
}

func activationReferenceLabel(reference activationReference) string {
	if reference.capability == "" {
		return "configuration"
	}
	if reference.capability == "config.source" {
		return fmt.Sprintf("config source kind %q", reference.provider)
	}
	return fmt.Sprintf(
		"capability %q provider %q",
		reference.capability,
		reference.provider,
	)
}

func moduleNames(modules []module.Module) []string {
	names := make([]string, 0, len(modules))
	for _, mod := range modules {
		names = append(names, mod.Name())
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func (p *planner) enableActivatedModule(
	enabled map[string]module.Module,
	mod module.Module,
	kind string,
	value string,
	source string,
	reason string,
) {
	if _, exists := enabled[mod.Name()]; exists {
		return
	}
	enabled[mod.Name()] = mod
	if auto, ok := mod.(module.AutoDescribed); ok {
		if _, exists := p.autoModules[mod.Name()]; !exists {
			p.autoModules[mod.Name()] = autoModuleState{spec: auto.AutoSpec()}
		}
	}
	p.addModuleReason(mod.Name(), reason)
	p.decisions = append(p.decisions, Decision{
		Kind:   kind,
		Target: mod.Name(),
		Value:  value,
		Source: source,
		Reason: reason,
		Stage:  stagePlan,
	})
}

func (p *planner) addModuleReason(name string, reason string) {
	reason = strings.TrimSpace(reason)
	if name == "" || reason == "" {
		return
	}
	for _, existing := range p.moduleReasons[name] {
		if existing == reason {
			return
		}
	}
	p.moduleReasons[name] = append(p.moduleReasons[name], reason)
}

func isBuiltinConfigSourceKind(kind string) bool {
	switch kind {
	case "file", "env", "flag":
		return true
	default:
		return false
	}
}

func cloneBindings(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(in))
	for key, values := range in {
		out[key] = append([]string(nil), values...)
	}
	return out
}
