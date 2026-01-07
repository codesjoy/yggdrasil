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

	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	librarypb "github.com/codesjoy/yggdrasil/v2/example/protogen/library/v1"
	_ "github.com/codesjoy/yggdrasil/v2/interceptor/logging"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	_ "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc"
	"github.com/codesjoy/yggdrasil/v2/status"
)

func dd() <-chan struct{} {
	cc := make(chan struct{})
	close(cc)
	return cc
}

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}
	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.example.sample.client"); err != nil {
		os.Exit(1)
	}
	waitConnCh := dd() // 声明但未初始化，值
	fmt.Println(2222)
	if waitConnCh == nil {
		panic("fdsfasd")
	}
	<-waitConnCh
	fmt.Println(11111)
	cli, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.sample")
	if err != nil {
		os.Exit(1)
	}

	client := librarypb.NewLibraryServiceClient(cli)
	ctx := metadata.WithStreamContext(context.TODO())

	_, err = client.GetShelf(ctx, &librarypb.GetShelfRequest{Name: "fdasf"})
	if err != nil {
		slog.Error("fault to call GetShelf", slog.Any("error", err))
		os.Exit(1)
	}
	if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
		fmt.Println(trailer)
	}
	if header, ok := metadata.FromHeaderCtx(ctx); ok {
		fmt.Println(header)
	}
	_, err = client.MoveBook(context.TODO(), &librarypb.MoveBookRequest{Name: "fdasf"})
	if err != nil {
		fmt.Println(status.FromError(err).ErrorInfo().Reason)
	}
	slog.Info("call success")
}
