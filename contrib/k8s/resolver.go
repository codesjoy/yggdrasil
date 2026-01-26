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
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/codesjoy/yggdrasil/v2/resolver"
)

// Resolver implements resolver.Resolver
type Resolver struct {
	name    string
	cfg     ResolverConfig
	bo      *backoff
	initErr error

	mu       sync.Mutex
	watchers map[string]map[resolver.Client]struct{}
	cancels  map[string]context.CancelFunc
	states   map[string]resolver.State
}

// NewResolver implements resolver.NewResolver
func NewResolver(name string, cfg ResolverConfig) (*Resolver, error) {
	return &Resolver{
		name:     name,
		cfg:      cfg,
		bo:       newBackoff(cfg.Backoff),
		watchers: map[string]map[resolver.Client]struct{}{},
		cancels:  map[string]context.CancelFunc{},
		states:   map[string]resolver.State{},
	}, nil
}

// Type implements resolver.Resolver
func (r *Resolver) Type() string {
	return "kubernetes"
}

// AddWatch implements resolver.Resolver
func (r *Resolver) AddWatch(appName string, w resolver.Client) error {
	if r.initErr != nil {
		return r.initErr
	}
	if appName == "" {
		return errors.New("empty app name")
	}
	r.mu.Lock()
	ws := r.watchers[appName]
	if ws == nil {
		ws = map[resolver.Client]struct{}{}
		r.watchers[appName] = ws
	}
	ws[w] = struct{}{}
	_, running := r.cancels[appName]
	if !running {
		ctx, cancel := context.WithCancel(context.Background())
		r.cancels[appName] = cancel
		go r.watchLoop(ctx, appName)
	}
	r.mu.Unlock()

	if state, ok := r.getState(appName); ok {
		w.UpdateState(state)
	}
	return nil
}

// DelWatch implements resolver.Resolver
func (r *Resolver) DelWatch(appName string, w resolver.Client) error {
	r.mu.Lock()
	ws := r.watchers[appName]
	if ws != nil {
		delete(ws, w)
		if len(ws) == 0 {
			delete(r.watchers, appName)
			if cancel, ok := r.cancels[appName]; ok {
				delete(r.cancels, appName)
				cancel()
			}
			delete(r.states, appName)
		}
	}
	r.mu.Unlock()
	return nil
}

func (r *Resolver) getState(appName string) (resolver.State, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.states[appName]
	return s, ok
}

func (r *Resolver) setState(appName string, state resolver.State) {
	r.mu.Lock()
	r.states[appName] = state
	r.mu.Unlock()
}

func (r *Resolver) snapshotWatchers(appName string) []resolver.Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws := r.watchers[appName]
	out := make([]resolver.Client, 0, len(ws))
	for w := range ws {
		out = append(out, w)
	}
	return out
}

func (r *Resolver) notify(appName string, state resolver.State) {
	for _, w := range r.snapshotWatchers(appName) {
		w.UpdateState(state)
	}
}

func (r *Resolver) watchLoop(ctx context.Context, appName string) {
	retries := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := r.watch(ctx, appName); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
		}
		retries++
		delay := r.bo.Backoff(retries)
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

func (r *Resolver) watch(ctx context.Context, appName string) error {
	kube, err := GetKubeClient(r.cfg.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	if r.cfg.Mode == string(modeEndpointSlice) {
		err = r.watchEndpointSlice(ctx, kube, appName)
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) {
			return err
		}
	}
	return r.watchEndpoints(ctx, kube, appName)
}

