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
