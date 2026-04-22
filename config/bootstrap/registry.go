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

package bootstrap

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/mitchellh/mapstructure"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source"
	envsource "github.com/codesjoy/yggdrasil/v3/config/source/env"
	filesource "github.com/codesjoy/yggdrasil/v3/config/source/file"
	flagsource "github.com/codesjoy/yggdrasil/v3/config/source/flag"
)

// SourceSpec describes a declarative bootstrap source.
type SourceSpec struct {
	Kind     string         `mapstructure:"kind"`
	Name     string         `mapstructure:"name"`
	Enabled  *bool          `mapstructure:"enabled"`
	Priority string         `mapstructure:"priority"`
	Config   map[string]any `mapstructure:"config"`
}

// Settings is the bootstrap section shape under yggdrasil.bootstrap.
type Settings struct {
	Sources []SourceSpec `mapstructure:"sources"`
}

// Builder creates a source and resolves its load priority.
type Builder func(spec SourceSpec) (source.Source, config.Priority, error)

// Registry contains declarative bootstrap source builders.
type Registry struct {
	mu       sync.RWMutex
	builders map[string]Builder
}

// NewRegistry creates a registry with the built-in source kinds.
func NewRegistry() *Registry {
	r := &Registry{builders: map[string]Builder{}}
	r.Register("file", buildFileSource)
	r.Register("env", buildEnvSource)
	r.Register("flag", buildFlagSource)
	return r
}

// Register registers a source builder for a bootstrap kind.
func (r *Registry) Register(kind string, builder Builder) {
	if strings.TrimSpace(kind) == "" || builder == nil {
		return
	}
	r.mu.Lock()
	r.builders[strings.ToLower(strings.TrimSpace(kind))] = builder
	r.mu.Unlock()
}

// Build creates a source from a declarative spec.
func (r *Registry) Build(spec SourceSpec) (source.Source, config.Priority, error) {
	kind := strings.ToLower(strings.TrimSpace(spec.Kind))
	if kind == "" {
		return nil, 0, errors.New("bootstrap source kind is required")
	}
	r.mu.RLock()
	builder := r.builders[kind]
	r.mu.RUnlock()
	if builder == nil {
		return nil, 0, fmt.Errorf("bootstrap source kind %q not supported", kind)
	}
	return builder(spec)
}

func parsePriority(text string, fallback config.Priority) (config.Priority, error) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "":
		return fallback, nil
	case "defaults":
		return config.PriorityDefaults, nil
	case "file":
		return config.PriorityFile, nil
	case "remote":
		return config.PriorityRemote, nil
	case "env":
		return config.PriorityEnv, nil
	case "flag":
		return config.PriorityFlag, nil
	case "override":
		return config.PriorityOverride, nil
	default:
		return 0, fmt.Errorf("unknown bootstrap priority %q", text)
	}
}

type fileConfig struct {
	Path   string `mapstructure:"path"`
	Watch  bool   `mapstructure:"watch"`
	Parser string `mapstructure:"parser"`
}

func buildFileSource(spec SourceSpec) (source.Source, config.Priority, error) {
	var cfg fileConfig
	if err := mapstructure.Decode(spec.Config, &cfg); err != nil {
		return nil, 0, err
	}
	if strings.TrimSpace(cfg.Path) == "" {
		return nil, 0, errors.New("bootstrap file source path is required")
	}
	priority, err := parsePriority(spec.Priority, config.PriorityFile)
	if err != nil {
		return nil, 0, err
	}
	if strings.TrimSpace(cfg.Parser) == "" {
		return filesource.NewSource(cfg.Path, cfg.Watch), priority, nil
	}
	parser, err := source.ParseParser(cfg.Parser)
	if err != nil {
		return nil, 0, err
	}
	return filesource.NewSource(cfg.Path, cfg.Watch, parser), priority, nil
}

type envConfig struct {
	Prefixes         []string `mapstructure:"prefixes"`
	StrippedPrefixes []string `mapstructure:"stripped_prefixes"`
	Delimiter        string   `mapstructure:"delimiter"`
	ParseArray       bool     `mapstructure:"parse_array"`
	ArraySep         string   `mapstructure:"array_sep"`
	Name             string   `mapstructure:"name"`
}

func buildEnvSource(spec SourceSpec) (source.Source, config.Priority, error) {
	var cfg envConfig
	if err := mapstructure.Decode(spec.Config, &cfg); err != nil {
		return nil, 0, err
	}
	priority, err := parsePriority(spec.Priority, config.PriorityEnv)
	if err != nil {
		return nil, 0, err
	}
	opts := make([]envsource.Option, 0, 3)
	if cfg.Delimiter != "" {
		opts = append(opts, envsource.SetKeyDelimiter(cfg.Delimiter))
	}
	if cfg.ParseArray {
		opts = append(opts, envsource.WithParseArray(cfg.ArraySep))
	}
	if cfg.Name != "" {
		opts = append(opts, envsource.WithName(cfg.Name))
	}
	return envsource.NewSource(cfg.Prefixes, cfg.StrippedPrefixes, opts...), priority, nil
}

func buildFlagSource(spec SourceSpec) (source.Source, config.Priority, error) {
	priority, err := parsePriority(spec.Priority, config.PriorityFlag)
	if err != nil {
		return nil, 0, err
	}
	return flagsource.NewSource(), priority, nil
}
