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

	"github.com/codesjoy/yggdrasil/v3/config"
)

// Module is the minimum abstraction unit managed by Hub.
type Module interface {
	Name() string
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

var (
	errHubSealed    = errors.New("module hub is already sealed")
	errHubNotSealed = errors.New("module hub is not sealed")
)
