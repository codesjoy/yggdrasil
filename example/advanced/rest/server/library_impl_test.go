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
	"strings"
	"testing"

	libraryv1 "github.com/codesjoy/yggdrasil/v2/example/protogen/library/v1"
)

func TestLibraryImplMethods(t *testing.T) {
	t.Parallel()

	svc := &LibraryImpl{}
	ctx := context.Background()

	t.Run("CreateShelf", func(t *testing.T) {
		resp, err := svc.CreateShelf(ctx, &libraryv1.CreateShelfRequest{
			Shelf: &libraryv1.Shelf{Theme: "Fiction"},
		})
		if err != nil {
			t.Fatalf("CreateShelf() error = %v", err)
		}
		if !strings.HasPrefix(resp.Name, "shelves/") {
			t.Fatalf("CreateShelf() name = %q, want prefix shelves/", resp.Name)
		}
		if resp.Theme != "Fiction" {
			t.Fatalf("CreateShelf() theme = %q, want Fiction", resp.Theme)
		}
	})

	t.Run("GetAndListShelf", func(t *testing.T) {
		getResp, err := svc.GetShelf(ctx, &libraryv1.GetShelfRequest{Name: "shelves/100"})
		if err != nil {
			t.Fatalf("GetShelf() error = %v", err)
		}
		if getResp.Name != "shelves/100" || getResp.Theme != "Sample Theme" {
			t.Fatalf("GetShelf() = %+v, want name shelves/100 and theme Sample Theme", getResp)
		}

		listResp, err := svc.ListShelves(ctx, &libraryv1.ListShelvesRequest{})
		if err != nil {
			t.Fatalf("ListShelves() error = %v", err)
		}
		if len(listResp.Shelves) != 3 {
			t.Fatalf("ListShelves() len = %d, want 3", len(listResp.Shelves))
		}
	})

	t.Run("DeleteAndMergeShelf", func(t *testing.T) {
		delResp, err := svc.DeleteShelf(ctx, &libraryv1.DeleteShelfRequest{Name: "shelves/100"})
		if err != nil {
			t.Fatalf("DeleteShelf() error = %v", err)
		}
		if delResp == nil {
			t.Fatal("DeleteShelf() response is nil")
		}

		mergeResp, err := svc.MergeShelves(
			ctx,
			&libraryv1.MergeShelvesRequest{Name: "shelves/merged"},
		)
		if err != nil {
			t.Fatalf("MergeShelves() error = %v", err)
		}
		if mergeResp.Name != "shelves/merged" || mergeResp.Theme != "Merged Theme" {
			t.Fatalf("MergeShelves() = %+v, want merged shelf response", mergeResp)
		}
	})

	t.Run("CreateGetListBook", func(t *testing.T) {
		createResp, err := svc.CreateBook(ctx, &libraryv1.CreateBookRequest{
			Parent: "shelves/1",
			Book: &libraryv1.Book{
				Author: "Author A",
				Title:  "Title A",
				Read:   true,
			},
		})
		if err != nil {
			t.Fatalf("CreateBook() error = %v", err)
		}
		if !strings.HasPrefix(createResp.Name, "shelves/1/books/") {
			t.Fatalf("CreateBook() name = %q, want prefix shelves/1/books/", createResp.Name)
		}
		if createResp.Author != "Author A" || createResp.Title != "Title A" || !createResp.Read {
			t.Fatalf("CreateBook() = %+v, fields do not match request", createResp)
		}

		getResp, err := svc.GetBook(ctx, &libraryv1.GetBookRequest{Name: "shelves/1/books/1"})
		if err != nil {
			t.Fatalf("GetBook() error = %v", err)
		}
		if getResp.Name != "shelves/1/books/1" || getResp.Author != "Sample Author" {
			t.Fatalf("GetBook() = %+v, want sample book", getResp)
		}

		listResp, err := svc.ListBooks(ctx, &libraryv1.ListBooksRequest{Parent: "shelves/1"})
		if err != nil {
			t.Fatalf("ListBooks() error = %v", err)
		}
		if len(listResp.Books) != 3 {
			t.Fatalf("ListBooks() len = %d, want 3", len(listResp.Books))
		}
	})

	t.Run("UpdateMoveDeleteBook", func(t *testing.T) {
		updateResp, err := svc.UpdateBook(ctx, &libraryv1.UpdateBookRequest{
			Book: &libraryv1.Book{
				Name:   "shelves/1/books/9",
				Author: "Updated Author",
				Title:  "Updated Title",
				Read:   true,
			},
		})
		if err != nil {
			t.Fatalf("UpdateBook() error = %v", err)
		}
		if updateResp.Name != "shelves/1/books/9" || updateResp.Author != "Updated Author" ||
			!updateResp.Read {
			t.Fatalf("UpdateBook() = %+v, want updated fields", updateResp)
		}

		moveResp, err := svc.MoveBook(ctx, &libraryv1.MoveBookRequest{
			Name:           "shelves/1/books/9",
			OtherShelfName: "shelves/2",
		})
		if err != nil {
			t.Fatalf("MoveBook() error = %v", err)
		}
		if moveResp.Name != "shelves/1/books/9" || moveResp.Author != "Moved Author" ||
			moveResp.Read {
			t.Fatalf("MoveBook() = %+v, want moved defaults", moveResp)
		}

		delResp, err := svc.DeleteBook(ctx, &libraryv1.DeleteBookRequest{Name: "shelves/1/books/9"})
		if err != nil {
			t.Fatalf("DeleteBook() error = %v", err)
		}
		if delResp == nil {
			t.Fatal("DeleteBook() response is nil")
		}
	})
}
