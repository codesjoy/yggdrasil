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

import "github.com/codesjoy/yggdrasil/v3/internal/settings"

type overrideSet struct {
	EnabledModules  map[string]struct{}
	DisabledModules map[string]struct{}
	ForcedDefaults  map[string]string
	ForcedTemplates map[string]templateSelection
	DisabledAuto    map[string]struct{}
}

func newOverrideSet() overrideSet {
	return overrideSet{
		EnabledModules:  map[string]struct{}{},
		DisabledModules: map[string]struct{}{},
		ForcedDefaults:  map[string]string{},
		ForcedTemplates: map[string]templateSelection{},
		DisabledAuto:    map[string]struct{}{},
	}
}

// Override mutates one planner override set.
type Override interface {
	apply(*overrideSet)
}

type overrideFunc func(*overrideSet)

func (fn overrideFunc) apply(target *overrideSet) {
	if fn != nil {
		fn(target)
	}
}

// EnableModule re-enables one named module candidate.
func EnableModule(name string) Override {
	return overrideFunc(func(target *overrideSet) {
		if target == nil || name == "" {
			return
		}
		delete(target.DisabledModules, name)
		target.EnabledModules[name] = struct{}{}
	})
}

// DisableModule disables one named module candidate.
func DisableModule(name string) Override {
	return overrideFunc(func(target *overrideSet) {
		if target == nil || name == "" {
			return
		}
		delete(target.EnabledModules, name)
		target.DisabledModules[name] = struct{}{}
	})
}

// ForceDefault forces one capability default selection.
func ForceDefault(path string, moduleName string) Override {
	return overrideFunc(func(target *overrideSet) {
		if target == nil || path == "" || moduleName == "" {
			return
		}
		target.ForcedDefaults[path] = moduleName
	})
}

// ForceTemplate forces one chain template selection.
func ForceTemplate(path string, template string, version string) Override {
	return overrideFunc(func(target *overrideSet) {
		if target == nil || path == "" || template == "" || version == "" {
			return
		}
		target.ForcedTemplates[path] = templateSelection{
			Name:    template,
			Version: version,
		}
	})
}

// DisableAuto disables mode/fallback planning for one capability or chain path.
func DisableAuto(path string) Override {
	return overrideFunc(func(target *overrideSet) {
		if target == nil || path == "" {
			return
		}
		target.DisabledAuto[path] = struct{}{}
	})
}

func overridesFromConfig(raw settings.Overrides) overrideSet {
	out := newOverrideSet()
	for _, name := range raw.DisableModules {
		if name != "" {
			out.DisabledModules[name] = struct{}{}
		}
	}
	for path, value := range raw.ForceDefaults {
		if path != "" && value != "" {
			out.ForcedDefaults[path] = value
		}
	}
	for path, value := range raw.ForceTemplates {
		template, version := parseTemplateReference(value)
		if path != "" && template != "" && version != "" {
			out.ForcedTemplates[path] = templateSelection{Name: template, Version: version}
		}
	}
	for _, path := range raw.DisableAuto {
		if path != "" {
			out.DisabledAuto[path] = struct{}{}
		}
	}
	return out
}
