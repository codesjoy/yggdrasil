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
	librarypb2 "github.com/codesjoy/yggdrasil/v2/example/protogen/library/v1"
	_ "github.com/codesjoy/yggdrasil/v2/interceptor/logging"
	"github.com/codesjoy/yggdrasil/v2/metadata"
	_ "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc"
	"github.com/codesjoy/yggdrasil/v2/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type LibraryImpl struct {
	librarypb2.UnimplementedLibraryServiceServer
	version string
}

func (s *LibraryImpl) CreateShelf(
	ctx context.Context,
	req *librarypb2.CreateShelfRequest,
) (*librarypb2.Shelf, error) {
	slog.Info(
		"CreateShelf called",
		"name",
		req.Shelf.Name,
		"theme",
		req.Shelf.Theme,
		"version",
		s.version,
	)
	_ = metadata.SetTrailer(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	_ = metadata.SetHeader(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	return &librarypb2.Shelf{
		Name:  req.Shelf.Name,
		Theme: req.Shelf.Theme,
	}, nil
}

func (s *LibraryImpl) GetShelf(
	ctx context.Context,
	req *librarypb2.GetShelfRequest,
) (*librarypb2.Shelf, error) {
	slog.Info("GetShelf called", "name", req.Name, "version", s.version)
	_ = metadata.SetTrailer(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	_ = metadata.SetHeader(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	return &librarypb2.Shelf{
		Name:  req.Name,
		Theme: "Traffic Splitting " + s.version,
	}, nil
}

func (s *LibraryImpl) ListShelves(
	ctx context.Context,
	req *librarypb2.ListShelvesRequest,
) (*librarypb2.ListShelvesResponse, error) {
	slog.Info("ListShelves called", "parent", req.PageSize, "version", s.version)
	_ = metadata.SetTrailer(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	_ = metadata.SetHeader(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	return &librarypb2.ListShelvesResponse{
		Shelves: []*librarypb2.Shelf{
			{Name: "shelves/1", Theme: "Traffic Splitting " + s.version + " 1"},
			{Name: "shelves/2", Theme: "Traffic Splitting " + s.version + " 2"},
		},
	}, nil
}

func (s *LibraryImpl) DeleteShelf(
	ctx context.Context,
	req *librarypb2.DeleteShelfRequest,
) (*emptypb.Empty, error) {
	slog.Info("DeleteShelf called", "name", req.Name, "version", s.version)
	_ = metadata.SetTrailer(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	_ = metadata.SetHeader(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	return &emptypb.Empty{}, nil
}

func (s *LibraryImpl) MergeShelves(
	ctx context.Context,
	req *librarypb2.MergeShelvesRequest,
) (*librarypb2.Shelf, error) {
	slog.Info(
		"MergeShelves called",
		"name",
		req.Name,
		"other_shelf",
		req.OtherShelf,
		"version",
		s.version,
	)
	_ = metadata.SetTrailer(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	_ = metadata.SetHeader(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	return &librarypb2.Shelf{
		Name:  req.Name,
		Theme: "Merged Theme - " + s.version,
	}, nil
}

func (s *LibraryImpl) CreateBook(
	ctx context.Context,
	req *librarypb2.CreateBookRequest,
) (*librarypb2.Book, error) {
	slog.Info(
		"CreateBook called",
		"parent",
		req.Parent,
		"book",
		req.Book.Name,
		"version",
		s.version,
	)
	_ = metadata.SetTrailer(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	_ = metadata.SetHeader(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	return &librarypb2.Book{
		Name:   req.Parent + "/books/" + req.Book.Name,
		Author: req.Book.Author,
		Title:  req.Book.Title,
		Read:   req.Book.Read,
	}, nil
}

func (s *LibraryImpl) GetBook(
	ctx context.Context,
	req *librarypb2.GetBookRequest,
) (*librarypb2.Book, error) {
	slog.Info("GetBook called", "name", req.Name, "version", s.version)
	_ = metadata.SetTrailer(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	_ = metadata.SetHeader(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	return &librarypb2.Book{
		Name:   req.Name,
		Author: "Traffic Splitting Author - " + s.version,
		Title:  "Traffic Splitting Book Title - " + s.version,
		Read:   false,
	}, nil
}

func (s *LibraryImpl) ListBooks(
	ctx context.Context,
	req *librarypb2.ListBooksRequest,
) (*librarypb2.ListBooksResponse, error) {
	slog.Info("ListBooks called", "parent", req.Parent, "version", s.version)
	_ = metadata.SetTrailer(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	_ = metadata.SetHeader(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	return &librarypb2.ListBooksResponse{
		Books: []*librarypb2.Book{
			{
				Name:   req.Parent + "/books/book1",
				Author: "Author 1 - " + s.version,
				Title:  "Book 1 - " + s.version,
			},
			{
				Name:   req.Parent + "/books/book2",
				Author: "Author 2 - " + s.version,
				Title:  "Book 2 - " + s.version,
			},
		},
	}, nil
}

func (s *LibraryImpl) DeleteBook(
	ctx context.Context,
	req *librarypb2.DeleteBookRequest,
) (*emptypb.Empty, error) {
	slog.Info("DeleteBook called", "name", req.Name, "version", s.version)
	_ = metadata.SetTrailer(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	_ = metadata.SetHeader(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	return &emptypb.Empty{}, nil
}

func (s *LibraryImpl) UpdateBook(
	ctx context.Context,
	req *librarypb2.UpdateBookRequest,
) (*librarypb2.Book, error) {
	slog.Info("UpdateBook called", "book", req.Book.Name, "version", s.version)
	_ = metadata.SetTrailer(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	_ = metadata.SetHeader(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	return &librarypb2.Book{
		Name:   req.Book.Name,
		Author: req.Book.Author,
		Title:  req.Book.Title,
		Read:   req.Book.Read,
	}, nil
}

func (s *LibraryImpl) MoveBook(
	ctx context.Context,
	req *librarypb2.MoveBookRequest,
) (*librarypb2.Book, error) {
	slog.Info(
		"MoveBook called",
		"name",
		req.Name,
		"other_shelf_name",
		req.OtherShelfName,
		"version",
		s.version,
	)
	_ = metadata.SetTrailer(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	_ = metadata.SetHeader(
		ctx,
		metadata.Pairs("server", "traffic-splitting-server", "version", s.version),
	)
	return nil, status.FromReason(
		errors.New("book not found"),
		librarypb.Reason_BOOK_NOT_FOUND,
		nil,
	)
}

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", "error", err)
		os.Exit(1)
	}

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.xds.example.traffic-splitting.server"); err != nil {
		slog.Error("failed to initialize yggdrasil", "error", err)
		os.Exit(1)
	}

	version := os.Getenv("SERVICE_VERSION")
	if version == "" {
		version = "v1"
	}

	slog.Info("Starting traffic splitting server...", "version", version)

	ss := &LibraryImpl{version: version}
	if err := yggdrasil.Serve(
		yggdrasil.WithServiceDesc(&librarypb2.LibraryServiceServiceDesc, ss),
	); err != nil {
		slog.Error("failed to serve", "error", err)
		os.Exit(1)
	}
}
