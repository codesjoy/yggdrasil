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

package app

import (
	"context"
	"log/slog"
	"maps"
	"time"

	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/internal/constant"
	"github.com/codesjoy/yggdrasil/v3/internal/instance"
)

func (runner *lifecycleRunner) register() error {
	if runner.registry == nil {
		return nil
	}

	if !runner.beginRegister() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := runner.registry.Register(ctx, runner); err != nil {
		runner.resetRegistering()
		slog.Error("fault to register application", slog.Any("error", err))
		return err
	}

	if runner.finishRegister() == registryStateCancel {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := runner.registry.Deregister(ctx, runner); err != nil {
			slog.Error(
				"fault to deregister application after concurrent stop",
				slog.Any("error", err),
			)
			return err
		}
		return nil
	}

	slog.Info("application has been registered")
	return nil
}

func (runner *lifecycleRunner) deregister(ctx context.Context) error {
	if runner.registry == nil {
		return nil
	}

	if !runner.beginDeregister() {
		return nil
	}

	if err := runner.registry.Deregister(ctx, runner); err != nil {
		slog.Error("fault to deregister application", slog.Any("error", err))
		return err
	}
	return nil
}

func (runner *lifecycleRunner) beginRegister() bool {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	switch runner.registryState {
	case registryStateDone, registryStateCancel, registryStateRegistering:
		return false
	default:
		runner.registryState = registryStateRegistering
		return true
	}
}

func (runner *lifecycleRunner) resetRegistering() {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	if runner.registryState == registryStateRegistering {
		runner.registryState = registryStateInit
	}
}

func (runner *lifecycleRunner) finishRegister() int {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	state := runner.registryState
	if state == registryStateRegistering {
		runner.registryState = registryStateDone
	}
	return state
}

func (runner *lifecycleRunner) beginDeregister() bool {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	switch runner.registryState {
	case registryStateRegistering:
		runner.registryState = registryStateCancel
		return false
	case registryStateDone:
		runner.registryState = registryStateCancel
		return true
	default:
		return false
	}
}

func (runner *lifecycleRunner) Region() string {
	return instance.Region()
}

func (runner *lifecycleRunner) Zone() string {
	return instance.Zone()
}

func (runner *lifecycleRunner) Campus() string {
	return instance.Campus()
}

func (runner *lifecycleRunner) Namespace() string {
	return instance.Namespace()
}

func (runner *lifecycleRunner) Name() string {
	return instance.Name()
}

func (runner *lifecycleRunner) Version() string {
	return instance.Version()
}

func (runner *lifecycleRunner) Metadata() map[string]string {
	return instance.Metadata()
}

func (runner *lifecycleRunner) Endpoints() []registry.Endpoint {
	endpoints := make([]registry.Endpoint, 0)
	if runner.server != nil {
		for _, item := range runner.server.Endpoints() {
			metadata := cloneEndpointMetadata(item.Metadata())
			metadata[registry.MDServerKind] = string(item.Kind())
			endpoints = append(endpoints, lifecycleEndpoint{
				address:  item.Address(),
				scheme:   item.Protocol(),
				metadata: metadata,
			})
		}
	}
	if runner.governor != nil && runner.governor.ShouldAdvertise() {
		info := runner.governor.Info()
		metadata := cloneEndpointMetadata(info.Attr)
		metadata[registry.MDServerKind] = string(constant.ServerKindGovernor)
		endpoints = append(endpoints, lifecycleEndpoint{
			address:  info.Address,
			scheme:   info.Scheme,
			metadata: metadata,
		})
	}
	return endpoints
}

func cloneEndpointMetadata(source map[string]string) map[string]string {
	if cloned := maps.Clone(source); cloned != nil {
		return cloned
	}
	return map[string]string{}
}
