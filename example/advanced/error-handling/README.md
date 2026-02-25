# Error Handling Example

This example demonstrates comprehensive error handling in Yggdrasil using the Reason system.

## Overview

This example shows how to:
- Define Reason enums in protobuf
- Use `protoc-gen-codesjoy-reason` to generate error handling code
- Return structured errors with metadata from the server using `xerror.WrapWithReason()`
- Parse and handle errors on the client using `status.FromError()`
- Implement retry mechanisms with exponential backoff

## Architecture

The example implements a simple library management service with the following error scenarios:

### User Errors (4xx)
- `USER_NOT_FOUND` - User does not exist
- `INVALID_CREDENTIALS` - Authentication failed
- `INVALID_INPUT` - Invalid request parameters
- `EMAIL_ALREADY_EXISTS` - Email already registered

### Resource Errors (4xx)
- `BOOK_NOT_FOUND` - Book does not exist
- `SHELF_NOT_FOUND` - Shelf does not exist
- `BOOK_ALREADY_BORROWED` - Book is already borrowed
- `SHELF_FULL` - Shelf has reached capacity

### System Errors (5xx)
- `INTERNAL_ERROR` - Unexpected internal error
- `DATABASE_ERROR` - Database operation failed
- `NETWORK_ERROR` - Network operation failed

## Error Handling Core Concepts

### Reason System

The Reason system provides:
1. **Structured error codes** - Enum values for each error type
2. **gRPC status code mapping** - Automatic mapping to gRPC codes (NOT_FOUND, INVALID_ARGUMENT, etc.)
3. **Error metadata propagation** - Attach context to errors for debugging
4. **HTTP status code mapping** - Automatic conversion for REST endpoints

### Error API

**Server-side:**
- `xerror.WrapWithReason(err, reason, msg, metadata)` - Create structured error with reason and metadata (`reason` implements `xerror.Reason`)
- `xerror.Wrap(err, code, msg)` - Wrap with gRPC code
- `xerror.New(code, msg)` - Create coded error

**Client-side:**
- `status.FromError(err)` - Parse transport error into `Status`
- `st.Code()` - Get gRPC status code
- `st.HTTPCode()` - Get HTTP status code
- `st.ErrorInfo().Reason/Domain/Metadata` - Get reason details and metadata

## Running the Example

### 1. Generate Code

```bash
cd example/proto/error-handling
make generate
```

This generates:
- `reason.pb.go` - Reason enum definitions
- `reason_reason.pb.go` - Reason methods and gRPC code mappings
- `service.pb.go` - Message definitions
- `service_rpc.pb.go` - Service client and server interfaces

### 2. Start Server

Terminal 1:
```bash
cd example/protogen/error-handling/server
go run main.go
```

The server will start on `127.0.0.1:55884`.

### 3. Run Client

Terminal 2:
```bash
cd example/protogen/error-handling/client
go run main.go
```

The client will run all 12 test scenarios and display the results.

## Expected Output

### Test 1: Successful CreateBook
```
✓ Book created successfully
```

### Test 2: USER_NOT_FOUND
```
✓ Correctly identified USER_NOT_FOUND
grpc_code: NOT_FOUND
http_code: 404
message: "user non-existent-user not found"
metadata: {user_id: "non-existent-user"}
```

### Test 3: INVALID_INPUT
```
✓ Correctly identified INVALID_INPUT
grpc_code: INVALID_ARGUMENT
http_code: 400
message: "invalid email format"
metadata: {field: "email", value: "invalid-email"}
```

### Test 4: INVALID_CREDENTIALS
```
✓ Correctly identified INVALID_CREDENTIALS
grpc_code: UNAUTHENTICATED
http_code: 401
message: "invalid credentials"
metadata: {email: "test@example.com"}
```

### Test 5: EMAIL_ALREADY_EXISTS
```
✓ Correctly identified EMAIL_ALREADY_EXISTS
grpc_code: ALREADY_EXISTS
http_code: 409
message: "email duplicate@example.com already registered"
metadata: {email: "duplicate@example.com"}
```

