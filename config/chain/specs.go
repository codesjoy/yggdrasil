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

package chain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseSourceSpecs parses bootstrap source declarations from JSON or the
// compact "kind:name:priority,kind:name:priority" format.
func ParseSourceSpecs(value string) ([]SourceSpec, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	switch value[0] {
	case '[':
		var specs []SourceSpec
		if err := json.Unmarshal([]byte(value), &specs); err != nil {
			return nil, fmt.Errorf("parse config source specs JSON array: %w", err)
		}
		normalizeSourceSpecs(specs)
		return specs, nil
	case '{':
		var spec SourceSpec
		if err := json.Unmarshal([]byte(value), &spec); err != nil {
			return nil, fmt.Errorf("parse config source spec JSON object: %w", err)
		}
		if spec.Config == nil {
			spec.Config = map[string]any{}
		}
		return []SourceSpec{spec}, nil
	default:
		return parseCompactSourceSpecs(value)
	}
}

func parseCompactSourceSpecs(value string) ([]SourceSpec, error) {
	parts := strings.Split(value, ",")
	specs := make([]SourceSpec, 0, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("config source spec[%d] is empty", i)
		}
		fields := strings.Split(part, ":")
		if len(fields) > 3 {
			return nil, fmt.Errorf(
				"config source spec[%d] %q has too many fields; want kind:name:priority",
				i,
				part,
			)
		}
		for j := range fields {
			fields[j] = strings.TrimSpace(fields[j])
		}
		if fields[0] == "" {
			return nil, fmt.Errorf("config source spec[%d] kind is required", i)
		}
		spec := SourceSpec{
			Kind:    fields[0],
			Enabled: boolPtr(true),
			Config:  map[string]any{},
		}
		if len(fields) > 1 {
			spec.Name = fields[1]
		}
		if len(fields) > 2 {
			spec.Priority = fields[2]
		}
		if strings.EqualFold(spec.Kind, "env") && spec.Name != "" {
			spec.Config["prefixes"] = []string{spec.Name}
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func normalizeSourceSpecs(specs []SourceSpec) {
	for i := range specs {
		if specs[i].Config == nil {
			specs[i].Config = map[string]any{}
		}
	}
}

func boolPtr(v bool) *bool {
	return &v
}
