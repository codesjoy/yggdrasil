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
	"sync/atomic"
	"time"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/codesjoy/yggdrasil/v2/balancer"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/status"
	"github.com/codesjoy/yggdrasil/v2/utils/xmap"
	polaris "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
	"google.golang.org/genproto/googleapis/rpc/code"
)

const polarisBalancerName = "polaris"

func init() {
	balancer.RegisterBuilder(polarisBalancerName, newPolarisBalancer)
}

type polarisBalancer struct {
	serviceName string
	cli         balancer.Client

	mu                sync.RWMutex
	remoteByName      map[string]remote.Client
	remoteByInstance  map[string]remote.Client
	instancesResponse *model.InstancesResponse

	governance governanceConfig
	router     routerAPI
	routerErr  error
	limit      limitAPI
	limitErr   error
	cb         circuitBreakerAPI
	cbErr      error
}

func newPolarisBalancer(
	serviceName, balancerName string,
	cli balancer.Client,
) (balancer.Balancer, error) {
	cfgMap := loadGovernanceConfigMap(serviceName)
	overrideMap := balancer.LoadConfig(serviceName, balancerName).Map(nil)
	if len(overrideMap) > 0 {
		xmap.MergeStringMap(cfgMap, overrideMap)
		xmap.CoverInterfaceMapToStringMap(cfgMap)
	}
	cfg := decodeGovernanceConfig(cfgMap)
	addresses := resolveSDKAddresses(serviceName, cfg.SDK, cfg.Addresses)
	sdkName := resolveSDKName(serviceName, cfg.SDK)
	holder := getSDKHolder(sdkName, addresses, nil)
	r, rErr := holder.getRouter()
	l, lErr := holder.getLimit()
	cb, cbErr := holder.getCircuitBreaker()
	return &polarisBalancer{
		serviceName:      serviceName,
		cli:              cli,
		remoteByName:     make(map[string]remote.Client),
		remoteByInstance: make(map[string]remote.Client),
		governance:       cfg,
		router:           r,
		routerErr:        rErr,
		limit:            l,
		limitErr:         lErr,
		cb:               cb,
		cbErr:            cbErr,
	}, nil
}

func (b *polarisBalancer) Type() string { return polarisBalancerName }

func (b *polarisBalancer) Close() error {
	b.mu.Lock()
	clients := make([]remote.Client, 0, len(b.remoteByName))
	for _, cli := range b.remoteByName {
		clients = append(clients, cli)
	}
	b.remoteByName = nil
	b.remoteByInstance = nil
	b.instancesResponse = nil
	picker := b.buildPickerLocked()
	b.mu.Unlock()
	b.cli.UpdateState(balancer.State{Picker: picker})
	var multiErr error
	for _, cli := range clients {
		multiErr = errors.Join(multiErr, cli.Close())
	}
	return multiErr
}

func (b *polarisBalancer) UpdateState(state resolver.State) {
	b.mu.Lock()
	if b.remoteByName == nil {
		b.mu.Unlock()
		return
	}

	nextByName := make(map[string]remote.Client, len(state.GetEndpoints()))
	nextByInstance := make(map[string]remote.Client, len(state.GetEndpoints()))
	for _, ep := range state.GetEndpoints() {
		if cli, ok := b.remoteByName[ep.Name()]; ok {
			nextByName[ep.Name()] = cli
			if id, ok := ep.GetAttributes()["instance_id"].(string); ok && id != "" {
				nextByInstance[id] = cli
			}
			continue
		}
		cli, err := b.cli.NewRemoteClient(
			ep,
			balancer.NewRemoteClientOptions{StateListener: b.updateRemoteClientState},
		)
		if err != nil {
			continue
		}
		if cli == nil {
			continue
		}
		nextByName[ep.Name()] = cli
		if id, ok := ep.GetAttributes()["instance_id"].(string); ok && id != "" {
			nextByInstance[id] = cli
		}
		cli.Connect()
	}

	var resp *model.InstancesResponse
	if attrs := state.GetAttributes(); attrs != nil {
		if v, ok := attrs["polaris_instances_response"].(*model.InstancesResponse); ok {
			resp = v
		}
	}
	b.remoteByName = nextByName
	b.remoteByInstance = nextByInstance
	b.instancesResponse = resp
	picker := b.buildPickerLocked()
	b.mu.Unlock()

	b.cli.UpdateState(balancer.State{Picker: picker})
}

func (b *polarisBalancer) updateRemoteClientState(_ remote.ClientState) {
	b.mu.RLock()
	if b.remoteByName == nil {
		b.mu.RUnlock()
		return
	}
	picker := b.buildPickerLocked()
	b.mu.RUnlock()
	b.cli.UpdateState(balancer.State{Picker: picker})
}

