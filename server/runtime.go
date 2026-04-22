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

package server

import (
	"fmt"

	internalutils "github.com/codesjoy/yggdrasil/v3/internal/utils"
	"github.com/codesjoy/yggdrasil/v3/remote/marshaler"
	"github.com/codesjoy/yggdrasil/v3/server/rest"
)

func newServer(runtimeSnapshot Runtime) (Server, error) {
	cfg := runtimeSnapshot.ServerSettings()
	statsHandler := runtimeSnapshot.ServerStatsHandler()
	restCfg := runtimeSnapshot.RESTConfig()
	opts := []rest.Option{
		rest.WithMiddlewareProviders(runtimeSnapshot.RESTMiddlewareProviders()),
	}
	s := &server{
		services:       map[string]*ServiceInfo{},
		servicesDesc:   map[string][]methodInfo{},
		restRouterDesc: []restRouterInfo{},
		stats:          statsHandler,
		runtime:        runtimeSnapshot,
	}
	if cfg.RestEnabled {
		s.restEnable = true
		var err error
		if restCfg != nil {
			supported := restCfg.Marshaler.Support
			if len(supported) == 0 {
				supported = []string{marshaler.SchemeJSONPb}
			}
			registry := marshaler.BuildMarshalerRegistryWithBuilders(
				runtimeSnapshot.MarshalerBuilders(),
				restCfg.Marshaler.Config.JSONPB,
				supported...,
			)
			opts = append(opts, rest.WithMarshalerRegistry(registry))
		}
		s.restSvr, err = rest.NewServer(restCfg, opts...)
		if err != nil {
			return nil, err
		}
	}
	s.initInterceptor()
	if err := s.initRemoteServer(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *server) initInterceptor() {
	cfg := s.runtime.ServerSettings()
	unaryNames := append([]string(nil), cfg.Interceptors.Unary...)
	unaryNames = internalutils.DedupStableStrings(unaryNames)
	s.unaryInterceptor = s.runtime.BuildUnaryServerInterceptor(unaryNames)
	streamNames := append([]string(nil), cfg.Interceptors.Stream...)
	streamNames = internalutils.DedupStableStrings(streamNames)
	s.streamInterceptor = s.runtime.BuildStreamServerInterceptor(streamNames)
}

func (s *server) initRemoteServer() error {
	protocols := s.runtime.ServerSettings().Transports
	if len(protocols) == 0 {
		return nil
	}
	for _, protocol := range protocols {
		provider := s.runtime.TransportServerProvider(protocol)
		if provider == nil {
			return fmt.Errorf("server transport provider for protocol %s not found", protocol)
		}
		svr, err := provider.NewServer(s.handleStream)
		if err != nil {
			return fmt.Errorf("fault to new %s remote server: %v", protocol, err)
		}
		s.servers = append(s.servers, svr)
	}
	return nil
}
