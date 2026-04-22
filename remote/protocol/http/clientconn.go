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

// Package protocolhttp implements an HTTP client for Yggdrasil remote communication.
package protocolhttp

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/codesjoy/pkg/basic/xerror"
	"google.golang.org/genproto/googleapis/rpc/code"

	"github.com/codesjoy/yggdrasil/v3/metadata"
	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/resolver"
	"github.com/codesjoy/yggdrasil/v3/stats"
	"github.com/codesjoy/yggdrasil/v3/stream"
)

func init() {
	remote.RegisterClientBuilder("http", newClient)
}

type clientConn struct {
	ctx    context.Context
	cancel context.CancelFunc

	mu sync.RWMutex
	// HTTP is connectionless from the client's perspective. READY here means
	// the client object is open and can issue requests, not that the endpoint
	// has been health-checked.
	state       remote.State
	endpoint    resolver.Endpoint
	serviceName string

	cfg   *ClientConfig
	hc    *http.Client
	codec marshalerSet

	statsHandler  stats.Handler
	onStateChange remote.OnStateChange
}

func newClient(
	ctx context.Context,
	serviceName string,
	endpoint resolver.Endpoint,
	statsHandler stats.Handler,
	onStateChange remote.OnStateChange,
) (remote.Client, error) {
	if statsHandler == nil {
		statsHandler = stats.NoOpHandler
	}

	resolved := currentSettings()
	globalCfg := &resolved.Client

	cfg := &ClientConfig{
		Timeout:   globalCfg.Timeout,
		Marshaler: globalCfg.Marshaler,
	}

	serviceConfig := resolved.ClientServices[serviceName]
	if serviceConfig.Timeout > 0 {
		cfg.Timeout = serviceConfig.Timeout
	}
	if serviceConfig.Marshaler != nil {
		cfg.Marshaler = serviceConfig.Marshaler
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	codec, err := newConfiguredMarshalers(cfg.Marshaler)
	if err != nil {
		return nil, err
	}

	ccCtx, cancel := context.WithCancel(ctx)

	cc := &clientConn{
		ctx:          ccCtx,
		cancel:       cancel,
		state:        remote.Ready,
		endpoint:     endpoint,
		serviceName:  serviceName,
		cfg:          cfg,
		hc:           &http.Client{Timeout: cfg.Timeout},
		codec:        codec,
		statsHandler: statsHandler,
		onStateChange: func(state remote.ClientState) {
			if onStateChange != nil {
				onStateChange(state)
			}
		},
	}
	if cc.onStateChange != nil {
		cc.onStateChange(remote.ClientState{Endpoint: endpoint, State: remote.Ready})
	}
	return cc, nil
}

func (cc *clientConn) NewStream(
	ctx context.Context,
	desc *stream.Desc,
	method string,
) (stream.ClientStream, error) {
	if desc != nil && (desc.ClientStreams || desc.ServerStreams) {
		return nil, xerror.New(code.Code_UNIMPLEMENTED, "http protocol does not support streaming")
	}

	if method == "" {
		return nil, xerror.New(code.Code_INVALID_ARGUMENT, "empty method")
	}
	if !strings.HasPrefix(method, "/") {
		method = "/" + method
	}

	tagInfo := &stats.RPCTagInfoBase{FullMethod: method}
	taggedCtx := cc.statsHandler.TagRPC(ctx, tagInfo)
	taggedCtx = metadata.WithStreamContext(taggedCtx)

	cs := &httpClientStream{
		ctx:                taggedCtx,
		method:             method,
		endpointAddr:       cc.endpoint.GetAddress(),
		httpClient:         cc.hc,
		beginTime:          time.Now(),
		statsHandler:       cc.statsHandler,
		configuredInbound:  cc.codec.inbound,
		configuredOutbound: cc.codec.outbound,
	}
	return cs, nil
}

func (cc *clientConn) Close() error {
	cc.mu.Lock()
	if cc.state == remote.Shutdown {
		cc.mu.Unlock()
		return nil
	}
	cc.state = remote.Shutdown
	cc.mu.Unlock()
	cc.cancel()
	return nil
}

func (cc *clientConn) Scheme() string {
	return "http"
}

func (cc *clientConn) State() remote.State {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.state
}

func (cc *clientConn) Connect() {
	cc.mu.Lock()
	if cc.state != remote.Shutdown {
		cc.state = remote.Ready
		if cc.onStateChange != nil {
			cc.onStateChange(remote.ClientState{Endpoint: cc.endpoint, State: remote.Ready})
		}
	}
	cc.mu.Unlock()
}
