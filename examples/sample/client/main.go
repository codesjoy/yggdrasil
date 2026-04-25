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

// Package main is a sample client for yggdrasil.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	librarypb "github.com/codesjoy/yggdrasil/v3/examples/protogen/library/v1"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	"github.com/codesjoy/yggdrasil/v3/rpc/status"
)

func main() {
	app, err := yapp.New("github.com.codesjoy.yggdrasil.example.sample.client")
	if err != nil {
		os.Exit(1)
	}
	cli, err := app.NewClient(context.Background(), "github.com.codesjoy.yggdrasil.example.sample")
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
		st := status.FromError(err)
		fmt.Println("Reason:", st.ErrorInfo().Reason)
		fmt.Println("Code:", st.Code())
		fmt.Println("HttpCode:", st.HTTPCode())
	}
	slog.Info("call success")
}