### Test 6: BOOK_NOT_FOUND
```
✓ Correctly identified BOOK_NOT_FOUND
grpc_code: NOT_FOUND
http_code: 404
message: "book non-existent-book not found"
metadata: {book_id: "non-existent-book"}
```

### Test 7: BOOK_ALREADY_BORROWED
```
✓ Correctly identified BOOK_ALREADY_BORROWED
grpc_code: FAILED_PRECONDITION
http_code: 400
message: "book is already borrowed"
metadata: {book_id: "...", borrower_id: "..."}
```

### Test 8: SHELF_FULL
```
✓ Correctly identified SHELF_FULL
grpc_code: RESOURCE_EXHAUSTED
http_code: 429
message: "shelf shelf-X is full"
metadata: {shelf_id: "...", capacity: "1", current_count: "1"}
```

### Test 9: DATABASE_ERROR
```
✓ Correctly identified DATABASE_ERROR
grpc_code: UNAVAILABLE
http_code: 503
message: "database connection failed"
metadata: {host: "localhost:5432", database: "library_db"}
```

### Test 10: NETWORK_ERROR
```
✓ Correctly identified NETWORK_ERROR
grpc_code: UNAVAILABLE
http_code: 503
message: "network timeout"
metadata: {target: "external-api.example.com", timeout: "30s"}
```

### Test 11: INTERNAL_ERROR
```
✓ Correctly identified INTERNAL_ERROR
grpc_code: INTERNAL
http_code: 500
message: "unexpected internal error"
metadata: {component: "library-service", version: "1.0.0"}
```

### Test 12: Retry Mechanism
```
✓ Retry mechanism test completed
attempts: 1
result: all retries exhausted (as expected for non-retryable error)
```

## Server Error Handling Patterns

### 1. Parameter Validation

```go
if req.Email == "" || !strings.Contains(req.Email, "@") {
    return nil, xerror.WrapWithReason(
        errors.New("invalid email format"),
        errorhandlingpb.Reason_INVALID_INPUT,
        "",
        map[string]string{"field": "email", "value": req.Email},
    )
}
```

### 2. Resource Not Found

```go
if !s.userExists(req.UserId) {
    return nil, xerror.WrapWithReason(
        fmt.Errorf("user %s not found", req.UserId),
        errorhandlingpb.Reason_USER_NOT_FOUND,
        "",
        map[string]string{"user_id": req.UserId},
    )
}
```

### 3. Business Logic Errors

```go
if book.BorrowerId != "" {
    return nil, xerror.WrapWithReason(
        errors.New("book is already borrowed"),
        errorhandlingpb.Reason_BOOK_ALREADY_BORROWED,
        "",
        map[string]string{
            "book_id": req.BookId,
            "borrower_id": book.BorrowerId,
        },
    )
}
```

### 4. System Errors

```go
return nil, xerror.WrapWithReason(
    errors.New("database connection failed"),
    errorhandlingpb.Reason_DATABASE_ERROR,
    "",
    map[string]string{"host": "localhost:5432"},
)
```

## Client Error Handling Patterns

### 1. Error Parsing

```go
_, err := client.GetUser(ctx, &pb.GetUserRequest{UserId: "123"})
if err != nil {
    st := status.FromError(err)
    slog.Info("Error details",
        "code", st.Code(),
        "http_code", st.HTTPCode(),
        "message", st.Message(),
    )
}
```

### 2. Reason Checking

```go
if info := st.ErrorInfo(); info != nil &&
    info.Reason == pb.Reason_USER_NOT_FOUND.Reason() &&
    info.Domain == pb.Reason_USER_NOT_FOUND.Domain() {
    slog.Info("✓ Correctly identified USER_NOT_FOUND")
    // Handle user not found
}
```

### 3. Metadata Extraction

