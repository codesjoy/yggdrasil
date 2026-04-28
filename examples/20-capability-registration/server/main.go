package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/codesjoy/yggdrasil/v3"
	grpcx "github.com/codesjoy/yggdrasil/v3/examples/20-capability-registration/grpcx"
	helloworld "github.com/codesjoy/yggdrasil/v3/examples/protogen/helloworld"
)

type greeterService struct {
	helloworld.UnimplementedGreeterServiceServer
}

func (s *greeterService) SayHello(
	_ context.Context,
	req *helloworld.SayHelloRequest,
) (*helloworld.SayHelloResponse, error) {
	return &helloworld.SayHelloResponse{
		Message: fmt.Sprintf("hello, %s, from grpcx", req.GetName()),
	}, nil
}

func composeBundle(rt yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
	if rt != nil {
		rt.Logger().Info("compose capability registration bundle", "protocol", grpcx.Protocol)
	}

	return &yggdrasil.BusinessBundle{
		RPCBindings: []yggdrasil.RPCBinding{{
			ServiceName: helloworld.GreeterServiceServiceDesc.ServiceName,
			Desc:        &helloworld.GreeterServiceServiceDesc,
			Impl:        &greeterService{},
		}},
		Diagnostics: []yggdrasil.BundleDiag{{
			Code:    "capability.registration.protocol",
			Message: grpcx.Protocol,
		}},
	}, nil
}

func main() {
	if err := yggdrasil.Run(
		context.Background(),
		"github.com.codesjoy.yggdrasil.example.20-capability-registration",
		composeBundle,
		yggdrasil.WithConfigPath("config.yaml"),
		yggdrasil.WithCapabilityRegistrations(grpcx.NewRegistration()),
	); err != nil {
		slog.Error("run app", slog.Any("error", err))
		os.Exit(1)
	}
}