func (b *polarisBalancer) buildPickerLocked() balancer.Picker {
	readyByInstance := make(map[string]remote.Client, len(b.remoteByInstance))
	for id, cli := range b.remoteByInstance {
		if cli.State() == remote.Ready {
			readyByInstance[id] = cli
		}
	}
	readyAny := make([]remote.Client, 0, len(b.remoteByName))
	for _, cli := range b.remoteByName {
		if cli.State() == remote.Ready {
			readyAny = append(readyAny, cli)
		}
	}
	return &polarisPicker{
		serviceName:       b.serviceName,
		instancesResponse: b.instancesResponse,
		readyByInstance:   readyByInstance,
		readyAny:          readyAny,
		governance:        b.governance,
		router:            b.router,
		routerErr:         b.routerErr,
		limit:             b.limit,
		limitErr:          b.limitErr,
		cb:                b.cb,
		cbErr:             b.cbErr,
	}
}

type polarisPicker struct {
	serviceName string

	instancesResponse *model.InstancesResponse
	readyByInstance   map[string]remote.Client
	readyAny          []remote.Client
	idx               int64

	governance governanceConfig
	router     routerAPI
	routerErr  error
	limit      limitAPI
	limitErr   error
	cb         circuitBreakerAPI
	cbErr      error
}

func (p *polarisPicker) Next(ri balancer.RPCInfo) (balancer.PickResult, error) {
	if len(p.readyByInstance) == 0 && len(p.readyAny) == 0 {
		return nil, balancer.ErrNoAvailableInstance
	}

	if p.governance.RateLimit.Enable {
		if p.limitErr != nil {
			return nil, p.limitErr
		}
		if err := p.checkRateLimit(ri.Ctx, ri.Method); err != nil {
			return nil, err
		}
	}

	var methodResource *model.MethodResource
	if p.governance.CircuitBreaker.Enable {
		if p.cbErr != nil {
			return nil, p.cbErr
		}
		dstNS := p.governance.Namespace
		if dstNS == "" {
			dstNS = "default"
		}
		srcNS := p.governance.CallerNamespace
		if srcNS == "" {
			srcNS = dstNS
		}
		srcSvc := p.governance.CallerService
		if srcSvc == "" {
			srcSvc = "unknown"
		}
		dst := &model.ServiceKey{Namespace: dstNS, Service: p.serviceName}
		src := &model.ServiceKey{Namespace: srcNS, Service: srcSvc}
		mr, err := model.NewMethodResource(dst, src, ri.Method)
		if err != nil {
			return nil, err
		}
		methodResource = mr
		cr, err := p.cb.Check(mr)
		if err != nil {
			return nil, err
		}
		if cr != nil && !cr.Pass {
			msg := "polaris circuit breaker open"
			if cr.RuleName != "" {
				msg = msg + ": " + cr.RuleName
			}
			return nil, xerror.New(code.Code_UNAVAILABLE, msg)
		}
	}

	selected, err := p.pickRemote(ri.Ctx, ri.Method)
	if err != nil {
		return nil, err
	}
	return &polarisPickResult{
		ctx:            ri.Ctx,
		endpoint:       selected,
		start:          time.Now(),
		methodResource: methodResource,
		cb:             p.cb,
	}, nil
}

func (p *polarisPicker) checkRateLimit(ctx context.Context, method string) error {
	dstNS := p.governance.Namespace
	if dstNS == "" {
		dstNS = "default"
	}

	qr := polaris.NewQuotaRequest()
	qr.SetNamespace(dstNS)
	qr.SetService(p.serviceName)
	qr.SetMethod(method)
	if p.governance.RateLimit.Token > 0 {
		qr.SetToken(p.governance.RateLimit.Token)
	}
	if p.governance.RateLimit.Timeout > 0 {
		qr.SetTimeout(p.governance.RateLimit.Timeout)
	}
	if p.governance.RateLimit.RetryCount > 0 {
		qr.SetRetryCount(p.governance.RateLimit.RetryCount)
	}
	for k, v := range p.governance.RateLimit.Arguments {
		qr.AddArgument(model.BuildCustomArgument(k, v))
	}
	if md, ok := metadata.FromOutContext(ctx); ok {
		for k, vs := range md {
			if len(vs) == 0 {
				continue
			}
			qr.AddArgument(model.BuildCustomArgument(k, vs[0]))
		}
	}

	future, err := p.limit.GetQuota(qr)
	if err != nil {
		return err
	}
	if p.governance.RateLimit.Release {
		defer future.Release()
	}
	resp := future.GetImmediately()
	if resp == nil {
		return xerror.New(code.Code_UNKNOWN, "polaris rate limit: empty response")
	}
	if resp.Code != model.QuotaResultOk {
		msg := resp.Info
		if msg == "" {
			msg = "polaris rate limit exceeded"
		}
		return xerror.New(code.Code_RESOURCE_EXHAUSTED, msg)
	}
	if resp.WaitMs > 0 {
		t := time.NewTimer(time.Duration(resp.WaitMs) * time.Millisecond)
		defer t.Stop()
		select {
		case <-ctx.Done():
			switch {
			case errors.Is(ctx.Err(), context.DeadlineExceeded):
				return xerror.Wrap(ctx.Err(), code.Code_DEADLINE_EXCEEDED, "")
			case errors.Is(ctx.Err(), context.Canceled):
				return xerror.Wrap(ctx.Err(), code.Code_CANCELLED, "")
			default:
				return xerror.Wrap(ctx.Err(), code.Code_UNKNOWN, "")
			}
		case <-t.C:
		}
	}
	return nil
}

