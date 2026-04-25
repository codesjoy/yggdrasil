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

// Package server provides the server runtime for the framework.
package server

import (
	"errors"
	"sync"

	"github.com/codesjoy/yggdrasil/v3/internal/constant"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
)

const (
	serverStateInit = iota
	serverStateRunning
	serverStateClosing
)

type serverInfo struct {
	protocol string
	address  string
	svrKind  constant.ServerKind
	metadata map[string]string
}

type server struct {
	mu                sync.RWMutex
	services          map[string]*ServiceInfo // service name -> service serverInfo
	servicesDesc      map[string][]methodInfo
	restRouterDesc    []restRouterInfo
	unaryInterceptor  interceptor.UnaryServerInterceptor
	streamInterceptor interceptor.StreamServerInterceptor
	servers           []remote.Server
	state             int
	serverWG          sync.WaitGroup
	stats             stats.Handler

	restSvr    rest.Server
	restEnable bool

	registerErr error

	runtime Runtime
}

// Runtime exposes the App-scoped runtime dependencies needed by the server package.
type Runtime interface {
	ServerSettings() Settings
	ServerStatsHandler() stats.Handler
	RESTConfig() *rest.Config
	RESTMiddlewareProviders() map[string]rest.Provider
	MarshalerBuilders() map[string]marshaler.MarshalerBuilder
	BuildUnaryServerInterceptor(names []string) interceptor.UnaryServerInterceptor
	BuildStreamServerInterceptor(names []string) interceptor.StreamServerInterceptor
	TransportServerProvider(protocol string) remote.TransportServerProvider
}

// New creates a new server with one explicit runtime snapshot.
func New(runtimeSnapshot Runtime) (Server, error) {
	if runtimeSnapshot == nil {
		return nil, errors.New("server runtime is required")
	}
	return newServer(runtimeSnapshot)
}
