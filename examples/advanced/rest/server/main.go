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
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/codesjoy/yggdrasil/v3"
	libraryv1 "github.com/codesjoy/yggdrasil/v3/examples/protogen/library/v1"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
)

type LibraryImpl struct {
	libraryv1.UnimplementedLibraryServiceServer
}

func (s *LibraryImpl) CreateShelf(
	ctx context.Context,
	req *libraryv1.CreateShelfRequest,
) (*libraryv1.Shelf, error) {
	slog.Info("CreateShelf called", "shelf", req.Shelf)

	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "create"))

	name := fmt.Sprintf("shelves/%d", time.Now().UnixNano())
	return &libraryv1.Shelf{
		Name:  name,
		Theme: req.Shelf.Theme,
	}, nil
}

func (s *LibraryImpl) GetShelf(
	ctx context.Context,
	req *libraryv1.GetShelfRequest,
) (*libraryv1.Shelf, error) {
	slog.Info("GetShelf called", "name", req.Name)

	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "get"))

	return &libraryv1.Shelf{
		Name:  req.Name,
		Theme: "Sample Theme",
	}, nil
}

func (s *LibraryImpl) ListShelves(
	ctx context.Context,
	_ *libraryv1.ListShelvesRequest,
) (*libraryv1.ListShelvesResponse, error) {
	slog.Info("ListShelves called")

	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "list"))

	return &libraryv1.ListShelvesResponse{
		Shelves: []*libraryv1.Shelf{
			{Name: "shelves/1", Theme: "Fiction"},
			{Name: "shelves/2", Theme: "Science"},
			{Name: "shelves/3", Theme: "History"},
		},
	}, nil
}

func (s *LibraryImpl) DeleteShelf(
	ctx context.Context,
	req *libraryv1.DeleteShelfRequest,
) (*emptypb.Empty, error) {
	slog.Info("DeleteShelf called", "name", req.Name)

	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "delete"))

	return &emptypb.Empty{}, nil
}

func (s *LibraryImpl) MergeShelves(
	ctx context.Context,
	req *libraryv1.MergeShelvesRequest,
) (*libraryv1.Shelf, error) {
	slog.Info("MergeShelves called", "name", req.Name)

	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "merge"))

	return &libraryv1.Shelf{
		Name:  req.Name,
		Theme: "Merged Theme",
	}, nil
}

func (s *LibraryImpl) CreateBook(
	ctx context.Context,
	req *libraryv1.CreateBookRequest,
) (*libraryv1.Book, error) {
	slog.Info("CreateBook called", "parent", req.Parent, "book", req.Book)

	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "create"))

	name := fmt.Sprintf("%s/books/%d", req.Parent, time.Now().UnixNano())
	return &libraryv1.Book{
		Name:   name,
		Author: req.Book.Author,
		Title:  req.Book.Title,
		Read:   req.Book.Read,
	}, nil
}

func (s *LibraryImpl) GetBook(
	ctx context.Context,
	req *libraryv1.GetBookRequest,
) (*libraryv1.Book, error) {
	slog.Info("GetBook called", "name", req.Name)

	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "get"))

	return &libraryv1.Book{
		Name:   req.Name,
		Author: "Sample Author",
		Title:  "Sample Title",
		Read:   true,
	}, nil
}

func (s *LibraryImpl) ListBooks(
	ctx context.Context,
	req *libraryv1.ListBooksRequest,
) (*libraryv1.ListBooksResponse, error) {
	slog.Info("ListBooks called", "parent", req.Parent)

	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "list"))

	return &libraryv1.ListBooksResponse{
		Books: []*libraryv1.Book{
			{
				Name:   fmt.Sprintf("%s/books/1", req.Parent),
				Author: "Author 1",
				Title:  "Title 1",
				Read:   false,
			},
			{
				Name:   fmt.Sprintf("%s/books/2", req.Parent),
				Author: "Author 2",
				Title:  "Title 2",
				Read:   true,
			},
			{
				Name:   fmt.Sprintf("%s/books/3", req.Parent),
				Author: "Author 3",
				Title:  "Title 3",
				Read:   false,
			},
		},
	}, nil
}

func (s *LibraryImpl) DeleteBook(
	ctx context.Context,
	req *libraryv1.DeleteBookRequest,
) (*emptypb.Empty, error) {
	slog.Info("DeleteBook called", "name", req.Name)

	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "delete"))

	return &emptypb.Empty{}, nil
}

func (s *LibraryImpl) UpdateBook(
	ctx context.Context,
	req *libraryv1.UpdateBookRequest,
) (*libraryv1.Book, error) {
	slog.Info("UpdateBook called", "book", req.Book)

	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "update"))

	return &libraryv1.Book{
		Name:   req.Book.Name,
		Author: req.Book.Author,
		Title:  req.Book.Title,
		Read:   req.Book.Read,
	}, nil
}

func (s *LibraryImpl) MoveBook(
	ctx context.Context,
	req *libraryv1.MoveBookRequest,
) (*libraryv1.Book, error) {
	slog.Info("MoveBook called", "name", req.Name, "other_shelf_name", req.OtherShelfName)

	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "move"))

	return &libraryv1.Book{
		Name:   req.Name,
		Author: "Moved Author",
		Title:  "Moved Title",
		Read:   false,
	}, nil
}

func main() {
	lib := &LibraryImpl{}
	err := yggdrasil.Run(
		context.Background(),
		func(yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
			return &yggdrasil.BusinessBundle{
				RPCBindings: []yggdrasil.RPCBinding{
					{
						ServiceName: libraryv1.LibraryServiceServiceDesc.ServiceName,
						Desc:        &libraryv1.LibraryServiceServiceDesc,
						Impl:        lib,
					},
				},
				RESTBindings: []yggdrasil.RESTBinding{
					{
						Name: "library-rest",
						Desc: &libraryv1.LibraryServiceRestServiceDesc,
						Impl: lib,
					},
				},
			}, nil
		},
		yggdrasil.WithConfigPath("./config.yaml"),
		yggdrasil.WithAppName("github.com.codesjoy.yggdrasil.example.advanced.rest"),
	)
	if err != nil {
		slog.Error("rest server exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}
