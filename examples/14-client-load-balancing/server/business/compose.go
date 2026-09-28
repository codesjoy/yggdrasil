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

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	helloworldpb "github.com/codesjoy/yggdrasil/v3/examples/protogen/helloworld"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
)

// AppName is the base service name this example registers with the runtime.
const AppName = "github.com/codesjoy.yggdrasil.example.14-client-load-balancing"

// ServerAppName returns the per-instance service name for a backend port.
func ServerAppName(port int) string {
	return fmt.Sprintf("%s.%d", AppName, port)
}

// Compose installs one greeter implementation bound to a specific backend instance ID.
func Compose(instanceID string) func(yapp.Runtime) (*yapp.BusinessBundle, error) {
	return func(rt yapp.Runtime) (*yapp.BusinessBundle, error) {
		if rt != nil {
			rt.Logger().Info("compose client load balancing bundle", "instance", instanceID)
		}

		return &yapp.BusinessBundle{
			RPCBindings: []yapp.RPCBinding{{
				ServiceName: helloworldpb.GreeterServiceServiceDesc.ServiceName,
				Desc:        &helloworldpb.GreeterServiceServiceDesc,
				Impl:        &GreeterService{instanceID: instanceID},
			}},
			Diagnostics: []yapp.BundleDiag{{
				Code:    "client.load_balancing.instance",
				Message: instanceID,
			}},
		}, nil
	}
}

// GreeterService is the greeter implementation backing one load-balanced instance.
type GreeterService struct {
	helloworldpb.UnimplementedGreeterServiceServer
	instanceID string
}

// SayHello answers a unary call and stamps the answering instance into the trailer.
func (s *GreeterService) SayHello(
	ctx context.Context,
	req *helloworldpb.SayHelloRequest,
) (*helloworldpb.SayHelloResponse, error) {
	_ = metadata.SetTrailer(ctx, metadata.Pairs(
		"server", s.instanceID,
		"instance-type", "load-balancing",
	))

	return &helloworldpb.SayHelloResponse{
		Message: fmt.Sprintf("Hello %s! from %s", req.Name, s.instanceID),
	}, nil
}

// SayHelloStream echoes one reply for every streamed request.
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
			Message: fmt.Sprintf("Hello %s! from %s", req.Name, s.instanceID),
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
		Message: fmt.Sprintf("Hello %v! from %s", names, s.instanceID),
	})
}

// SayHelloServerStream streams five replies for a single request.
func (s *GreeterService) SayHelloServerStream(
	req *helloworldpb.SayHelloServerStreamRequest,
	stream helloworldpb.GreeterServiceSayHelloServerStreamServer,
) error {
	for i := 0; i < 5; i++ {
		if err := stream.Send(&helloworldpb.SayHelloServerStreamResponse{
			Message: fmt.Sprintf("Hello %s! (message %d) from %s", req.Name, i+1, s.instanceID),
		}); err != nil {
			return err
		}
	}
	return nil
}
