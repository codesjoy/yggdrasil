# Sample Client - 客户端示例

本示例演示如何使用 Yggdrasil 框架构建 gRPC 客户端，包括服务调用、元数据传递和错误处理。

## 你会得到什么

- 完整的 gRPC 客户端实现
- 元数据传递（Header 和 Trailer）
- 错误处理和错误 reason 提取
- 日志拦截器

## 前置条件

- Go 1.24 或更高版本
- 已安装 Yggdrasil 框架
- 已生成 protobuf 代码（在 `../protogen/` 目录）
- 服务端已启动（见 [server/README.md](../server/README.md)）

## 启动方式

### 1. 确保服务端已运行

首先在另一个终端启动服务端：

```bash
cd example/sample/server
go run main.go
```

### 2. 进入客户端目录

```bash
cd example/sample/client
```

### 3. 运行客户端

```bash
go run main.go
```

## 预期输出

```
trailer: map[server:sample-server]
header: map[server:sample-server]
Reason: book_not_found
Code: 5
HttpCode: 404
call success
```

## 客户端功能

### 调用的 RPC 方法

| 方法 | 说明 |
|------|------|
| GetShelf | 获取书架信息 |
| MoveBook | 移动书籍（演示错误处理） |

### 错误处理

- 提取错误 reason
- 获取错误码
- 获取 HTTP 状态码

### 元数据传递

- 读取 Response Header
- 读取 Response Trailer

## 配置说明

### config.yaml 配置文件

```yaml
yggdrasil:
  application:
    namespace: "dev"
  client:
    github.com.codesjoy.yggdrasil.example.sample:
      remote:
        endpoints:
          - address: "127.0.0.1:55879"
            protocol: "grpc"
  interceptor:
    unary_client: "logging"
    stream_client: "logging"
    config:
      logging:
        print_req_and_res: true

  remote:
    logger_level: debug
```

### 配置字段说明

| 字段 | 说明 |
|------|------|
| `application.namespace` | 应用命名空间 |
| `client.<service>.remote.endpoints` | 服务端点列表 |
| `interceptor.unary_client` | 一元 RPC 拦截器 |
| `interceptor.stream_client` | 流式 RPC 拦截器 |
| `remote.logger_level` | 远程通信日志级别 |

## 代码结构说明

```go
func main() {
    if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
        slog.Error("failed to load config file", slog.Any("error", err))
        os.Exit(1)
    }
    
    if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.example.sample.client"); err != nil {
        os.Exit(1)
    }
    
    cli, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.sample")
    if err != nil {
        os.Exit(1)
    }
    defer cli.Close()

    client := librarypb.NewLibraryServiceClient(cli)
    
    // 1. 调用 GetShelf（成功示例）
    ctx := metadata.WithStreamContext(context.TODO())
    _, err = client.GetShelf(ctx, &librarypb.GetShelfRequest{Name: "fdasf"})
    if err != nil {
        slog.Error("fault to call GetShelf", slog.Any("error", err))
        os.Exit(1)
    }
    
    // 2. 读取元数据
    if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
        fmt.Println(trailer)
    }
    if header, ok := metadata.FromHeaderCtx(ctx); ok {
        fmt.Println(header)
    }
    
    // 3. 调用 MoveBook（错误示例）
    _, err = client.MoveBook(context.TODO(), &librarypb.MoveBookRequest{Name: "fdasf"})
    if err != nil {
        st := status.FromError(err)
        fmt.Println("Reason:", st.ErrorInfo().Reason)
        fmt.Println("Code:", st.Code())
        fmt.Println("HttpCode:", st.HTTPCode())
    }
    
    slog.Info("call success")
}
```

## 使用示例

### 创建客户端

```go
cli, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.sample")
if err != nil {
    slog.Error("failed to create client", slog.Any("error", err))
    return
}
defer cli.Close()
```

### 创建服务客户端

```go
client := librarypb.NewLibraryServiceClient(cli)
```

### 调用 RPC 方法（带元数据）

```go
ctx := metadata.WithStreamContext(context.TODO())
resp, err := client.GetShelf(ctx, &librarypb.GetShelfRequest{
    Name: "shelves/1",
})
if err != nil {
    slog.Error("call failed", slog.Any("error", err))
    return
}
```

### 读取元数据

```go
if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
    fmt.Println("Trailer:", trailer)
}
if header, ok := metadata.FromHeaderCtx(ctx); ok {
    fmt.Println("Header:", header)
}
```

