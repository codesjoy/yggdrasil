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
	// nolint:gosec
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	yregistry "github.com/codesjoy/yggdrasil/v2/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Registry is a registry for etcd.
type Registry struct {
	cfg RegistryConfig
	cli *clientv3.Client

	mu    sync.Mutex
	regs  map[string]registryEntry
	close chan struct{}
	once  sync.Once
}

type registryEntry struct {
	cancel context.CancelFunc
	lease  clientv3.LeaseID
}

// NewRegistry creates a new registry.
func NewRegistry(cfg RegistryConfig) (*Registry, error) {
	if cfg.Prefix == "" {
		cfg.Prefix = "/yggdrasil/registry"
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 10 * time.Second
	}
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = 3 * time.Second
	}
	cli, err := newClient(cfg.Client)
	if err != nil {
		return nil, err
	}
	return &Registry{
		cfg:   cfg,
		cli:   cli,
		regs:  map[string]registryEntry{},
		close: make(chan struct{}),
	}, nil
}

// Type returns the registry type.
func (r *Registry) Type() string { return "etcd" }

// Register adds a service instance to the registry.
func (r *Registry) Register(ctx context.Context, inst yregistry.Instance) error {
	if inst == nil {
		return errors.New("nil instance")
	}
	key, value, err := r.buildKeyValue(inst)
	if err != nil {
		return err
	}
	keepAlive := r.cfg.KeepAlive == nil || *r.cfg.KeepAlive

	r.mu.Lock()
	ent, ok := r.regs[key]
	if ok && ent.cancel != nil {
		ent.cancel()
		delete(r.regs, key)
	}
	bgCtx, cancel := context.WithCancel(context.Background())
	r.regs[key] = registryEntry{cancel: cancel}
	r.mu.Unlock()

	if err := r.putOnce(ctx, key, value); err != nil {
		cancel()
		r.mu.Lock()
		delete(r.regs, key)
		r.mu.Unlock()
		return err
	}

	go func() {
		defer func() {
			r.mu.Lock()
			delete(r.regs, key)
			r.mu.Unlock()
		}()

		if keepAlive {
			r.keepAliveLoop(bgCtx, key, value)
			return
		}

		select {
		case <-r.close:
			cancel()
		case <-bgCtx.Done():
		}
	}()

	return nil
}

// Deregister deletes a service instance from the registry.
func (r *Registry) Deregister(ctx context.Context, inst yregistry.Instance) error {
	if inst == nil {
		return errors.New("nil instance")
	}
	key, _, err := r.buildKeyValue(inst)
	if err != nil {
		return err
	}

	r.mu.Lock()
	ent, ok := r.regs[key]
	if ok && ent.cancel != nil {
		ent.cancel()
		delete(r.regs, key)
	}
	r.mu.Unlock()

	_, err = r.cli.Delete(ctx, key)
	return err
}

// Close closes the registry.
func (r *Registry) Close() error {
	r.once.Do(func() {
		close(r.close)
		r.mu.Lock()
		for _, ent := range r.regs {
			if ent.cancel != nil {
				ent.cancel()
			}
		}
		r.regs = map[string]registryEntry{}
		r.mu.Unlock()
		_ = r.cli.Close()
	})
	return nil
}

func (r *Registry) putOnce(ctx context.Context, key, value string) error {
	resp, err := r.cli.Grant(ctx, int64(r.cfg.TTL/time.Second))
	if err != nil {
		return err
	}
	_, err = r.cli.Put(ctx, key, value, clientv3.WithLease(resp.ID))
	if err != nil {
		return err
	}
	r.mu.Lock()
	ent := r.regs[key]
	ent.lease = resp.ID
	r.regs[key] = ent
	r.mu.Unlock()
	return nil
}

func (r *Registry) keepAliveLoop(ctx context.Context, key, value string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.close:
			return
		default:
		}

		resp, err := r.cli.Grant(ctx, int64(r.cfg.TTL/time.Second))
		if err != nil {
			time.Sleep(r.cfg.RetryInterval)
			continue
		}

		ka, kerr := r.cli.KeepAlive(ctx, resp.ID)
		if kerr != nil {
			time.Sleep(r.cfg.RetryInterval)
			continue
		}

		_, err = r.cli.Put(ctx, key, value, clientv3.WithLease(resp.ID))
		if err != nil {
			time.Sleep(r.cfg.RetryInterval)
			continue
		}

		r.mu.Lock()
		ent := r.regs[key]
		ent.lease = resp.ID
		r.regs[key] = ent
		r.mu.Unlock()

	LoopKeepAlive:
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.close:
				return
			case kaResp, ok := <-ka:
				if !ok {
					break LoopKeepAlive
				}
				if kaResp.TTL <= 0 {
					break LoopKeepAlive
				}
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-r.close:
			return
		case <-time.After(r.cfg.RetryInterval):
		}
	}
}

func (r *Registry) buildKeyValue(inst yregistry.Instance) (string, string, error) {
	eps := inst.Endpoints()
	epRecs := make([]endpointRecord, len(eps))
	for i, ep := range eps {
		epRecs[i] = endpointRecord{
			Scheme:   ep.Scheme(),
			Address:  ep.Address(),
			Metadata: ep.Metadata(),
		}
	}
	rec := instanceRecord{
		Namespace: inst.Namespace(),
		Name:      inst.Name(),
		Version:   inst.Version(),
		Region:    inst.Region(),
		Zone:      inst.Zone(),
		Campus:    inst.Campus(),
		Metadata:  inst.Metadata(),
		Endpoints: epRecs,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return "", "", err
	}
	h := sha1.New() // nolint:gosec
	h.Write(b)
	sum := hex.EncodeToString(h.Sum(nil))

	parts := []string{r.cfg.Prefix}
	if inst.Namespace() != "" {
		parts = append(parts, inst.Namespace())
	}
	if inst.Name() != "" {
		parts = append(parts, inst.Name())
	}
	parts = append(parts, sum)
	key := strings.Join(parts, "/")
	return key, string(b), nil
}

type demoInstance struct {
	namespace string
	name      string
	version   string
	region    string
	zone      string
	campus    string
	metadata  map[string]string
	endpoints []yregistry.Endpoint
}

func (d demoInstance) Region() string                  { return d.region }
func (d demoInstance) Zone() string                    { return d.zone }
func (d demoInstance) Campus() string                  { return d.campus }
func (d demoInstance) Namespace() string               { return d.namespace }
func (d demoInstance) Name() string                    { return d.name }
func (d demoInstance) Version() string                 { return d.version }
func (d demoInstance) Metadata() map[string]string     { return d.metadata }
func (d demoInstance) Endpoints() []yregistry.Endpoint { return d.endpoints }

type demoEndpoint struct {
	scheme   string
	address  string
	metadata map[string]string
}

func (d demoEndpoint) Scheme() string              { return d.scheme }
func (d demoEndpoint) Address() string             { return d.address }
func (d demoEndpoint) Metadata() map[string]string { return d.metadata }
