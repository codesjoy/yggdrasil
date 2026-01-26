# 错误处理示例

本示例演示如何在 Yggdrasil 框架中处理错误，包括自定义错误 reason、错误传播和重试机制。

## 你会得到什么

- 自定义错误 reason 的定义和使用
- 错误传播和包装
- 错误码映射
- 重试机制
- 错误恢复

## 错误处理核心概念

### 1. 错误 Reason

Yggdrasil 使用 `reason` 来表示业务错误类型，比传统的错误码更具语义化。

**定义 Reason**:

```protobuf
enum Reason {
  REASON_UNSPECIFIED = 0;
  USER_NOT_FOUND = 1;
  BOOK_NOT_FOUND = 2;
  INVALID_INPUT = 3;
  INTERNAL_ERROR = 4;
}
```

**使用 Reason**:

```go
import "github.com/codesjoy/yggdrasil/v2/status"

return nil, status.FromReason(
    errors.New("user not found"),
    librarypb.Reason_USER_NOT_FOUND,
    nil,
)
```

### 2. 错误码映射

Yggdrasil 自动将错误 reason 映射到 gRPC 错误码和 HTTP 状态码：

| Reason | gRPC Code | HTTP Code |
|--------|-----------|------------|
| USER_NOT_FOUND | NOT_FOUND (5) | 404 |
| INVALID_INPUT | INVALID_ARGUMENT (3) | 400 |
| INTERNAL_ERROR | INTERNAL (13) | 500 |

### 3. 错误传播

错误从服务端传播到客户端时，保持原始错误信息：

**服务端**:

```go
func (s *Server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    user, err := s.db.GetUser(req.Id)
    if err != nil {
        if errors.Is(err, ErrUserNotFound) {
            return nil, status.FromReason(
                err,
                pb.Reason_USER_NOT_FOUND,
                nil,
            )
        }
        return nil, status.FromError(err)
    }
    return user, nil
}
```

**客户端**:

```go
resp, err := client.GetUser(ctx, &pb.GetUserRequest{Id: "123"})
if err != nil {
    st := status.FromError(err)
    
    if st.ErrorInfo().Reason == pb.Reason_USER_NOT_FOUND {
        slog.Info("user not found", "id", "123")
        return nil
    }
    
    slog.Error("failed to get user", slog.Any("error", err))
    return err
}
```

## 错误处理模式

### 1. 基本错误处理

**服务端**:

```go
func (s *Server) CreateBook(ctx context.Context, req *pb.CreateBookRequest) (*pb.Book, error) {
    if req.Book.Title == "" {
        return nil, status.FromReason(
            errors.New("book title is required"),
            pb.Reason_INVALID_INPUT,
            nil,
        )
    }
    
    book, err := s.db.CreateBook(req.Book)
    if err != nil {
        return nil, status.FromError(err)
    }
    
    return book, nil
}
```

### 2. 错误包装

使用 `fmt.Errorf` 包装错误，保留原始错误信息：

```go
func (s *Server) CreateBook(ctx context.Context, req *pb.CreateBookRequest) (*pb.Book, error) {
    book, err := s.db.CreateBook(req.Book)
    if err != nil {
        return nil, status.FromError(fmt.Errorf("failed to create book: %w", err))
    }
    return book, nil
}
```

### 3. 错误恢复

使用 `recover()` 捕获 panic，返回错误：

```go
func (s *Server) ProcessData(ctx context.Context, req *pb.ProcessDataRequest) (*pb.ProcessDataResponse, error) {
    defer func() {
        if r := recover(); r != nil {
            slog.Error("panic recovered", "error", r)
        }
    }()
    
    result := s.process(req.Data)
    return &pb.ProcessDataResponse{Result: result}, nil
}
```

### 4. 重试机制

**指数退避重试**:

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

func isRetryable(st *status.Status) bool {
    code := st.Code()
    
    switch code {
    case codes.DeadlineExceeded, codes.Unavailable, codes.Aborted:
        return true
    default:
        return false
    }
}
```

**使用重试**:

```go
err := retryWithBackoff(func() error {
    _, err := client.Call(ctx, req)
    return err
}, 3)
```

### 5. 错误聚合

聚合多个错误：

```go
type MultiError struct {
    Errors []error
}

func (e *MultiError) Error() string {
    var sb strings.Builder
    for i, err := range e.Errors {
        if i > 0 {
            sb.WriteString("; ")
        }
        sb.WriteString(err.Error())
    }
    return sb.String()
}

func (e *MultiError) Add(err error) {
    if err != nil {
        e.Errors = append(e.Errors, err)
    }
}

