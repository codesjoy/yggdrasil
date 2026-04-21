# Sample Server - 服务端示例

本示例演示如何使用 Yggdrasil 框架构建一个完整的 gRPC 服务端，包括服务实现、REST API 支持和自定义 HTTP 处理。

## 你会得到什么

- 一个完整的 gRPC 服务（LibraryService）
- 同时支持 gRPC 和 REST/HTTP 协议
- 自定义 HTTP 处理器
- 日志拦截器（记录请求和响应）
- 元数据传递（Header 和 Trailer）

## 前置条件

- Go 1.24 或更高版本
- 已安装 Yggdrasil 框架
- 已生成 protobuf 代码（在 `../../protogen/` 目录）

## 启动方式

### 1. 进入服务端目录

```bash
cd example/sample/server
```

### 2. 运行服务

```bash
go run main.go
```

服务将在以下端口启动：
- **gRPC 端口**: `127.0.0.1:55879`（在 config.yaml 中配置）
- **REST 端口**: `3000`

### 3. 验证服务运行

**检查 gRPC 服务**:

```bash
grpcurl -plaintext 127.0.0.1:55879 list
```

预期输出：
```
google.example.library.v1.LibraryService
grpc.reflection.v1.ServerReflection
```

**检查 REST 服务**:

```bash
curl http://localhost:3000/web
```

预期输出：
```
hello web
```

## 服务功能

### LibraryService 方法

| 方法 | HTTP 方法 | 路径 | 说明 |
|------|-----------|------|------|
| CreateShelf | POST | /v1/shelves | 创建书架 |
| GetShelf | GET | /v1/shelves/{name} | 获取书架 |
| ListShelves | GET | /v1/shelves | 列出所有书架 |
| DeleteShelf | DELETE | /v1/shelves/{name} | 删除书架 |
| MergeShelves | POST | /v1/{name}:merge | 合并书架 |
| CreateBook | POST | /v1/{parent}/books | 创建书籍 |
| GetBook | GET | /v1/books/{name} | 获取书籍 |
| ListBooks | GET | /v1/{parent}/books | 列出书籍 |
| DeleteBook | DELETE | /v1/books/{name} | 删除书籍 |
| UpdateBook | PATCH | /v1/books/{name} | 更新书籍 |
| MoveBook | POST | /v1/books/{name}:move | 移动书籍 |

### 自定义 HTTP 处理器

- 路径: `/web`
- 方法: GET
- 功能: 简单的 HTTP 响应

## 配置说明

### config.yaml 配置文件

```yaml
yggdrasil:
  server:
    protocol:
      - "grpc"

  rest:
    enable: true
    port: 3000
    middleware:
      all:
        - "logger"

  remote:
    protocol:
      grpc:
        address: "127.0.0.1:55879"
    logger_level: debug

  interceptor:
    unary_server: "logging"
    stream_server: "logging"
    config:
      logging:
        print_req_and_res: true

  logger:
    handler:
      default:
        type: "text"
        config:
          level: "debug"
    writer:
      default:
        type: "console"
```

### 配置字段说明

| 字段 | 说明 |
|------|------|
| `server.protocol` | 启用的协议列表（支持 gRPC） |
| `rest.enable` | 是否启用 REST API |
| `rest.port` | REST 服务监听端口 |
| `rest.middleware` | REST 中间件列表 |
| `remote.protocol.grpc.address` | gRPC 服务监听地址 |
| `interceptor.unary_server` | 一元 RPC 拦截器 |
| `interceptor.stream_server` | 流式 RPC 拦截器 |
| `logger.handler` | 日志处理器配置 |
| `logger.writer` | 日志输出配置 |

> 自定义 `logger.writer` 需要自行保证并发安全（内置 `console`/`file(lumberjack)` 已具备并发写保障）。

## 代码结构说明

```go
// 1. 定义服务实现
type LibraryImpl struct {
    librarypb2.UnimplementedLibraryServiceServer
}

// 2. 实现 RPC 方法
func (s *LibraryImpl) CreateShelf(ctx context.Context, req *librarypb2.CreateShelfRequest) (*librarypb2.Shelf, error) {
    // 设置元数据
    _ = metadata.SetTrailer(ctx, metadata.Pairs("trailer", "test"))
    _ = metadata.SetHeader(ctx, metadata.Pairs("header", "test"))
    
    return &librarypb2.Shelf{Name: "test", Theme: "test"}, nil
}

// 3. 自定义 HTTP 处理器
func WebHandler(w http.ResponseWriter, _ *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("hello web"))
}

// 4. 主函数
func main() {
    if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.example.sample"); err != nil {
        os.Exit(1)
    }

    ss := &LibraryImpl{}
    if err := yggdrasil.Serve(
        yggdrasil.WithServiceDesc(&librarypb2.LibraryServiceServiceDesc, ss),
        yggdrasil.WithRestServiceDesc(&librarypb2.LibraryServiceRestServiceDesc, ss),
        yggdrasil.WithRestRawHandleDesc(&server.RestRawHandlerDesc{
            Method:  http.MethodGet,
            Path:    "/web",
            Handler: WebHandler,
        }),
    ); err != nil {
        os.Exit(1)
    }
}
```

