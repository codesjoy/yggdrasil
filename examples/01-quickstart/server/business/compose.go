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

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	helloworldpb "github.com/codesjoy/yggdrasil/v3/examples/protogen/helloworld"
)

// AppName is the service name this example registers with the runtime.
const AppName = "github.com/codesjoy.yggdrasil.example.01-quickstart"

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
