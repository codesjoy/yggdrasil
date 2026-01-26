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
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
	yresolver "github.com/codesjoy/yggdrasil/v2/resolver"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Resolver implements resolver.Resolver.
type Resolver struct {
	name string
	cfg  ResolverConfig
	cli  *clientv3.Client

	mu       sync.Mutex
	watchers map[string]map[yresolver.Client]struct{}
	cancels  map[string]context.CancelFunc
}

// LoadResolverConfig loads resolver config from config.
func LoadResolverConfig(resolverName string) ResolverConfig {
	var cfg ResolverConfig
	_ = config.Get(config.Join(config.KeyBase, "resolver", resolverName, "config")).Scan(&cfg)
	return cfg
}

// NewResolver creates a new etcd resolver.
func NewResolver(name string, cfg ResolverConfig) (*Resolver, error) {
	cli, err := newClient(cfg.Client)
	if err != nil {
		return nil, err
	}
	return &Resolver{
		name:     name,
		cfg:      cfg,
		cli:      cli,
		watchers: map[string]map[yresolver.Client]struct{}{},
		cancels:  map[string]context.CancelFunc{},
	}, nil
}

// Type implements resolver.Resolver.
func (r *Resolver) Type() string { return "etcd" }

// AddWatch implements resolver.Resolver.
func (r *Resolver) AddWatch(serviceName string, watcher yresolver.Client) error {
	r.mu.Lock()
	ws := r.watchers[serviceName]
	if ws == nil {
		ws = map[yresolver.Client]struct{}{}
		r.watchers[serviceName] = ws
	}
	ws[watcher] = struct{}{}
	_, running := r.cancels[serviceName]
	if !running {
		ctx, cancel := context.WithCancel(context.Background())
		r.cancels[serviceName] = cancel
		go r.watchLoop(ctx, serviceName)
	}
	r.mu.Unlock()
	return nil
}

// DelWatch implements resolver.Resolver.
func (r *Resolver) DelWatch(serviceName string, watcher yresolver.Client) error {
	r.mu.Lock()
	ws := r.watchers[serviceName]
	if ws != nil {
		delete(ws, watcher)
		if len(ws) == 0 {
			delete(r.watchers, serviceName)
			if cancel, ok := r.cancels[serviceName]; ok {
				delete(r.cancels, serviceName)
				cancel()
			}
		}
	}
	r.mu.Unlock()
	return nil
}

func (r *Resolver) watchLoop(ctx context.Context, serviceName string) {
	debounce := r.cfg.Debounce
	if debounce <= 0 {
		debounce = 0
	}
	kick := make(chan struct{}, 1)

	notify := func() {
		select {
		case kick <- struct{}{}:
		default:
		}
	}

	notify()

	go func() {
		defer close(kick)
		prefix := r.servicePrefix(serviceName)
		var rev int64
		for {
			getResp, err := r.cli.Get(ctx, prefix, clientv3.WithPrefix())
			if err == nil {
				rev = getResp.Header.Revision
			}
			wch := r.cli.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithRev(rev+1))
			for resp := range wch {
				if resp.Canceled {
					return
				}
				notify()
			}
			select {
			case <-ctx.Done():
				return
			default:
				return
			}
		}
	}()

	var timer *time.Timer
	for {
		if debounce > 0 && timer == nil {
			timer = time.NewTimer(time.Hour)
			if !timer.Stop() {
				<-timer.C
			}
		}
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case _, ok := <-kick:
			if !ok {
				if timer != nil {
					timer.Stop()
				}
				return
			}
			if debounce <= 0 {
				r.fetchAndNotify(ctx, serviceName)
				continue
			}
			timer.Reset(debounce)
		case <-func() <-chan time.Time {
			if debounce <= 0 || timer == nil {
				return nil
			}
			return timer.C
		}():
			r.fetchAndNotify(ctx, serviceName)
		}
	}
}

func (r *Resolver) fetchAndNotify(ctx context.Context, serviceName string) {
	state, err := r.fetchState(ctx, serviceName)
	if err != nil {
		return
	}
	for _, w := range r.snapshotWatchers(serviceName) {
		w.UpdateState(state)
	}
}

func (r *Resolver) snapshotWatchers(serviceName string) []yresolver.Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws := r.watchers[serviceName]
	out := make([]yresolver.Client, 0, len(ws))
	for w := range ws {
		out = append(out, w)
	}
	return out
}

func (r *Resolver) fetchState(ctx context.Context, serviceName string) (yresolver.State, error) {
	prefix := r.servicePrefix(serviceName)
	resp, err := r.cli.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	ns := r.cfg.Namespace
	if ns == "" {
		ns = "default"
	}

	state := yresolver.BaseState{
		Attributes: map[string]any{
			"service":   serviceName,
			"namespace": ns,
			"revision":  resp.Header.Revision,
		},
		Endpoints: make([]yresolver.Endpoint, 0),
	}

	allow := toProtocolAllow(r.cfg.Protocols)
	instances := map[string]struct{}{}

	for _, kv := range resp.Kvs {
		var rec instanceRecord
		if err := json.Unmarshal(kv.Value, &rec); err != nil {
			continue
		}
		if rec.Name != serviceName {
			continue
		}
		if ns != "default" && rec.Namespace != ns {
			continue
		}

		for _, ep := range rec.Endpoints {
			if !allow[ep.Scheme] {
				continue
			}
			instKey := instanceKey(rec.Namespace, rec.Name, rec.Version, ep.Scheme, ep.Address)
			if _, ok := instances[instKey]; ok {
				continue
			}
			instances[instKey] = struct{}{}

			attrs := map[string]any{
				"instance_version": rec.Version,
				"instance_region":  rec.Region,
				"instance_zone":    rec.Zone,
				"instance_campus":  rec.Campus,
			}
			for k, v := range rec.Metadata {
				attrs[k] = v
			}
			for k, v := range ep.Metadata {
				attrs[k] = v
			}
			state.Endpoints = append(state.Endpoints, yresolver.BaseEndpoint{
				Address:    ep.Address,
				Protocol:   ep.Scheme,
				Attributes: attrs,
			})
		}
	}

	sort.Slice(state.Endpoints, func(i, j int) bool {
		return state.Endpoints[i].Name() < state.Endpoints[j].Name()
	})

	return state, nil
}

func (r *Resolver) servicePrefix(serviceName string) string {
	ns := r.cfg.Namespace
	if ns == "" {
		ns = "default"
	}
	return strings.Join([]string{r.cfg.Prefix, ns, serviceName}, "/")
}

func instanceKey(namespace, name, version, scheme, address string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s", namespace, name, version, scheme, address)
}

func toProtocolAllow(list []string) map[string]bool {
	if len(list) == 0 {
		return map[string]bool{"grpc": true, "http": true}
	}
	out := make(map[string]bool)
	for _, s := range list {
		out[s] = true
	}
	return out
}
