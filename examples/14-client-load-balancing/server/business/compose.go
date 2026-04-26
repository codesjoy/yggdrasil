package business

import (
	"context"
	"fmt"
	"io"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	helloworldpb "github.com/codesjoy/yggdrasil/v3/examples/protogen/helloworld"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
)

const AppName = "github.com.codesjoy.yggdrasil.example.14-client-load-balancing"

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

type GreeterService struct {
	helloworldpb.UnimplementedGreeterServiceServer
	instanceID string
}

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
