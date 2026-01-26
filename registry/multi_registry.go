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

	"github.com/codesjoy/yggdrasil/v2/config"
)

const multiRegistryType = "multi_registry"

func init() {
	RegisterBuilder(multiRegistryType, newMultiRegistry)
}

type multiRegistryConfig struct {
	Registries []multiRegistryItem `mapstructure:"registries"`
	FailFast   bool                `mapstructure:"failFast"`
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
	var multiErr error
	for _, r := range m.registries {
		if err := r.Register(ctx, inst); err != nil {
			if m.failFast {
				return err
			}
			multiErr = errors.Join(multiErr, err)
		}
	}
	return multiErr
}

func (m *multiRegistry) Deregister(ctx context.Context, inst Instance) error {
	var multiErr error
	for _, r := range m.registries {
		if err := r.Deregister(ctx, inst); err != nil {
			if m.failFast {
				return err
			}
			multiErr = errors.Join(multiErr, err)
		}
	}
	return multiErr
}

func newMultiRegistry(cfgVal config.Value) (Registry, error) {
	var cfg multiRegistryConfig
	if err := cfgVal.Scan(&cfg); err != nil {
		return nil, err
	}

	registries := make([]Registry, 0, len(cfg.Registries))
	for _, item := range cfg.Registries {
		if item.Type == "" {
			return nil, errors.New("multi_registry: empty child type")
		}
		childCfgVal := valueFromMap(item.Config)
		r, err := New(item.Type, childCfgVal)
		if err != nil {
			return nil, err
		}
		registries = append(registries, r)
	}
	return &multiRegistry{failFast: cfg.FailFast, registries: registries}, nil
}

func valueFromMap(m map[string]any) config.Value {
	c := config.NewConfig(".")
	_ = c.Set("x", m)
	return c.Get("x")
}
