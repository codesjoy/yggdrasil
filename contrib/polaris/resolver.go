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
	"context"
	"errors"
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
	yresolver "github.com/codesjoy/yggdrasil/v2/resolver"
	polaris "github.com/polarismesh/polaris-go"
)

// ResolverConfig is the config for the Polaris resolver.
type ResolverConfig struct {
	Addresses       []string          `mapstructure:"addresses"`
	SDK             string            `mapstructure:"sdk"`
	Namespace       string            `mapstructure:"namespace"`
	Protocols       []string          `mapstructure:"protocols"`
	RefreshInterval time.Duration     `mapstructure:"refreshInterval"`
	Timeout         time.Duration     `mapstructure:"timeout"`
	RetryCount      int               `mapstructure:"retryCount"`
	SkipRouteFilter bool              `mapstructure:"skipRouteFilter"`
	Metadata        map[string]string `mapstructure:"metadata"`
}

// LoadResolverConfig loads the resolver config for the given name.
func LoadResolverConfig(resolverName string) ResolverConfig {
	var cfg ResolverConfig
	_ = config.Get(config.Join(config.KeyBase, "resolver", resolverName, "config")).Scan(&cfg)
	return cfg
}

// Resolver is the Polaris resolver.
type Resolver struct {
	name    string
	cfg     ResolverConfig
	api     consumerAPI
	initErr error

	mu       sync.Mutex
	watchers map[string]map[yresolver.Client]struct{}
	cancels  map[string]context.CancelFunc
}

// NewResolver creates a new Polaris resolver.
func NewResolver(name string, cfg ResolverConfig) (*Resolver, error) {
	sdkName := resolveSDKName(name, cfg.SDK)
	cfg.Addresses = resolveSDKAddresses(name, cfg.SDK, cfg.Addresses)
	api, err := getSDKHolder(sdkName, cfg.Addresses, nil).getConsumer()
	if err != nil {
		return nil, err
	}
	return &Resolver{
		name:     name,
		cfg:      cfg,
		api:      api,
		watchers: map[string]map[yresolver.Client]struct{}{},
		cancels:  map[string]context.CancelFunc{},
	}, nil
}

// NewResolverWithError creates a new Polaris resolver with the given error.
func NewResolverWithError(name string, cfg ResolverConfig, initErr error) *Resolver {
	return &Resolver{
		name:     name,
		cfg:      cfg,
		initErr:  initErr,
		watchers: map[string]map[yresolver.Client]struct{}{},
		cancels:  map[string]context.CancelFunc{},
	}
}

// Type returns the type of the resolver.
func (r *Resolver) Type() string { return "polaris" }

// AddWatch adds a watcher for the given app name.
func (r *Resolver) AddWatch(appName string, watcher yresolver.Client) error {
	if r.initErr != nil {
		return r.initErr
	}
	if appName == "" {
		return errors.New("empty app name")
	}

	r.mu.Lock()
	ws := r.watchers[appName]
	if ws == nil {
		ws = map[yresolver.Client]struct{}{}
		r.watchers[appName] = ws
	}
	ws[watcher] = struct{}{}
	_, running := r.cancels[appName]
	if !running {
		ctx, cancel := context.WithCancel(context.Background())
		r.cancels[appName] = cancel
		go r.watchLoop(ctx, appName)
	}
	r.mu.Unlock()
	return nil
}

// DelWatch removes the watcher for the given app name.
func (r *Resolver) DelWatch(appName string, watcher yresolver.Client) error {
	if r.initErr != nil {
		return r.initErr
	}
	r.mu.Lock()
	ws := r.watchers[appName]
	if ws != nil {
		delete(ws, watcher)
		if len(ws) == 0 {
			delete(r.watchers, appName)
			if cancel, ok := r.cancels[appName]; ok {
				delete(r.cancels, appName)
				cancel()
			}
		}
	}
	r.mu.Unlock()
	return nil
}

func (r *Resolver) watchLoop(ctx context.Context, appName string) {
	interval := r.cfg.RefreshInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	r.fetchAndNotify(ctx, appName)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.fetchAndNotify(ctx, appName)
		}
	}
}

func (r *Resolver) fetchAndNotify(ctx context.Context, appName string) {
	state, err := r.fetchState(ctx, appName)
	if err != nil {
		return
	}
	for _, w := range r.snapshotWatchers(appName) {
		w.UpdateState(state)
	}
}

func (r *Resolver) snapshotWatchers(appName string) []yresolver.Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws := r.watchers[appName]
	out := make([]yresolver.Client, 0, len(ws))
	for w := range ws {
		out = append(out, w)
	}
	return out
}

func (r *Resolver) fetchState(ctx context.Context, appName string) (yresolver.State, error) {
	namespace := r.cfg.Namespace
	if namespace == "" {
		namespace = "default"
	}

	req := &polaris.GetInstancesRequest{}
	req.Service = appName
	req.Namespace = namespace
	req.SkipRouteFilter = r.cfg.SkipRouteFilter
	if r.cfg.Metadata != nil {
		req.Metadata = r.cfg.Metadata
	}

	if d := effectiveTimeout(ctx, r.cfg.Timeout); d > 0 {
		req.Timeout = &d
	}
	if r.cfg.RetryCount > 0 {
		retry := r.cfg.RetryCount
		req.RetryCount = &retry
	}

	resp, err := r.api.GetInstances(req)
	if err != nil {
		return nil, err
	}

	state := yresolver.BaseState{
		Attributes: map[string]any{
			"service":                    appName,
			"namespace":                  namespace,
			"revision":                   resp.Revision,
			"polaris_instances_response": resp,
		},
		Endpoints: make([]yresolver.Endpoint, 0, len(resp.Instances)),
	}
	protocolAllow := toAllowSet(r.cfg.Protocols, []string{"grpc"})
	for _, inst := range resp.Instances {
		proto := inst.GetProtocol()
		if !protocolAllow[proto] {
			continue
		}
		addr := netAddr(inst.GetHost(), inst.GetPort())
		attrs := map[string]any{
			"instance_id": inst.GetId(),
			"weight":      inst.GetWeight(),
			"priority":    inst.GetPriority(),
			"version":     inst.GetVersion(),
		}
		for k, v := range inst.GetMetadata() {
			attrs[k] = v
		}
		state.Endpoints = append(state.Endpoints, yresolver.BaseEndpoint{
			Address:    addr,
			Protocol:   proto,
			Attributes: attrs,
		})
	}
	return state, nil
}

func toAllowSet(list []string, def []string) map[string]bool {
	if len(list) == 0 {
		list = def
	}
	out := make(map[string]bool, len(list))
	for _, s := range list {
		out[s] = true
	}
	return out
}
