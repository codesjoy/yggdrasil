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

package yggdrasil

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/internal/settings"
)

func validateStartup(opts *options) error {
	resolved, err := resolveStartupSettings(opts)
	if err != nil {
		return err
	}
	strict := resolved.Admin.Validation.Strict
	enable := strict || resolved.Admin.Validation.Enable
	if err := settings.Validate(resolved); err != nil {
		return err
	}
	if !enable || opts == nil {
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
		for _, a := range attrs {
			args = append(args, a)
		}
		slog.Warn(msg, args...)
	}

	if len(opts.serviceDesc) > 0 && len(resolved.Server.Transports) == 0 {
		addErr(
			"rpc services registered without any server protocol",
			errors.New("set yggdrasil.server.transports to at least one protocol"),
		)
	}
	if (len(opts.restServiceDesc) > 0 || len(opts.restRawHandleDesc) > 0) && !resolved.Server.RestEnabled {
		addErr(
			"rest handlers registered while rest server is disabled",
			errors.New("configure yggdrasil.transports.http.rest"),
		)
	}

	return multiErr
}

func resolveStartupSettings(opts *options) (settings.Resolved, error) {
	resolved := settings.Resolved{}
	if opts != nil {
		resolved = opts.resolvedSettings
	}
	if opts == nil ||
		(resolved.Logging.Handlers == nil &&
			resolved.Discovery.Registry.Type == "" &&
			len(resolved.Server.Transports) == 0 &&
			resolved.Transports.Rest == nil) {
		root, err := settings.NewCatalog(config.Default()).Root().Current()
		if err != nil {
			return settings.Resolved{}, err
		}
		resolved, err = settings.Compile(root)
		if err != nil {
			return settings.Resolved{}, err
		}
	}
	if opts != nil {
		opts.resolvedSettings = resolved
	}
	return resolved, nil
}
