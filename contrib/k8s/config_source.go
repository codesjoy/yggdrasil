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

package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/codesjoy/yggdrasil/v2/config/source"
)

// ConfigSourceConfig defines the configuration for a Kubernetes ConfigMap or Secret source.
type ConfigSourceConfig struct {
	Namespace   string          `mapstructure:"namespace"`
	Name        string          `mapstructure:"name"`
	Key         string          `mapstructure:"key"`
	MergeAllKey bool            `mapstructure:"mergeAllKey"`
	Format      source.Parser   `mapstructure:"format"`
	Priority    source.Priority `mapstructure:"priority"`
	Watch       bool            `mapstructure:"watch"`
	Kubeconfig  string          `mapstructure:"kubeconfig"`
}

// NewConfigMapSource creates a new ConfigMap source.
func NewConfigMapSource(cfg ConfigSourceConfig) (source.Source, error) {
	if cfg.Name == "" {
		return nil, errors.New("empty configmap name")
	}
	if cfg.Priority == 0 {
		cfg.Priority = source.PriorityRemote
	}
	return &configSource{
		resourceType: "configmap",
		cfg:          cfg,
		watch:        cfg.Watch,
		closeCh:      make(chan struct{}),
	}, nil
}

// NewSecretSource creates a new Secret source.
func NewSecretSource(cfg ConfigSourceConfig) (source.Source, error) {
	if cfg.Name == "" {
		return nil, errors.New("empty secret name")
	}
	if cfg.Priority == 0 {
		cfg.Priority = source.PriorityRemote
	}
	return &configSource{
		resourceType: "secret",
		cfg:          cfg,
		watch:        cfg.Watch,
		closeCh:      make(chan struct{}),
	}, nil
}

type configSource struct {
	resourceType string
	cfg          ConfigSourceConfig
	watch        bool

	closeOnce sync.Once
	closeCh   chan struct{}
}

func (s *configSource) Name() string {
	return s.cfg.Name
}

func (s *configSource) Type() string {
	return s.resourceType
}

func (s *configSource) Read() (source.Data, error) {
	data, parser, err := s.fetch()
	if err != nil {
		return nil, err
	}
	if s.cfg.MergeAllKey {
		return source.NewMapSourceData(s.cfg.Priority, data), nil
	}
	var key string
	if s.cfg.Key != "" {
		key = s.cfg.Key
	} else {
		key = inferKeyFromData(data)
	}
	val, ok := data[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found", key)
	}
	str, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("key %q is not a string", key)
	}
	if parser == nil {
		parser = inferParser(s.cfg.Key)
	}
	return source.NewBytesSourceData(s.cfg.Priority, []byte(str), parser), nil
}

func (s *configSource) Changeable() bool {
	return s.watch
}

func (s *configSource) Watch() (<-chan source.Data, error) {
	if !s.watch {
		return nil, errors.New("watch disabled for this source")
	}
	kube, err := GetKubeClient(s.cfg.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get kube client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stop := func() {
		cancel()
		close(s.closeCh)
	}

	out := make(chan source.Data)
	go func() {
		defer close(out)
		var last string
		for {
			ch, err := s.doWatch(ctx, kube)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				time.Sleep(time.Second)
				continue
			}
			for ev := range ch {
				if ev.Type == watch.Deleted {
					stop()
					return
				}
				if ev.Type != watch.Added && ev.Type != watch.Modified {
					continue
				}
				data, parser, err := s.fetch()
				if err != nil {
					continue
				}
				var content string
				if s.cfg.MergeAllKey {
					mapData := source.NewMapSourceData(s.cfg.Priority, data)
					if bs := mapData.Data(); len(bs) > 0 {
						content = string(bs)
					}
				} else {
					var key string
					if s.cfg.Key != "" {
						key = s.cfg.Key
					} else {
						key = inferKeyFromData(data)
					}
					val, ok := data[key]
					if !ok {
						continue
					}
					str, ok := val.(string)
					if !ok {
						continue
					}
					content = str
					if parser == nil {
						parser = inferParser(key)
					}
				}
				if content != last {
					last = content
					if s.cfg.MergeAllKey {
						out <- source.NewMapSourceData(s.cfg.Priority, data)
					} else {
						out <- source.NewBytesSourceData(s.cfg.Priority, []byte(content), parser)
					}
				}
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()
	return out, nil
}

func (s *configSource) Close() error {
	s.closeOnce.Do(func() {
		select {
		case <-s.closeCh:
		default:
			close(s.closeCh)
		}
	})
	return nil
}

func (s *configSource) fetch() (map[string]any, source.Parser, error) {
	kube, err := GetKubeClient(s.cfg.Kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get kube client: %w", err)
	}
	var (
		data   map[string]any
		parser source.Parser
	)
	if s.resourceType == "configmap" {
		cm, err := kube.CoreV1().
			ConfigMaps(s.cfg.Namespace).
			Get(context.Background(), s.cfg.Name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get configmap: %w", err)
		}
		data = make(map[string]any, len(cm.Data))
		for k, v := range cm.Data {
			data[k] = v
		}
		if s.cfg.Format != nil {
			parser = s.cfg.Format
		} else if s.cfg.Key != "" {
			parser = inferParser(s.cfg.Key)
		}
	} else {
		secret, err := kube.CoreV1().Secrets(s.cfg.Namespace).Get(context.Background(), s.cfg.Name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get secret: %w", err)
		}
		data = make(map[string]any, len(secret.Data))
		for k, v := range secret.Data {
			data[k] = string(v)
		}
		if s.cfg.Format != nil {
			parser = s.cfg.Format
		} else if s.cfg.Key != "" {
			parser = inferParser(s.cfg.Key)
		}
	}
	return data, parser, nil
}

func (s *configSource) doWatch(
	ctx context.Context,
	kube kubernetes.Interface,
) (<-chan watch.Event, error) {
	opts := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", s.cfg.Name),
	}
	var w watch.Interface
	var err error
	if s.resourceType == "configmap" {
		w, err = kube.CoreV1().ConfigMaps(s.cfg.Namespace).Watch(ctx, opts)
	} else {
		w, err = kube.CoreV1().Secrets(s.cfg.Namespace).Watch(ctx, opts)
	}
	if err != nil {
		return nil, err
	}
	return w.ResultChan(), nil
}

func inferParser(key string) source.Parser {
	ext := strings.ToLower(filepath.Ext(key))
	switch ext {
	case ".json":
		return json.Unmarshal
	case ".yaml", ".yml":
		var p source.Parser
		_ = p.UnmarshalText([]byte("yaml"))
		return p
	default:
		var p source.Parser
		_ = p.UnmarshalText([]byte("yaml"))
		return p
	}
}

func inferKeyFromData(data map[string]any) string {
	for k := range data {
		if strings.Contains(strings.ToLower(k), ".yaml") ||
			strings.Contains(strings.ToLower(k), ".yml") ||
			strings.Contains(strings.ToLower(k), ".json") ||
			strings.Contains(strings.ToLower(k), ".toml") {
			return k
		}
	}
	if len(data) > 0 {
		for k := range data {
			return k
		}
	}
	return "config"
}
