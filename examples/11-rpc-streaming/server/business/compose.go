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

const AppName = "github.com.codesjoy.yggdrasil.example.11-rpc-streaming"

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

type GreeterService struct {
	helloworldpb.UnimplementedGreeterServiceServer
}

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

func (s *GreeterService) SayError(
	_ context.Context,
	req *helloworldpb.SayErrorRequest,
) (*helloworldpb.SayErrorResponse, error) {
	return &helloworldpb.SayErrorResponse{
		Message: fmt.Sprintf("Error: %s", req.Name),
	}, nil
}
