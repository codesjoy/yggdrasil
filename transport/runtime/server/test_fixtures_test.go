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
	"context"

	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
)

type TestService interface {
	TestMethod(context.Context, any) (any, error)
	TestStream(stream.ServerStream) error
}

type TestServiceImpl struct{}

func (*TestServiceImpl) TestMethod(context.Context, any) (any, error) {
	return nil, nil
}

func (*TestServiceImpl) TestStream(stream.ServerStream) error {
	return nil
}

type testServerStream struct {
	method            string
	ctx               context.Context
	startErr          error
	startClientStream bool
	startServerStream bool
	finishReply       any
	finishErr         error
	header            metadata.MD
	trailer           metadata.MD
}

func (s *testServerStream) Method() string {
	return s.method
}

func (s *testServerStream) Start(isClientStream, isServerStream bool) error {
	s.startClientStream = isClientStream
	s.startServerStream = isServerStream
	return s.startErr
}

func (s *testServerStream) Finish(reply any, err error) {
	s.finishReply = reply
	s.finishErr = err
}

func (s *testServerStream) SetHeader(md metadata.MD) error {
	s.header = metadata.Join(s.header, md)
	return nil
}

func (s *testServerStream) SendHeader(md metadata.MD) error {
	s.header = metadata.Join(s.header, md)
	return nil
}

func (s *testServerStream) SetTrailer(md metadata.MD) {
	s.trailer = metadata.Join(s.trailer, md)
}

func (s *testServerStream) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func (s *testServerStream) SendMsg(any) error {
	return nil
}

func (s *testServerStream) RecvMsg(any) error {
	return nil
}
