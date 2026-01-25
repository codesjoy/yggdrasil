package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	helloworldv1 "github.com/codesjoy/yggdrasil/v2/example/protogen/helloworld"
	libraryv1 "github.com/codesjoy/yggdrasil/v2/example/protogen/library/v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

type LibraryMSImpl struct {
	libraryv1.UnimplementedLibraryServiceServer
	serverID string
}

func (s *LibraryMSImpl) GetShelf(
	ctx context.Context,
	req *libraryv1.GetShelfRequest,
) (*libraryv1.Shelf, error) {
	slog.Info("GetShelf called", "name", req.Name, "server", s.serverID)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", s.serverID))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", s.serverID))
	return &libraryv1.Shelf{
		Name:  req.Name,
		Theme: "multi-service-server-" + s.serverID,
	}, nil
}

func (s *LibraryMSImpl) CreateShelf(
	ctx context.Context,
	req *libraryv1.CreateShelfRequest,
) (*libraryv1.Shelf, error) {
	slog.Info("CreateShelf called", "name", req.Shelf.Name, "server", s.serverID)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", s.serverID))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", s.serverID))
	return &libraryv1.Shelf{
		Name:  req.Shelf.Name,
		Theme: "multi-service-server-" + s.serverID,
	}, nil
}

func (s *LibraryMSImpl) ListShelves(
	ctx context.Context,
	req *libraryv1.ListShelvesRequest,
) (*libraryv1.ListShelvesResponse, error) {
	slog.Info("ListShelves called", "parent", req.Parent, "server", s.serverID)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", s.serverID))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", s.serverID))
	return &libraryv1.ListShelvesResponse{
		Shelves: []*libraryv1.Shelf{
			{Name: "shelf-1", Theme: "multi-service-server-" + s.serverID},
			{Name: "shelf-2", Theme: "multi-service-server-" + s.serverID},
		},
	}, nil
}

func (s *LibraryMSImpl) DeleteShelf(
	ctx context.Context,
	req *libraryv1.DeleteShelfRequest,
) (*emptypb.Empty, error) {
	slog.Info("DeleteShelf called", "name", req.Name, "server", s.serverID)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", s.serverID))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", s.serverID))
	return &emptypb.Empty{}, nil
}

func (s *LibraryMSImpl) MergeShelves(
	ctx context.Context,
	req *libraryv1.MergeShelvesRequest,
) (*libraryv1.Shelf, error) {
	// Minimal implementation
	return &libraryv1.Shelf{Name: req.Name, Theme: "Merged"}, nil
}

func (s *LibraryMSImpl) CreateBook(
	ctx context.Context,
	req *libraryv1.CreateBookRequest,
) (*libraryv1.Book, error) {
	return &libraryv1.Book{Name: req.Book.Name}, nil
}

func (s *LibraryMSImpl) GetBook(
	ctx context.Context,
	req *libraryv1.GetBookRequest,
) (*libraryv1.Book, error) {
	return &libraryv1.Book{Name: req.Name}, nil
}

func (s *LibraryMSImpl) ListBooks(
	ctx context.Context,
	req *libraryv1.ListBooksRequest,
) (*libraryv1.ListBooksResponse, error) {
	return &libraryv1.ListBooksResponse{}, nil
}

func (s *LibraryMSImpl) DeleteBook(
	ctx context.Context,
	req *libraryv1.DeleteBookRequest,
) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *LibraryMSImpl) UpdateBook(
	ctx context.Context,
	req *libraryv1.UpdateBookRequest,
) (*libraryv1.Book, error) {
	return req.Book, nil
}

func (s *LibraryMSImpl) MoveBook(
	ctx context.Context,
	req *libraryv1.MoveBookRequest,
) (*libraryv1.Book, error) {
	return &libraryv1.Book{Name: req.Name}, nil
}

type GreeterMSImpl struct {
	helloworldv1.UnimplementedGreeterServiceServer
	serverID string
}

func (s *GreeterMSImpl) SayHello(
	ctx context.Context,
	req *helloworldv1.SayHelloRequest,
) (*helloworldv1.SayHelloResponse, error) {
	slog.Info("SayHello called", "name", req.Name, "server", s.serverID)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", s.serverID))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", s.serverID))
	return &helloworldv1.SayHelloResponse{
		Message: "Hello " + req.Name + " from multi-service-server-" + s.serverID,
	}, nil
}

