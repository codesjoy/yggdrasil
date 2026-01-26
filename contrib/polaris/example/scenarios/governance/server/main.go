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
	"errors"
	"log/slog"
	"os"

	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	librarypb "github.com/codesjoy/yggdrasil/v2/example/protogen/library"
	libraryv1 "github.com/codesjoy/yggdrasil/v2/example/protogen/library/v1"
	_ "github.com/codesjoy/yggdrasil/v2/interceptor/logging"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	_ "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc"
	"github.com/codesjoy/yggdrasil/v2/status"

	_ "github.com/codesjoy/yggdrasil/contrib/polaris/v2"
)

type LibraryImpl struct {
	libraryv1.UnimplementedLibraryServiceServer
}

func (s *LibraryImpl) CreateShelf(
	ctx context.Context,
	_ *libraryv1.CreateShelfRequest,
) (*libraryv1.Shelf, error) {
	_ = metadata.SetTrailer(ctx, metadata.Pairs("trailer", "polaris-governance"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("header", "polaris-governance"))
	return &libraryv1.Shelf{Name: "test", Theme: "test"}, nil
}

func (s *LibraryImpl) GetShelf(
	ctx context.Context,
	request *libraryv1.GetShelfRequest,
) (*libraryv1.Shelf, error) {
	_ = metadata.SetTrailer(ctx, metadata.Pairs("trailer", "polaris-governance"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("lane", yggdrasil.InstanceMetadata()["lane"]))
	_ = metadata.SetHeader(ctx, metadata.Pairs("header", "polaris-governance"))
	return &libraryv1.Shelf{Name: request.Name, Theme: "test"}, nil
}

func (s *LibraryImpl) MoveBook(
	_ context.Context,
	_ *libraryv1.MoveBookRequest,
) (*libraryv1.Book, error) {
	return nil, status.FromReason(
		errors.New("book not found"),
		librarypb.Reason_BOOK_NOT_FOUND,
		nil,
	)
}

func (s *LibraryImpl) GetBook(
	_ context.Context,
	request *libraryv1.GetBookRequest,
) (*libraryv1.Book, error) {
	return &libraryv1.Book{Name: request.Name}, nil
}

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.polaris.example.governance.server"); err != nil {
		slog.Error("init failed", slog.Any("error", err))
		os.Exit(1)
	}

	svc := &LibraryImpl{}
	if err := yggdrasil.Serve(
		yggdrasil.WithServiceDesc(&libraryv1.LibraryServiceServiceDesc, svc),
	); err != nil {
		slog.Error("serve failed", slog.Any("error", err))
		os.Exit(1)
	}
}
