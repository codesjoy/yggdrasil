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

package settings

import "fmt"

var legacyTopLevelKeys = map[string]struct{}{
	"config":     {},
	"server":     {},
	"transports": {},
	"clients":    {},
	"discovery":  {},
	"balancers":  {},
	"logging":    {},
	"telemetry":  {},
	"admin":      {},
	"extensions": {},
}

// ValidateV3RootShape enforces v3 root shape where framework config must be under yggdrasil.*.
func ValidateV3RootShape(raw map[string]any) error {
	if raw == nil {
		return nil
	}
	_, hasYggdrasil := raw["yggdrasil"]
	for key := range legacyTopLevelKeys {
		if _, ok := raw[key]; !ok {
			continue
		}
		if hasYggdrasil {
			return fmt.Errorf("legacy top-level key %q is not allowed with yggdrasil.* v3 config", key)
		}
		return fmt.Errorf("legacy top-level key %q is not allowed; use yggdrasil.%s", key, key)
	}
	return nil
}