```go
if st.ErrorInfo() != nil {
    slog.Info("Error metadata", "metadata", st.ErrorInfo().Metadata)
}
```

### 4. Retry Mechanism

```go
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
            time.Sleep(backoff)
            continue
        }

        return err
    }

    return fmt.Errorf("max retries reached: %w", lastErr)
}

func isRetryable(st *status.Status) bool {
    switch st.Code() {
    case code.Code_DEADLINE_EXCEEDED, code.Code_UNAVAILABLE, code.Code_ABORTED:
        return true
    default:
        return false
    }
}
```

## Common Pitfalls

### 1. Forgetting to Check Error Info

❌ **Bad:**
```go
if err != nil {
    slog.Error("Error", "error", err)
}
```

✅ **Good:**
```go
if err != nil {
    st := status.FromError(err)
    if st.ErrorInfo() != nil {
        slog.Error("Error", "metadata", st.ErrorInfo().Metadata)
    }
}
```

### 2. Not Using Reason Checking

❌ **Bad:**
```go
if err != nil && strings.Contains(err.Error(), "not found") {
    // Fragile string matching
}
```

✅ **Good:**
```go
if info := st.ErrorInfo(); info != nil &&
    info.Reason == pb.Reason_USER_NOT_FOUND.Reason() &&
    info.Domain == pb.Reason_USER_NOT_FOUND.Domain() {
    // Reliable reason checking after status.FromError(err)
}
```

### 3. Missing Metadata

❌ **Bad:**
```go
return nil, xerror.WrapWithReason(
    errors.New("user not found"),
    pb.Reason_USER_NOT_FOUND,
    "",
    nil, // Missing context!
)
```

✅ **Good:**
```go
return nil, xerror.WrapWithReason(
    errors.New("user not found"),
    pb.Reason_USER_NOT_FOUND,
    "",
    map[string]string{"user_id": req.UserId}, // Include context
)
```

### 4. Retry Non-Retryable Errors

❌ **Bad:**
```go
// Retrying all errors
for i := 0; i < 3; i++ {
    err := fn()
    if err == nil {
        return nil
    }
    time.Sleep(time.Second)
}
```

✅ **Good:**
```go
// Only retry retryable errors
for i := 0; i < 3; i++ {
    err := fn()
    if err == nil {
        return nil
    }
    st := status.FromError(err)
    if !isRetryable(st) {
        return err
    }
    time.Sleep(time.Second)
}
```

## Best Practices

1. **Always use Reason enums** - Define all error cases in protobuf
2. **Include metadata** - Provide context for debugging (IDs, field names, etc.)
3. **Check specific reasons** - Parse by `status.FromError(err)` and compare `ErrorInfo().Reason/Domain`
4. **Implement retry logic** - Use exponential backoff for retryable errors
5. **Log error details** - Include code, message, and metadata in logs
6. **Handle non-retryable errors** - Return immediately for client errors (4xx)
7. **Test all error scenarios** - Ensure all reasons are handled correctly

## File Structure

```
example/
├── proto/
│   └── error-handling/
│       ├── Makefile
│       ├── reason.proto          # Reason enum definitions
│       └── service.proto         # Service definition
└── protogen/
    └── error-handling/
        ├── README.md             # This file
        ├── reason.pb.go          # Generated enum
        ├── reason_reason.pb.go   # Generated methods
        ├── service.pb.go         # Generated messages
        ├── service_rpc.pb.go     # Generated service
        ├── server/
        │   ├── main.go           # Server implementation
        │   └── config.yaml       # Server config
        └── client/
            ├── main.go           # Client implementation
            └── config.yaml       # Client config
```

## Summary

This example demonstrates a complete error handling workflow in Yggdrasil:
- ✅ Define Reason enums in protobuf
- ✅ Generate error handling code with protoc plugins
- ✅ Return structured errors with metadata from server
- ✅ Parse and handle errors on client
- ✅ Implement retry mechanisms with exponential backoff
- ✅ Handle different error types (user, resource, system errors)
