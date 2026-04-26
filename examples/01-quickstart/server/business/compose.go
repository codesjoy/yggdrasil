package business

import (
	"context"
	"fmt"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	helloworldpb "github.com/codesjoy/yggdrasil/v3/examples/protogen/helloworld"
)

const AppName = "github.com.codesjoy.yggdrasil.example.01-quickstart"

type quickstartConfig struct {
	Greeting string `mapstructure:"greeting"`
}

type greeterService struct {
	helloworldpb.UnimplementedGreeterServiceServer
	greeting string
}

// Compose installs the smallest end-to-end bundle used by the quickstart path.
func Compose(rt yapp.Runtime) (*yapp.BusinessBundle, error) {
	cfg := quickstartConfig{}
	if manager := rt.Config(); manager != nil {
		if err := manager.Section("app", "quickstart").Decode(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.Greeting == "" {
		cfg.Greeting = "hello from quickstart"
	}

	rt.Logger().Info("compose quickstart bundle", "greeting", cfg.Greeting)

	return &yapp.BusinessBundle{
		RPCBindings: []yapp.RPCBinding{{
			ServiceName: helloworldpb.GreeterServiceServiceDesc.ServiceName,
			Desc:        &helloworldpb.GreeterServiceServiceDesc,
			Impl:        &greeterService{greeting: cfg.Greeting},
		}},
		Diagnostics: []yapp.BundleDiag{{
			Code:    "quickstart.greeting",
			Message: cfg.Greeting,
		}},
	}, nil
}

func (s *greeterService) SayHello(
	_ context.Context,
	req *helloworldpb.SayHelloRequest,
) (*helloworldpb.SayHelloResponse, error) {
	return &helloworldpb.SayHelloResponse{
		Message: fmt.Sprintf("%s, %s", s.greeting, req.GetName()),
	}, nil
}
