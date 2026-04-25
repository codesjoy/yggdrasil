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

package assembly

import (
	"fmt"
	"sort"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/module"
)

type autoModuleState struct {
	spec         module.AutoSpec
	matchedRules []MatchedAutoRule
}

func (p *planner) resolveModules() error {
	all := map[string]module.Module{}
	order := make([]string, 0, len(p.input.Modules))
	for _, mod := range p.input.Modules {
		if mod == nil {
			continue
		}
		name := mod.Name()
		if name == "" {
			continue
		}
		all[name] = mod
		order = append(order, name)
	}
	if err := p.recordDisabledModuleOverrides(all, p.configOverrides.DisabledModules, "config_override", "disabled by yggdrasil.overrides.disable_modules"); err != nil {
		return err
	}
	if err := p.recordDisabledModuleOverrides(all, p.codeOverrides.DisabledModules, "code_override", "disabled by WithPlanOverrides"); err != nil {
		return err
	}

	enabled := map[string]module.Module{}
	for _, name := range order {
		mod := all[name]
		if mod == nil || isDisabledModule(name, p.configOverrides, p.codeOverrides) {
			continue
		}
		if _, ok := requiredModules[name]; ok {
			enabled[name] = mod
			p.moduleReasons[name] = append(p.moduleReasons[name], moduleReason(name))
			continue
		}
		auto, ok := mod.(module.AutoDescribed)
		if !ok {
			enabled[name] = mod
			p.moduleReasons[name] = append(p.moduleReasons[name], moduleReason(name))
			continue
		}
		autoSpec := auto.AutoSpec()
		if err := p.validateAutoModule(mod, autoSpec); err != nil {
			return err
		}
		matchReasons, matchedRules := p.matchAutoModule(mod, autoSpec)
		if _, forceEnabled := p.codeOverrides.EnabledModules[name]; forceEnabled {
			matchReasons = append(matchReasons, "enabled by WithPlanOverrides")
			p.decisions = append(p.decisions, Decision{
				Kind:   "override.enable_module",
				Target: name,
				Value:  "enabled",
				Source: "code_override",
				Reason: "enabled by WithPlanOverrides",
				Stage:  stagePlan,
			})
		}
		if len(matchReasons) == 0 {
			continue
		}
		enabled[name] = mod
		p.autoModules[name] = autoModuleState{
			spec:         autoSpec,
			matchedRules: matchedRules,
		}
		p.moduleReasons[name] = append(p.moduleReasons[name], matchReasons...)
		p.matchedAutoRules = append(p.matchedAutoRules, matchedRules...)
	}
	p.modules = p.expandModuleDependencies(enabled, all)
	for _, mod := range p.modules {
		if _, ok := p.moduleReasons[mod.Name()]; ok {
			continue
		}
		p.moduleReasons[mod.Name()] = append(p.moduleReasons[mod.Name()], "dependency closure")
	}
	return nil
}

func (p *planner) recordDisabledModuleOverrides(
	all map[string]module.Module,
	disabled map[string]struct{},
	source string,
	reason string,
) error {
	for _, name := range sortedKeys(disabled) {
		if _, ok := all[name]; !ok {
			return newError(
				ErrConflictingOverride,
				stagePlan,
				fmt.Sprintf("unknown module override target %q", name),
				nil,
				map[string]string{"module": name},
			)
		}
		if _, protected := requiredModules[name]; protected {
			return newError(
				ErrConflictingOverride,
				stagePlan,
				fmt.Sprintf("module %q cannot be disabled", name),
				nil,
				map[string]string{"module": name},
			)
		}
		p.decisions = append(p.decisions, Decision{
			Kind:   "override.disable_module",
			Target: name,
			Value:  "disabled",
			Source: source,
			Reason: reason,
			Stage:  stagePlan,
		})
	}
	return nil
}

func (p *planner) collectProviders() {
	index := map[string]map[string]struct{}{}
	for _, mod := range p.modules {
		provider, ok := mod.(module.CapabilityProvider)
		if !ok {
			continue
		}
		for _, cap := range provider.Capabilities() {
			if cap.Spec.Name == "" {
				continue
			}
			name := cap.Name
			if name == "" {
				name = mod.Name()
			}
			if _, ok := index[cap.Spec.Name]; !ok {
				index[cap.Spec.Name] = map[string]struct{}{}
			}
			index[cap.Spec.Name][name] = struct{}{}
		}
	}
	for specName, items := range index {
		names := make([]string, 0, len(items))
		for name := range items {
			names = append(names, name)
		}
		sort.Strings(names)
		p.availableProviders[specName] = names
	}
}

func (p *planner) affectedPaths() map[string][]string {
	out := map[string][]string{
		"mode":      {},
		"defaults":  {},
		"chains":    {},
		"overrides": {},
		"modules":   {},
	}
	if p.mode.Name != "" {
		out["mode"] = []string{"yggdrasil.mode"}
	}
	for _, item := range p.matchedAutoRules {
		out["modules"] = append(out["modules"], item.AffectedPaths...)
	}
	for path := range p.selectedDefaults {
		out["defaults"] = append(out["defaults"], capabilityConfigPaths(path)...)
	}
	for path := range p.selectedChains {
		out["chains"] = append(out["chains"], chainConfigPaths(path)...)
	}
	for path := range p.codeOverrides.ForcedDefaults {
		out["overrides"] = append(out["overrides"], "yggdrasil.overrides.force_defaults."+path)
	}
	for path := range p.configOverrides.ForcedDefaults {
		out["overrides"] = append(out["overrides"], "yggdrasil.overrides.force_defaults."+path)
	}
	for path := range p.codeOverrides.ForcedTemplates {
		out["overrides"] = append(out["overrides"], "yggdrasil.overrides.force_templates."+path)
	}
	for path := range p.configOverrides.ForcedTemplates {
		out["overrides"] = append(out["overrides"], "yggdrasil.overrides.force_templates."+path)
	}
	for path := range p.codeOverrides.DisabledAuto {
		out["overrides"] = append(out["overrides"], "yggdrasil.overrides.disable_auto."+path)
	}
	for path := range p.configOverrides.DisabledAuto {
		out["overrides"] = append(out["overrides"], "yggdrasil.overrides.disable_auto."+path)
	}
	for key := range out {
		out[key] = dedupStrings(out[key])
		sort.Strings(out[key])
	}
	return out
}

