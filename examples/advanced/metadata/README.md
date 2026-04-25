# 元数据传递示例

本示例演示如何在 Yggdrasil 框架中使用元数据（metadata）在服务间传递上下文信息。

## 你会得到什么

- Request Metadata 的设置和读取
- Response Header 和 Trailer 的使用
- 跨服务元数据传递
- 自定义元数据拦截器

## 元数据类型

Yggdrasil 支持以下类型的元数据：

| 类型 | 说明 | 使用场景 |
|------|------|----------|
| Request Metadata | 请求头元数据 | 用户 ID、追踪 ID、认证信息 |
| Response Header | 响应头元数据 | 自定义响应头 |
| Response Trailer | 响应尾元数据 | 服务端处理后的附加信息 |

## Request Metadata

### 客户端设置 Request Metadata

```go
import "github.com/codesjoy/yggdrasil/v3/rpc/metadata"

ctx := metadata.NewContext(context.Background(),
    metadata.Pairs(
        "user-id", "12345",
        "trace-id", "abc-123",
        "authorization", "Bearer token",
    ),
)

resp, err := client.Call(ctx, req)
```

### 服务端读取 Request Metadata

```go
import "github.com/codesjoy/yggdrasil/v3/rpc/metadata"

func (s *Server) Method(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    md, ok := metadata.FromInContext(ctx)
    if !ok {
        slog.Warn("no metadata found")
    } else {
        userId := md["user-id"]
        traceId := md["trace-id"]
        slog.Info("received request", "userId", userId, "traceId", traceId)
    }
    
    return &pb.Response{}, nil
}
```

### 在中间件中读取 Metadata

```go
func MetadataInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    md, ok := metadata.FromInContext(ctx)
    if !ok {
        return handler(ctx, req)
    }
    
    slog.Info("request metadata",
        "method", info.FullMethod,
        "user-id", md["user-id"],
        "trace-id", md["trace-id"],
    )
    
    return handler(ctx, req)
}
```

## Response Header

### 服务端设置 Response Header

```go
func (s *Server) Method(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    _ = metadata.SetHeader(ctx, metadata.Pairs(
        "server-version", "1.0.0",
        "request-id", generateRequestId(),
    ))
    
    return &pb.Response{}, nil
}
```

### 客户端读取 Response Header

```go
ctx := metadata.WithStreamContext(context.Background())
resp, err := client.Call(ctx, req)

if header, ok := metadata.FromHeaderCtx(ctx); ok {
    slog.Info("response header", "header", header)
}
```

## Response Trailer

### 服务端设置 Response Trailer

```go
func (s *Server) Method(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    _ = metadata.SetTrailer(ctx, metadata.Pairs(
        "server", "my-server",
        "processing-time", "100ms",
    ))
    
    return &pb.Response{}, nil
}
```

### 客户端读取 Response Trailer

```go
ctx := metadata.WithStreamContext(context.Background())
resp, err := client.Call(ctx, req)

if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
    slog.Info("response trailer", "trailer", trailer)
}
```

## 跨服务元数据传递

### 传递追踪 ID

```go
func (s *OrderService) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.Order, error) {
    md, ok := metadata.FromInContext(ctx)
    if !ok {
        return nil, status.Error(codes.Internal, "missing metadata")
    }
    
    traceId := md["trace-id"]
    if len(traceId) == 0 {
        traceId = []string{generateTraceId()}
    }
    
    newCtx := metadata.NewContext(context.Background(),
        metadata.Pairs("trace-id", traceId[0]),
    )
    
    order, err := s.inventoryClient.CheckStock(newCtx, &pb.CheckStockRequest{
        ProductId: req.ProductId,
    })
    if err != nil {
        return nil, err
    }
    
    return s.createOrder(ctx, req)
}
```

### 传递用户上下文

```go
func (s *OrderService) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.Order, error) {
    md, ok := metadata.FromInContext(ctx)
    if !ok {
        return nil, status.Error(codes.Unauthenticated, "missing user context")
    }
    
    userId := md["user-id"]
    if len(userId) == 0 {
        return nil, status.Error(codes.Unauthenticated, "missing user-id")
    }
    
    ctx = context.WithValue(ctx, "userId", userId[0])
    
    return s.createOrder(ctx, req)
}
```

## 自定义元数据拦截器

### 自动添加元数据

