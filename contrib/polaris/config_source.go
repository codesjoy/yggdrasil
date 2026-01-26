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

package polaris

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config/source"
	"github.com/pelletier/go-toml/v2"
	polaris "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
	"gopkg.in/yaml.v3"
)

// ConfigSourceConfig is the config for the Polaris config source.
type ConfigSourceConfig struct {
	Addresses    []string      `mapstructure:"addresses"`
	SDK          string        `mapstructure:"sdk"`
	Namespace    string        `mapstructure:"namespace"`
	FileGroup    string        `mapstructure:"fileGroup"`
	FileName     string        `mapstructure:"fileName"`
	Subscribe    *bool         `mapstructure:"subscribe"`
	Mode         int           `mapstructure:"mode"`
	Format       source.Parser `mapstructure:"format"`
	FetchTimeout time.Duration `mapstructure:"fetchTimeout"`

	API configAPI `mapstructure:"-"`
}

// NewConfigSource creates a new Polaris config source.
func NewConfigSource(cfg ConfigSourceConfig) (source.Source, error) {
	if strings.TrimSpace(cfg.FileName) == "" {
		return nil, errors.New("empty polaris config fileName")
	}
	subscribe := cfg.Subscribe == nil || *cfg.Subscribe
	return &configSource{
		name:      cfg.FileName,
		cfg:       cfg,
		subscribe: subscribe,
		closeCh:   make(chan struct{}),
	}, nil
}

type configSource struct {
	name      string
	cfg       ConfigSourceConfig
	subscribe bool

	closeOnce sync.Once
	closeCh   chan struct{}
}

func (s *configSource) Name() string { return s.name }

func (s *configSource) Type() string { return "polaris" }

func (s *configSource) Read() (source.Data, error) {
	file, parser, err := s.fetchConfigFile()
	if err != nil {
		return nil, err
	}
	return source.NewBytesSourceData(source.PriorityRemote, []byte(file.GetContent()), parser), nil
}

func (s *configSource) Changeable() bool { return s.subscribe }

func (s *configSource) Watch() (<-chan source.Data, error) {
	if !s.subscribe {
		return nil, errors.New("polaris config source is not subscribable")
	}
	file, parser, err := s.fetchConfigFile()
	if err != nil {
		return nil, err
	}

	out := make(chan source.Data)
	ch := file.AddChangeListenerWithChannel()
	go func() {
		defer close(out)
		for {
			select {
			case <-s.closeCh:
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				out <- source.NewBytesSourceData(source.PriorityRemote, []byte(ev.NewValue), parser)
			}
		}
	}()
	return out, nil
}

func (s *configSource) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeCh)
	})
	return nil
}

func (s *configSource) fetchConfigFile() (model.ConfigFile, source.Parser, error) {
	namespace := s.cfg.Namespace
	if namespace == "" {
		namespace = yggdrasil.InstanceNamespace()
	}
	fileGroup := s.cfg.FileGroup
	if fileGroup == "" {
		fileGroup = "default"
	}

	parser := s.cfg.Format
	if parser == nil {
		parser = inferParserFromFilename(s.cfg.FileName)
	}

	client := s.cfg.API
	if client == nil {
		sdkName := resolveSDKName("default", s.cfg.SDK)
		addresses := resolveSDKConfigAddresses("default", s.cfg.SDK, s.cfg.Addresses)
		api, err := getSDKHolder(sdkName, nil, addresses).getConfig()
		if err != nil {
			return nil, nil, err
		}
		client = api
	}

	req := &polaris.GetConfigFileRequest{GetConfigFileRequest: &model.GetConfigFileRequest{
		Namespace: namespace,
		FileGroup: fileGroup,
		FileName:  s.cfg.FileName,
		Subscribe: s.subscribe,
		Mode:      model.GetConfigFileRequestMode(s.cfg.Mode),
	}}
	if req.Mode == 0 {
		req.Mode = model.SDKMode
	}

	if s.cfg.FetchTimeout <= 0 {
		file, err := client.FetchConfigFile(req)
		if err != nil {
			return nil, nil, err
		}
		return file, parser, nil
	}

	type resp struct {
		file model.ConfigFile
		err  error
	}
	ch := make(chan resp, 1)
	go func() {
		f, err := client.FetchConfigFile(req)
		ch <- resp{file: f, err: err}
	}()
	t := time.NewTimer(s.cfg.FetchTimeout)
	defer t.Stop()
	select {
	case r := <-ch:
		if r.err != nil {
			return nil, nil, r.err
		}
		return r.file, parser, nil
	case <-t.C:
		return nil, nil, errors.New("polaris config fetch timeout")
	}
}

func inferParserFromFilename(name string) source.Parser {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".json":
		return json.Unmarshal
	case ".toml":
		return toml.Unmarshal
	case ".yaml", ".yml":
		return yaml.Unmarshal
	default:
		return yaml.Unmarshal
	}
}
