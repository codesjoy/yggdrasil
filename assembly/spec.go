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

// Package assembly contains the declarative plan/spec types used by the
// high-level Open/Prepare/Compose flow.
package assembly

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
)

// IdentitySpec contains resolved app identity information.
type IdentitySpec struct {
	AppName string `json:"app_name" yaml:"app_name"`
}

// Mode contains the final mode identity used for planning.
type Mode struct {
	Name    string `json:"name"    yaml:"name"`
	Profile string `json:"profile" yaml:"profile"`
	Bundle  string `json:"bundle"  yaml:"bundle"`
}

// ModuleRef is the canonical module reference stored in the plan.
type ModuleRef struct {
	Name    string   `json:"name"              yaml:"name"`
	Kind    string   `json:"kind"              yaml:"kind"`
	Source  string   `json:"source"            yaml:"source"`
	Reasons []string `json:"reasons,omitempty" yaml:"reasons,omitempty"`
}

// Chain is the fully expanded chain representation stored in the spec.
type Chain struct {
	Template string   `json:"template,omitempty" yaml:"template,omitempty"`
	Version  string   `json:"version,omitempty"  yaml:"version,omitempty"`
	Items    []string `json:"items,omitempty"    yaml:"items,omitempty"`
}

// Decision describes one deterministic planning decision.
type Decision struct {
	Kind   string `json:"kind"   yaml:"kind"`
	Target string `json:"target" yaml:"target"`
	Value  string `json:"value"  yaml:"value"`
	Source string `json:"source" yaml:"source"`
	Reason string `json:"reason" yaml:"reason"`
	Stage  string `json:"stage"  yaml:"stage"`
}

// Warning is one non-fatal planning warning.
type Warning struct {
	Code    string `json:"code"    yaml:"code"`
	Message string `json:"message" yaml:"message"`
}

// Conflict is one planning conflict item.
type Conflict struct {
	Code    string `json:"code"    yaml:"code"`
	Message string `json:"message" yaml:"message"`
}