```go
func TraceInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    md, ok := metadata.FromInContext(ctx)
    if !ok {
        md = metadata.Pairs("trace-id", generateTraceId())
        ctx = metadata.NewContext(ctx, md)
    } else if len(md["trace-id"]) == 0 {
        md["trace-id"] = []string{generateTraceId()}
        ctx = metadata.NewContext(ctx, md)
    }
    
    _ = metadata.SetHeader(ctx, metadata.Pairs(
        "server", os.Getenv("HOSTNAME"),
    ))
    
    return handler(ctx, req)
}
```

### 自动传递元数据

```go
func MetadataForwardingInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
    md, ok := metadata.FromInContext(ctx)
    if ok {
        ctx = metadata.NewOutgoingContext(ctx, md)
    }
    
    return invoker(ctx, method, req, reply, cc, opts...)
}
```

## 元数据最佳实践

### 1. 使用统一的元数据键名

定义常量避免硬编码：

```go
const (
    TraceIDKey = "trace-id"
    UserIDKey   = "user-id"
    AuthTokenKey = "authorization"
)
```

### 2. 验证必需的元数据

```go
func validateMetadata(ctx context.Context, requiredKeys []string) error {
    md, ok := metadata.FromInContext(ctx)
    if !ok {
        return status.Error(codes.InvalidArgument, "missing metadata")
    }
    
    for _, key := range requiredKeys {
        if len(md[key]) == 0 {
            return status.Errorf(codes.InvalidArgument, "missing required metadata: %s", key)
        }
    }
    
    return nil
}
```

### 3. 自动传递元数据

使用拦截器自动传递元数据：

```go
func MetadataForwardingInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
    md, ok := metadata.FromInContext(ctx)
    if ok {
        ctx = metadata.NewOutgoingContext(ctx, md)
    }
    
    return invoker(ctx, method, req, reply, cc, opts...)
}
```

### 4. 使用 Trailer 返回处理信息

```go
func (s *Server) ProcessRequest(ctx context.Context, req *pb.ProcessRequest) (*pb.ProcessResponse, error) {
    start := time.Now()
    
    resp, err := s.doProcess(ctx, req)
    
    _ = metadata.SetTrailer(ctx, metadata.Pairs(
        "processing-time", time.Since(start).String(),
        "server", s.hostname,
    ))
    
    return resp, err
}
```

### 5. 敏感信息处理

不要在元数据中传递敏感信息：

```go
ctx := metadata.NewContext(context.Background(),
    metadata.Pairs(
        "user-id", "12345",
    ),
)

ctx = context.WithValue(ctx, "user-token", "secret-token")
```

## 常见问题

**Q: 元数据会自动传递吗？**

A: 不会。需要显式地在调用链中传递：

```go
newCtx := metadata.NewContext(context.Background(),
    metadata.Pairs("trace-id", "abc-123"),
)
client.Call(newCtx, req)
```

**Q: 元数据大小有限制吗？**

A: 有。gRPC 元数据大小限制为 8KB。避免在元数据中传递大量数据。

**Q: 如何处理元数据不存在的情况？**

A: 使用 `FromInContext` 返回的 ok 值：

```go
md, ok := metadata.FromInContext(ctx)
if !ok {
    slog.Warn("no metadata found")
    return nil, status.Error(codes.InvalidArgument, "missing metadata")
}
```

**Q: 如何在流式 RPC 中使用元数据？**

A: 流式 RPC 的元数据只能在流开始时设置：

```go
ctx := metadata.NewContext(context.Background(),
    metadata.Pairs("user-id", "12345"),
)

stream, err := client.Method(ctx)
```

**Q: 如何区分不同服务的元数据？**

A: 使用前缀：

```go
const (
    UserPrefix    = "user-"
    ServicePrefix = "svc-"
)

md := metadata.Pairs(
    UserPrefix+"id", "12345",
    ServicePrefix+"version", "1.0.0",
)
```

**Q: 元数据是线程安全的吗？**

A: 元数据本身是只读的，可以安全地并发读取。但不要修改从 `FromInContext` 获取的 metadata。

## 相关文档

- [Yggdrasil 主文档](../../../README.md)
- [Sample Server 示例](../../sample/server/)
- [Sample Client 示例](../../sample/client/)
- [中间件示例](../middleware/)
- [流式通信示例](../streaming/)
