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

import restserver "github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"

func (si *serverInfo) Address() string {
	return si.address
}

func (si *serverInfo) Metadata() map[string]string {
	return si.metadata
}

func (si *serverInfo) Kind() EndpointKind {
	return si.svrKind
}

func (si *serverInfo) Protocol() string {
	return si.protocol
}

func (s *server) Endpoints() []Endpoint {
	endpoints := make([]Endpoint, len(s.servers))
	for i, item := range s.servers {
		e := item.Info()
		address := e.AdvertiseAddress
		if address == "" {
			address = e.Address
		}
		endpoints[i] = &serverInfo{
			protocol: e.Protocol,
			address:  address,
			metadata: e.Attributes,
			svrKind:  EndpointKindRPC,
		}
	}
	if s.restEnable {
		info := s.restSvr.Info()
		address := info.GetAddress()
		if provider, ok := info.(restserver.AdvertiseAddressProvider); ok {
			if advertised := provider.GetAdvertiseAddress(); advertised != "" {
				address = advertised
			}
		}
		endpoints = append(endpoints, &serverInfo{
			protocol: "http",
			address:  address,
			metadata: info.GetAttributes(),
			svrKind:  EndpointKindRest,
		})
	}
	return endpoints
}
