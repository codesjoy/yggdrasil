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
	deploymentType string
}

func (s *LibraryImpl) CreateShelf(
	ctx context.Context,
	req *librarypb2.CreateShelfRequest,
) (*librarypb2.Shelf, error) {
	slog.Info("CreateShelf called", "name", req.Shelf.Name, "theme", req.Shelf.Theme, "deployment_type", s.deploymentType)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	return &librarypb2.Shelf{
		Name:  req.Shelf.Name,
		Theme: req.Shelf.Theme,
	}, nil
}

func (s *LibraryImpl) GetShelf(
	ctx context.Context,
	req *librarypb2.GetShelfRequest,
) (*librarypb2.Shelf, error) {
	slog.Info("GetShelf called", "name", req.Name, "deployment_type", s.deploymentType)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))

	theme := "Stable Version"
	if s.deploymentType == "canary" {
		theme = "Canary Version"
	}

	return &librarypb2.Shelf{
		Name:  req.Name,
		Theme: theme,
	}, nil
}

func (s *LibraryImpl) ListShelves(
	ctx context.Context,
	req *librarypb2.ListShelvesRequest,
) (*librarypb2.ListShelvesResponse, error) {
	slog.Info("ListShelves called", "parent", req.Parent, "deployment_type", s.deploymentType)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))

	theme := "Stable Version"
	if s.deploymentType == "canary" {
		theme = "Canary Version"
	}

	return &librarypb2.ListShelvesResponse{
		Shelves: []*librarypb2.Shelf{
			{Name: "shelves/1", Theme: theme + " 1"},
			{Name: "shelves/2", Theme: theme + " 2"},
		},
	}, nil
}

func (s *LibraryImpl) DeleteShelf(
	ctx context.Context,
	req *librarypb2.DeleteShelfRequest,
) (*emptypb.Empty, error) {
	slog.Info("DeleteShelf called", "name", req.Name, "deployment_type", s.deploymentType)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	return &emptypb.Empty{}, nil
}

func (s *LibraryImpl) MergeShelves(
	ctx context.Context,
	req *librarypb2.MergeShelvesRequest,
) (*librarypb2.Shelf, error) {
	slog.Info("MergeShelves called", "name", req.Name, "other_shelf", req.OtherShelf, "deployment_type", s.deploymentType)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	return &librarypb2.Shelf{
		Name:  req.Name,
		Theme: "Merged Theme - " + s.deploymentType,
	}, nil
}

func (s *LibraryImpl) CreateBook(
	ctx context.Context,
	req *librarypb2.CreateBookRequest,
) (*librarypb2.Book, error) {
	slog.Info("CreateBook called", "parent", req.Parent, "book", req.Book.Name, "deployment_type", s.deploymentType)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
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
	slog.Info("GetBook called", "name", req.Name, "deployment_type", s.deploymentType)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	return &librarypb2.Book{
		Name:   req.Name,
		Author: "Canary Author - " + s.deploymentType,
		Title:  "Canary Book Title - " + s.deploymentType,
		Read:   false,
	}, nil
}

func (s *LibraryImpl) ListBooks(
	ctx context.Context,
	req *librarypb2.ListBooksRequest,
) (*librarypb2.ListBooksResponse, error) {
	slog.Info("ListBooks called", "parent", req.Parent, "deployment_type", s.deploymentType)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	return &librarypb2.ListBooksResponse{
		Books: []*librarypb2.Book{
			{Name: req.Parent + "/books/book1", Author: "Author 1 - " + s.deploymentType, Title: "Book 1 - " + s.deploymentType},
			{Name: req.Parent + "/books/book2", Author: "Author 2 - " + s.deploymentType, Title: "Book 2 - " + s.deploymentType},
		},
	}, nil
}

func (s *LibraryImpl) DeleteBook(
	ctx context.Context,
	req *librarypb2.DeleteBookRequest,
) (*emptypb.Empty, error) {
	slog.Info("DeleteBook called", "name", req.Name, "deployment_type", s.deploymentType)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	return &emptypb.Empty{}, nil
}

func (s *LibraryImpl) UpdateBook(
	ctx context.Context,
	req *librarypb2.UpdateBookRequest,
) (*librarypb2.Book, error) {
	slog.Info("UpdateBook called", "book", req.Book.Name, "deployment_type", s.deploymentType)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
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
	slog.Info("MoveBook called", "name", req.Name, "other_shelf_name", req.OtherShelfName, "deployment_type", s.deploymentType)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", "canary-server", "deployment", s.deploymentType))
	return nil, status.FromReason(errors.New("book not found"), librarypb.Reason_BOOK_NOT_FOUND, nil)
}

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", "error", err)
		os.Exit(1)
	}

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.xds.example.canary.server"); err != nil {
		slog.Error("failed to initialize yggdrasil", "error", err)
		os.Exit(1)
	}

	deploymentType := os.Getenv("DEPLOYMENT_TYPE")
	if deploymentType == "" {
		deploymentType = "stable"
	}

	slog.Info("Starting canary deployment server...", "deployment_type", deploymentType)

	ss := &LibraryImpl{deploymentType: deploymentType}
	if err := yggdrasil.Serve(
		yggdrasil.WithServiceDesc(&librarypb2.LibraryServiceServiceDesc, ss),
	); err != nil {
		slog.Error("failed to serve", "error", err)
		os.Exit(1)
	}
}
