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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/codesjoy/pkg/basic/xerror"
	"google.golang.org/genproto/googleapis/rpc/code"

	"github.com/codesjoy/yggdrasil/v3"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/file"
	errorhandlingpb "github.com/codesjoy/yggdrasil/v3/example/protogen/error-handling"
)

// LibraryServer implements the LibraryService
type LibraryServer struct {
	errorhandlingpb.UnimplementedLibraryServiceServer

	mu        sync.RWMutex
	users     map[string]*errorhandlingpb.User
	books     map[string]*errorhandlingpb.Book
	shelves   map[string]*errorhandlingpb.Shelf
	bookIDs   []string
	userIDs   []string
	shelfIDs  []string
	emailToID map[string]string
}

func NewLibraryServer() *LibraryServer {
	return &LibraryServer{
		users:     make(map[string]*errorhandlingpb.User),
		books:     make(map[string]*errorhandlingpb.Book),
		shelves:   make(map[string]*errorhandlingpb.Shelf),
		bookIDs:   []string{"book-1", "book-2", "book-3"},
		userIDs:   []string{"user-1", "user-2"},
		shelfIDs:  []string{"shelf-1", "shelf-2"},
		emailToID: make(map[string]string),
	}
}

func reasonErr(err error, reason xerror.Reason, metadata map[string]string) error {
	return xerror.WrapWithReason(err, reason, "", metadata)
}

func (s *LibraryServer) CreateUser(
	ctx context.Context,
	req *errorhandlingpb.CreateUserRequest,
) (*errorhandlingpb.CreateUserResponse, error) {
	slog.Info("CreateUser called", "email", req.Email, "name", req.Name)

	// Validate email format
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		return nil, reasonErr(
			errors.New("invalid email format"),
			errorhandlingpb.Reason_INVALID_INPUT,
			map[string]string{"field": "email", "value": req.Email},
		)
	}

	// Validate password
	if req.Password == "" || len(req.Password) < 6 {
		return nil, reasonErr(
			errors.New("password too short"),
			errorhandlingpb.Reason_INVALID_INPUT,
			map[string]string{"field": "password", "min_length": "6"},
		)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if email already exists
	if _, exists := s.emailToID[req.Email]; exists {
		return nil, reasonErr(
			fmt.Errorf("email %s already registered", req.Email),
			errorhandlingpb.Reason_EMAIL_ALREADY_EXISTS,
			map[string]string{"email": req.Email},
		)
	}

	// Create user
	userID := fmt.Sprintf("user-%d", len(s.users)+1)
	user := &errorhandlingpb.User{
		Id:    userID,
		Email: req.Email,
		Name:  req.Name,
	}

	s.users[userID] = user
	s.userIDs = append(s.userIDs, userID)
	s.emailToID[req.Email] = userID

	slog.Info("User created successfully", "user_id", userID)
	return &errorhandlingpb.CreateUserResponse{User: user}, nil
}