//nolint:staticcheck // SA1019: corev1.Endpoints is deprecated in v1.33+, kept for backward compatibility with older Kubernetes clusters
func (r *Resolver) watchEndpoints(
	ctx context.Context,
	kube kubernetes.Interface,
	appName string,
) error {
	listOpts := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", appName),
	}
	for {
		w, err := kube.CoreV1().Endpoints(r.cfg.Namespace).Watch(ctx, listOpts)
		if err != nil {
			return fmt.Errorf("failed to watch endpoints: %w", err)
		}

		state, err := r.listEndpoints(ctx, kube, appName)
		if err != nil {
			w.Stop()
			return fmt.Errorf("failed to list endpoints: %w", err)
		}
		r.setState(appName, state)
		r.notify(appName, state)

		for ev := range w.ResultChan() {
			if ev.Type == watch.Deleted {
				emptyState := resolver.BaseState{Endpoints: []resolver.Endpoint{}}
				r.setState(appName, emptyState)
				r.notify(appName, emptyState)
				continue
			}
			if ev.Type != watch.Added && ev.Type != watch.Modified {
				continue
			}
			state, err := r.listEndpoints(ctx, kube, appName)
			if err != nil {
				w.Stop()
				return fmt.Errorf("failed to list endpoints after event: %w", err)
			}
			r.setState(appName, state)
			r.notify(appName, state)
		}
		w.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

//nolint:staticcheck // SA1019: corev1.Endpoints is deprecated in v1.33+, kept for backward compatibility with older Kubernetes clusters
func (r *Resolver) listEndpoints(
	ctx context.Context,
	kube kubernetes.Interface,
	appName string,
) (resolver.State, error) {
	opts := metav1.GetOptions{}
	ep, err := kube.CoreV1().Endpoints(r.cfg.Namespace).Get(ctx, appName, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoints: %w", err)
	}
	return r.endpointsToState(ep), nil
}

//nolint:staticcheck // SA1019: corev1.Endpoints is deprecated in v1.33+, kept for backward compatibility with older Kubernetes clusters
func (r *Resolver) endpointsToState(ep *corev1.Endpoints) resolver.State {
	baseState := resolver.BaseState{
		Attributes: map[string]any{
			"service":   ep.Name,
			"namespace": ep.Namespace,
		},
		Endpoints: []resolver.Endpoint{},
	}

	for _, subset := range ep.Subsets {
		for _, addr := range subset.Addresses {
			if addr.IP == "" {
				continue
			}
			port := r.selectPort(subset.Ports)
			if port == nil {
				continue
			}
			epAddr := net.JoinHostPort(addr.IP, strconv.Itoa(int(port.Port)))
			attrs := map[string]any{
				"hostname": addr.Hostname,
				"nodeName": addr.NodeName,
			}
			if addr.TargetRef != nil {
				attrs["targetRefKind"] = addr.TargetRef.Kind
				attrs["targetRefName"] = addr.TargetRef.Name
			}
			if r.cfg.EndpointAttributes != nil {
				for k, v := range r.cfg.EndpointAttributes {
					attrs[k] = v
				}
			}
			baseState.Endpoints = append(baseState.Endpoints, resolver.BaseEndpoint{
				Address:    epAddr,
				Protocol:   r.cfg.Protocol,
				Attributes: attrs,
			})
		}
	}
	return baseState
}

//nolint:staticcheck // SA1019: corev1.EndpointPort is deprecated in v1.33+, kept for backward compatibility with older Kubernetes clusters
func (r *Resolver) selectPort(ports []corev1.EndpointPort) *corev1.EndpointPort {
	if len(ports) == 0 {
		return nil
	}
	if r.cfg.PortName != "" {
		for i := range ports {
			if ports[i].Name == r.cfg.PortName {
				return &ports[i]
			}
		}
	}
	if r.cfg.Port != 0 {
		for i := range ports {
			if ports[i].Port == r.cfg.Port {
				return &ports[i]
			}
		}
	}
	return &ports[0]
}

func (r *Resolver) watchEndpointSlice(
	ctx context.Context,
	kube kubernetes.Interface,
	appName string,
) error {
	listOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("kubernetes.io/service-name=%s", appName),
	}
	for {
		w, err := kube.DiscoveryV1().EndpointSlices(r.cfg.Namespace).Watch(ctx, listOpts)
		if err != nil {
			return fmt.Errorf("failed to watch endpointslices: %w", err)
		}

		state, err := r.listEndpointSlice(ctx, kube, appName)
		if err != nil {
			w.Stop()
			return fmt.Errorf("failed to list endpointslices: %w", err)
		}
		r.setState(appName, state)
		r.notify(appName, state)

		for ev := range w.ResultChan() {
			if ev.Type == watch.Deleted {
				emptyState := resolver.BaseState{Endpoints: []resolver.Endpoint{}}
				r.setState(appName, emptyState)
				r.notify(appName, emptyState)
				continue
			}
			if ev.Type != watch.Added && ev.Type != watch.Modified {
				continue
			}
			state, err := r.listEndpointSlice(ctx, kube, appName)
			if err != nil {
				w.Stop()
				return fmt.Errorf("failed to list endpointslices after event: %w", err)
			}
			r.setState(appName, state)
			r.notify(appName, state)
		}
		w.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

func (r *Resolver) listEndpointSlice(
	ctx context.Context,
	kube kubernetes.Interface,
	appName string,
) (resolver.State, error) {
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("kubernetes.io/service-name=%s", appName),
	}
	slices, err := kube.DiscoveryV1().EndpointSlices(r.cfg.Namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list endpointslices: %w", err)
	}
	return r.endpointSlicesToState(slices.Items), nil
}

func (r *Resolver) endpointSlicesToState(slices []discoveryv1.EndpointSlice) resolver.State {
	baseState := resolver.BaseState{
		Attributes: map[string]any{},
		Endpoints:  []resolver.Endpoint{},
	}
	for _, slice := range slices {
		for _, port := range slice.Ports {
			if r.cfg.PortName != "" && *port.Name != r.cfg.PortName {
				continue
			}
			if r.cfg.Port != 0 && *port.Port != r.cfg.Port {
				continue
			}
			for _, endpoint := range slice.Endpoints {
				if len(endpoint.Addresses) == 0 {
					continue
				}
				for _, addr := range endpoint.Addresses {
					if addr == "" {
						continue
					}
					epAddr := net.JoinHostPort(addr, strconv.Itoa(int(*port.Port)))
					attrs := map[string]any{}
					if endpoint.NodeName != nil {
						attrs["nodeName"] = *endpoint.NodeName
					}
					if endpoint.Zone != nil {
						attrs["zone"] = *endpoint.Zone
					}
					if endpoint.TargetRef != nil {
						attrs["targetRefKind"] = endpoint.TargetRef.Kind
						attrs["targetRefName"] = endpoint.TargetRef.Name
					}
					if r.cfg.EndpointAttributes != nil {
						for k, v := range r.cfg.EndpointAttributes {
							attrs[k] = v
						}
					}
					baseState.Endpoints = append(baseState.Endpoints, resolver.BaseEndpoint{
						Address:    epAddr,
						Protocol:   r.cfg.Protocol,
						Attributes: attrs,
					})
				}
			}
		}
	}
	return baseState
}
