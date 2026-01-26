package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	_ "github.com/codesjoy/yggdrasil/v2/interceptor/logging"
	_ "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc"
)

type Book struct {
	Name   string `json:"name"`
	Author string `json:"author"`
	Title  string `json:"title"`
	Read   bool   `json:"read"`
}

type Shelf struct {
	Name  string `json:"name"`
	Theme string `json:"theme"`
}

type CreateShelfRequest struct {
	Shelf Shelf `json:"shelf"`
}

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}
	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.example.advanced.rest.client"); err != nil {
		os.Exit(1)
	}

	// Wait for REST server to be ready before starting tests
	slog.Info("Waiting for REST server to be ready...")
	if err := waitForServer("http://localhost:55887", 10, 1*time.Second); err != nil {
		slog.Error("server not ready", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("=== Testing REST API ===")

	slog.Info("=== 1. Testing Create Shelf ===")
	if err := testCreateShelf(); err != nil {
		slog.Error("create shelf test failed", slog.Any("error", err))
	}

	slog.Info("=== 2. Testing Get Shelf ===")
	shelfName := "shelves/test-1"
	if err := testGetShelf(shelfName); err != nil {
		slog.Error("get shelf test failed", slog.Any("error", err))
	}

	slog.Info("=== 3. Testing List Shelves ===")
	if err := testListShelves(); err != nil {
		slog.Error("list shelves test failed", slog.Any("error", err))
	}

	slog.Info("=== 4. Testing Create Book ===")
	if err := testCreateBook(shelfName); err != nil {
		slog.Error("create book test failed", slog.Any("error", err))
	}

	slog.Info("=== 5. Testing Get Book ===")
	bookName := "books/test-1"
	if err := testGetBook(bookName); err != nil {
		slog.Error("get book test failed", slog.Any("error", err))
	}

	slog.Info("=== 6. Testing List Books ===")
	if err := testListBooks(shelfName); err != nil {
		slog.Error("list books test failed", slog.Any("error", err))
	}

	slog.Info("=== 7. Testing Update Book ===")
	if err := testUpdateBook(bookName); err != nil {
		slog.Error("update book test failed", slog.Any("error", err))
	}

	slog.Info("=== 8. Testing Delete Book ===")
	if err := testDeleteBook(bookName); err != nil {
		slog.Error("delete book test failed", slog.Any("error", err))
	}

	slog.Info("=== 9. Testing Delete Shelf ===")
	if err := testDeleteShelf(shelfName); err != nil {
		slog.Error("delete shelf test failed", slog.Any("error", err))
	}

	slog.Info("All REST API tests completed successfully!")
}

// waitForServer waits for the REST server to be ready by polling the health endpoint
func waitForServer(url string, maxAttempts int, interval time.Duration) error {
	client := &http.Client{Timeout: 1 * time.Second}
	for i := 0; i < maxAttempts; i++ {
		resp, err := client.Get(url + "/v1/shelves")
		if err == nil {
			resp.Body.Close()
			slog.Info("Server is ready")
			return nil
		}
		slog.Info("Waiting for server to be ready...", "attempt", i+1, "max", maxAttempts)
		time.Sleep(interval)
	}
	return fmt.Errorf("server not ready after %d attempts", maxAttempts)
}

func testCreateShelf() error {
	req := CreateShelfRequest{
		Shelf: Shelf{
			Name:  "shelves/test-1",
			Theme: "Fiction",
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest("POST", "http://localhost:55887/v1/shelves", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var shelf Shelf
	if err := json.NewDecoder(resp.Body).Decode(&shelf); err != nil {
		return err
	}

	slog.Info("Created shelf", "shelf", shelf)
	return nil
}

func testGetShelf(name string) error {
	resp, err := http.Get(fmt.Sprintf("http://localhost:55887/v1/%s", name))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var shelf Shelf
	if err := json.NewDecoder(resp.Body).Decode(&shelf); err != nil {
		return err
	}

	slog.Info("Got shelf", "shelf", shelf)
	return nil
}

func testListShelves() error {
	resp, err := http.Get("http://localhost:55887/v1/shelves")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Shelves []Shelf `json:"shelves"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	slog.Info("Listed shelves", "count", len(result.Shelves))
	return nil
}

func testCreateBook(parent string) error {
	req := struct {
		Book Book `json:"book"`
	}{
		Book: Book{
			Name:   "books/test-1",
			Author: "Test Author",
			Title:  "Test Book",
			Read:   true,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:55887/v1/%s/books", parent), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var book Book
	if err := json.NewDecoder(resp.Body).Decode(&book); err != nil {
		return err
	}

	slog.Info("Created book", "book", book)
	return nil
}

func testGetBook(name string) error {
	resp, err := http.Get(fmt.Sprintf("http://localhost:55887/v1/%s", name))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var book Book
	if err := json.NewDecoder(resp.Body).Decode(&book); err != nil {
		return err
	}

	slog.Info("Got book", "book", book)
	return nil
}

func testListBooks(parent string) error {
	resp, err := http.Get(fmt.Sprintf("http://localhost:55887/v1/%s/books", parent))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Books []Book `json:"books"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	slog.Info("Listed books", "count", len(result.Books))
	return nil
}

func testUpdateBook(name string) error {
	req := struct {
		Book Book `json:"book"`
	}{
		Book: Book{
			Name:   name,
			Author: "Updated Author",
			Title:  "Updated Title",
			Read:   false,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest("PATCH", fmt.Sprintf("http://localhost:55887/v1/%s", name), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var book Book
	if err := json.NewDecoder(resp.Body).Decode(&book); err != nil {
		return err
	}

	slog.Info("Updated book", "book", book)
	return nil
}

func testDeleteBook(name string) error {
	httpReq, err := http.NewRequest("DELETE", fmt.Sprintf("http://localhost:55887/v1/%s", name), nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	slog.Info("Deleted book", "name", name)
	return nil
}

func testDeleteShelf(name string) error {
	httpReq, err := http.NewRequest("DELETE", fmt.Sprintf("http://localhost:55887/v1/%s", name), nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	slog.Info("Deleted shelf", "name", name)
	return nil
}
