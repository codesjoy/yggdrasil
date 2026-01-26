package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	helloworldpb "github.com/codesjoy/yggdrasil/v2/example/protogen/helloworld"
	_ "github.com/codesjoy/yggdrasil/v2/interceptor/logging"
	_ "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc"
	"github.com/codesjoy/yggdrasil/v2/status"
	"google.golang.org/genproto/googleapis/rpc/code"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidInput = errors.New("invalid input")
)

type GreeterServer struct {
	helloworldpb.UnimplementedGreeterServiceServer
}

func (s *GreeterServer) SayHello(ctx context.Context, req *helloworldpb.SayHelloRequest) (*helloworldpb.SayHelloResponse, error) {
	slog.Info("SayHello called", "name", req.Name)

	if req.Name == "" {
		return nil, status.WithCode(code.Code_INVALID_ARGUMENT, ErrInvalidInput)
	}

	return &helloworldpb.SayHelloResponse{
		Message: fmt.Sprintf("Hello %s!", req.Name),
	}, nil
}

func (s *GreeterServer) SayHelloStream(stream helloworldpb.GreeterServiceSayHelloStreamServer) error {
	slog.Info("SayHelloStream started")

	for {
		req, err := stream.Recv()
		if err != nil {
			slog.Error("SayHelloStream error", "error", err)
			return err
		}

		resp := &helloworldpb.SayHelloStreamResponse{
			Message: fmt.Sprintf("Hello %s!", req.Name),
		}

		if err := stream.Send(resp); err != nil {
			slog.Error("SayHelloStream send error", "error", err)
			return err
		}
	}
}

func (s *GreeterServer) SayHelloClientStream(stream helloworldpb.GreeterServiceSayHelloClientStreamServer) error {
	slog.Info("SayHelloClientStream started")

	var names []string
	for {
		req, err := stream.Recv()
		if err != nil {
			break
		}

		slog.Info("SayHelloClientStream received", "name", req.Name)
		names = append(names, req.Name)
	}

	if len(names) == 0 {
		return status.WithCode(code.Code_INVALID_ARGUMENT, ErrInvalidInput)
	}

	message := fmt.Sprintf("Hello %v!", names)
	resp := &helloworldpb.SayHelloClientStreamResponse{
		Message: message,
	}

	slog.Info("SayHelloClientStream sending response", "names", names)
	return stream.SendAndClose(resp)
}

func (s *GreeterServer) SayHelloServerStream(req *helloworldpb.SayHelloServerStreamRequest, stream helloworldpb.GreeterServiceSayHelloServerStreamServer) error {
	slog.Info("SayHelloServerStream started", "name", req.Name)

	if req.Name == "" {
		return status.WithCode(code.Code_INVALID_ARGUMENT, ErrInvalidInput)
	}

	for i := 0; i < 5; i++ {
		resp := &helloworldpb.SayHelloServerStreamResponse{
			Message: fmt.Sprintf("Hello %s! (message %d)", req.Name, i+1),
		}

		if err := stream.Send(resp); err != nil {
			slog.Error("SayHelloServerStream send error", "error", err)
			return err
		}

		time.Sleep(500 * time.Millisecond)
	}

	slog.Info("SayHelloServerStream completed")
	return nil
}

func (s *GreeterServer) SayError(ctx context.Context, req *helloworldpb.SayErrorRequest) (*helloworldpb.SayErrorResponse, error) {
	slog.Info("SayError called", "name", req.Name)

	if req.Name == "not_found" {
		return nil, status.WithCode(code.Code_NOT_FOUND, ErrUserNotFound)
	}

	if req.Name == "invalid" {
		return nil, status.WithCode(code.Code_INVALID_ARGUMENT, ErrInvalidInput)
	}

	return &helloworldpb.SayErrorResponse{
		Message: fmt.Sprintf("Error: %s", req.Name),
	}, nil
}

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.example.advanced.error-handling"); err != nil {
		os.Exit(1)
	}

	ss := &GreeterServer{}
	if err := yggdrasil.Serve(
		yggdrasil.WithServiceDesc(&helloworldpb.GreeterServiceServiceDesc, ss),
	); err != nil {
		os.Exit(1)
	}
}

func isRetryable(st *status.Status) bool {
	c := st.Code()

	switch c {
	case code.Code_DEADLINE_EXCEEDED, code.Code_UNAVAILABLE, code.Code_ABORTED:
		return true
	default:
		return false
	}
}

func retryWithBackoff(fn func() error, maxAttempts int) error {
	var lastErr error

	for i := 0; i < maxAttempts; i++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		st := status.FromError(err)
		if isRetryable(st) {
			backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
			slog.Warn("retrying", "attempt", i+1, "backoff", backoff)
			time.Sleep(backoff)
			continue
		}

		return err
	}

	return fmt.Errorf("max retries reached: %w", lastErr)
}
