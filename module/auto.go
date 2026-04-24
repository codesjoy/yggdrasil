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

import "github.com/codesjoy/yggdrasil/v3/config"

// AutoMode is the resolved planning mode exposed to module auto rules.
type AutoMode struct {
	Name    string
	Profile string
	Bundle  string
}

// AutoRuleContext is the deterministic input visible to one auto rule.
type AutoRuleContext struct {
	AppName  string
	Snapshot config.Snapshot
	Mode     AutoMode
}

// AutoRule determines whether one module should join the auto assembly result.
type AutoRule interface {
	Match(ctx AutoRuleContext) bool
	Describe() string
	AffectedPaths() []string
}

// DefaultPolicy declares module fallback preference for default selection.
type DefaultPolicy struct {
	Profiles []string
	Score    int
}

// AutoSpec declares the auto-assembly contract of one module.
type AutoSpec struct {
	Provides      []CapabilitySpec
	AutoRules     []AutoRule
	DefaultPolicy *DefaultPolicy
}

// AutoDescribed exposes optional auto-assembly metadata.
type AutoDescribed interface {
	AutoSpec() AutoSpec
}
