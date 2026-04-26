// Copyright 2024 The codesjoy Authors.
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
	"math"
	"os"
	"time"

	"github.com/codesjoy/pkg/basic/xerror"
	"google.golang.org/genproto/googleapis/rpc/code"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	errorhandlingpb "github.com/codesjoy/yggdrasil/v3/examples/protogen/error-handling"
	"github.com/codesjoy/yggdrasil/v3/rpc/status"
)

const serverName = "github.com.codesjoy.yggdrasil.example.13-error-reason"

func main() {
	app, err := yapp.New("", yapp.WithConfigPath("config.yaml"))
	if err != nil {
		os.Exit(1)
	}
	defer func() { _ = app.Stop(context.Background()) }()

	cli, err := app.NewClient(
		context.Background(),
		serverName,
	)
	if err != nil {
		slog.Error("failed to create client", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() { _ = cli.Close() }()

	client := errorhandlingpb.NewLibraryServiceClient(cli)
	ctx := context.Background()

	// Test 1: Successful call
	slog.Info("=== Test 1: Successful CreateBook ===")
	testCreateBook(ctx, client)

	// Test 2: USER_NOT_FOUND
	slog.Info("=== Test 2: USER_NOT_FOUND ===")
	testGetUserNotFound(ctx, client)

	// Test 3: INVALID_INPUT - invalid email
	slog.Info("=== Test 3: INVALID_INPUT (invalid email) ===")
	testCreateUserInvalidEmail(ctx, client)

	// Test 4: INVALID_CREDENTIALS
	slog.Info("=== Test 4: INVALID_CREDENTIALS ===")
	testAuthenticateUserInvalid(ctx, client)

	// Test 5: EMAIL_ALREADY_EXISTS
	slog.Info("=== Test 5: EMAIL_ALREADY_EXISTS ===")
	testCreateUserEmailExists(ctx, client)

	// Test 6: BOOK_NOT_FOUND
	slog.Info("=== Test 6: BOOK_NOT_FOUND ===")
	testGetBookNotFound(ctx, client)

	// Test 7: BOOK_ALREADY_BORROWED
	slog.Info("=== Test 7: BOOK_ALREADY_BORROWED ===")
	testBorrowBookAlreadyBorrowed(ctx, client)

	// Test 8: SHELF_FULL
	slog.Info("=== Test 8: SHELF_FULL ===")
	testAddBookToShelfFull(ctx, client)

	// Test 9: DATABASE_ERROR
	slog.Info("=== Test 9: DATABASE_ERROR ===")
	testTriggerDatabaseError(ctx, client)

	// Test 10: NETWORK_ERROR
	slog.Info("=== Test 10: NETWORK_ERROR ===")
	testTriggerNetworkError(ctx, client)

	// Test 11: INTERNAL_ERROR
	slog.Info("=== Test 11: INTERNAL_ERROR ===")
	testTriggerInternalError(ctx, client)

	// Test 12: Retry mechanism
	slog.Info("=== Test 12: Retry Mechanism (with INTERNAL_ERROR) ===")
	testRetryMechanism(ctx, client)

	slog.Info("All error handling tests completed!")
}

func testCreateBook(ctx context.Context, client errorhandlingpb.LibraryServiceClient) {
	resp, err := client.CreateBook(ctx, &errorhandlingpb.CreateBookRequest{
		Title:  "The Go Programming Language",
		Author: "Alan A. A. Donovan",
		Isbn:   "978-0134190440",
	})
	if err != nil {
		slog.Error("CreateBook failed", "error", err)
		return
	}

	slog.Info("✓ Book created successfully",
		"book_id", resp.Book.Id,
		"title", resp.Book.Title,
		"author", resp.Book.Author,
	)
}

func testGetUserNotFound(ctx context.Context, client errorhandlingpb.LibraryServiceClient) {
	_, err := client.GetUser(ctx, &errorhandlingpb.GetUserRequest{
		UserId: "non-existent-user",
	})
	if err != nil {
		st := status.FromError(err)

		if isReason(st, errorhandlingpb.Reason_USER_NOT_FOUND) {
			slog.Info("✓ Correctly identified USER_NOT_FOUND",
				"grpc_code", st.Code(),
				"http_code", st.HTTPCode(),
				"message", st.Message(),
			)

			if st.ErrorInfo() != nil {
				slog.Info("Error metadata", "metadata", st.ErrorInfo().Metadata)
			}
			return
		}
	}

	slog.Error("✗ Expected USER_NOT_FOUND error")
}

func testCreateUserInvalidEmail(ctx context.Context, client errorhandlingpb.LibraryServiceClient) {
	_, err := client.CreateUser(ctx, &errorhandlingpb.CreateUserRequest{
		Email:    "invalid-email",
		Name:     "Test User",
		Password: "password123",
	})
	if err != nil {
		st := status.FromError(err)

		if isReason(st, errorhandlingpb.Reason_INVALID_INPUT) {
			slog.Info("✓ Correctly identified INVALID_INPUT",
				"grpc_code", st.Code(),
				"http_code", st.HTTPCode(),
				"message", st.Message(),
			)

			if st.ErrorInfo() != nil {
				slog.Info("Error metadata", "metadata", st.ErrorInfo().Metadata)
			}
			return
		}
	}

	slog.Error("✗ Expected INVALID_INPUT error")
}

func testAuthenticateUserInvalid(ctx context.Context, client errorhandlingpb.LibraryServiceClient) {
	_, err := client.AuthenticateUser(ctx, &errorhandlingpb.AuthenticateUserRequest{
		Email:    "test@example.com",
		Password: "wrong-password",
	})
	if err != nil {
		st := status.FromError(err)

		if isReason(st, errorhandlingpb.Reason_INVALID_CREDENTIALS) {
			slog.Info("✓ Correctly identified INVALID_CREDENTIALS",
				"grpc_code", st.Code(),
				"http_code", st.HTTPCode(),
				"message", st.Message(),
			)

			if st.ErrorInfo() != nil {
				slog.Info("Error metadata", "metadata", st.ErrorInfo().Metadata)
			}
			return
		}
	}

	slog.Error("✗ Expected INVALID_CREDENTIALS error")
}

func testCreateUserEmailExists(ctx context.Context, client errorhandlingpb.LibraryServiceClient) {
	// First, create a user
	_, _ = client.CreateUser(ctx, &errorhandlingpb.CreateUserRequest{
		Email:    "duplicate@example.com",
		Name:     "First User",
		Password: "password123",
	})

	// Try to create another user with the same email
	_, err := client.CreateUser(ctx, &errorhandlingpb.CreateUserRequest{
		Email:    "duplicate@example.com",
		Name:     "Second User",
		Password: "password123",
	})
	if err != nil {
		st := status.FromError(err)

		if isReason(st, errorhandlingpb.Reason_EMAIL_ALREADY_EXISTS) {
			slog.Info("✓ Correctly identified EMAIL_ALREADY_EXISTS",
				"grpc_code", st.Code(),
				"http_code", st.HTTPCode(),
				"message", st.Message(),
			)

			if st.ErrorInfo() != nil {
				slog.Info("Error metadata", "metadata", st.ErrorInfo().Metadata)
			}
			return
		}
	}

	slog.Error("✗ Expected EMAIL_ALREADY_EXISTS error")
}

func testGetBookNotFound(ctx context.Context, client errorhandlingpb.LibraryServiceClient) {
	_, err := client.GetBook(ctx, &errorhandlingpb.GetBookRequest{
		BookId: "non-existent-book",
	})
	if err != nil {
		st := status.FromError(err)

		if isReason(st, errorhandlingpb.Reason_BOOK_NOT_FOUND) {
			slog.Info("✓ Correctly identified BOOK_NOT_FOUND",
				"grpc_code", st.Code(),
				"http_code", st.HTTPCode(),
				"message", st.Message(),
			)

			if st.ErrorInfo() != nil {
				slog.Info("Error metadata", "metadata", st.ErrorInfo().Metadata)
			}
			return
		}
	}

	slog.Error("✗ Expected BOOK_NOT_FOUND error")
}

func testBorrowBookAlreadyBorrowed(
	ctx context.Context,
	client errorhandlingpb.LibraryServiceClient,
) {
	// First, create a book and borrow it
	bookResp, _ := client.CreateBook(ctx, &errorhandlingpb.CreateBookRequest{
		Title:  "Test Book",
		Author: "Test Author",
	})

	userResp, _ := client.CreateUser(ctx, &errorhandlingpb.CreateUserRequest{
		Email:    "borrower@example.com",
		Name:     "Borrower",
		Password: "password123",
	})

	// Borrow the book
	_, _ = client.BorrowBook(ctx, &errorhandlingpb.BorrowBookRequest{
		BookId: bookResp.Book.Id,
		UserId: userResp.User.Id,
	})

	// Try to borrow it again
	_, err := client.BorrowBook(ctx, &errorhandlingpb.BorrowBookRequest{
		BookId: bookResp.Book.Id,
		UserId: userResp.User.Id,
	})
	if err != nil {
		st := status.FromError(err)

		if isReason(st, errorhandlingpb.Reason_BOOK_ALREADY_BORROWED) {
			slog.Info("✓ Correctly identified BOOK_ALREADY_BORROWED",
				"grpc_code", st.Code(),
				"http_code", st.HTTPCode(),
				"message", st.Message(),
			)

			if st.ErrorInfo() != nil {
				slog.Info("Error metadata", "metadata", st.ErrorInfo().Metadata)
			}
			return
		}
	}

	slog.Error("✗ Expected BOOK_ALREADY_BORROWED error")
}

func testAddBookToShelfFull(ctx context.Context, client errorhandlingpb.LibraryServiceClient) {
	// Create a shelf with capacity 1
	shelfResp, _ := client.CreateShelf(ctx, &errorhandlingpb.CreateShelfRequest{
		Name:     "Small Shelf",
		Capacity: 1,
	})

	// Add a book to the shelf
	_, _ = client.AddBookToShelf(ctx, &errorhandlingpb.AddBookToShelfRequest{
		ShelfId: shelfResp.Shelf.Id,
		BookId:  "book-1",
	})

	// Try to add another book
	_, err := client.AddBookToShelf(ctx, &errorhandlingpb.AddBookToShelfRequest{
		ShelfId: shelfResp.Shelf.Id,
		BookId:  "book-2",
	})
	if err != nil {
		st := status.FromError(err)

		if isReason(st, errorhandlingpb.Reason_SHELF_FULL) {
			slog.Info("✓ Correctly identified SHELF_FULL",
				"grpc_code", st.Code(),
				"http_code", st.HTTPCode(),
				"message", st.Message(),
			)

			if st.ErrorInfo() != nil {
				slog.Info("Error metadata", "metadata", st.ErrorInfo().Metadata)
			}
			return
		}
	}

	slog.Error("✗ Expected SHELF_FULL error")
}

func testTriggerDatabaseError(ctx context.Context, client errorhandlingpb.LibraryServiceClient) {
	_, err := client.TriggerError(ctx, &errorhandlingpb.TriggerErrorRequest{
		ErrorType: "database_error",
	})
	if err != nil {
		st := status.FromError(err)

		if isReason(st, errorhandlingpb.Reason_DATABASE_ERROR) {
			slog.Info("✓ Correctly identified DATABASE_ERROR",
				"grpc_code", st.Code(),
				"http_code", st.HTTPCode(),
				"message", st.Message(),
			)

			if st.ErrorInfo() != nil {
				slog.Info("Error metadata", "metadata", st.ErrorInfo().Metadata)
			}
			return
		}
	}

	slog.Error("✗ Expected DATABASE_ERROR")
}

func testTriggerNetworkError(ctx context.Context, client errorhandlingpb.LibraryServiceClient) {
	_, err := client.TriggerError(ctx, &errorhandlingpb.TriggerErrorRequest{
		ErrorType: "network_error",
	})
	if err != nil {
		st := status.FromError(err)

		if isReason(st, errorhandlingpb.Reason_NETWORK_ERROR) {
			slog.Info("✓ Correctly identified NETWORK_ERROR",
				"grpc_code", st.Code(),
				"http_code", st.HTTPCode(),
				"message", st.Message(),
			)

			if st.ErrorInfo() != nil {
				slog.Info("Error metadata", "metadata", st.ErrorInfo().Metadata)
			}
			return
		}
	}

	slog.Error("✗ Expected NETWORK_ERROR")
}

func testTriggerInternalError(ctx context.Context, client errorhandlingpb.LibraryServiceClient) {
	_, err := client.TriggerError(ctx, &errorhandlingpb.TriggerErrorRequest{
		ErrorType: "internal_error",
	})
	if err != nil {
		st := status.FromError(err)

		if isReason(st, errorhandlingpb.Reason_INTERNAL_ERROR) {
			slog.Info("✓ Correctly identified INTERNAL_ERROR",
				"grpc_code", st.Code(),
				"http_code", st.HTTPCode(),
				"message", st.Message(),
			)

			if st.ErrorInfo() != nil {
				slog.Info("Error metadata", "metadata", st.ErrorInfo().Metadata)
			}
			return
		}
	}

	slog.Error("✗ Expected INTERNAL_ERROR")
}

func testRetryMechanism(ctx context.Context, client errorhandlingpb.LibraryServiceClient) {
	attempts := 0
	maxAttempts := 3

	err := retryWithBackoff(func() error {
		attempts++
		slog.Info("Attempt", "count", attempts)

		_, err := client.TriggerError(ctx, &errorhandlingpb.TriggerErrorRequest{
			ErrorType: "internal_error",
		})
		return err
	}, maxAttempts)
	if err != nil {
		slog.Info("✓ Retry mechanism test completed",
			"attempts", attempts,
			"result", "all retries exhausted (as expected for non-retryable error)",
		)
	}
}

func isReason(st *status.Status, reason xerror.Reason) bool {
	if st == nil || reason == nil {
		return false
	}
	info := st.ErrorInfo()
	if info == nil {
		return false
	}
	return info.GetReason() == reason.Reason() && info.GetDomain() == reason.Domain()
}

func isRetryable(st *status.Status) bool {
	switch st.Code() {
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

		// Check if error is retryable
		if isRetryable(st) {
			backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
			slog.Warn("Retrying...", "attempt", i+1, "backoff", backoff)
			time.Sleep(backoff)
			continue
		}

		// Non-retryable error, return immediately
		slog.Info("Non-retryable error", "reason", st.Message())
		return err
	}

	return fmt.Errorf("max retries reached: %w", lastErr)
}
