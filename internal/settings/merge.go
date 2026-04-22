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

package settings

import (
	"maps"
	"slices"

	"github.com/codesjoy/yggdrasil/v3/client"
	"github.com/codesjoy/yggdrasil/v3/internal/backoff"
	grpcprotocol "github.com/codesjoy/yggdrasil/v3/remote/protocol/grpc"
	protocolhttp "github.com/codesjoy/yggdrasil/v3/remote/protocol/http"
	"github.com/codesjoy/yggdrasil/v3/resolver"
)

func mergeClientServiceConfig(base client.ServiceConfig, overlay clientServiceConfigOverlay) client.ServiceConfig {
	out := base
	if overlay.FastFail != nil {
		out.FastFail = *overlay.FastFail
	}
	if overlay.Resolver != nil {
		out.Resolver = *overlay.Resolver
	}
	if overlay.Balancer != nil {
		out.Balancer = *overlay.Balancer
	}
	if overlay.Backoff != nil {
		out.Backoff = mergeBackoffConfig(base.Backoff, *overlay.Backoff)
	}
	if overlay.Remote != nil {
		if overlay.Remote.Endpoints != nil {
			out.Remote.Endpoints = slices.Clone(*overlay.Remote.Endpoints)
		}
		if overlay.Remote.Attributes != nil {
			out.Remote.Attributes = mergeRemoteAttributes(base.Remote.Attributes, *overlay.Remote.Attributes)
		}
	}
	if overlay.Interceptors != nil {
		if overlay.Interceptors.Unary != nil {
			out.Interceptors.Unary = mergeInterceptorNames(base.Interceptors.Unary, *overlay.Interceptors.Unary)
		}
		if overlay.Interceptors.Stream != nil {
			out.Interceptors.Stream = mergeInterceptorNames(base.Interceptors.Stream, *overlay.Interceptors.Stream)
		}
	}
	return out
}

func mergeHTTPClientConfig(base protocolhttp.ClientConfig, overlay HTTPClientTransport) protocolhttp.ClientConfig {
	out := base
	if overlay.Timeout != nil {
		out.Timeout = *overlay.Timeout
	}
	if overlay.Marshaler != nil {
		out.Marshaler = overlay.Marshaler
	}
	return out
}

func mergeGRPCClientConfig(base grpcprotocol.Config, overlay grpcClientConfigOverlay) grpcprotocol.Config {
	out := base
	if overlay.WaitConnTimeout != nil {
		out.WaitConnTimeout = *overlay.WaitConnTimeout
	}
	if overlay.ConnectTimeout != nil {
		out.ConnectTimeout = *overlay.ConnectTimeout
	}
	if overlay.MaxSendMsgSize != nil {
		out.MaxSendMsgSize = *overlay.MaxSendMsgSize
	}
	if overlay.MaxRecvMsgSize != nil {
		out.MaxRecvMsgSize = *overlay.MaxRecvMsgSize
	}
	if overlay.Compressor != nil {
		out.Compressor = *overlay.Compressor
	}
	if overlay.BackOffMaxDelay != nil {
		out.BackOffMaxDelay = *overlay.BackOffMaxDelay
	}
	if overlay.MinConnectTimeout != nil {
		out.MinConnectTimeout = *overlay.MinConnectTimeout
	}
	if overlay.Network != nil {
		out.Network = *overlay.Network
	}
	out.Transport = mergeGRPCTransportConfig(base.Transport, overlay.Transport)
	return out
}

func mergeGRPCTransportConfig(base grpcprotocol.ClientTransportOptions, overlay grpcClientTransportOptionsOverlay) grpcprotocol.ClientTransportOptions {
	out := base
	if overlay.UserAgent != nil {
		out.UserAgent = *overlay.UserAgent
	}
	if overlay.CredsProto != nil {
		out.CredsProto = *overlay.CredsProto
	}
	if overlay.Authority != nil {
		out.Authority = *overlay.Authority
	}
	if overlay.KeepaliveParams != nil {
		out.KeepaliveParams = *overlay.KeepaliveParams
	}
	if overlay.InitialWindowSize != nil {
		out.InitialWindowSize = *overlay.InitialWindowSize
	}
	if overlay.InitialConnWindowSize != nil {
		out.InitialConnWindowSize = *overlay.InitialConnWindowSize
	}
	if overlay.WriteBufferSize != nil {
		out.WriteBufferSize = *overlay.WriteBufferSize
	}
	if overlay.ReadBufferSize != nil {
		out.ReadBufferSize = *overlay.ReadBufferSize
	}
	if overlay.MaxHeaderListSize != nil {
		out.MaxHeaderListSize = overlay.MaxHeaderListSize
	}
	return out
}

func mergeBackoffConfig(base backoff.Config, overlay backoffConfigOverlay) backoff.Config {
	out := base
	if overlay.BaseDelay != nil {
		out.BaseDelay = *overlay.BaseDelay
	}
	if overlay.Multiplier != nil {
		out.Multiplier = *overlay.Multiplier
	}
	if overlay.Jitter != nil {
		out.Jitter = *overlay.Jitter
	}
	if overlay.MaxDelay != nil {
		out.MaxDelay = *overlay.MaxDelay
	}
	return out
}

func mergeRemoteAttributes(base, overlay map[string]any) map[string]any {
	if len(overlay) == 0 {
		return map[string]any{}
	}
	out := cloneAnyMap(base)
	maps.Copy(out, overlay)
	return out
}

func mergeInterceptorNames(base, overlay []string) []string {
	if len(overlay) == 0 {
		return []string{}
	}
	return dedupStrings(append(append([]string{}, base...), overlay...))
}

func cloneAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	return maps.Clone(src)
}

func cloneNestedMap(src map[string]map[string]any) map[string]map[string]any {
	if src == nil {
		return map[string]map[string]any{}
	}
	out := make(map[string]map[string]any, len(src))
	for key, value := range src {
		out[key] = cloneAnyMap(value)
	}
	return out
}

func dedupStrings(values []string) []string {
	values = slices.DeleteFunc(values, func(item string) bool { return item == "" })
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, item := range values {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func cloneEndpoints(src []resolver.BaseEndpoint) []resolver.BaseEndpoint {
	return slices.Clone(src)
}
