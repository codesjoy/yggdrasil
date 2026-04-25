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

package runtime

import (
	"encoding/json"
	"sort"
	"strings"

	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
)

// ReloadRequiresRestart reports whether the spec diff or changed paths require a full restart.
func ReloadRequiresRestart(
	diff *yassembly.SpecDiff,
	changedPaths []string,
	businessInputPaths []string,
	businessInstalled bool,
) bool {
	if businessInstalled && IntersectsPaths(changedPaths, businessInputPaths) {
		return true
	}
	if diff == nil || !diff.HasChanges {
		return false
	}
	for _, domain := range diff.AffectedDomains {
		switch domain {
		case "mode", "modules", "defaults", "chains", "overrides":
			return true
		}
	}
	return false
}

// ChangedConfigPaths returns deduplicated config paths that differ between two assembly results.
func ChangedConfigPaths(prevPlan, nextPlan *yassembly.Result) []string {
	if prevPlan == nil || nextPlan == nil {
		return nil
	}
	oldValue, err := rootAsAny(prevPlan.EffectiveResolved.Root)
	if err != nil {
		return nil
	}
	newValue, err := rootAsAny(nextPlan.EffectiveResolved.Root)
	if err != nil {
		return nil
	}
	changes := map[string]struct{}{}
	collectChangedPaths(
		"yggdrasil",
		lookupRootSection(oldValue),
		lookupRootSection(newValue),
		changes,
	)
	return dedupStrings(sortedKeys(changes))
}

// CapabilityBindingsEqual reports whether two capability binding maps are equal.
func CapabilityBindingsEqual(left, right map[string][]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValues := range left {
		rightValues, ok := right[key]
		if !ok {
			return false
		}
		if len(leftValues) != len(rightValues) {
			return false
		}
		for index := range leftValues {
			if leftValues[index] != rightValues[index] {
				return false
			}
		}
	}
	return true
}

func rootAsAny(root settings.Root) (any, error) {
	data, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func lookupRootSection(value any) any {
	root, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return root["Yggdrasil"]
}

func collectChangedPaths(prefix string, oldValue, newValue any, changes map[string]struct{}) {
	switch oldMap := oldValue.(type) {
	case map[string]any:
		newMap, _ := newValue.(map[string]any)
		seen := map[string]struct{}{}
		for key := range oldMap {
			seen[key] = struct{}{}
		}
		for key := range newMap {
			seen[key] = struct{}{}
		}
		if len(seen) == 0 {
			return
		}
		for _, key := range sortedKeys(seen) {
			nextPrefix := prefix + "." + strings.ToLower(key)
			collectChangedPaths(nextPrefix, oldMap[key], newMap[key], changes)
		}
	default:
		if !valuesEqual(oldValue, newValue) {
			changes[prefix] = struct{}{}
		}
	}
}

func valuesEqual(left, right any) bool {
	leftBytes, leftErr := json.Marshal(left)
	rightBytes, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return left == right
	}
	return string(leftBytes) == string(rightBytes)
}

// IntersectsPaths reports whether any path in left matches or is a prefix of any path in right.
func IntersectsPaths(left, right []string) bool {
	for _, leftItem := range left {
		for _, rightItem := range right {
			if pathPrefixMatch(leftItem, rightItem) {
				return true
			}
		}
	}
	return false
}

func pathPrefixMatch(left, right string) bool {
	if left == right {
		return true
	}
	return strings.HasPrefix(left, right+".") || strings.HasPrefix(right, left+".")
}

func dedupStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
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
