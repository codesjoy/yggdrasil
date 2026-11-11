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

// Package module provides the module hub runtime core.
package module

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/codesjoy/yggdrasil/v3/config"
	configchain "github.com/codesjoy/yggdrasil/v3/config/chain"
)

// Module is the minimum abstraction unit managed by Hub.
type Module interface {
	Name() string
}

// IsolationMode describes whether a module can run with App-local runtime
// state or needs process-global compatibility defaults.
type IsolationMode string

const (
	// IsolationModeAppLocal indicates the module is compatible with App-local
	// runtime state.
	IsolationModeAppLocal IsolationMode = "app_local"
	// IsolationModeRequiresProcessDefaults indicates the module depends on
	// process-global defaults such as slog.Default or OTel globals.
	IsolationModeRequiresProcessDefaults IsolationMode = "requires_process_defaults"
)

// IsolationReporter optionally declares a module's process-global dependency
// requirements.
type IsolationReporter interface {
	IsolationMode() IsolationMode
}

// Dependent declares hard dependencies.
type Dependent interface {
	DependsOn() []string
}

// Ordered is a tie-breaker for modules in the same DAG layer.
type Ordered interface {
	InitOrder() int
}

// Configurable declares the config view path consumed by this module.
type Configurable interface {
	ConfigPath() string
}

// ConfigSourceProvider optionally contributes declarative config source builders.
type ConfigSourceProvider interface {
	ConfigSourceBuilders() map[string]configchain.ContextBuilder
}

// Initializable initializes long-lived resources.
type Initializable interface {
	Init(ctx context.Context, view config.View) error
}

// Startable starts serving behavior.
type Startable interface {
	Start(ctx context.Context) error
}

// Stoppable stops resources. It must be idempotent.
type Stoppable interface {
	Stop(ctx context.Context) error
}

// Reloadable supports staged reload.
type Reloadable interface {
	PrepareReload(ctx context.Context, view config.View) (ReloadCommitter, error)
}

// ReloadCommitter applies or rolls back prepared reload state.
type ReloadCommitter interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// ReloadFinalizer releases resources that must remain available until every
// reload committer has committed successfully.
type ReloadFinalizer interface {
	FinalizeReload(ctx context.Context) error
}

// ReloadReporter exposes module reload diagnostics.
type ReloadReporter interface {
	ReloadState() ReloadState
}

// ReloadPhase is the staged reload phase.
type ReloadPhase string

// Reload phases for module hot-reload state machine.
const (
	ReloadPhaseIdle       ReloadPhase = "idle"
	ReloadPhasePreparing  ReloadPhase = "preparing"
	ReloadPhaseCommitting ReloadPhase = "committing"
	ReloadPhaseRollback   ReloadPhase = "rollback"
	ReloadPhaseDegraded   ReloadPhase = "degraded"
)

// ReloadFailedStage marks the failed stage in one reload cycle.
type ReloadFailedStage string

// Reload failed stages identifying which phase failed.
const (
	ReloadFailedStageNone     ReloadFailedStage = ""
	ReloadFailedStagePrepare  ReloadFailedStage = "prepare"
	ReloadFailedStageCommit   ReloadFailedStage = "commit"
	ReloadFailedStageRollback ReloadFailedStage = "rollback"
	ReloadFailedStageFinalize ReloadFailedStage = "finalize"
)

// ReloadState is the global/module reload state.
type ReloadState struct {
	Phase           ReloadPhase       `json:"phase"`
	RestartRequired bool              `json:"restart_required"`
	Diverged        bool              `json:"diverged"`
	FailedModule    string            `json:"failed_module"`
	FailedStage     ReloadFailedStage `json:"failed_stage"`
	LastError       error             `json:"-"`
	LastErrorText   string            `json:"last_error"`
}

func (s ReloadState) withError(err error) ReloadState {
	s.LastError = err
	if err == nil {
		s.LastErrorText = ""
		return s
	}
	s.LastErrorText = err.Error()
	return s
}

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

// Scope describes the runtime lifetime contract of one module.
type Scope int

const (
	// ScopeApp lives for the entire App lifetime.
	ScopeApp Scope = iota
	// ScopeProvider exposes factories/providers without owning hot-path instances.
	ScopeProvider
	// ScopeRuntimeFactory builds hot-path runtime objects and must not be registered in Hub.
	ScopeRuntimeFactory
)

// Scoped exposes the declared runtime scope of one module.
type Scoped interface {
	Scope() Scope
}

func moduleScope(mod Module) Scope {
	if scoped, ok := mod.(Scoped); ok {
		return scoped.Scope()
	}
	return ScopeApp
}

// StopOnce guards one stop path and returns the first result for all later calls.
type StopOnce struct {
	once sync.Once
	err  error
}

// Do executes fn at most once. Panics are converted into errors and cached.
func (s *StopOnce) Do(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return nil
	}
	s.once.Do(func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.err = fmt.Errorf("stop panic recovered: %v", recovered)
			}
		}()
		s.err = fn(ctx)
	})
	return s.err
}

var (
	errHubSealed    = errors.New("module hub is already sealed")
	errHubNotSealed = errors.New("module hub is not sealed")
)