func (s *LibraryServer) GetUser(
	ctx context.Context,
	req *errorhandlingpb.GetUserRequest,
) (*errorhandlingpb.GetUserResponse, error) {
	slog.Info("GetUser called", "user_id", req.UserId)

	if req.UserId == "" {
		return nil, reasonErr(
			errors.New("user_id is required"),
			errorhandlingpb.Reason_INVALID_INPUT,
			map[string]string{"field": "user_id"},
		)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[req.UserId]
	if !exists {
		return nil, reasonErr(
			fmt.Errorf("user %s not found", req.UserId),
			errorhandlingpb.Reason_USER_NOT_FOUND,
			map[string]string{"user_id": req.UserId},
		)
	}

	return &errorhandlingpb.GetUserResponse{User: user}, nil
}

func (s *LibraryServer) AuthenticateUser(
	ctx context.Context,
	req *errorhandlingpb.AuthenticateUserRequest,
) (*errorhandlingpb.AuthenticateUserResponse, error) {
	slog.Info("AuthenticateUser called", "email", req.Email)

	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, exists := s.emailToID[req.Email]
	if !exists {
		return nil, reasonErr(
			errors.New("invalid credentials"),
			errorhandlingpb.Reason_INVALID_CREDENTIALS,
			map[string]string{"email": req.Email},
		)
	}

	user := s.users[userID]

	// Simple password check (in production, use bcrypt)
	if req.Password != "password123" {
		return nil, reasonErr(
			errors.New("invalid credentials"),
			errorhandlingpb.Reason_INVALID_CREDENTIALS,
			map[string]string{"email": req.Email},
		)
	}

	slog.Info("User authenticated successfully", "user_id", userID)
	return &errorhandlingpb.AuthenticateUserResponse{
		User:  user,
		Token: fmt.Sprintf("token-%s", userID),
	}, nil
}

func (s *LibraryServer) CreateBook(
	ctx context.Context,
	req *errorhandlingpb.CreateBookRequest,
) (*errorhandlingpb.CreateBookResponse, error) {
	slog.Info("CreateBook called", "title", req.Title, "author", req.Author)

	if req.Title == "" || req.Author == "" {
		return nil, reasonErr(
			errors.New("title and author are required"),
			errorhandlingpb.Reason_INVALID_INPUT,
			map[string]string{"missing_fields": "title, author"},
		)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	bookID := fmt.Sprintf("book-%d", len(s.books)+1)
	book := &errorhandlingpb.Book{
		Id:     bookID,
		Title:  req.Title,
		Author: req.Author,
		Isbn:   req.Isbn,
	}

	s.books[bookID] = book
	s.bookIDs = append(s.bookIDs, bookID)

	slog.Info("Book created successfully", "book_id", bookID)
	return &errorhandlingpb.CreateBookResponse{Book: book}, nil
}

func (s *LibraryServer) GetBook(
	ctx context.Context,
	req *errorhandlingpb.GetBookRequest,
) (*errorhandlingpb.GetBookResponse, error) {
	slog.Info("GetBook called", "book_id", req.BookId)

	if req.BookId == "" {
		return nil, reasonErr(
			errors.New("book_id is required"),
			errorhandlingpb.Reason_INVALID_INPUT,
			map[string]string{"field": "book_id"},
		)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	book, exists := s.books[req.BookId]
	if !exists {
		return nil, reasonErr(
			fmt.Errorf("book %s not found", req.BookId),
			errorhandlingpb.Reason_BOOK_NOT_FOUND,
			map[string]string{"book_id": req.BookId},
		)
	}

	return &errorhandlingpb.GetBookResponse{Book: book}, nil
}

func (s *LibraryServer) BorrowBook(
	ctx context.Context,
	req *errorhandlingpb.BorrowBookRequest,
) (*errorhandlingpb.BorrowBookResponse, error) {
	slog.Info("BorrowBook called", "book_id", req.BookId, "user_id", req.UserId)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if user exists
	if _, exists := s.users[req.UserId]; !exists {
		return nil, reasonErr(
			fmt.Errorf("user %s not found", req.UserId),
			errorhandlingpb.Reason_USER_NOT_FOUND,
			map[string]string{"user_id": req.UserId},
		)
	}

	// Check if book exists
	book, exists := s.books[req.BookId]
	if !exists {
		return nil, reasonErr(
			fmt.Errorf("book %s not found", req.BookId),
			errorhandlingpb.Reason_BOOK_NOT_FOUND,
			map[string]string{"book_id": req.BookId},
		)
	}

	// Check if book is already borrowed
	if book.BorrowerId != "" {
		return nil, reasonErr(
			errors.New("book is already borrowed"),
			errorhandlingpb.Reason_BOOK_ALREADY_BORROWED,
			map[string]string{
				"book_id":     req.BookId,
				"borrower_id": book.BorrowerId,
			},
		)
	}

	// Borrow the book
	book.BorrowerId = req.UserId

	slog.Info("Book borrowed successfully", "book_id", req.BookId, "user_id", req.UserId)
	return &errorhandlingpb.BorrowBookResponse{Success: true}, nil
}

func (s *LibraryServer) ReturnBook(
	ctx context.Context,
	req *errorhandlingpb.ReturnBookRequest,
) (*errorhandlingpb.ReturnBookResponse, error) {
	slog.Info("ReturnBook called", "book_id", req.BookId)

	s.mu.Lock()
	defer s.mu.Unlock()

	book, exists := s.books[req.BookId]
	if !exists {
		return nil, reasonErr(
			fmt.Errorf("book %s not found", req.BookId),
			errorhandlingpb.Reason_BOOK_NOT_FOUND,
			map[string]string{"book_id": req.BookId},
		)
	}

	book.BorrowerId = ""

	slog.Info("Book returned successfully", "book_id", req.BookId)
	return &errorhandlingpb.ReturnBookResponse{Success: true}, nil
}

func (s *LibraryServer) CreateShelf(
	ctx context.Context,
	req *errorhandlingpb.CreateShelfRequest,
) (*errorhandlingpb.CreateShelfResponse, error) {
	slog.Info("CreateShelf called", "name", req.Name, "capacity", req.Capacity)

	if req.Name == "" {
		return nil, reasonErr(
			errors.New("name is required"),
			errorhandlingpb.Reason_INVALID_INPUT,
			map[string]string{"field": "name"},
		)
	}

	if req.Capacity <= 0 {
		return nil, reasonErr(
			errors.New("capacity must be positive"),
			errorhandlingpb.Reason_INVALID_INPUT,
			map[string]string{"field": "capacity", "min": "1"},
		)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	shelfID := fmt.Sprintf("shelf-%d", len(s.shelves)+1)
	shelf := &errorhandlingpb.Shelf{
		Id:           shelfID,
		Name:         req.Name,
		Capacity:     req.Capacity,
		CurrentCount: 0,
	}

	s.shelves[shelfID] = shelf
	s.shelfIDs = append(s.shelfIDs, shelfID)

	slog.Info("Shelf created successfully", "shelf_id", shelfID)
	return &errorhandlingpb.CreateShelfResponse{Shelf: shelf}, nil
}

func (s *LibraryServer) AddBookToShelf(
	ctx context.Context,
	req *errorhandlingpb.AddBookToShelfRequest,
) (*errorhandlingpb.AddBookToShelfResponse, error) {
	slog.Info("AddBookToShelf called", "shelf_id", req.ShelfId, "book_id", req.BookId)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if shelf exists
	shelf, exists := s.shelves[req.ShelfId]
	if !exists {
		return nil, reasonErr(
			fmt.Errorf("shelf %s not found", req.ShelfId),
			errorhandlingpb.Reason_SHELF_NOT_FOUND,
			map[string]string{"shelf_id": req.ShelfId},
		)
	}

	// Check if book exists
	if _, exists := s.books[req.BookId]; !exists {
		return nil, reasonErr(
			fmt.Errorf("book %s not found", req.BookId),
			errorhandlingpb.Reason_BOOK_NOT_FOUND,
			map[string]string{"book_id": req.BookId},
		)
	}

	// Check if shelf is full
	if shelf.CurrentCount >= shelf.Capacity {
		return nil, reasonErr(
			fmt.Errorf("shelf %s is full", req.ShelfId),
			errorhandlingpb.Reason_SHELF_FULL,
			map[string]string{
				"shelf_id":      req.ShelfId,
				"capacity":      fmt.Sprintf("%d", shelf.Capacity),
				"current_count": fmt.Sprintf("%d", shelf.CurrentCount),
			},
		)
	}

	// Add book to shelf
	shelf.CurrentCount++

	slog.Info("Book added to shelf successfully", "shelf_id", req.ShelfId, "book_id", req.BookId)
	return &errorhandlingpb.AddBookToShelfResponse{Success: true}, nil
}

func (s *LibraryServer) TriggerError(
	ctx context.Context,
	req *errorhandlingpb.TriggerErrorRequest,
) (*errorhandlingpb.TriggerErrorResponse, error) {
	slog.Info("TriggerError called", "error_type", req.ErrorType)

	switch req.ErrorType {
	case "database_error":
		return nil, reasonErr(
			errors.New("database connection failed"),
			errorhandlingpb.Reason_DATABASE_ERROR,
			map[string]string{
				"host":     "localhost:5432",
				"database": "library_db",
			},
		)
	case "network_error":
		return nil, reasonErr(
			errors.New("network timeout"),
			errorhandlingpb.Reason_NETWORK_ERROR,
			map[string]string{
				"target":  "external-api.example.com",
				"timeout": "30s",
			},
		)
	case "internal_error":
		return nil, reasonErr(
			errors.New("unexpected internal error"),
			errorhandlingpb.Reason_INTERNAL_ERROR,
			map[string]string{
				"component": "library-service",
				"version":   "1.0.0",
			},
		)
	default:
		return nil, xerror.Wrap(errors.New("unknown error type"), code.Code_INVALID_ARGUMENT, "")
	}
}

func main() {
	if err := config.Default().LoadLayer("example:file", config.PriorityFile, file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}

	ss := NewLibraryServer()
	err := yggdrasil.Run(
		context.Background(),
		func(yggdrasil.Runtime) (*yggdrasil.BusinessBundle, error) {
			return &yggdrasil.BusinessBundle{
				RPCBindings: []yggdrasil.RPCBinding{
					{
						ServiceName: errorhandlingpb.LibraryServiceServiceDesc.ServiceName,
						Desc:        &errorhandlingpb.LibraryServiceServiceDesc,
						Impl:        ss,
					},
				},
			}, nil
		},
		yggdrasil.WithAppName("github.com.codesjoy.yggdrasil.example.protogen.error-handling"),
	)
	if err != nil {
		os.Exit(1)
	}
}
