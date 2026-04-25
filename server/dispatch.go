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
	"strings"

	"github.com/codesjoy/pkg/basic/xerror"
	"google.golang.org/genproto/googleapis/rpc/code"

	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	"github.com/codesjoy/yggdrasil/v3/remote"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
)

func (s *server) handleStream(ss remote.ServerStream) {
	serviceName, methodName, err := splitMethodTarget(ss.Method())
	if err != nil {
		ss.Finish(nil, xerror.New(code.Code_UNIMPLEMENTED, err.Error()))
		return
	}

	srv, knownService := s.services[serviceName]
	if knownService {
		if md, ok := srv.Methods[methodName]; ok {
			s.processUnaryRPC(md, srv, ss)
			return
		}
		if sd, ok := srv.Streams[methodName]; ok {
			s.processStreamRPC(sd, srv, ss)
			return
		}
	}
	if !knownService {
		ss.Finish(nil, xerror.New(code.Code_UNIMPLEMENTED, fmt.Sprintf("unknown service %v", serviceName)))
		return
	}
	ss.Finish(
		nil,
		xerror.New(code.Code_UNIMPLEMENTED, fmt.Sprintf("unknown method %v for service %v", methodName, serviceName)),
	)
}

func (s *server) processUnaryRPC(desc *MethodDesc, srv *ServiceInfo, ss remote.ServerStream) {
	var (
		reply any
		err   error
	)
	defer func() {
		ss.Finish(reply, err)
	}()
	if err = ss.Start(false, false); err != nil {
		return
	}

	ctx := metadata.WithStreamContext(ss.Context())
	reply, err = desc.Handler(srv.ServiceImpl, ctx, ss.RecvMsg, s.unaryInterceptor)
	if header, ok := metadata.FromHeaderCtx(ctx); ok {
		_ = ss.SetHeader(header)
	}
	if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
		ss.SetTrailer(trailer)
	}
}

func (s *server) processStreamRPC(desc *stream.Desc, srv *ServiceInfo, ss remote.ServerStream) {
	var err error
	defer func() {
		ss.Finish(nil, err)
	}()
	if err = ss.Start(desc.ClientStreams, desc.ServerStreams); err != nil {
		return
	}
	si := &interceptor.StreamServerInfo{
		FullMethod:     ss.Method(),
		IsClientStream: desc.ClientStreams,
		IsServerStream: desc.ServerStreams,
	}
	err = s.streamInterceptor(srv.ServiceImpl, ss, si, desc.Handler)
}

func splitMethodTarget(method string) (serviceName, methodName string, err error) {
	method = strings.TrimPrefix(method, "/")
	pos := strings.LastIndex(method, "/")
	if pos == -1 {
		return "", "", fmt.Errorf("malformed method name: %q", method)
	}
	return method[:pos], method[pos+1:], nil
}