## 使用示例

### 使用 gRPC 客户端调用

```bash
# 创建书架
grpcurl -plaintext -d '{"shelf":{"theme":"Fiction"}}' 127.0.0.1:55879 google.example.library.v1.LibraryService/CreateShelf

# 获取书架
grpcurl -plaintext -d '{"name":"shelves/1"}' 127.0.0.1:55879 google.example.library.v1.LibraryService/GetShelf

# 列出所有书架
grpcurl -plaintext 127.0.0.1:55879 google.example.library.v1.LibraryService/ListShelves
```

### 使用 REST API 调用

```bash
# 创建书架
curl -X POST http://localhost:3000/v1/shelves \
  -H "Content-Type: application/json" \
  -d '{"shelf":{"theme":"Fiction"}}'

# 获取书架
curl http://localhost:3000/v1/shelves/shelves/1

# 列出所有书架
curl http://localhost:3000/v1/shelves

# 自定义 HTTP 处理器
curl http://localhost:3000/web
```

## 预期日志输出

```
time=2025-01-26T10:00:00.000Z level=INFO msg="loading config from file" path=./config.yaml
time=2025-01-26T10:00:00.100Z level=INFO msg="server starting" address=127.0.0.1:55879 protocol=gRPC
time=2025-01-26T10:00:00.200Z level=INFO msg="REST server starting" port=3000
time=2025-01-26T10:00:01.000Z level=INFO msg="received request" method=CreateShelf
time=2025-01-26T10:00:01.050Z level=INFO msg="request completed" method=CreateShelf duration=50ms
```

## 技术要点

### 1. 服务注册

使用 `yggdrasil.Serve()` 启动服务，可以注册多个服务描述：

```go
yggdrasil.Serve(
    yggdrasil.WithServiceDesc(&librarypb2.LibraryServiceServiceDesc, ss),
    yggdrasil.WithRestServiceDesc(&librarypb2.LibraryServiceRestServiceDesc, ss),
)
```

### 2. REST API 支持

Yggdrasil 自动从 proto 文件生成 REST API，支持 Google API 规范。

### 3. 自定义 HTTP 处理器

可以使用 `WithRestRawHandleDesc` 注册自定义 HTTP 处理器：

```go
yggdrasil.WithRestRawHandleDesc(&server.RestRawHandlerDesc{
    Method:  http.MethodGet,
    Path:    "/web",
    Handler: WebHandler,
})
```

### 4. 元数据传递

在 RPC 方法中设置 Header 和 Trailer：

```go
_ = metadata.SetHeader(ctx, metadata.Pairs("header", "test"))
_ = metadata.SetTrailer(ctx, metadata.Pairs("trailer", "test"))
```

### 5. 错误处理

使用 `xerror.WrapWithReason()` 创建带错误 reason 的错误：

```go
return nil, xerror.WrapWithReason(
    errors.New("test reason"),
    librarypb.Reason_BOOK_NOT_FOUND,
    "",
    nil,
)
```

## 常见问题

**Q: 如何修改服务监听端口？**

A: 修改 `config.yaml` 中的 `remote.protocol.grpc.address` 字段。

**Q: 如何禁用 REST API？**

A: 修改 `config.yaml`，设置 `rest.enable: false`。

**Q: 如何添加新的 RPC 方法？**

A: 1. 在 proto 文件中定义方法
2. 重新生成代码：`cd example && buf generate`
3. 在 `LibraryImpl` 中实现新方法

**Q: 如何启用 HTTPS？**

A: 配置 TLS 证书：
```yaml
remote:
  protocol:
    grpc:
      address: ":443"
      tls:
        cert_file: "/path/to/cert.pem"
        key_file: "/path/to/key.pem"
```

**Q: 如何修改日志级别？**

A: 修改 `config.yaml` 中的 `logger.handler.default.config.level` 字段（debug/info/warn/error）。

**Q: 如何添加自定义拦截器？**

A: 实现拦截器接口并注册：
```go
yggdrasil.Serve(
    yggdrasil.WithServerUnaryInterceptor(myInterceptor),
)
```

## 最佳实践

1. **使用配置文件**: 避免硬编码，使用配置文件管理配置
2. **添加日志**: 使用拦截器记录请求和响应，便于调试
3. **错误处理**: 使用 `xerror.WrapWithReason()` 创建结构化错误
4. **元数据传递**: 使用 metadata 传递跨服务上下文
5. **REST 规范**: 遵循 RESTful API 设计规范
6. **版本控制**: 在 API 路径中包含版本号（如 `/v1/`）

## 相关文档

- [Yggdrasil 主文档](../../../README.md)
- [Sample Client 示例](../client/)
- [REST API 示例](../advanced/rest/)

## 退出

按 `Ctrl+C` 优雅退出服务。
