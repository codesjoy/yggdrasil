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

// Package status provides a way to handle error details.
package status

import (
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
)

// Reason defines the reason for the error
type Reason interface {
	Reason() string
	Domain() string
	Code() code.Code
}

// NewReason returns a new reason.
func NewReason(reason Reason, meta map[string]string) *errdetails.ErrorInfo {
	return &errdetails.ErrorInfo{
		Reason:   reason.Reason(),
		Domain:   reason.Domain(),
		Metadata: meta,
	}
}
