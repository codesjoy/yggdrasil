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

package registry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/config"
)

const multiRegistryType = "multi_registry"

// BuiltinProvider returns the built-in multi_registry provider.
func BuiltinProvider() Provider {
	return NewProvider(multiRegistryType, newMultiRegistry)
}

// BuiltinProviderWithFactory returns the built-in multi_registry provider bound to an explicit child registry factory.
func BuiltinProviderWithFactory(factory func(string, map[string]any) (Registry, error)) Provider {
	return NewProvider(multiRegistryType, func(cfgVal map[string]any) (Registry, error) {
		return newMultiRegistryWithFactory(cfgVal, factory)
	})
}

type multiRegistryConfig struct {
	Registries []multiRegistryItem `mapstructure:"registries"`
	FailFast   *bool               `mapstructure:"fail_fast"`
	LegacyFast *bool               `mapstructure:"failFast"`
}

type multiRegistryItem struct {
	Type   string         `mapstructure:"type"`
	Config map[string]any `mapstructure:"config"`
}

type multiRegistry struct {
	failFast   bool
	registries []Registry
}

func (m *multiRegistry) Type() string { return multiRegistryType }

func (m *multiRegistry) Register(ctx context.Context, inst Instance) error {
	if ctx == nil {
		ctx = context.Background()
	}
	registered := make([]Registry, 0, len(m.registries))
	for _, r := range m.registries {
		if err := r.Register(ctx, inst); err != nil {
			if m.failFast {
				rollbackErr := deregisterRegistries(
					context.WithoutCancel(ctx),
					registered,
					inst,
				)
				if rollbackErr != nil {
					return errors.Join(
						fmt.Errorf("register child %q: %w", r.Type(), err),
						fmt.Errorf("register rollback failed: %w", rollbackErr),
					)
				}
				return fmt.Errorf("register child %q: %w", r.Type(), err)
			}
			slog.Warn(
				"best-effort registry child registration failed",
				slog.String("type", r.Type()),
				slog.Any("error", err),
			)
			continue
		}
		registered = append(registered, r)
	}
	return nil
}

func (m *multiRegistry) Deregister(ctx context.Context, inst Instance) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return deregisterRegistries(ctx, m.registries, inst)
}

func deregisterRegistries(
	ctx context.Context,
	registries []Registry,
	inst Instance,
) error {
	var multiErr error
	for index := len(registries) - 1; index >= 0; index-- {
		r := registries[index]
		if err := r.Deregister(ctx, inst); err != nil {
			multiErr = errors.Join(multiErr, fmt.Errorf(
				"deregister child %q: %w",
				r.Type(),
				err,
			))
		}
	}
	return multiErr
}

func newMultiRegistry(cfgVal map[string]any) (Registry, error) {
	return newMultiRegistryWithFactory(cfgVal, New)
}

func newMultiRegistryWithFactory(
	cfgVal map[string]any,
	factory func(string, map[string]any) (Registry, error),
) (Registry, error) {
	var cfg multiRegistryConfig
	if err := config.NewSnapshot(cfgVal).Decode(&cfg); err != nil {
		return nil, err
	}
	failFast := false
	if cfg.LegacyFast != nil {
		failFast = *cfg.LegacyFast
	}
	if cfg.FailFast != nil {
		failFast = *cfg.FailFast
	}

	registries := make([]Registry, 0, len(cfg.Registries))
	for _, item := range cfg.Registries {
		item.Type = strings.TrimSpace(item.Type)
		if item.Type == "" {
			return nil, errors.New("multi_registry: empty child type")
		}
		r, err := factory(item.Type, item.Config)
		if err != nil {
			return nil, errors.Join(
				fmt.Errorf("multi_registry child %q: %w", item.Type, err),
				closeRegistries(registries),
			)
		}
		registries = append(registries, r)
	}
	return &multiRegistry{failFast: failFast, registries: registries}, nil
}

func closeRegistries(registries []Registry) error {
	var err error
	for index := len(registries) - 1; index >= 0; index-- {
		closer, ok := registries[index].(io.Closer)
		if !ok {
			continue
		}
		err = errors.Join(err, closer.Close())
	}
	return err
}