func (s *GreeterMSImpl) SayError(
	ctx context.Context,
	req *helloworldv1.SayErrorRequest,
) (*helloworldv1.SayErrorResponse, error) {
	slog.Info("SayError called", "name", req.Name, "server", s.serverID)
	_ = metadata.SetTrailer(ctx, metadata.Pairs("server", s.serverID))
	_ = metadata.SetHeader(ctx, metadata.Pairs("server", s.serverID))
	return &helloworldv1.SayErrorResponse{
		Message: "Error from multi-service-server-" + s.serverID,
	}, nil
}

func (s *GreeterMSImpl) SayHelloStream(
	stream helloworldv1.GreeterService_SayHelloStreamServer,
) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		slog.Info("SayHelloStream called", "name", req.Name, "server", s.serverID)
		err = stream.Send(&helloworldv1.SayHelloStreamResponse{
			Message: "Hello " + req.Name + " from multi-service-server-" + s.serverID,
		})
		if err != nil {
			return err
		}
	}
}

func (s *GreeterMSImpl) SayHelloClientStream(
	stream helloworldv1.GreeterService_SayHelloClientStreamServer,
) error {
	var names []string
	for {
		req, err := stream.Recv()
		if err != nil {
			break
		}
		names = append(names, req.Name)
	}
	slog.Info("SayHelloClientStream called", "names", names, "server", s.serverID)
	return stream.SendAndClose(&helloworldv1.SayHelloClientStreamResponse{
		Message: "Hello all from multi-service-server-" + s.serverID,
	})
}

func (s *GreeterMSImpl) SayHelloServerStream(
	req *helloworldv1.SayHelloServerStreamRequest,
	stream helloworldv1.GreeterService_SayHelloServerStreamServer,
) error {
	slog.Info("SayHelloServerStream called", "name", req.Name, "server", s.serverID)
	for i := 0; i < 3; i++ {
		err := stream.Send(&helloworldv1.SayHelloServerStreamResponse{
			Message: "Hello " + req.Name + " from multi-service-server-" + s.serverID,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", "error", err)
		os.Exit(1)
	}

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.xds.example.multi-service.server"); err != nil {
		slog.Error("failed to initialize yggdrasil", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting multi-service server...")

	serverID := os.Getenv("SERVER_ID")
	if serverID == "" {
		serverID = "1"
	}

	// Assuming a simpler Serve approach for multi-service or just using Init/Serve pattern if supported.
	// Since yggdrasil.Serve takes a service desc, we might need a way to register multiple services.
	// However, looking at the commented out code, it used grpc.NewServer() and explicit registration.
	// But `yggdrasil` framework might wrap it.
	// Let's stick to the commented out logic but adapted to v2 imports if possible,
	// OR better yet, use yggdrasil.Serve which seems to support variadic ServerOption,
	// but the `yggdrasil.WithServiceDesc` takes one service.

	// Wait, if yggdrasil.Serve only supports one service, then multi-service might need to use standard grpc server + yggdrasil interceptors?
	// The commented out code did exactly that: `grpc.NewServer(...)` and `listener`.
	// Let's restore that pattern as it seems intended for multi-service.

	// Re-verify the config loading part from other examples.
	// They use yggdrasil.Init and yggdrasil.Serve.

	// If I use `yggdrasil.Serve`, looking at `basic/server/main.go`:
	// yggdrasil.Serve(yggdrasil.WithServiceDesc(...))
	// It accepts `...ServerOption`.

	// Let's try to pass multiple `WithServiceDesc` if allowed?
	// `WithServiceDesc` returns a `ServerOption`. So yes, we can pass multiple!

	libraryImpl := &LibraryMSImpl{serverID: serverID}
	greeterImpl := &GreeterMSImpl{serverID: serverID}

	if err := yggdrasil.Serve(
		yggdrasil.WithServiceDesc(&libraryv1.LibraryServiceServiceDesc, libraryImpl),
		yggdrasil.WithServiceDesc(&helloworldv1.GreeterServiceServiceDesc, greeterImpl),
	); err != nil {
		slog.Error("failed to serve", "error", err)
		os.Exit(1)
	}
}
