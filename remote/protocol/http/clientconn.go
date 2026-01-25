package http

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
	"github.com/codesjoy/yggdrasil/v2/status"
	"github.com/codesjoy/yggdrasil/v2/stream"
	"google.golang.org/genproto/googleapis/rpc/code"
)

func init() {
	remote.RegisterClientBuilder("http", newClient)
}

type clientConn struct {
	ctx    context.Context
	cancel context.CancelFunc

	mu          sync.RWMutex
	state       remote.State
	endpoint    resolver.Endpoint
	serviceName string

	cfg   *ClientConfig
	hc    *http.Client
	cache *marshalerCache

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

	baseKey := config.Join(config.KeyBase, "remote", "protocol", scheme, "client")
	globalCfg := &ClientConfig{}
	if err := config.Get(baseKey).Scan(globalCfg); err != nil {
		return nil, err
	}

	cfg := &ClientConfig{
		Timeout:   globalCfg.Timeout,
		Marshaler: globalCfg.Marshaler,
	}

	serviceKey := config.Join(baseKey, serviceName)
	serviceConfig := &ClientConfig{}
	if err := config.Get(serviceKey).Scan(serviceConfig); err == nil {
		if serviceConfig.Timeout > 0 {
			cfg.Timeout = serviceConfig.Timeout
		}
		if serviceConfig.Marshaler != nil {
			cfg.Marshaler = serviceConfig.Marshaler
		}
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	ccCtx, cancel := context.WithCancel(ctx)

	var inboundCfg, outboundCfg *MarshalerConfig
	if cfg.Marshaler != nil {
		inboundCfg = cfg.Marshaler.Inbound
		outboundCfg = cfg.Marshaler.Outbound
	}

	cc := &clientConn{
		ctx:         ccCtx,
		cancel:      cancel,
		state:       remote.Ready,
		endpoint:    endpoint,
		serviceName: serviceName,
		cfg:         cfg,
		hc:          &http.Client{Timeout: cfg.Timeout},
		cache: &marshalerCache{
			inboundCfg:  inboundCfg,
			outboundCfg: outboundCfg,
		},
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
		return nil, status.New(code.Code_UNIMPLEMENTED, "http protocol does not support streaming").Err()
	}

	if method == "" {
		return nil, status.New(code.Code_INVALID_ARGUMENT, "empty method").Err()
	}
	if !strings.HasPrefix(method, "/") {
		method = "/" + method
	}

	tagInfo := &stats.RPCTagInfoBase{FullMethod: method}
	taggedCtx := cc.statsHandler.TagRPC(ctx, tagInfo)
	taggedCtx = metadata.WithStreamContext(taggedCtx)

	cs := &httpClientStream{
		ctx:          taggedCtx,
		method:       method,
		endpointAddr: cc.endpoint.GetAddress(),
		httpClient:   cc.hc,
		beginTime:    time.Now(),
		statsHandler: cc.statsHandler,
		cache:        cc.cache,
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
