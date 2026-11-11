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

	"github.com/codesjoy/yggdrasil/v3/internal/backoff"
	grpcprotocol "github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc"
	rpchttp "github.com/codesjoy/yggdrasil/v3/transport/protocol/rpchttp"
	"github.com/codesjoy/yggdrasil/v3/transport/runtime/client"
)

func mergeClientServiceSettings(
	base client.ServiceSettings,
	overlay clientServiceConfigOverlay,
) client.ServiceSettings {
	out := base
	mergePointer(&out.FastFail, overlay.FastFail)
	mergePointer(&out.Resolver, overlay.Resolver)
	mergePointer(&out.Balancer, overlay.Balancer)
	if overlay.Backoff != nil {
		out.Backoff = mergeBackoffConfig(base.Backoff, *overlay.Backoff)
	}
	if overlay.Remote != nil {
		if overlay.Remote.Endpoints != nil {
			out.Remote.Endpoints = slices.Clone(*overlay.Remote.Endpoints)
		}
		if overlay.Remote.Attributes != nil {
			out.Remote.Attributes = mergeRemoteAttributes(
				base.Remote.Attributes,
				*overlay.Remote.Attributes,
			)
		}
	}
	if overlay.Interceptors != nil {
		if overlay.Interceptors.Unary != nil {
			out.Interceptors.Unary = mergeInterceptorNames(
				base.Interceptors.Unary,
				*overlay.Interceptors.Unary,
			)
		}
		if overlay.Interceptors.Stream != nil {
			out.Interceptors.Stream = mergeInterceptorNames(
				base.Interceptors.Stream,
				*overlay.Interceptors.Stream,
			)
		}
	}
	return out
}

func mergeHTTPClientConfig(
	base rpchttp.ClientConfig,
	overlay HTTPClientTransport,
) rpchttp.ClientConfig {
	out := base
	mergePointer(&out.Timeout, overlay.Timeout)
	if overlay.Marshaler != nil {
		out.Marshaler = overlay.Marshaler
	}
	mergePointer(&out.SecurityProfile, overlay.SecurityProfile)
	return out
}

func mergeGRPCClientConfig(
	base grpcprotocol.ClientConfig,
	overlay grpcClientConfigOverlay,
) grpcprotocol.ClientConfig {
	out := base
	mergePointer(&out.WaitConnTimeout, overlay.WaitConnTimeout)
	mergePointer(&out.ConnectTimeout, overlay.ConnectTimeout)
	mergePointer(&out.MaxSendMsgSize, overlay.MaxSendMsgSize)
	mergePointer(&out.MaxRecvMsgSize, overlay.MaxRecvMsgSize)
	mergePointer(&out.Compressor, overlay.Compressor)
	mergePointer(&out.BackOffMaxDelay, overlay.BackOffMaxDelay)
	mergePointer(&out.MinConnectTimeout, overlay.MinConnectTimeout)
	mergePointer(&out.Network, overlay.Network)
	out.Transport = mergeGRPCTransportConfig(base.Transport, overlay.Transport)
	return out
}

func mergeGRPCTransportConfig(
	base grpcprotocol.ClientTransportOptions,
	overlay grpcClientTransportOptionsOverlay,
) grpcprotocol.ClientTransportOptions {
	out := base
	mergePointer(&out.UserAgent, overlay.UserAgent)
	mergePointer(&out.SecurityProfile, overlay.SecurityProfile)
	mergePointer(&out.Authority, overlay.Authority)
	mergePointer(&out.KeepaliveParams, overlay.KeepaliveParams)
	mergePointer(&out.InitialWindowSize, overlay.InitialWindowSize)
	mergePointer(&out.InitialConnWindowSize, overlay.InitialConnWindowSize)
	mergePointer(&out.WriteBufferSize, overlay.WriteBufferSize)
	mergePointer(&out.ReadBufferSize, overlay.ReadBufferSize)
	if overlay.MaxHeaderListSize != nil {
		out.MaxHeaderListSize = overlay.MaxHeaderListSize
	}
	return out
}

func mergeBackoffConfig(base backoff.Config, overlay backoffConfigOverlay) backoff.Config {
	out := base
	mergePointer(&out.BaseDelay, overlay.BaseDelay)
	mergePointer(&out.Multiplier, overlay.Multiplier)
	mergePointer(&out.Jitter, overlay.Jitter)
	mergePointer(&out.MaxDelay, overlay.MaxDelay)
	return out
}

func mergePointer[T any](target *T, overlay *T) {
	if overlay != nil {
		*target = *overlay
	}
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
