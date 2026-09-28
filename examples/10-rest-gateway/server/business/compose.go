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
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	libraryv1 "github.com/codesjoy/yggdrasil/v3/examples/protogen/library/v1"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
)

// AppName is the service name this example registers with the runtime.
const AppName = "github.com.codesjoy.yggdrasil.example.10-rest-gateway"

// Compose installs both RPC and generated REST bindings for the library service.
func Compose(rt yapp.Runtime) (*yapp.BusinessBundle, error) {
	if rt != nil {
		rt.Logger().Info("compose rest gateway bundle")
	}

	lib := &LibraryService{}
	return &yapp.BusinessBundle{
		RPCBindings: []yapp.RPCBinding{{
			ServiceName: libraryv1.LibraryServiceServiceDesc.ServiceName,
			Desc:        &libraryv1.LibraryServiceServiceDesc,
			Impl:        lib,
		}},
		RESTBindings: []yapp.RESTBinding{{
			Name: "library-rest",
			Desc: &libraryv1.LibraryServiceRestServiceDesc,
			Impl: lib,
		}},
		Diagnostics: []yapp.BundleDiag{{
			Code:    "rest.gateway.install",
			Message: "LibraryService RPC and REST bindings installed",
		}},
	}, nil
}

// LibraryService is the in-memory library implementation shared by the RPC and REST bindings.
type LibraryService struct {
	libraryv1.UnimplementedLibraryServiceServer
}

// CreateShelf creates a shelf and echoes the requested theme.
func (s *LibraryService) CreateShelf(
	ctx context.Context,
	req *libraryv1.CreateShelfRequest,
) (*libraryv1.Shelf, error) {
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "create"))

	name := fmt.Sprintf("shelves/%d", time.Now().UnixNano())
	return &libraryv1.Shelf{
		Name:  name,
		Theme: req.Shelf.Theme,
	}, nil
}

// GetShelf returns a shelf with a fixed sample theme.
func (s *LibraryService) GetShelf(
	ctx context.Context,
	req *libraryv1.GetShelfRequest,
) (*libraryv1.Shelf, error) {
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "get"))

	return &libraryv1.Shelf{
		Name:  req.Name,
		Theme: "Sample Theme",
	}, nil
}

// ListShelves returns a fixed set of sample shelves.
func (s *LibraryService) ListShelves(
	ctx context.Context,
	_ *libraryv1.ListShelvesRequest,
) (*libraryv1.ListShelvesResponse, error) {
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

// DeleteShelf acknowledges a shelf deletion.
func (s *LibraryService) DeleteShelf(
	ctx context.Context,
	req *libraryv1.DeleteShelfRequest,
) (*emptypb.Empty, error) {
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "delete"))

	return &emptypb.Empty{}, nil
}

// MergeShelves returns a shelf representing the merged source shelves.
func (s *LibraryService) MergeShelves(
	ctx context.Context,
	req *libraryv1.MergeShelvesRequest,
) (*libraryv1.Shelf, error) {
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "merge"))

	return &libraryv1.Shelf{
		Name:  req.Name,
		Theme: "Merged Theme",
	}, nil
}

// CreateBook creates a book under the requested parent shelf.
func (s *LibraryService) CreateBook(
	ctx context.Context,
	req *libraryv1.CreateBookRequest,
) (*libraryv1.Book, error) {
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

// GetBook returns a book with sample content.
func (s *LibraryService) GetBook(
	ctx context.Context,
	req *libraryv1.GetBookRequest,
) (*libraryv1.Book, error) {
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "get"))

	return &libraryv1.Book{
		Name:   req.Name,
		Author: "Sample Author",
		Title:  "Sample Title",
		Read:   true,
	}, nil
}

// ListBooks returns a fixed set of sample books for the requested parent.
func (s *LibraryService) ListBooks(
	ctx context.Context,
	req *libraryv1.ListBooksRequest,
) (*libraryv1.ListBooksResponse, error) {
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

// DeleteBook acknowledges a book deletion.
func (s *LibraryService) DeleteBook(
	ctx context.Context,
	req *libraryv1.DeleteBookRequest,
) (*emptypb.Empty, error) {
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "delete"))

	return &emptypb.Empty{}, nil
}

// UpdateBook echoes the updated book fields.
func (s *LibraryService) UpdateBook(
	ctx context.Context,
	req *libraryv1.UpdateBookRequest,
) (*libraryv1.Book, error) {
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "update"))

	return &libraryv1.Book{
		Name:   req.Book.Name,
		Author: req.Book.Author,
		Title:  req.Book.Title,
		Read:   req.Book.Read,
	}, nil
}

// MoveBook returns the book under its new name.
func (s *LibraryService) MoveBook(
	ctx context.Context,
	req *libraryv1.MoveBookRequest,
) (*libraryv1.Book, error) {
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "move"))

	return &libraryv1.Book{
		Name:   req.Name,
		Author: "Moved Author",
		Title:  "Moved Title",
		Read:   false,
	}, nil
}