func (e *MultiError) ToStatus() error {
    if len(e.Errors) == 0 {
        return nil
    }
    
    return status.FromReason(e, pb.Reason_INTERNAL_ERROR, nil)
}
```

## 错误最佳实践

### 1. 定义清晰的错误 Reason

```protobuf
enum Reason {
  REASON_UNSPECIFIED = 0;
  
  User errors (4xx)
  USER_NOT_FOUND = 1;
  INVALID_CREDENTIALS = 2;
  INVALID_INPUT = 3;
  
  Resource errors (4xx)
  BOOK_NOT_FOUND = 10;
  BOOK_ALREADY_EXISTS = 11;
  
  System errors (5xx)
  INTERNAL_ERROR = 100;
  DATABASE_ERROR = 101;
  NETWORK_ERROR = 102;
}
```

### 2. 错误日志记录

记录足够的错误信息，但不记录敏感数据：

```go
func (s *Server) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
    if req.Email == "" {
        slog.Warn("invalid input", "field", "email")
        return nil, status.FromReason(
            errors.New("email is required"),
            pb.Reason_INVALID_INPUT,
            nil,
        )
    }
    
    user, err := s.db.CreateUser(req.User)
    if err != nil {
        slog.Error("failed to create user", "email", req.Email, "error", err)
        return nil, status.FromError(err)
    }
    
    return user, nil
}
```

### 3. 错误上下文

添加错误上下文信息：

```go
func (s *Server) GetBook(ctx context.Context, req *pb.GetBookRequest) (*pb.Book, error) {
    book, err := s.db.GetBook(req.Id)
    if err != nil {
        if errors.Is(err, ErrBookNotFound) {
            return nil, status.FromReason(
                fmt.Errorf("book not found: %s", req.Id),
                pb.Reason_BOOK_NOT_FOUND,
                map[string]string{"book_id": req.Id},
            )
        }
        return nil, status.FromError(err)
    }
    return book, nil
}
```

### 4. 错误恢复

在关键路径上添加 panic 恢复：

```go
func (s *Server) StreamBooks(req *pb.StreamBooksRequest, stream pb.LibraryService_StreamBooksServer) error {
    defer func() {
        if r := recover(); r != nil {
            slog.Error("panic in StreamBooks", "error", r)
            _ = stream.Send(&pb.Book{Error: "internal error"})
        }
    }()
    
    for book := range s.db.ListBooks() {
        if err := stream.Send(book); err != nil {
            return err
        }
    }
    return nil
}
```

## 常见问题

**Q: 如何选择合适的错误码？**

A: 遵循以下原则：
- 4xx 错误：客户端错误（如 INVALID_ARGUMENT、NOT_FOUND）
- 5xx 错误：服务端错误（如 INTERNAL、UNAVAILABLE）
- 使用具体的错误码，避免使用 UNKNOWN

**Q: 什么时候使用自定义 reason？**

A: 当需要区分业务逻辑错误时使用自定义 reason，例如：
- `USER_NOT_FOUND` vs `BOOK_NOT_FOUND`
- `INVALID_CREDENTIALS` vs `INVALID_INPUT`

**Q: 如何处理错误详情？**

A: 使用 `st.Details()` 获取错误详情：

```go
st := status.FromError(err)
if details := st.Details(); len(details) > 0 {
    for _, detail := range details {
        if d, ok := detail.(*pb.ErrorDetail); ok {
            slog.Info("error detail", "field", d.Field, "message", d.Message)
        }
    }
}
```

**Q: 如何实现重试？**

A: 使用指数退避算法：

```go
for i := 0; i < maxRetries; i++ {
    err := doRequest()
    if err == nil {
        return nil
    }
    
    if !isRetryable(err) {
        return err
    }
    
    time.Sleep(time.Duration(math.Pow(2, float64(i))) * time.Second)
}
```

**Q: 如何处理并发错误？**

A: 使用 errgroup 或自定义错误聚合：

```go
var g errgroup.Group
var mu sync.Mutex
var errs []error

g.Go(func() error {
    if err := doWork1(); err != nil {
        mu.Lock()
        errs = append(errs, err)
        mu.Unlock()
    }
    return nil
})

g.Wait()
```

**Q: 错误日志应该记录什么？**

A: 记录以下信息：
- 错误类型和消息
- 请求参数（脱敏）
- 错误堆栈（对于内部错误）
- 相关的上下文信息

## 相关文档

- [Yggdrasil 主文档](../../../README.md)
- [Sample Server 示例](../../sample/server/)
- [Sample Client 示例](../../sample/client/)
- [中间件示例](../middleware/)
