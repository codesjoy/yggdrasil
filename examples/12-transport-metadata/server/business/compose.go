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

const AppName = "github.com.codesjoy.yggdrasil.example.12-transport-metadata"

// Compose installs a greeter service that focuses on transport metadata behavior.
func Compose(rt yapp.Runtime) (*yapp.BusinessBundle, error) {
	if rt != nil {
		rt.Logger().Info("compose transport metadata bundle")
	}

	return &yapp.BusinessBundle{
		RPCBindings: []yapp.RPCBinding{{
			ServiceName: helloworldpb.GreeterServiceServiceDesc.ServiceName,
			Desc:        &helloworldpb.GreeterServiceServiceDesc,
			Impl:        &GreeterService{},
		}},
		Diagnostics: []yapp.BundleDiag{{
			Code:    "transport.metadata.binding",
			Message: "GreeterService metadata example installed",
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
	_ = metadata.SetHeader(ctx, metadata.Pairs(
		"server-version", "1.0.0",
		"request-id", fmt.Sprintf("req-%d", time.Now().Unix()),
		"processed-by", "metadata-server",
	))
	_ = metadata.SetTrailer(ctx, metadata.Pairs(
		"server", "metadata-server",
		"processing-time", "10ms",
		"status", "success",
	))

	return &helloworldpb.SayHelloResponse{
		Message: fmt.Sprintf("Hello %s!", req.Name),
	}, nil
}

func (s *GreeterService) SayHelloStream(
	stream helloworldpb.GreeterServiceSayHelloStreamServer,
) error {
	_ = metadata.SetHeader(stream.Context(), metadata.Pairs("stream-type", "bidirectional"))

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			_ = metadata.SetTrailer(stream.Context(), metadata.Pairs(
				"stream-status", "closed",
				"messages-count", "0",
			))
			return nil
		}
		if err != nil {
			_ = metadata.SetTrailer(stream.Context(), metadata.Pairs(
				"stream-status", "error",
				"error", err.Error(),
			))
			return err
		}

		if err := stream.Send(&helloworldpb.SayHelloStreamResponse{
			Message: fmt.Sprintf("Hello %s!", req.Name),
		}); err != nil {
			_ = metadata.SetTrailer(stream.Context(), metadata.Pairs(
				"stream-status", "error",
				"error", err.Error(),
			))
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

	_ = metadata.SetTrailer(stream.Context(), metadata.Pairs(
		"server", "metadata-server",
		"processed-count", fmt.Sprintf("%d", len(names)),
	))
	return stream.SendAndClose(&helloworldpb.SayHelloClientStreamResponse{
		Message: fmt.Sprintf("Hello %v!", names),
	})
}

func (s *GreeterService) SayHelloServerStream(
	req *helloworldpb.SayHelloServerStreamRequest,
	stream helloworldpb.GreeterServiceSayHelloServerStreamServer,
) error {
	_ = metadata.SetHeader(stream.Context(), metadata.Pairs(
		"stream-type", "server-streaming",
		"target-name", req.Name,
	))

	for i := 0; i < 5; i++ {
		if err := stream.Send(&helloworldpb.SayHelloServerStreamResponse{
			Message: fmt.Sprintf("Hello %s! (message %d)", req.Name, i+1),
		}); err != nil {
			_ = metadata.SetTrailer(stream.Context(), metadata.Pairs(
				"stream-status", "error",
				"error", err.Error(),
				"messages-sent", fmt.Sprintf("%d", i),
			))
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}

	_ = metadata.SetTrailer(stream.Context(), metadata.Pairs(
		"server", "metadata-server",
		"messages-sent", "5",
		"status", "completed",
	))
	return nil
}