func isDisabledModule(name string, configOverrides, codeOverrides overrideSet) bool {
	if _, ok := configOverrides.DisabledModules[name]; ok {
		return true
	}
	if _, ok := codeOverrides.DisabledModules[name]; ok {
		return true
	}
	return false
}

func (p *planner) validateAutoModule(mod module.Module, autoSpec module.AutoSpec) error {
	name := mod.Name()
	declared := map[string]module.CapabilitySpec{}
	for _, spec := range autoSpec.Provides {
		if spec.Name == "" {
			return newError(
				ErrInvalidAutoRule,
				stagePlan,
				fmt.Sprintf("module %q auto provides empty capability name", name),
				nil,
				map[string]string{"module": name},
			)
		}
		declared[spec.Name] = spec
	}
	provider, ok := mod.(module.CapabilityProvider)
	if !ok {
		return nil
	}
	actual := map[string]module.CapabilitySpec{}
	for _, cap := range provider.Capabilities() {
		if cap.Spec.Name == "" {
			continue
		}
		actual[cap.Spec.Name] = cap.Spec
	}
	if len(declared) == 0 {
		return nil
	}
	for specName, spec := range declared {
		got, ok := actual[specName]
		if !ok {
			return newError(
				ErrInvalidAutoRule,
				stagePlan,
				fmt.Sprintf(
					"module %q auto spec declares capability %q but does not provide it",
					name,
					specName,
				),
				nil,
				map[string]string{"module": name, "capability": specName},
			)
		}
		if got.Cardinality != spec.Cardinality || got.Type != spec.Type {
			return newError(
				ErrInvalidAutoRule,
				stagePlan,
				fmt.Sprintf(
					"module %q auto spec capability %q does not match runtime capability declaration",
					name,
					specName,
				),
				nil,
				map[string]string{"module": name, "capability": specName},
			)
		}
	}
	for specName := range actual {
		if _, ok := declared[specName]; ok {
			continue
		}
		return newError(
			ErrInvalidAutoRule,
			stagePlan,
			fmt.Sprintf("module %q auto spec is missing capability %q", name, specName),
			nil,
			map[string]string{"module": name, "capability": specName},
		)
	}
	return nil
}

func (p *planner) matchAutoModule(
	mod module.Module,
	autoSpec module.AutoSpec,
) ([]string, []MatchedAutoRule) {
	ctx := module.AutoRuleContext{
		AppName:  p.input.Identity.AppName,
		Snapshot: p.input.Snapshot,
		Mode: module.AutoMode{
			Name:    p.mode.Name,
			Profile: p.mode.Profile,
			Bundle:  p.mode.Bundle,
		},
	}
	reasons := make([]string, 0, len(autoSpec.AutoRules))
	matched := make([]MatchedAutoRule, 0, len(autoSpec.AutoRules))
	for _, rule := range autoSpec.AutoRules {
		if rule == nil || !rule.Match(ctx) {
			continue
		}
		desc := strings.TrimSpace(rule.Describe())
		if desc == "" {
			desc = "matched auto rule"
		}
		reasons = append(reasons, desc)
		matched = append(matched, MatchedAutoRule{
			Module:        mod.Name(),
			Description:   desc,
			AffectedPaths: dedupStrings(rule.AffectedPaths()),
		})
	}
	return reasons, matched
}

func (p *planner) expandModuleDependencies(
	enabled, available map[string]module.Module,
) []module.Module {
	visited := map[string]struct{}{}
	var visit func(string)
	visit = func(name string) {
		if _, ok := visited[name]; ok {
			return
		}
		mod := available[name]
		if mod == nil {
			return
		}
		visited[name] = struct{}{}
		if dep, ok := mod.(module.Dependent); ok {
			for _, next := range dep.DependsOn() {
				next = strings.TrimSpace(next)
				if next == "" {
					continue
				}
				if isDisabledModule(next, p.configOverrides, p.codeOverrides) {
					continue
				}
				visit(next)
				if _, ok := p.moduleReasons[next]; !ok {
					p.moduleReasons[next] = []string{fmt.Sprintf("dependency of %s", name)}
				}
			}
		}
	}
	for name := range enabled {
		visit(name)
	}
	out := make([]module.Module, 0, len(visited))
	for _, mod := range p.input.Modules {
		if mod == nil {
			continue
		}
		if _, ok := visited[mod.Name()]; !ok {
			continue
		}
		out = append(out, mod)
	}
	return out
}

func moduleKind(name string) string {
	if _, ok := frameworkModules[name]; ok {
		return "builtin"
	}
	return "module"
}

func moduleSource(name string) string {
	if _, ok := frameworkModules[name]; ok {
		return "framework"
	}
	return "user"
}

func moduleReason(name string) string {
	if _, ok := requiredModules[name]; ok {
		return "framework default"
	}
	return "explicit option"
}
