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
	"errors"

	yassembly "github.com/codesjoy/yggdrasil/v3/assembly"
)

// Stage identifies one app assembly stage.
type Stage string

// Assembly stage constants.
const (
	StagePrepare Stage = "prepare"
	StageCompose Stage = "compose"
	StageInstall Stage = "install"
	StageReload  Stage = "reload"
)

// ErrorRecord stores one stage error and its update sequence.
type ErrorRecord struct {
	Err *yassembly.Error
	Seq uint64
}

// StageErrors stores the latest error by stage.
type StageErrors struct {
	Prepare *yassembly.Error `json:"prepare"`
	Compose *yassembly.Error `json:"compose"`
	Install *yassembly.Error `json:"install"`
	Reload  *yassembly.Error `json:"reload"`
}

// ErrorSnapshot is a detached copy of ErrorState diagnostics.
type ErrorSnapshot struct {
	LastError      *yassembly.Error
	LastErrorStage string
	Errors         StageErrors
}

// ErrorState tracks assembly errors across stages.
type ErrorState struct {
	Prepare ErrorRecord
	Compose ErrorRecord
	Install ErrorRecord
	Reload  ErrorRecord

	Latest      *yassembly.Error
	LatestStage Stage
	NextSeq     uint64
}

// Record stores one stage error, normalizing it into a structured assembly error.
func (s *ErrorState) Record(stage Stage, err *yassembly.Error) {
	if err == nil {
		s.Clear(stage)
		return
	}

	s.NextSeq++
	record := ErrorRecord{Err: err, Seq: s.NextSeq}
	switch stage {
	case StagePrepare:
		s.Prepare = record
	case StageCompose:
		s.Compose = record
	case StageInstall:
		s.Install = record
	case StageReload:
		s.Reload = record
	default:
		return
	}
	s.RecalculateLatest()
}

// Clear removes one stage error.
func (s *ErrorState) Clear(stage Stage) {
	switch stage {
	case StagePrepare:
		s.Prepare = ErrorRecord{}
	case StageCompose:
		s.Compose = ErrorRecord{}
	case StageInstall:
		s.Install = ErrorRecord{}
	case StageReload:
		s.Reload = ErrorRecord{}
	default:
		return
	}
	s.RecalculateLatest()
}

// Snapshot returns a detached error snapshot for diagnostics.
func (s *ErrorState) Snapshot() ErrorSnapshot {
	return ErrorSnapshot{
		LastError:      CloneError(s.Latest),
		LastErrorStage: string(s.LatestStage),
		Errors: StageErrors{
			Prepare: CloneError(s.Prepare.Err),
			Compose: CloneError(s.Compose.Err),
			Install: CloneError(s.Install.Err),
			Reload:  CloneError(s.Reload.Err),
		},
	}
}

// RecalculateLatest refreshes the latest stage error pointers.
func (s *ErrorState) RecalculateLatest() {
	s.Latest = nil
	s.LatestStage = ""

	bestStage := Stage("")
	bestSeq := uint64(0)
	bestErr := (*yassembly.Error)(nil)
	for _, item := range []struct {
		stage  Stage
		record ErrorRecord
	}{
		{stage: StagePrepare, record: s.Prepare},
		{stage: StageCompose, record: s.Compose},
		{stage: StageInstall, record: s.Install},
		{stage: StageReload, record: s.Reload},
	} {
		if item.record.Err == nil || item.record.Seq < bestSeq {
			continue
		}
		bestSeq = item.record.Seq
		bestStage = item.stage
		bestErr = item.record.Err
	}

	s.Latest = bestErr
	s.LatestStage = bestStage
}

// WrapStageError wraps plain errors into structured assembly errors for one stage.
func WrapStageError(stage string, err error) error {
	if err == nil {
		return nil
	}
	if structured, ok := err.(*yassembly.Error); ok {
		if structured.Stage == "" {
			structured.Stage = stage
		}
		return structured
	}
	code := yassembly.ErrRuntimeSurfaceUnavailable
	switch stage {
	case "prepare":
		code = yassembly.ErrRuntimeSurfaceUnavailable
	case "compose":
		code = yassembly.ErrComposeFailed
	case "install":
		code = yassembly.ErrInstallValidationFailed
	case "reload":
		code = yassembly.ErrRuntimeReconcileFailed
	}
	return yassembly.NewError(code, stage, err.Error(), err, nil)
}

// NormalizeError converts any error into a structured assembly error for the given stage.
func NormalizeError(stage Stage, err error) *yassembly.Error {
	if err == nil {
		return nil
	}

	var structured *yassembly.Error
	if errors.As(err, &structured) {
		clone := CloneError(structured)
		clone.Stage = string(stage)
		return clone
	}

	return yassembly.NewError(DefaultErrorCode(stage), string(stage), err.Error(), err, nil)
}

// DefaultErrorCode reports the default error code for one stage.
func DefaultErrorCode(stage Stage) yassembly.ErrorCode {
	switch stage {
	case StageCompose:
		return yassembly.ErrComposeFailed
	case StageInstall:
		return yassembly.ErrInstallValidationFailed
	case StageReload:
		return yassembly.ErrRuntimeReconcileFailed
	default:
		return yassembly.ErrRuntimeSurfaceUnavailable
	}
}

// CloneError returns a deep-enough clone for diagnostics surfaces.
func CloneError(err *yassembly.Error) *yassembly.Error {
	if err == nil {
		return nil
	}

	clone := *err
	if err.Context != nil {
		clone.Context = make(map[string]string, len(err.Context))
		for key, value := range err.Context {
			clone.Context[key] = value
		}
	}
	return &clone
}
