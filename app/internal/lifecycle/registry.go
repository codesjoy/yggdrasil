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

package lifecycle

import (
	"context"
	"log/slog"
	"maps"
	"time"

	"github.com/codesjoy/yggdrasil/v3/discovery/registry"
	"github.com/codesjoy/yggdrasil/v3/internal/instance"
	yserver "github.com/codesjoy/yggdrasil/v3/transport/runtime/server"
)

func (runner *Runner) register() error {
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

func (runner *Runner) deregister(ctx context.Context) error {
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

func (runner *Runner) beginRegister() bool {
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

func (runner *Runner) resetRegistering() {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	if runner.registryState == registryStateRegistering {
		runner.registryState = registryStateInit
	}
}

func (runner *Runner) finishRegister() int {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	state := runner.registryState
	if state == registryStateRegistering {
		runner.registryState = registryStateDone
	}
	return state
}

func (runner *Runner) beginDeregister() bool {
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

// Region returns the instance region.
func (runner *Runner) Region() string {
	return instance.Region()
}

// Zone returns the instance zone.
func (runner *Runner) Zone() string {
	return instance.Zone()
}

// Campus returns the instance campus.
func (runner *Runner) Campus() string {
	return instance.Campus()
}

// Namespace returns the instance namespace.
func (runner *Runner) Namespace() string {
	return instance.Namespace()
}

// Name returns the instance name.
func (runner *Runner) Name() string {
	return instance.Name()
}

// Version returns the instance version.
func (runner *Runner) Version() string {
	return instance.Version()
}

// Metadata returns the instance metadata.
func (runner *Runner) Metadata() map[string]string {
	return instance.Metadata()
}

// Endpoints returns the advertised service endpoints.
func (runner *Runner) Endpoints() []registry.Endpoint {
	endpoints := make([]registry.Endpoint, 0)
	if runner.server != nil {
		for _, item := range runner.server.Endpoints() {
			metadata := cloneEndpointMetadata(item.Metadata())
			metadata[registry.MDServerKind] = string(item.Kind())
			endpoints = append(endpoints, endpoint{
				address:  item.Address(),
				scheme:   item.Protocol(),
				metadata: metadata,
			})
		}
	}
	if runner.governor != nil && runner.governor.ShouldAdvertise() {
		info := runner.governor.Info()
		metadata := cloneEndpointMetadata(info.Attr)
		metadata[registry.MDServerKind] = string(yserver.EndpointKindGovernor)
		endpoints = append(endpoints, endpoint{
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