func (p *polarisPicker) pickRemote(ctx context.Context, method string) (remote.Client, error) {
	if p.governance.Routing.Enable && p.instancesResponse != nil {
		if p.routerErr != nil {
			return nil, p.routerErr
		}
		readyDst := p.filterReadyInstances(p.instancesResponse)
		filtered, err := p.processRouters(ctx, method, readyDst)
		if err != nil {
			return nil, err
		}
		filtered = p.filterReadyInstances(filtered)
		if len(filtered.Instances) == 0 {
			if p.governance.Routing.RecoverAll {
				return p.randAllReady()
			}
			return nil, balancer.ErrNoAvailableInstance
		}
		one, err := p.processLoadBalance(filtered)
		if err != nil {
			return nil, err
		}
		inst := one.GetInstance()
		if inst != nil {
			if cli, ok := p.readyByInstance[inst.GetId()]; ok {
				return cli, nil
			}
		}
	}

	return p.randAllReady()
}

func (p *polarisPicker) randAllReady() (remote.Client, error) {
	if len(p.readyAny) == 0 {
		return nil, balancer.ErrNoAvailableInstance
	}
	idx := int(atomic.AddInt64(&p.idx, 1)-1) % len(p.readyAny)
	return p.readyAny[idx], nil
}

func (p *polarisPicker) filterReadyInstances(
	dst *model.InstancesResponse,
) *model.InstancesResponse {
	if dst == nil || len(dst.Instances) == 0 || len(p.readyByInstance) == 0 {
		return dst
	}
	instances := make([]model.Instance, 0, len(dst.Instances))
	for _, inst := range dst.Instances {
		if inst == nil {
			continue
		}
		if _, ok := p.readyByInstance[inst.GetId()]; ok {
			instances = append(instances, inst)
		}
	}
	out := *dst
	out.Instances = instances
	return &out
}

func (p *polarisPicker) processRouters(
	ctx context.Context,
	method string,
	dst *model.InstancesResponse,
) (*model.InstancesResponse, error) {
	dstNS := p.governance.Namespace
	if dstNS == "" {
		dstNS = "default"
	}
	srcNS := p.governance.CallerNamespace
	if srcNS == "" {
		srcNS = dstNS
	}
	srcSvc := p.governance.CallerService
	if srcSvc == "" {
		srcSvc = "unknown"
	}

	req := &polaris.ProcessRoutersRequest{ProcessRoutersRequest: model.ProcessRoutersRequest{
		Routers:       append([]string{}, p.governance.Routing.Routers...),
		SourceService: model.ServiceInfo{Service: srcSvc, Namespace: srcNS},
		DstInstances:  dst,
		Method:        method,
	}}
	for k, v := range p.governance.Routing.Arguments {
		req.AddArguments(model.BuildCustomArgument(k, v))
	}
	if md, ok := metadata.FromOutContext(ctx); ok {
		for k, vs := range md {
			if len(vs) == 0 {
				continue
			}
			req.AddArguments(model.BuildCustomArgument(k, vs[0]))
		}
	}
	if p.governance.Routing.Timeout > 0 {
		req.SetTimeout(p.governance.Routing.Timeout)
	}
	if p.governance.Routing.RetryCount > 0 {
		req.SetRetryCount(p.governance.Routing.RetryCount)
	}
	instancesResp, err := p.router.ProcessRouters(req)
	if err != nil {
		return nil, err
	}
	return instancesResp, nil
}

func (p *polarisPicker) processLoadBalance(
	dst model.ServiceInstances,
) (*model.OneInstanceResponse, error) {
	req := &polaris.ProcessLoadBalanceRequest{
		ProcessLoadBalanceRequest: model.ProcessLoadBalanceRequest{
			DstInstances: dst,
			LbPolicy:     p.governance.Routing.LbPolicy,
		},
	}
	return p.router.ProcessLoadBalance(req)
}

type polarisPickResult struct {
	ctx      context.Context
	endpoint remote.Client
	start    time.Time

	methodResource *model.MethodResource
	cb             circuitBreakerAPI
}

func (r *polarisPickResult) RemoteClient() remote.Client { return r.endpoint }

func (r *polarisPickResult) Report(err error) {
	if r.methodResource == nil || r.cb == nil {
		return
	}
	retStatus := model.RetSuccess
	retCode := "0"
	if err != nil {
		retStatus = model.RetFail
		retCode = status.FromError(err).Code().String()
	}
	_ = r.cb.Report(&model.ResourceStat{
		Resource:  r.methodResource,
		RetCode:   retCode,
		Delay:     time.Since(r.start),
		RetStatus: retStatus,
	})
}
