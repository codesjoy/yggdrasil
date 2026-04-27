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
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/module"
)

type planner struct {
	input    Input
	registry plannerRegistry

	mode            modeDefinition
	configOverrides overrideSet
	codeOverrides   overrideSet

	modules            []module.Module
	moduleReasons      map[string][]string
	availableProviders map[string][]string
	autoModules        map[string]autoModuleState

	selectedDefaults  map[string]string
	defaultSources    map[string]string
	defaultCandidates map[string][]DefaultCandidate
	selectedChains    map[string]Chain
	chainSources      map[string]string
	matchedAutoRules  []MatchedAutoRule

	decisions []Decision
	warnings  []Warning
	conflicts []Conflict
}

// Plan builds one executable plan result without instantiating runtime objects.
func Plan(ctx context.Context, in Input) (*Result, error) {
	_ = ctx
	p := &planner{
		input:              in,
		configOverrides:    overridesFromConfig(in.Resolved.Overrides),
		codeOverrides:      newOverrideSet(),
		moduleReasons:      map[string][]string{},
		availableProviders: map[string][]string{},
		autoModules:        map[string]autoModuleState{},
		selectedDefaults:   map[string]string{},
		defaultSources:     map[string]string{},
		defaultCandidates:  map[string][]DefaultCandidate{},
		selectedChains:     map[string]Chain{},
		chainSources:       map[string]string{},
	}
	p.registry = newDefaultPlannerRegistry()
	for _, item := range in.Overrides {
		if item == nil {
			continue
		}
		item.apply(&p.codeOverrides)
	}
	return p.build()
}

// DryRun renders only the stable assembly specification for one planner input.
func DryRun(ctx context.Context, in Input) (*Spec, error) {
	result, err := Plan(ctx, in)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.Spec, nil
}

func (p *planner) build() (*Result, error) {
	if err := p.resolveMode(); err != nil {
		return nil, err
	}
	if err := p.resolveModules(); err != nil {
		return nil, err
	}
	if err := p.validateModuleIsolation(); err != nil {
		return nil, err
	}
	p.collectProviders()
	if err := p.resolveDefaults(); err != nil {
		return nil, err
	}
	if err := p.resolveChains(); err != nil {
		return nil, err
	}
	effective, err := p.buildEffectiveResolved()
	if err != nil {
		return nil, err
	}
	bindings := compileCapabilityBindings(effective)
	if err := p.validateBindings(bindings); err != nil {
		return nil, err
	}

	spec := p.buildSpec()
	hash, err := Hash(spec)
	if err != nil {
		return nil, err
	}
	return &Result{
		Spec:                  spec,
		Modules:               append([]module.Module(nil), p.modules...),
		EffectiveResolved:     effective,
		CapabilityBindings:    bindings,
		ChainSources:          cloneStringMap(p.chainSources),
		DefaultSources:        cloneStringMap(p.defaultSources),
		AffectedPathsByDomain: p.affectedPaths(),
		BusinessInputPaths:    append([]string(nil), businessInputPaths...),
		MatchedAutoRules:      cloneMatchedAutoRules(p.matchedAutoRules),
		DefaultCandidates:     cloneDefaultCandidateMap(p.defaultCandidates),
		Hash:                  hash,
	}, nil
}

func (p *planner) validateModuleIsolation() error {
	for _, mod := range p.modules {
		reporter, ok := mod.(module.IsolationReporter)
		if !ok || reporter.IsolationMode() != module.IsolationModeRequiresProcessDefaults ||
			p.input.ProcessDefaults {
			continue
		}
		warning := Warning{
			Code: "ModuleRequiresProcessDefaults",
			Message: fmt.Sprintf(
				"module %q declares process-default dependencies but process defaults are disabled",
				mod.Name(),
			),
		}
		p.warnings = append(p.warnings, warning)
		if p.input.Resolved.Admin.Validation.Strict {
			return newError(
				ErrIsolationRequiresProcessDefaults,
				stagePlan,
				warning.Message,
				nil,
				map[string]string{"module": mod.Name()},
			)
		}
	}
	return nil
}

func (p *planner) resolveMode() error {
	modeName := strings.TrimSpace(p.input.Resolved.Mode)
	if modeName == "" {
		return nil
	}
	if definition, ok := p.registry.mode(modeName); ok {
		p.mode = definition
		p.decisions = append(p.decisions, Decision{
			Kind:   "mode",
			Target: "yggdrasil.mode",
			Value:  modeName,
			Source: "config",
			Reason: "resolved built-in mode",
			Stage:  stagePlan,
		})
		return nil
	}
	return newError(
		ErrInvalidMode,
		stagePlan,
		fmt.Sprintf("unsupported mode %q", modeName),
		nil,
		map[string]string{"mode": modeName},
	)
}

func (p *planner) validateBindings(bindings map[string][]string) error {
	for capability, names := range bindings {
		for _, name := range names {
			if err := p.requireProvider(capability, name, ErrUnknownExplicitBinding); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *planner) buildSpec() *Spec {
	spec := &Spec{
		Identity: p.input.Identity,
		Mode: Mode{
			Name:    p.mode.Name,
			Profile: p.mode.Profile,
			Bundle:  p.mode.Bundle,
		},
		Defaults:  cloneStringMap(p.selectedDefaults),
		Chains:    cloneChains(p.selectedChains),
		Decisions: append([]Decision(nil), p.decisions...),
		Warnings:  append([]Warning(nil), p.warnings...),
		Conflicts: append([]Conflict(nil), p.conflicts...),
	}
	for _, mod := range p.modules {
		ref := ModuleRef{
			Name:    mod.Name(),
			Kind:    moduleKind(mod.Name()),
			Source:  moduleSource(mod.Name()),
			Reasons: append([]string(nil), p.moduleReasons[mod.Name()]...),
		}
		spec.Modules = append(spec.Modules, ref)
	}
	sort.Slice(
		spec.Modules,
		func(i, j int) bool { return spec.Modules[i].Name < spec.Modules[j].Name },
	)
	sort.Slice(spec.Decisions, func(i, j int) bool {
		if spec.Decisions[i].Kind != spec.Decisions[j].Kind {
			return spec.Decisions[i].Kind < spec.Decisions[j].Kind
		}
		if spec.Decisions[i].Target != spec.Decisions[j].Target {
			return spec.Decisions[i].Target < spec.Decisions[j].Target
		}
		if spec.Decisions[i].Source != spec.Decisions[j].Source {
			return spec.Decisions[i].Source < spec.Decisions[j].Source
		}
		return spec.Decisions[i].Value < spec.Decisions[j].Value
	})
	return spec
}
