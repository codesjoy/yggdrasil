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

package protocolhttp

import (
	"github.com/codesjoy/yggdrasil/v2/remote/marshaler"
)

const scheme = "http"

type marshalerSet struct {
	inbound  marshaler.Marshaler
	outbound marshaler.Marshaler
}

func newConfiguredMarshalers(cfg *MarshalerConfigSet) (marshalerSet, error) {
	if cfg == nil {
		return marshalerSet{}, nil
	}
	inbound, err := buildConfiguredMarshaler(cfg.Inbound)
	if err != nil {
		return marshalerSet{}, err
	}
	outbound, err := buildConfiguredMarshaler(cfg.Outbound)
	if err != nil {
		return marshalerSet{}, err
	}
	return marshalerSet{inbound: inbound, outbound: outbound}, nil
}

func buildConfiguredMarshaler(cfg *MarshalerConfig) (marshaler.Marshaler, error) {
	if cfg == nil {
		return nil, nil
	}
	return marshaler.BuildMarshallerWithConfig(cfg.Type, cfg.Config)
}

func selectInboundMarshaler(
	configured marshaler.Marshaler,
	contentType string,
) marshaler.Marshaler {
	if configured != nil {
		return configured
	}
	if resolved := marshaler.MarshalerForContentType(contentType); resolved != nil {
		return resolved
	}
	return marshaler.MarshalerForValue(nil)
}

func selectOutboundMarshaler(
	configured marshaler.Marshaler,
	accept string,
	fallback marshaler.Marshaler,
) marshaler.Marshaler {
	if configured != nil {
		return configured
	}
	if accept == "" {
		if fallback != nil {
			return fallback
		}
		return marshaler.MarshalerForValue(nil)
	}
	if resolved := marshaler.MarshalerForContentType(accept); resolved != nil {
		return resolved
	}
	if fallback != nil {
		return fallback
	}
	return marshaler.MarshalerForValue(nil)
}
