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

package etcd

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config/source"
	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"
)

// NewConfigSource creates a new etcd config source.
func NewConfigSource(cfg ConfigSourceConfig) (source.Source, error) {
	mode := cfg.Mode
	if mode == "" {
		switch {
		case strings.TrimSpace(cfg.Key) != "":
			mode = ConfigSourceModeBlob
		case strings.TrimSpace(cfg.Prefix) != "":
			mode = ConfigSourceModeKV
		default:
			mode = ConfigSourceModeBlob
		}
	}
	cfg.Mode = mode
	if cfg.Format == nil {
		cfg.Format = yaml.Unmarshal
	}

	watch := cfg.Watch == nil || *cfg.Watch
	if cfg.Key == "" && cfg.Prefix == "" {
		return nil, errors.New("empty etcd config key/prefix")
	}
	if cfg.Key != "" && cfg.Prefix != "" {
		return nil, errors.New("both etcd config key and prefix are set")
	}

	cli, err := newClient(cfg.Client)
	if err != nil {
		return nil, err
	}

	name := cfg.Name
	if strings.TrimSpace(name) == "" {
		if cfg.Key != "" {
			name = cfg.Key
		} else {
			name = cfg.Prefix
		}
	}

	dialTimeout := cfg.Client.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}

	return &configSource{
		name:        name,
		cfg:         cfg,
		cli:         cli,
		watch:       watch,
		dialTimeout: dialTimeout,
		closeCh:     make(chan struct{}),
	}, nil
}

type configSource struct {
	name        string
	cfg         ConfigSourceConfig
	cli         *clientv3.Client
	watch       bool
	dialTimeout time.Duration

	closeOnce sync.Once
	closeCh   chan struct{}
}

func (s *configSource) Type() string     { return "etcd" }
func (s *configSource) Name() string     { return s.name }
func (s *configSource) Changeable() bool { return s.watch }

func (s *configSource) Read() (source.Data, error) {
	switch s.cfg.Mode {
	case ConfigSourceModeBlob:
		return s.readBlob()
	case ConfigSourceModeKV:
		return s.readKV()
	default:
		return nil, errors.New("unknown etcd config source mode")
	}
}

func (s *configSource) Watch() (<-chan source.Data, error) {
	if !s.watch {
		return nil, errors.New("etcd config source is not changeable")
	}

	out := make(chan source.Data)
	go func() {
		defer close(out)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			<-s.closeCh
			cancel()
		}()

		key := s.watchKey()
		ch := s.cli.Watch(ctx, key, s.watchOptions()...)
		for {
			select {
			case <-ctx.Done():
				return
			case resp, ok := <-ch:
				if !ok {
					return
				}
				if resp.Canceled {
					return
				}
				data, err := s.Read()
				if err != nil {
					continue
				}
				out <- data
			}
		}
	}()
	return out, nil
}

func (s *configSource) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeCh)
		_ = s.cli.Close()
	})
	return nil
}

func (s *configSource) watchKey() string {
	if s.cfg.Key != "" {
		return s.cfg.Key
	}
	return s.cfg.Prefix
}

func (s *configSource) watchOptions() []clientv3.OpOption {
	if s.cfg.Mode == ConfigSourceModeKV {
		return []clientv3.OpOption{clientv3.WithPrefix()}
	}
	return nil
}

func (s *configSource) readBlob() (source.Data, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.dialTimeout)
	defer cancel()
	resp, err := s.cli.Get(ctx, s.cfg.Key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return source.NewBytesSourceData(source.PriorityRemote, nil, s.cfg.Format), nil
	}
	return source.NewBytesSourceData(source.PriorityRemote, resp.Kvs[0].Value, s.cfg.Format), nil
}

func (s *configSource) readKV() (source.Data, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.dialTimeout)
	defer cancel()
	resp, err := s.cli.Get(ctx, s.cfg.Prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	out := map[string]any{}
	for _, kv := range resp.Kvs {
		rel := strings.TrimPrefix(string(kv.Key), s.cfg.Prefix)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			continue
		}
		rel = strings.ReplaceAll(rel, "/", ".")
		parts := splitConfigPath(rel, ".")
		val := parseScalarOrDoc(kv.Value, s.cfg.Format)
		setNested(out, parts, val)
	}
	return source.NewMapSourceData(source.PriorityRemote, out), nil
}

func parseScalarOrDoc(b []byte, parser source.Parser) any {
	if parser == nil {
		return string(b)
	}
	var v any
	if err := parser(b, &v); err == nil {
		return v
	}
	return string(b)
}

var keyRegx = regexp.MustCompile(`{([\w.-]+)}`)

func splitConfigPath(key, delimiter string) []string {
	matches := make([]string, 0)
	key = keyRegx.ReplaceAllStringFunc(key, func(s string) string {
		matches = append(matches, s[1:len(s)-1])
		return "{}"
	})
	paths := strings.Split(key, delimiter)
	j := 0
	for i, item := range paths {
		if item == "{}" {
			paths[i] = matches[j]
			j++
		}
	}
	return paths
}

func setNested(dst map[string]any, path []string, v any) {
	if len(path) == 0 {
		return
	}
	head, tail := path[0], path[1:]
	if len(tail) == 0 {
		dst[head] = v
		return
	}
	nxt, ok := dst[head]
	if !ok {
		m := map[string]any{}
		dst[head] = m
		nxt = m
	}
	nxtMap, ok := nxt.(map[string]any)
	if !ok {
		dst[head] = v
		return
	}
	setNested(nxtMap, tail, v)
}

// nolint:unused
type parserMap struct {
	json source.Parser
	yaml source.Parser
	toml source.Parser
}

// UnmarshalText implements encoding.TextUnmarshaler.
// nolint:unused
func (pm *parserMap) UnmarshalText(text []byte) error {
	switch string(text) {
	case "json":
		pm.json = json.Unmarshal
		pm.yaml = nil
		pm.toml = nil
	case "yaml", "yml":
		pm.json = nil
		pm.yaml = yaml.Unmarshal
		pm.toml = nil
	case "toml":
		pm.json = nil
		pm.yaml = nil
		pm.toml = nil
	default:
		pm.json = nil
		pm.yaml = yaml.Unmarshal
		pm.toml = nil
	}
	return nil
}

// Parser returns the parser for the given format.
// nolint:unused
func (pm *parserMap) Parser() source.Parser {
	if pm.json != nil {
		return pm.json
	}
	if pm.yaml != nil {
		return pm.yaml
	}
	if pm.toml != nil {
		return pm.toml
	}
	return yaml.Unmarshal
}
