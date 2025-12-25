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

// Package status provides utilities for handling status.
package status

import (
	"context"

	"github.com/codesjoy/yggdrasil/pkg/metadata"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
)

// Reason represents a reason.
type Reason interface {
	Reason() string
	Domain() string
	Code() code.Code
}

// Message represents a message.
type Message interface {
	Message(language string) string
}

// NewReason returns a new reason.
func NewReason(reason Reason, meta map[string]string) *errdetails.ErrorInfo {
	return &errdetails.ErrorInfo{
		Reason:   reason.Reason(),
		Domain:   reason.Domain(),
		Metadata: meta,
	}
}

// NewLocalizedMsg returns a new localized message.
func NewLocalizedMsg(ctx context.Context, msg Message) *errdetails.LocalizedMessage {
	var languages []string
	if meta, ok := metadata.FromInContext(ctx); ok {
		if values, ok := meta["language"]; ok {
			languages = append(values, languages...)
		}
	}
	return NewLocalizedMsgWithLang(languages, msg)
}

// NewLocalizedMsgWithLang returns a new localized message with languages.
func NewLocalizedMsgWithLang(languages []string, msg Message) *errdetails.LocalizedMessage {
	languages = append(languages, "zh-CN")
	for _, language := range languages {
		localMsg := msg.Message(language)
		if len(localMsg) > 0 {
			return &errdetails.LocalizedMessage{
				Locale:  language,
				Message: localMsg,
			}
		}
	}
	return nil
}

// NewDebugInfo returns a new debug info.
func NewDebugInfo(stacks []string, msg string) *errdetails.DebugInfo {
	return &errdetails.DebugInfo{
		StackEntries: stacks,
		Detail:       msg,
	}
}
