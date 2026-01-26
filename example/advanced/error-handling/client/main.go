package main

import (
	"context"
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
	"github.com/codesjoy/yggdrasil/v2/metadata"
	_ "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc"
	"github.com/codesjoy/yggdrasil/v2/status"
	"google.golang.org/genproto/googleapis/rpc/code"
)

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}
	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.example.advanced.error-handling.client"); err != nil {
		os.Exit(1)
	}

	cli, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.advanced.error-handling")
	if err != nil {
		slog.Error("failed to create client", slog.Any("error", err))
		os.Exit(1)
	}
	defer cli.Close()

	client := helloworldpb.NewGreeterServiceClient(cli)
	ctx := metadata.WithStreamContext(context.Background())

	slog.Info("=== Testing Successful Call ===")
	if err := testSuccessfulCall(ctx, client); err != nil {
		slog.Error("successful call failed", slog.Any("error", err))
	}

	slog.Info("=== Testing Not Found Error ===")
	if err := testNotFoundError(ctx, client); err != nil {
		slog.Error("not found test failed", slog.Any("error", err))
	}

	slog.Info("=== Testing Invalid Input Error ===")
	if err := testInvalidInputError(ctx, client); err != nil {
		slog.Error("invalid input test failed", slog.Any("error", err))
	}

	slog.Info("=== Testing Retry Mechanism ===")
	if err := testRetryMechanism(ctx, client); err != nil {
		slog.Error("retry test failed", slog.Any("error", err))
	}

	slog.Info("All error handling tests completed!")
}

func testSuccessfulCall(ctx context.Context, client helloworldpb.GreeterServiceClient) error {
	slog.Info("Calling SayHello...")

	resp, err := client.SayHello(ctx, &helloworldpb.SayHelloRequest{
		Name: "World",
	})
	if err != nil {
		return err
	}

	slog.Info("SayHello response", "message", resp.Message)

	if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
		slog.Info("Response trailer", "trailer", trailer)
	}
	if header, ok := metadata.FromHeaderCtx(ctx); ok {
		slog.Info("Response header", "header", header)
	}

	return nil
}

func testNotFoundError(ctx context.Context, client helloworldpb.GreeterServiceClient) error {
	slog.Info("Calling SayError with 'not_found'...")

	_, err := client.SayError(ctx, &helloworldpb.SayErrorRequest{
		Name: "not_found",
	})
	if err != nil {
		st := status.FromError(err)
		slog.Info("Error details",
			"code", st.Code(),
			"http_code", st.HTTPCode(),
			"message", st.Message(),
		)

		if st.ErrorInfo() != nil && st.ErrorInfo().Metadata != nil {
			slog.Info("Error metadata", "metadata", st.ErrorInfo().Metadata)
		}

		return nil
	}

	return fmt.Errorf("expected error but got success")
}

func testInvalidInputError(ctx context.Context, client helloworldpb.GreeterServiceClient) error {
	slog.Info("Calling SayError with 'invalid'...")

	_, err := client.SayError(ctx, &helloworldpb.SayErrorRequest{
		Name: "invalid",
	})
	if err != nil {
		st := status.FromError(err)
		slog.Info("Error details",
			"code", st.Code(),
			"http_code", st.HTTPCode(),
			"message", st.Message(),
		)

		if st.ErrorInfo() != nil && st.ErrorInfo().Metadata != nil {
			slog.Info("Error metadata", "metadata", st.ErrorInfo().Metadata)
		}

		return nil
	}

	return fmt.Errorf("expected error but got success")
}

func testRetryMechanism(ctx context.Context, client helloworldpb.GreeterServiceClient) error {
	slog.Info("Testing retry mechanism with SayHello...")

	err := retryWithBackoff(func() error {
		_, err := client.SayHello(ctx, &helloworldpb.SayHelloRequest{
			Name: "RetryTest",
		})
		return err
	}, 3)
	if err != nil {
		return err
	}

	slog.Info("Retry test completed successfully")
	return nil
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
