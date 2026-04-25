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

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security/insecure"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security/local"
	ytls "github.com/codesjoy/yggdrasil/v3/transport/support/security/tls"
)

// Validate validates the resolved framework configuration.
// This layer is intentionally limited to pure configuration semantics and must
// not depend on the live runtime provider registry.
func Validate(resolved Resolved) error {
	strict := resolved.Admin.Validation.Strict
	enable := strict || resolved.Admin.Validation.Enable
	if !enable {
		return nil
	}

	var multiErr error
	addErr := func(msg string, err error, attrs ...slog.Attr) {
		if err == nil {
			return
		}
		if strict {
			multiErr = errors.Join(multiErr, fmt.Errorf("%s: %w", msg, err))
			return
		}
		attrs = append(attrs, slog.Any("error", err))
		args := make([]any, 0, len(attrs))
		for _, attr := range attrs {
			args = append(args, attr)
		}
		slog.Warn(msg, args...)
	}

	builtinProviders := map[string]security.Provider{
		"insecure": insecure.BuiltinProvider(),
		"local":    local.BuiltinProvider(),
		"tls":      ytls.BuiltinProvider(),
	}
	for name, spec := range resolved.Transports.SecurityProfiles {
		if strings.TrimSpace(spec.Type) == "" {
			addErr(
				"transport security profile invalid",
				fmt.Errorf("profile=%s: missing type", name),
				slog.String("profile", name),
			)
			continue
		}
		provider := builtinProviders[spec.Type]
		if provider == nil {
			continue
		}
		if _, err := provider.Compile(name, spec.Config); err != nil {
			addErr(
				"transport security profile invalid",
				fmt.Errorf("profile=%s type=%s: %w", name, spec.Type, err),
				slog.String("profile", name),
				slog.String("type", spec.Type),
			)
		}
	}

	return multiErr
}
