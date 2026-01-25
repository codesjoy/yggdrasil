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
}

func (s *LibraryImpl) CreateShelf(
	ctx context.Context,
	req *librarypb2.CreateShelfRequest,
) (*librarypb2.Shelf, error) {
	slog.Info("CreateShelf called", "name", req.Shelf.Name, "theme", req.Shelf.Theme)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "basic-server"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "basic-server"))
	return &librarypb2.Shelf{
		Name:  req.Shelf.Name,
		Theme: req.Shelf.Theme,
	}, nil
}

func (s *LibraryImpl) GetShelf(
	ctx context.Context,
	req *librarypb2.GetShelfRequest,
) (*librarypb2.Shelf, error) {
	slog.Info("GetShelf called", "name", req.Name)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "basic-server"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "basic-server"))
	return &librarypb2.Shelf{
		Name:  req.Name,
		Theme: "Basic Service Theme",
	}, nil
}

func (s *LibraryImpl) ListShelves(
	ctx context.Context,
	req *librarypb2.ListShelvesRequest,
) (*librarypb2.ListShelvesResponse, error) {
	slog.Info("ListShelves called", "page_size", req.PageSize)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "basic-server"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "basic-server"))
	return &librarypb2.ListShelvesResponse{
		Shelves: []*librarypb2.Shelf{
			{Name: "shelves/1", Theme: "Basic Service Theme 1"},
			{Name: "shelves/2", Theme: "Basic Service Theme 2"},
		},
	}, nil
}

func (s *LibraryImpl) DeleteShelf(
	ctx context.Context,
	req *librarypb2.DeleteShelfRequest,
) (*emptypb.Empty, error) {
	slog.Info("DeleteShelf called", "name", req.Name)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "basic-server"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "basic-server"))
	return &emptypb.Empty{}, nil
}

func (s *LibraryImpl) MergeShelves(
	ctx context.Context,
	req *librarypb2.MergeShelvesRequest,
) (*librarypb2.Shelf, error) {
	slog.Info("MergeShelves called", "name", req.Name, "other_shelf", req.OtherShelf)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "basic-server"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "basic-server"))
	return &librarypb2.Shelf{
		Name:  req.Name,
		Theme: "Merged Theme",
	}, nil
}

func (s *LibraryImpl) CreateBook(
	ctx context.Context,
	req *librarypb2.CreateBookRequest,
) (*librarypb2.Book, error) {
	slog.Info("CreateBook called", "parent", req.Parent, "book", req.Book.Name)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "basic-server"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "basic-server"))
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
	slog.Info("GetBook called", "name", req.Name)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "basic-server"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "basic-server"))
	return &librarypb2.Book{
		Name:   req.Name,
		Author: "Basic Author",
		Title:  "Basic Book Title",
		Read:   false,
	}, nil
}

func (s *LibraryImpl) ListBooks(
	ctx context.Context,
	req *librarypb2.ListBooksRequest,
) (*librarypb2.ListBooksResponse, error) {
	slog.Info("ListBooks called", "parent", req.Parent)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "basic-server"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "basic-server"))
	return &librarypb2.ListBooksResponse{
		Books: []*librarypb2.Book{
			{Name: req.Parent + "/books/book1", Author: "Author 1", Title: "Book 1"},
			{Name: req.Parent + "/books/book2", Author: "Author 2", Title: "Book 2"},
		},
	}, nil
}

func (s *LibraryImpl) DeleteBook(
	ctx context.Context,
	req *librarypb2.DeleteBookRequest,
) (*emptypb.Empty, error) {
	slog.Info("DeleteBook called", "name", req.Name)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "basic-server"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "basic-server"))
	return &emptypb.Empty{}, nil
}

func (s *LibraryImpl) UpdateBook(
	ctx context.Context,
	req *librarypb2.UpdateBookRequest,
) (*librarypb2.Book, error) {
	slog.Info("UpdateBook called", "book", req.Book.Name)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "basic-server"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "basic-server"))
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
	slog.Info("MoveBook called", "name", req.Name, "other_shelf_name", req.OtherShelfName)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "basic-server"))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "basic-server"))
	return nil, status.FromReason(errors.New("book not found"), librarypb.Reason_BOOK_NOT_FOUND, nil)
}

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", "error", err)
		os.Exit(1)
	}

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.xds.example.basic.server"); err != nil {
		slog.Error("failed to initialize yggdrasil", "error", err)
		os.Exit(1)
	}

	ss := &LibraryImpl{}
	if err := yggdrasil.Serve(
		yggdrasil.WithServiceDesc(&librarypb2.LibraryServiceServiceDesc, ss),
	); err != nil {
		slog.Error("failed to serve", "error", err)
		os.Exit(1)
	}
}
