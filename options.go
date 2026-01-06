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

// Package yggdrasil provides a framework for building microservices.
package yggdrasil

import (
	"time"

	"github.com/codesjoy/yggdrasil/v2/governor"
	"github.com/codesjoy/yggdrasil/v2/registry"
	"github.com/codesjoy/yggdrasil/v2/server"

	"github.com/codesjoy/yggdrasil/v2/application"
)

type restServiceDesc struct {
	ss     interface{}
	Prefix []string
}

type options struct {
	serviceDesc       map[*server.ServiceDesc]interface{}
	restServiceDesc   map[*server.RestServiceDesc]restServiceDesc
	restRawHandleDesc []*server.RestRawHandlerDesc
	server            server.Server
	governor          *governor.Server
	internalSvr       []application.InternalServer
	registry          registry.Registry
	shutdownTimeout   time.Duration
	startBeforeHook   []func() error
	stopBeforeHook    []func() error
	stopAfterHook     []func() error
}

func (opts *options) getAppOpts() []application.Option {
	return []application.Option{
		application.WithServer(opts.server),
		application.WithGovernor(opts.governor),
		application.WithRegistry(opts.registry),
		application.WithShutdownTimeout(opts.shutdownTimeout),
		application.WithBeforeStartHook(opts.startBeforeHook...),
		application.WithBeforeStopHook(opts.stopBeforeHook...),
		application.WithAfterStopHook(opts.stopAfterHook...),
		application.WithInternalServer(opts.internalSvr...),
	}
}

// Option define the framework options
type Option func(*options) error

// WithBeforeStartHook register the before start hook
func WithBeforeStartHook(fns ...func() error) Option {
	return func(opts *options) error {
		opts.startBeforeHook = append(opts.startBeforeHook, fns...)
		return nil
	}
}

// WithBeforeStopHook register the before stop hook
func WithBeforeStopHook(fns ...func() error) Option {
	return func(opts *options) error {
		opts.stopBeforeHook = append(opts.stopBeforeHook, fns...)
		return nil
	}
}

// WithAfterStopHook register the after stop hook
func WithAfterStopHook(fns ...func() error) Option {
	return func(opts *options) error {
		opts.stopAfterHook = append(opts.stopAfterHook, fns...)
		return nil
	}
}

// WithServiceDescMap register a service
func WithServiceDescMap(desc map[*server.ServiceDesc]interface{}) Option {
	return func(opts *options) error {
		for k, v := range desc {
			opts.serviceDesc[k] = v
		}
		return nil
	}
}

// WithServiceDesc register a service
func WithServiceDesc(desc *server.ServiceDesc, impl interface{}) Option {
	return func(opts *options) error {
		opts.serviceDesc[desc] = impl
		return nil
	}
}

// WithRestServiceDesc register a rest service
func WithRestServiceDesc(desc *server.RestServiceDesc, impl interface{}, prefix ...string) Option {
	return func(opts *options) error {
		opts.restServiceDesc[desc] = restServiceDesc{
			ss:     impl,
			Prefix: prefix,
		}
		return nil
	}
}

// WithRestRawHandleDesc register a raw handler
func WithRestRawHandleDesc(desc ...*server.RestRawHandlerDesc) Option {
	return func(opts *options) error {
		opts.restRawHandleDesc = append(opts.restRawHandleDesc, desc...)
		return nil
	}
}

// WithInternalServer register an internal server
func WithInternalServer(svr ...application.InternalServer) Option {
	return func(opts *options) error {
		opts.internalSvr = append(opts.internalSvr, svr...)
		return nil
	}
}