// Spec is the canonical declarative planning result.
type Spec struct {
	Identity IdentitySpec      `json:"identity"           yaml:"identity"`
	Mode     Mode              `json:"mode"               yaml:"mode"`
	Modules  []ModuleRef       `json:"modules,omitempty"  yaml:"modules,omitempty"`
	Defaults map[string]string `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	Chains   map[string]Chain  `json:"chains,omitempty"   yaml:"chains,omitempty"`

	Decisions []Decision `json:"decisions,omitempty" yaml:"decisions,omitempty"`
	Warnings  []Warning  `json:"warnings,omitempty"  yaml:"warnings,omitempty"`
	Conflicts []Conflict `json:"conflicts,omitempty" yaml:"conflicts,omitempty"`
}

// Template is one named and versioned built-in chain template.
type Template struct {
	Name    string   `json:"name"    yaml:"name"`
	Version string   `json:"version" yaml:"version"`
	Items   []string `json:"items"   yaml:"items"`
}

// MatchedAutoRule records one auto rule that enabled a module.
type MatchedAutoRule struct {
	Module        string   `json:"module"                   yaml:"module"`
	Description   string   `json:"description"              yaml:"description"`
	AffectedPaths []string `json:"affected_paths,omitempty" yaml:"affected_paths,omitempty"`
}

// DefaultCandidate records one module fallback default candidate.
type DefaultCandidate struct {
	Module   string `json:"module"             yaml:"module"`
	Provider string `json:"provider"           yaml:"provider"`
	Source   string `json:"source"             yaml:"source"`
	Score    int    `json:"score"              yaml:"score"`
	Selected bool   `json:"selected,omitempty" yaml:"selected,omitempty"`
}

// Input is the immutable planner input.
type Input struct {
	Identity  IdentitySpec
	Resolved  settings.Resolved
	Snapshot  config.Snapshot
	Modules   []module.Module
	Overrides []Override
}

// Result is the executable planner result used by App.Prepare and Reload.
type Result struct {
	Spec *Spec

	Modules               []module.Module
	EffectiveResolved     settings.Resolved
	CapabilityBindings    map[string][]string
	ChainSources          map[string]string
	DefaultSources        map[string]string
	AffectedPathsByDomain map[string][]string
	BusinessInputPaths    []string
	MatchedAutoRules      []MatchedAutoRule
	DefaultCandidates     map[string][]DefaultCandidate
	Hash                  string
}

// ModuleDiffEntry describes one module add/remove change.
type ModuleDiffEntry struct {
	Name    string   `json:"name"              yaml:"name"`
	Kind    string   `json:"kind"              yaml:"kind"`
	Source  string   `json:"source"            yaml:"source"`
	Action  string   `json:"action"            yaml:"action"`
	Reasons []string `json:"reasons,omitempty" yaml:"reasons,omitempty"`
}

// ValueDiffEntry describes one scalar change.
type ValueDiffEntry struct {
	Target string `json:"target"        yaml:"target"`
	Old    string `json:"old,omitempty" yaml:"old,omitempty"`
	New    string `json:"new,omitempty" yaml:"new,omitempty"`
}

// ChainDiffEntry describes one chain expansion change.
type ChainDiffEntry struct {
	Target string `json:"target" yaml:"target"`
	Old    Chain  `json:"old"    yaml:"old"`
	New    Chain  `json:"new"    yaml:"new"`
}

// SpecDiff is the stable plan diff surface for diagnostics and reload decisions.
type SpecDiff struct {
	HasChanges bool `json:"has_changes" yaml:"has_changes"`

	AffectedDomains []string          `json:"affected_domains,omitempty" yaml:"affected_domains,omitempty"`
	Mode            *ValueDiffEntry   `json:"mode,omitempty"             yaml:"mode,omitempty"`
	Modules         []ModuleDiffEntry `json:"modules,omitempty"          yaml:"modules,omitempty"`
	Defaults        []ValueDiffEntry  `json:"defaults,omitempty"         yaml:"defaults,omitempty"`
	Chains          []ChainDiffEntry  `json:"chains,omitempty"           yaml:"chains,omitempty"`
	Overrides       []ValueDiffEntry  `json:"overrides,omitempty"        yaml:"overrides,omitempty"`
}

type canonicalKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type canonicalChain struct {
	Key      string   `json:"key"`
	Template string   `json:"template,omitempty"`
	Version  string   `json:"version,omitempty"`
	Items    []string `json:"items,omitempty"`
}

type canonicalSpec struct {
	Identity  IdentitySpec     `json:"identity"`
	Mode      Mode             `json:"mode"`
	Modules   []ModuleRef      `json:"modules,omitempty"`
	Defaults  []canonicalKV    `json:"defaults,omitempty"`
	Chains    []canonicalChain `json:"chains,omitempty"`
	Decisions []Decision       `json:"decisions,omitempty"`
	Warnings  []Warning        `json:"warnings,omitempty"`
	Conflicts []Conflict       `json:"conflicts,omitempty"`
}

// Explain renders one canonical Spec document in JSON.
func Explain(spec *Spec) ([]byte, error) {
	return json.MarshalIndent(toCanonicalSpec(spec), "", "  ")
}

// Hash computes one stable SHA-256 hash over the canonical Spec.
func Hash(spec *Spec) (string, error) {
	data, err := json.Marshal(toCanonicalSpec(spec))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Diff computes one stable diff between two specs.
func Diff(oldSpec, newSpec *Spec) (*SpecDiff, error) {
	oldCanonical := toCanonicalSpec(oldSpec)
	newCanonical := toCanonicalSpec(newSpec)
	diff := &SpecDiff{}
	domains := map[string]struct{}{}

	if oldCanonical.Mode != newCanonical.Mode {
		diff.Mode = &ValueDiffEntry{
			Target: "mode",
			Old:    oldCanonical.Mode.Name,
			New:    newCanonical.Mode.Name,
		}
		domains["mode"] = struct{}{}
	}

	oldModules := make(map[string]ModuleRef, len(oldCanonical.Modules))
	newModules := make(map[string]ModuleRef, len(newCanonical.Modules))
	for _, item := range oldCanonical.Modules {
		oldModules[item.Name] = item
	}
	for _, item := range newCanonical.Modules {
		newModules[item.Name] = item
	}
	for name, item := range oldModules {
		if _, ok := newModules[name]; !ok {
			diff.Modules = append(diff.Modules, ModuleDiffEntry{
				Name:    item.Name,
				Kind:    item.Kind,
				Source:  item.Source,
				Action:  "removed",
				Reasons: append([]string(nil), item.Reasons...),
			})
		}
	}
	for name, item := range newModules {
		if _, ok := oldModules[name]; !ok {
			diff.Modules = append(diff.Modules, ModuleDiffEntry{
				Name:    item.Name,
				Kind:    item.Kind,
				Source:  item.Source,
				Action:  "added",
				Reasons: append([]string(nil), item.Reasons...),
			})
		}
	}
	if len(diff.Modules) > 0 {
		domains["modules"] = struct{}{}
	}
	sort.Slice(
		diff.Modules,
		func(i, j int) bool { return diff.Modules[i].Name < diff.Modules[j].Name },
	)

	oldDefaults := canonicalKVMap(oldCanonical.Defaults)
	newDefaults := canonicalKVMap(newCanonical.Defaults)
	for _, key := range unionKeys(oldDefaults, newDefaults) {
		if oldDefaults[key] == newDefaults[key] {
			continue
		}
		diff.Defaults = append(diff.Defaults, ValueDiffEntry{
			Target: key,
			Old:    oldDefaults[key],
			New:    newDefaults[key],
		})
	}
	if len(diff.Defaults) > 0 {
		domains["defaults"] = struct{}{}
	}

	oldChains := canonicalChainMap(oldCanonical.Chains)
	newChains := canonicalChainMap(newCanonical.Chains)
	for _, key := range unionKeys(oldChains, newChains) {
		if chainsEqual(oldChains[key], newChains[key]) {
			continue
		}
		diff.Chains = append(diff.Chains, ChainDiffEntry{
			Target: key,
			Old: Chain{
				Template: oldChains[key].Template,
				Version:  oldChains[key].Version,
				Items:    append([]string(nil), oldChains[key].Items...),
			},
			New: Chain{
				Template: newChains[key].Template,
				Version:  newChains[key].Version,
				Items:    append([]string(nil), newChains[key].Items...),
			},
		})
	}
	if len(diff.Chains) > 0 {
		domains["chains"] = struct{}{}
	}

	oldOverrides := overrideDecisions(oldCanonical.Decisions)
	newOverrides := overrideDecisions(newCanonical.Decisions)
	for _, key := range unionKeys(oldOverrides, newOverrides) {
		if oldOverrides[key] == newOverrides[key] {
			continue
		}
		diff.Overrides = append(diff.Overrides, ValueDiffEntry{
			Target: key,
			Old:    oldOverrides[key],
			New:    newOverrides[key],
		})
	}
	if len(diff.Overrides) > 0 {
		domains["overrides"] = struct{}{}
	}

	diff.HasChanges = diff.Mode != nil ||
		len(diff.Modules) > 0 ||
		len(diff.Defaults) > 0 ||
		len(diff.Chains) > 0 ||
		len(diff.Overrides) > 0
	diff.AffectedDomains = unionKeys(domains, map[string]struct{}{})

	oldHash, err := Hash(oldSpec)
	if err != nil {
		return nil, err
	}
	newHash, err := Hash(newSpec)
	if err != nil {
		return nil, err
	}
	if oldHash == newHash && diff.HasChanges {
		return nil, NewError(
			ErrPlanDiffInconsistent,
			stagePlan,
			"plan diff reports changes but canonical hashes are equal",
			nil,
			map[string]string{"old_hash": oldHash, "new_hash": newHash},
		)
	}
	if oldHash != newHash && !diff.HasChanges {
		return nil, NewError(
			ErrPlanDiffInconsistent,
			stagePlan,
			"plan diff reports no changes but canonical hashes differ",
			nil,
			map[string]string{"old_hash": oldHash, "new_hash": newHash},
		)
	}
	return diff, nil
}

func toCanonicalSpec(spec *Spec) canonicalSpec {
	if spec == nil {
		return canonicalSpec{}
	}
	out := canonicalSpec{
		Identity:  spec.Identity,
		Mode:      spec.Mode,
		Modules:   append([]ModuleRef(nil), spec.Modules...),
		Decisions: append([]Decision(nil), spec.Decisions...),
		Warnings:  append([]Warning(nil), spec.Warnings...),
		Conflicts: append([]Conflict(nil), spec.Conflicts...),
	}
	sort.Slice(
		out.Modules,
		func(i, j int) bool { return out.Modules[i].Name < out.Modules[j].Name },
	)
	sort.Slice(out.Decisions, func(i, j int) bool {
		if out.Decisions[i].Kind != out.Decisions[j].Kind {
			return out.Decisions[i].Kind < out.Decisions[j].Kind
		}
		if out.Decisions[i].Target != out.Decisions[j].Target {
			return out.Decisions[i].Target < out.Decisions[j].Target
		}
		if out.Decisions[i].Source != out.Decisions[j].Source {
			return out.Decisions[i].Source < out.Decisions[j].Source
		}
		return out.Decisions[i].Value < out.Decisions[j].Value
	})
	sort.Slice(out.Warnings, func(i, j int) bool {
		if out.Warnings[i].Code != out.Warnings[j].Code {
			return out.Warnings[i].Code < out.Warnings[j].Code
		}
		return out.Warnings[i].Message < out.Warnings[j].Message
	})
	sort.Slice(out.Conflicts, func(i, j int) bool {
		if out.Conflicts[i].Code != out.Conflicts[j].Code {
			return out.Conflicts[i].Code < out.Conflicts[j].Code
		}
		return out.Conflicts[i].Message < out.Conflicts[j].Message
	})
	for key, value := range spec.Defaults {
		out.Defaults = append(out.Defaults, canonicalKV{Key: key, Value: value})
	}
	sort.Slice(
		out.Defaults,
		func(i, j int) bool { return out.Defaults[i].Key < out.Defaults[j].Key },
	)
	for key, value := range spec.Chains {
		out.Chains = append(out.Chains, canonicalChain{
			Key:      key,
			Template: value.Template,
			Version:  value.Version,
			Items:    append([]string(nil), value.Items...),
		})
	}
	sort.Slice(out.Chains, func(i, j int) bool { return out.Chains[i].Key < out.Chains[j].Key })
	return out
}

func dedupStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func sortedKeys[T any](items map[string]T) []string {
	out := make([]string, 0, len(items))
	for key := range items {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneChains(in map[string]Chain) map[string]Chain {
	if len(in) == 0 {
		return map[string]Chain{}
	}
	out := make(map[string]Chain, len(in))
	for key, value := range in {
		out[key] = Chain{
			Template: value.Template,
			Version:  value.Version,
			Items:    append([]string(nil), value.Items...),
		}
	}
	return out
}

func cloneMatchedAutoRules(in []MatchedAutoRule) []MatchedAutoRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]MatchedAutoRule, 0, len(in))
	for _, item := range in {
		out = append(out, MatchedAutoRule{
			Module:        item.Module,
			Description:   item.Description,
			AffectedPaths: append([]string(nil), item.AffectedPaths...),
		})
	}
	return out
}

func cloneDefaultCandidateMap(in map[string][]DefaultCandidate) map[string][]DefaultCandidate {
	if len(in) == 0 {
		return map[string][]DefaultCandidate{}
	}
	out := make(map[string][]DefaultCandidate, len(in))
	for key, items := range in {
		next := make([]DefaultCandidate, 0, len(items))
		next = append(next, items...)
		out[key] = next
	}
	return out
}

func canonicalKVMap(items []canonicalKV) map[string]string {
	out := make(map[string]string, len(items))
	for _, item := range items {
		out[item.Key] = item.Value
	}
	return out
}

func canonicalChainMap(items []canonicalChain) map[string]canonicalChain {
	out := make(map[string]canonicalChain, len(items))
	for _, item := range items {
		out[item.Key] = item
	}
	return out
}

func unionKeys[T any](left, right map[string]T) []string {
	set := map[string]struct{}{}
	for key := range left {
		set[key] = struct{}{}
	}
	for key := range right {
		set[key] = struct{}{}
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func overrideDecisions(items []Decision) map[string]string {
	out := map[string]string{}
	for _, item := range items {
		if !strings.HasPrefix(item.Kind, "override.") {
			continue
		}
		out[item.Kind+":"+item.Target] = item.Value + "|" + item.Source
	}
	return out
}

func chainsEqual(left, right canonicalChain) bool {
	if left.Template != right.Template || left.Version != right.Version {
		return false
	}
	if len(left.Items) != len(right.Items) {
		return false
	}
	for i := range left.Items {
		if left.Items[i] != right.Items[i] {
			return false
		}
	}
	return true
}
