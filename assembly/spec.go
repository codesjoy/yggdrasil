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
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
)

// IdentitySpec contains resolved app identity information.
type IdentitySpec struct {
	AppName   string            `json:"app_name"            yaml:"app_name"`
	Namespace string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Version   string            `json:"version,omitempty"   yaml:"version,omitempty"`
	Region    string            `json:"region,omitempty"    yaml:"region,omitempty"`
	Zone      string            `json:"zone,omitempty"      yaml:"zone,omitempty"`
	Campus    string            `json:"campus,omitempty"    yaml:"campus,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"  yaml:"metadata,omitempty"`
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
	Identity        IdentitySpec
	Resolved        settings.Resolved
	Snapshot        config.Snapshot
	Modules         []module.Module
	Overrides       []Override
	ProcessDefaults bool
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

// ModuleReasonDiffEntry describes diagnostic reason changes for one module.
type ModuleReasonDiffEntry struct {
	Name string   `json:"name"          yaml:"name"`
	Old  []string `json:"old,omitempty" yaml:"old,omitempty"`
	New  []string `json:"new,omitempty" yaml:"new,omitempty"`
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

	AffectedDomains []string                `json:"affected_domains,omitempty" yaml:"affected_domains,omitempty"`
	Mode            *ValueDiffEntry         `json:"mode,omitempty"             yaml:"mode,omitempty"`
	Modules         []ModuleDiffEntry       `json:"modules,omitempty"          yaml:"modules,omitempty"`
	ModuleReasons   []ModuleReasonDiffEntry `json:"module_reasons,omitempty"   yaml:"module_reasons,omitempty"`
	Identity        []ValueDiffEntry        `json:"identity,omitempty"         yaml:"identity,omitempty"`
	Defaults        []ValueDiffEntry        `json:"defaults,omitempty"         yaml:"defaults,omitempty"`
	Chains          []ChainDiffEntry        `json:"chains,omitempty"           yaml:"chains,omitempty"`
	Overrides       []ValueDiffEntry        `json:"overrides,omitempty"        yaml:"overrides,omitempty"`
	Decisions       []ValueDiffEntry        `json:"decisions,omitempty"        yaml:"decisions,omitempty"`
	Warnings        []ValueDiffEntry        `json:"warnings,omitempty"         yaml:"warnings,omitempty"`
	Conflicts       []ValueDiffEntry        `json:"conflicts,omitempty"        yaml:"conflicts,omitempty"`
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
	diff := buildSpecDiff(oldCanonical, newCanonical)
	if err := validateSpecDiff(oldSpec, newSpec, diff); err != nil {
		return nil, err
	}
	return diff, nil
}

func buildSpecDiff(oldCanonical, newCanonical canonicalSpec) *SpecDiff {
	diff := &SpecDiff{}
	domains := map[string]struct{}{}
	diffSpecMetadata(diff, oldCanonical, newCanonical, domains)
	diffSpecDiagnostics(diff, oldCanonical, newCanonical, domains)
	diff.HasChanges = specDiffHasChanges(diff)
	diff.AffectedDomains = sortedKeys(domains)
	return diff
}

func diffSpecMetadata(
	diff *SpecDiff,
	oldCanonical, newCanonical canonicalSpec,
	domains map[string]struct{},
) {
	if oldCanonical.Mode != newCanonical.Mode {
		diff.Mode = &ValueDiffEntry{
			Target: "mode",
			Old:    oldCanonical.Mode.Name,
			New:    newCanonical.Mode.Name,
		}
		domains["mode"] = struct{}{}
	}

	diff.Identity = diffValueSets(
		identityDiffMap(oldCanonical.Identity),
		identityDiffMap(newCanonical.Identity),
	)
	if len(diff.Identity) > 0 {
		domains["identity"] = struct{}{}
	}

	diffModules(diff, oldCanonical.Modules, newCanonical.Modules)
	if len(diff.Modules) > 0 {
		domains["modules"] = struct{}{}
	}
	if len(diff.ModuleReasons) > 0 {
		domains["module_reasons"] = struct{}{}
	}

	oldDefaults := canonicalKVMap(oldCanonical.Defaults)
	newDefaults := canonicalKVMap(newCanonical.Defaults)
	diff.Defaults = diffValueSets(oldDefaults, newDefaults)
	if len(diff.Defaults) > 0 {
		domains["defaults"] = struct{}{}
	}

	diffChains(diff, oldCanonical.Chains, newCanonical.Chains)
	if len(diff.Chains) > 0 {
		domains["chains"] = struct{}{}
	}

	oldOverrides := overrideDecisions(oldCanonical.Decisions)
	newOverrides := overrideDecisions(newCanonical.Decisions)
	diff.Overrides = diffValueSets(oldOverrides, newOverrides)
	if len(diff.Overrides) > 0 {
		domains["overrides"] = struct{}{}
	}
}

func diffSpecDiagnostics(
	diff *SpecDiff,
	oldCanonical, newCanonical canonicalSpec,
	domains map[string]struct{},
) {
	diff.Decisions = diffValueSets(
		decisionDiffMap(oldCanonical.Decisions),
		decisionDiffMap(newCanonical.Decisions),
	)
	if len(diff.Decisions) > 0 {
		domains["decisions"] = struct{}{}
	}

	diff.Warnings = diffValueSets(
		warningDiffMap(oldCanonical.Warnings),
		warningDiffMap(newCanonical.Warnings),
	)
	if len(diff.Warnings) > 0 {
		domains["warnings"] = struct{}{}
	}

	diff.Conflicts = diffValueSets(
		conflictDiffMap(oldCanonical.Conflicts),
		conflictDiffMap(newCanonical.Conflicts),
	)
	if len(diff.Conflicts) > 0 {
		domains["conflicts"] = struct{}{}
	}
}

func diffModules(diff *SpecDiff, oldModules, newModules []ModuleRef) {
	oldByName := make(map[string]ModuleRef, len(oldModules))
	newByName := make(map[string]ModuleRef, len(newModules))
	for _, item := range oldModules {
		oldByName[item.Name] = item
	}
	for _, item := range newModules {
		newByName[item.Name] = item
	}
	for name, item := range oldByName {
		if _, ok := newByName[name]; !ok {
			diff.Modules = append(diff.Modules, newModuleDiffEntry(item, "removed"))
		}
	}
	for name, item := range newByName {
		if _, ok := oldByName[name]; !ok {
			diff.Modules = append(diff.Modules, newModuleDiffEntry(item, "added"))
		}
	}
	for name, oldItem := range oldByName {
		newItem, ok := newByName[name]
		if !ok || slices.Equal(oldItem.Reasons, newItem.Reasons) {
			continue
		}
		diff.ModuleReasons = append(diff.ModuleReasons, ModuleReasonDiffEntry{
			Name: name,
			Old:  append([]string(nil), oldItem.Reasons...),
			New:  append([]string(nil), newItem.Reasons...),
		})
	}
	slices.SortFunc(diff.Modules, func(left, right ModuleDiffEntry) int {
		return cmp.Compare(left.Name, right.Name)
	})
	slices.SortFunc(diff.ModuleReasons, func(left, right ModuleReasonDiffEntry) int {
		return cmp.Compare(left.Name, right.Name)
	})
}

func newModuleDiffEntry(item ModuleRef, action string) ModuleDiffEntry {
	return ModuleDiffEntry{
		Name:    item.Name,
		Kind:    item.Kind,
		Source:  item.Source,
		Action:  action,
		Reasons: append([]string(nil), item.Reasons...),
	}
}

func diffChains(diff *SpecDiff, oldItems, newItems []canonicalChain) {
	oldChains := canonicalChainMap(oldItems)
	newChains := canonicalChainMap(newItems)
	for _, key := range unionKeys(oldChains, newChains) {
		if chainsEqual(oldChains[key], newChains[key]) {
			continue
		}
		diff.Chains = append(diff.Chains, ChainDiffEntry{
			Target: key,
			Old:    oldChains[key].chain(),
			New:    newChains[key].chain(),
		})
	}
}

func specDiffHasChanges(diff *SpecDiff) bool {
	return diff.Mode != nil ||
		len(diff.Modules) > 0 ||
		len(diff.ModuleReasons) > 0 ||
		len(diff.Identity) > 0 ||
		len(diff.Defaults) > 0 ||
		len(diff.Chains) > 0 ||
		len(diff.Overrides) > 0 ||
		len(diff.Decisions) > 0 ||
		len(diff.Warnings) > 0 ||
		len(diff.Conflicts) > 0
}

func validateSpecDiff(oldSpec, newSpec *Spec, diff *SpecDiff) error {
	oldHash, err := Hash(oldSpec)
	if err != nil {
		return err
	}
	newHash, err := Hash(newSpec)
	if err != nil {
		return err
	}
	if oldHash == newHash && diff.HasChanges {
		return NewError(
			ErrPlanDiffInconsistent,
			stagePlan,
			"plan diff reports changes but canonical hashes are equal",
			nil,
			map[string]string{"old_hash": oldHash, "new_hash": newHash},
		)
	}
	if oldHash != newHash && !diff.HasChanges {
		return NewError(
			ErrPlanDiffInconsistent,
			stagePlan,
			"plan diff reports no changes but canonical hashes differ",
			nil,
			map[string]string{"old_hash": oldHash, "new_hash": newHash},
		)
	}
	return nil
}

func identityDiffMap(identity IdentitySpec) map[string]string {
	metadata, _ := json.Marshal(identity.Metadata)
	return map[string]string{
		"app_name":  identity.AppName,
		"namespace": identity.Namespace,
		"version":   identity.Version,
		"region":    identity.Region,
		"zone":      identity.Zone,
		"campus":    identity.Campus,
		"metadata":  string(metadata),
	}
}

func decisionDiffMap(decisions []Decision) map[string]string {
	return indexedDiffMap(decisions, func(item Decision) string {
		return strings.Join(
			[]string{item.Kind, item.Target, item.Source, item.Stage},
			"\x00",
		)
	})
}

func warningDiffMap(warnings []Warning) map[string]string {
	return indexedDiffMap(warnings, func(item Warning) string { return item.Code })
}

func conflictDiffMap(conflicts []Conflict) map[string]string {
	return indexedDiffMap(conflicts, func(item Conflict) string { return item.Code })
}

func indexedDiffMap[T any](items []T, baseKey func(T) string) map[string]string {
	out := make(map[string]string, len(items))
	occurrences := make(map[string]int, len(items))
	for _, item := range items {
		base := baseKey(item)
		value, _ := json.Marshal(item)
		key := indexedValueKey(base, occurrences[base])
		occurrences[base]++
		out[key] = string(value)
	}
	return out
}

func indexedValueKey(base string, occurrence int) string {
	if occurrence == 0 {
		return base
	}
	return fmt.Sprintf("%s#%d", base, occurrence+1)
}

func diffValueSets(oldValues, newValues map[string]string) []ValueDiffEntry {
	keys := unionKeys(oldValues, newValues)
	out := make([]ValueDiffEntry, 0, len(keys))
	for _, key := range keys {
		oldValue := oldValues[key]
		newValue := newValues[key]
		if oldValue == newValue {
			continue
		}
		out = append(out, ValueDiffEntry{
			Target: strings.ReplaceAll(key, "\x00", ":"),
			Old:    oldValue,
			New:    newValue,
		})
	}
	return out
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
	sortModuleRefs(out.Modules)
	sortDecisions(out.Decisions)
	sortWarnings(out.Warnings)
	sortConflicts(out.Conflicts)
	for key, value := range spec.Defaults {
		out.Defaults = append(out.Defaults, canonicalKV{Key: key, Value: value})
	}
	slices.SortFunc(out.Defaults, func(left, right canonicalKV) int {
		return cmp.Compare(left.Key, right.Key)
	})
	for key, value := range spec.Chains {
		out.Chains = append(out.Chains, canonicalChain{
			Key:      key,
			Template: value.Template,
			Version:  value.Version,
			Items:    append([]string(nil), value.Items...),
		})
	}
	slices.SortFunc(out.Chains, func(left, right canonicalChain) int {
		return cmp.Compare(left.Key, right.Key)
	})
	return out
}

func sortModuleRefs(items []ModuleRef) {
	slices.SortFunc(items, func(left, right ModuleRef) int {
		return cmp.Compare(left.Name, right.Name)
	})
}

func sortDecisions(items []Decision) {
	slices.SortFunc(items, func(left, right Decision) int {
		return cmp.Or(
			cmp.Compare(left.Kind, right.Kind),
			cmp.Compare(left.Target, right.Target),
			cmp.Compare(left.Source, right.Source),
			cmp.Compare(left.Value, right.Value),
			cmp.Compare(left.Reason, right.Reason),
			cmp.Compare(left.Stage, right.Stage),
		)
	})
}

func sortWarnings(items []Warning) {
	slices.SortFunc(items, func(left, right Warning) int {
		return cmp.Or(
			cmp.Compare(left.Code, right.Code),
			cmp.Compare(left.Message, right.Message),
		)
	})
}

func sortConflicts(items []Conflict) {
	slices.SortFunc(items, func(left, right Conflict) int {
		return cmp.Or(
			cmp.Compare(left.Code, right.Code),
			cmp.Compare(left.Message, right.Message),
		)
	})
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

func (item canonicalChain) chain() Chain {
	return Chain{
		Template: item.Template,
		Version:  item.Version,
		Items:    append([]string(nil), item.Items...),
	}
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
