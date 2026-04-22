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

package rpchttp

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/codesjoy/yggdrasil/v3/metadata"
	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/remote/marshaler"
	"github.com/codesjoy/yggdrasil/v3/stats"
)

// ServerProvider returns the built-in http server transport provider.
func ServerProvider() remote.TransportServerProvider {
	return ServerProviderWithSettings(Settings{}, stats.NoOpHandler, nil)
}

// ServerProviderWithSettings returns the built-in http server transport provider bound to explicit settings.
func ServerProviderWithSettings(
	settings Settings,
	statsHandler stats.Handler,
	builders map[string]marshaler.MarshallerBuilder,
) remote.TransportServerProvider {
	return remote.NewTransportServerProvider(Protocol, func(handle remote.MethodHandle) (remote.Server, error) {
		opts := settings.Server
		if opts.Attr == nil {
			opts.Attr = map[string]string{}
		}
		codec, err := newConfiguredMarshalersWithBuilders(builders, opts.Marshaler)
		if err != nil {
			return nil, err
		}
		s := &server{
			opts:         &opts,
			handle:       handle,
			statsHandler: statsHandler,
			codec:        codec,
		}
		s.httpSvr = &http.Server{
			Handler:      http.HandlerFunc(s.serveHTTP),
			ReadTimeout:  opts.ReadTimeout,
			WriteTimeout: opts.WriteTimeout,
			IdleTimeout:  opts.IdleTimeout,
		}
		return s, nil
	})
}

type server struct {
	opts *ServerConfig

	mu      sync.Mutex
	lis     net.Listener
	httpSvr *http.Server
	closed  bool
	address string

	handle remote.MethodHandle

	statsHandler stats.Handler
	codec        marshalerSet
}

func (s *server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return http.ErrServerClosed
	}
	if s.lis != nil {
		return nil
	}
	lc := net.ListenConfig{}
	lis, err := lc.Listen(context.Background(), s.opts.Network, s.opts.Address)
	if err != nil {
		return err
	}
	s.lis = lis
	s.address = lis.Addr().String()
	return nil
}

func (s *server) Handle() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return http.ErrServerClosed
	}
	svr := s.httpSvr
	lis := s.lis
	s.mu.Unlock()
	if lis == nil {
		return net.ErrClosed
	}
	return svr.Serve(lis)
}

func (s *server) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	svr := s.httpSvr
	s.mu.Unlock()
	return svr.Shutdown(ctx)
}

func (s *server) Info() remote.ServerInfo {
	s.mu.Lock()
	addr := s.address
	s.mu.Unlock()
	return remote.ServerInfo{
		Protocol:   scheme,
		Address:    addr,
		Attributes: s.opts.Attr,
	}
}

func (s *server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	method := r.URL.Path
	if method == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if !strings.HasPrefix(method, "/") {
		method = "/" + method
	}

	ctx := r.Context()
	ctx = metadata.WithInContext(ctx, extractMetadataWithPrefix(r.Header, MetadataHeaderPrefix))
	ctx = s.statsHandler.TagRPC(ctx, &stats.RPCTagInfoBase{FullMethod: method})
	ctx = metadata.WithStreamContext(ctx)
	localAddr := s.localAddr()
	ctx = attachPeer(ctx, r, localAddr)

	ssCtx, cancel := context.WithCancel(ctx)
	ss := &httpServerStream{
		ctx:                ssCtx,
		cancel:             cancel,
		method:             method,
		req:                r,
		w:                  w,
		localAddr:          localAddr,
		maxBodyBytes:       localAddrOrZero(s.opts.MaxBodyBytes),
		statsHandler:       s.statsHandler,
		beginTime:          time.Now(),
		remoteEndpoint:     r.RemoteAddr,
		localEndpoint:      addrString(localAddr),
		configuredInbound:  s.codec.inbound,
		configuredOutbound: s.codec.outbound,
	}
	s.handle(ss)
}

func addrString(a net.Addr) string {
	if a == nil {
		return ""
	}
	return a.String()
}

func localAddrOrZero(v int64) int64 {
	if v <= 0 {
		return 4 * 1024 * 1024
	}
	return v
}

func (s *server) localAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lis == nil {
		return nil
	}
	return s.lis.Addr()
}
