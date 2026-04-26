package business

import (
	"context"
	"fmt"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	helloworldpb "github.com/codesjoy/yggdrasil/v3/examples/protogen/helloworld"
)

const AppName = "github.com.codesjoy.yggdrasil.example.03-diagnostics-reload"

type diagnosticsConfig struct {
	Greeting string `mapstructure:"greeting"`
}

type greeterService struct {
	helloworldpb.UnimplementedGreeterServiceServer
	greeting string
}

// Compose installs a minimal RPC service plus diagnostics metadata used by the reload example.
func Compose(rt yapp.Runtime) (*yapp.BusinessBundle, error) {
	cfg := diagnosticsConfig{}
	if manager := rt.Config(); manager != nil {
		if err := manager.Section("app", "diagnostics_reload").Decode(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.Greeting == "" {
		cfg.Greeting = "hello from diagnostics reload"
	}

	rt.Logger().Info("compose diagnostics reload bundle", "greeting", cfg.Greeting)

	return &yapp.BusinessBundle{
		RPCBindings: []yapp.RPCBinding{{
			ServiceName: helloworldpb.GreeterServiceServiceDesc.ServiceName,
			Desc:        &helloworldpb.GreeterServiceServiceDesc,
			Impl:        &greeterService{greeting: cfg.Greeting},
		}},
		Diagnostics: []yapp.BundleDiag{
			{
				Code:    "reload.overlay",
				Message: "watch reload.yaml for plan changes",
			},
			{
				Code:    "reload.greeting",
				Message: cfg.Greeting,
			},
		},
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
