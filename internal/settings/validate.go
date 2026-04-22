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

	validateCredential := func(protoName, serviceName string, client bool, key string) {
		if protoName != "tls" {
			return
		}
		if err := validateTLSCredentialConfig(resolved, serviceName, client); err != nil {
			addErr(
				"remote credentials config invalid",
				fmt.Errorf("name=%s: %w", protoName, err),
				slog.String("name", protoName),
				slog.String("key", key),
			)
		}
	}

	validateCredential(
		resolved.Transports.GRPC.Server.CredsProto,
		"",
		false,
		"yggdrasil.transports.grpc.server.creds_proto",
	)
	validateCredential(
		resolved.Transports.GRPC.Client.Transport.CredsProto,
		"",
		true,
		"yggdrasil.transports.grpc.client.transport.creds_proto",
	)
	for serviceName, cfg := range resolved.Transports.GRPC.ClientServices {
		validateCredential(
			cfg.Transport.CredsProto,
			serviceName,
			true,
			fmt.Sprintf("yggdrasil.clients.services.%s.transports.grpc.transport.creds_proto", serviceName),
		)
	}

	return multiErr
}
