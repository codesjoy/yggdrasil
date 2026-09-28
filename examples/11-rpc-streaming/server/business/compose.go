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

package business

import (
	"context"
	"fmt"
	"io"
	"time"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	helloworldpb "github.com/codesjoy/yggdrasil/v3/examples/protogen/helloworld"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
)

// AppName is the service name this example registers with the runtime.
const AppName = "github.com/codesjoy.yggdrasil.example.11-rpc-streaming"

// Compose installs the streaming greeter service using the convenience bootstrap path.
func Compose(rt yapp.Runtime) (*yapp.BusinessBundle, error) {
	if rt != nil {
		rt.Logger().Info("compose rpc streaming bundle")
	}

	return &yapp.BusinessBundle{
		RPCBindings: []yapp.RPCBinding{{
			ServiceName: helloworldpb.GreeterServiceServiceDesc.ServiceName,
			Desc:        &helloworldpb.GreeterServiceServiceDesc,
			Impl:        &GreeterService{},
		}},
		Diagnostics: []yapp.BundleDiag{{
			Code:    "rpc.streaming.binding",
			Message: "GreeterService streaming RPC installed",
		}},
	}, nil
}

// GreeterService implements every streaming shape of the greeter service.
type GreeterService struct {
	helloworldpb.UnimplementedGreeterServiceServer
}

// SayHello answers a unary call and exercises response header and trailer metadata.
func (s *GreeterService) SayHello(
	ctx context.Context,
	req *helloworldpb.SayHelloRequest,
) (*helloworldpb.SayHelloResponse, error) {
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "streaming-server"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "streaming-server"))

	return &helloworldpb.SayHelloResponse{
		Message: fmt.Sprintf("Hello %s!", req.Name),
	}, nil
}

// SayHelloStream answers each message of a bidirectional stream.
func (s *GreeterService) SayHelloStream(
	stream helloworldpb.GreeterServiceSayHelloStreamServer,
) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if err := stream.Send(&helloworldpb.SayHelloStreamResponse{
			Message: fmt.Sprintf("Hello %s!", req.Name),
		}); err != nil {
			return err
		}
	}
}

// SayHelloClientStream replies once after the client closes its send side.
func (s *GreeterService) SayHelloClientStream(
	stream helloworldpb.GreeterServiceSayHelloClientStreamServer,
) error {
	var names []string
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		names = append(names, req.Name)
	}

	return stream.SendAndClose(&helloworldpb.SayHelloClientStreamResponse{
		Message: fmt.Sprintf("Hello %v!", names),
	})
}

// SayHelloServerStream streams five paced replies for a single request.
func (s *GreeterService) SayHelloServerStream(
	req *helloworldpb.SayHelloServerStreamRequest,
	stream helloworldpb.GreeterServiceSayHelloServerStreamServer,
) error {
	for i := 0; i < 5; i++ {
		if err := stream.Send(&helloworldpb.SayHelloServerStreamResponse{
			Message: fmt.Sprintf("Hello %s! (message %d)", req.Name, i+1),
		}); err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

// SayError returns a plain response used to demonstrate error shape handling.
func (s *GreeterService) SayError(
	_ context.Context,
	req *helloworldpb.SayErrorRequest,
) (*helloworldpb.SayErrorResponse, error) {
	return &helloworldpb.SayErrorResponse{
		Message: fmt.Sprintf("Error: %s", req.Name),
	}, nil
}
