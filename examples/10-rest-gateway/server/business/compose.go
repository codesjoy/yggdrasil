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

type LibraryService struct {
	libraryv1.UnimplementedLibraryServiceServer
}

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

func (s *LibraryService) DeleteShelf(
	ctx context.Context,
	req *libraryv1.DeleteShelfRequest,
) (*emptypb.Empty, error) {
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "delete"))

	return &emptypb.Empty{}, nil
}

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

func (s *LibraryService) DeleteBook(
	ctx context.Context,
	req *libraryv1.DeleteBookRequest,
) (*emptypb.Empty, error) {
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "rest-server"))
	_ = metadata.SetTrailer(ctx, metadata.Pairs("operation", "delete"))

	return &emptypb.Empty{}, nil
}

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
