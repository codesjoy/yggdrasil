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

// Package main is a sample server for yggdrasil.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/codesjoy/pkg/basic/xerror"

	"github.com/codesjoy/yggdrasil/v3"
	librarypb "github.com/codesjoy/yggdrasil/v3/example/protogen/library"
	librarypb2 "github.com/codesjoy/yggdrasil/v3/example/protogen/library/v1"
	"github.com/codesjoy/yggdrasil/v3/metadata"
)

type LibraryImpl struct {
	librarypb2.UnimplementedLibraryServiceServer
}

func (s *LibraryImpl) CreateShelf(
	ctx context.Context,
	_ *librarypb2.CreateShelfRequest,
) (*librarypb2.Shelf, error) {
	_ = metadata.SetTrailer(ctx, metadata.Pairs("trailer", "test"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("header", "test"))
	return &librarypb2.Shelf{Name: "test", Theme: "test"}, nil
}

func (s *LibraryImpl) GetShelf(
	ctx context.Context,
	request *librarypb2.GetShelfRequest,
) (*librarypb2.Shelf, error) {
	_ = metadata.SetTrailer(ctx, metadata.Pairs("trailer", "test"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("header", "test"))
	return &librarypb2.Shelf{Name: request.Name, Theme: "test"}, nil
}

func (s *LibraryImpl) MoveBook(
	_ context.Context,
	_ *librarypb2.MoveBookRequest,
) (*librarypb2.Book, error) {
	return nil, xerror.WrapWithReason(
		errors.New("test reason"),
		librarypb.Reason_BOOK_NOT_FOUND,
		"",
		nil,
	)
}

func (s *LibraryImpl) GetBook(
	_ context.Context,
	request *librarypb2.GetBookRequest,
) (*librarypb2.Book, error) {
	return &librarypb2.Book{Name: request.Name}, nil
}

func WebHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("hello web"))
}

func main() {
	err := yggdrasil.Run(
		context.Background(),
		func(yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
			return &yggdrasil.BusinessBundle{
				RPCBindings: []yggdrasil.RPCBinding{
					{
						ServiceName: librarypb2.LibraryServiceServiceDesc.ServiceName,
						Desc:        &librarypb2.LibraryServiceServiceDesc,
						Impl:        &LibraryImpl{},
					},
				},
				RESTBindings: []yggdrasil.RESTBinding{
					{
						Name: "library-rest",
						Desc: &librarypb2.LibraryServiceRestServiceDesc,
						Impl: &LibraryImpl{},
					},
				},
				RawHTTP: []yggdrasil.RawHTTPBinding{
					{
						Method:  http.MethodGet,
						Path:    "/web",
						Handler: WebHandler,
					},
				},
			}, nil
		},
		yggdrasil.WithAppName("github.com.codesjoy.yggdrasil.example.sample"),
	)
	if err != nil {
		os.Exit(1)
	}
}