### 错误处理

```go
_, err = client.MoveBook(context.TODO(), &librarypb.MoveBookRequest{Name: "test"})
if err != nil {
    st := status.FromError(err)
    
    fmt.Println("Reason:", st.ErrorInfo().Reason)
    fmt.Println("Code:", st.Code())
    fmt.Println("HttpCode:", st.HTTPCode())
    fmt.Println("Message:", st.Message())
}
```

## 错误码说明

Yggdrasil 使用标准的 gRPC 错误码：

| Code | 数值 | 说明 |
|------|------|------|
| OK | 0 | 成功 |
| CANCELLED | 1 | 操作被取消 |
| UNKNOWN | 2 | 未知错误 |
| INVALID_ARGUMENT | 3 | 无效参数 |
| DEADLINE_EXCEEDED | 4 | 超时 |
| NOT_FOUND | 5 | 资源未找到 |
| ALREADY_EXISTS | 6 | 资源已存在 |
| PERMISSION_DENIED | 7 | 权限不足 |
| RESOURCE_EXHAUSTED | 8 | 资源耗尽 |
| FAILED_PRECONDITION | 9 | 前置条件失败 |
| ABORTED | 10 | 操作中止 |
| OUT_OF_RANGE | 11 | 超出范围 |
| UNIMPLEMENTED | 12 | 未实现 |
| INTERNAL | 13 | 内部错误 |
| UNAVAILABLE | 14 | 服务不可用 |
| DATA_LOSS | 15 | 数据丢失 |
| UNAUTHENTICATED | 16 | 未认证 |

## 技术要点

### 1. 创建客户端

使用 `yggdrasil.NewClient()` 创建客户端：

```go
cli, err := yggdrasil.NewClient("service-name")
defer cli.Close()
```

### 2. 元数据传递

使用 `metadata.WithStreamContext()` 包装上下文：

```go
ctx := metadata.WithStreamContext(context.Background())
```

### 3. 错误处理

使用 `status.FromError()` 从错误中提取信息：

```go
st := status.FromError(err)
reason := st.ErrorInfo().Reason
code := st.Code()
httpCode := st.HTTPCode()
```

### 4. 客户端拦截器

客户端可以配置拦截器来处理请求和响应：

```yaml
interceptor:
  unary_client: "logging"
  stream_client: "logging"
```

## 常见问题

**Q: 如何修改服务端点地址？**

A: 修改 `config.yaml` 中的 `client.<service>.remote.endpoints` 字段。

**Q: 如何添加自定义元数据？**

A: 使用 `metadata.NewContext()` 或 `metadata.AppendToOutgoingContext()`：

```go
ctx := metadata.AppendToOutgoingContext(context.Background(),
    "user-id", "12345",
    "trace-id", "abc-123",
)
```

**Q: 如何处理重试？**

A: 使用客户端拦截器或手动实现重试逻辑：

```go
maxRetries := 3
for i := 0; i < maxRetries; i++ {
    resp, err := client.Call(ctx, req)
    if err == nil {
        return resp, nil
    }
    
    if i < maxRetries-1 {
        time.Sleep(time.Second * time.Duration(i+1))
    }
}
```

**Q: 如何设置超时？**

A: 使用 `context.WithTimeout()`：

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := client.Call(ctx, req)
```

**Q: 如何使用 REST API？**

A: 使用标准的 HTTP 客户端：

```bash
curl http://localhost:3000/v1/shelves/shelves/1
```

**Q: 如何禁用日志拦截器？**

A: 修改 `config.yaml`，删除或注释掉拦截器配置：

```yaml
interceptor:
  # unary_client: "logging"
  # stream_client: "logging"
```

## 最佳实践

1. **使用 defer 关闭客户端**: 确保 client.Close() 被调用
2. **使用元数据传递上下文**: 使用 `metadata.WithStreamContext()`
3. **正确处理错误**: 使用 `status.FromError()` 提取错误信息
4. **设置合理的超时**: 使用 `context.WithTimeout()` 避免长时间阻塞
5. **实现重试机制**: 对于可重试的错误，实现指数退避重试
6. **使用连接池**: 客户端会自动管理连接池，无需手动管理

## 相关文档

- [Yggdrasil 主文档](../../../README.md)
- [Sample Server 示例](../server/)
- [错误处理示例](../advanced/error-handling/)
- [元数据传递示例](../advanced/metadata/)

## 退出

程序会自动退出，无需手动停止。
