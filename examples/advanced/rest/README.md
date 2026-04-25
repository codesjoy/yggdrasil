# REST API 示例

本示例演示如何在 Yggdrasil 框架中从 proto 文件生成 REST API，同时支持 gRPC 和 HTTP/JSON。

## 你会得到什么

- 从 proto 文件生成的 REST API
- 同时支持 gRPC 和 HTTP/JSON 协议
- REST 中间件配置
- HTTP/JSON 编解码
- 自定义 HTTP 处理器

## 功能特性

- **自动生成**: 从 proto 文件自动生成 REST API
- **双协议**: 同时支持 gRPC 和 HTTP/JSON
- **中间件**: REST 请求中间件
- **编解码**: 自动 JSON 编解码
- **路由**: 遵循 Google API 设计规范

## Proto 文件定义

Yggdrasil 使用 Google API 规范定义 REST API：

```protobuf
import "google/api/annotations.proto";

service LibraryService {
  rpc CreateShelf(CreateShelfRequest) returns (Shelf) {
    option (google.api.http) = {
      post: "/v1/shelves"
      body: "shelf"
    };
  }

  rpc GetShelf(GetShelfRequest) returns (Shelf) {
    option (google.api.http) = {
      get: "/v1/{name=shelves/*}"
    };
  }
}
```

## REST API 生成

### 生成 REST API 代码

使用 `protoc-gen-yggdrasil-rest` 插件生成 REST API：

```bash
protoc --go_out=. \
  --yggdrasil-rest_out=. \
  library.proto
```

### 生成的代码

生成的代码包含：
- REST 服务描述 (`LibraryServiceRestServiceDesc`)
- HTTP 路由器
- JSON 编解码器

## 配置 REST API

### 启用 REST API

在配置文件中启用 REST：

```yaml
yggdrasil:
  rest:
    enable: true
    port: 3000
```

### REST 中间件配置

```yaml
yggdrasil:
  rest:
    enable: true
    port: 3000
    middleware:
      all:
        - "logger"
      GET:
        - "logger"
      POST:
        - "auth"
        - "logger"
```

### 注册 REST 服务

```go
app, err := yggdrasil.New("advanced-rest-server",
    yggdrasil.WithRPCService(&librarypb.LibraryServiceServiceDesc, ss),
    yggdrasil.WithRESTService(&librarypb.LibraryServiceRestServiceDesc, ss),
)
if err != nil { panic(err) }
if err := app.Start(context.Background()); err != nil { panic(err) }
```

## 使用 REST API

### 创建资源

**请求**:

```bash
curl -X POST http://localhost:3000/v1/shelves \
  -H "Content-Type: application/json" \
  -d '{
    "shelf": {
      "theme": "Fiction"
    }
  }'
```

**响应**:

```json
{
  "name": "shelves/1",
  "theme": "Fiction"
}
```

### 获取资源

**请求**:

```bash
curl http://localhost:3000/v1/shelves/shelves/1
```

**响应**:

```json
{
  "name": "shelves/1",
  "theme": "Fiction"
}
```

### 列出资源

**请求**:

```bash
curl http://localhost:3000/v1/shelves
```

**响应**:

```json
{
  "shelves": [
    {
      "name": "shelves/1",
      "theme": "Fiction"
    },
    {
      "name": "shelves/2",
      "theme": "Science"
    }
  ]
}
```

### 删除资源

**请求**:

```bash
curl -X DELETE http://localhost:3000/v1/shelves/shelves/1
```

### 更新资源

**请求**:

```bash
curl -X PATCH http://localhost:3000/v1/shelves/shelves/1 \
  -H "Content-Type: application/json" \
  -d '{
    "shelf": {
      "theme": "Fantasy"
    }
  }'
```

## HTTP 方法映射

| gRPC 方法 | HTTP 方法 | 路径 |
|-----------|-----------|------|
| CreateShelf | POST | /v1/shelves |
| GetShelf | GET | /v1/shelves/{name} |
| ListShelves | GET | /v1/shelves |
| DeleteShelf | DELETE | /v1/shelves/{name} |
| MergeShelves | POST | /v1/{name}:merge |
| CreateBook | POST | /v1/{parent}/books |
| GetBook | GET | /v1/books/{name} |
| ListBooks | GET | /v1/{parent}/books |
| DeleteBook | DELETE | /v1/books/{name} |
| UpdateBook | PATCH | /v1/books/{name} |
| MoveBook | POST | /v1/books/{name}:move |

## 自定义 HTTP 处理器

### 注册自定义处理器

```go
func CustomHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "message": "Hello from custom handler",
    })
}

app, err := yggdrasil.New("advanced-rest-custom",
    yggdrasil.WithRESTHandlers(&server.RestRawHandlerDesc{
        Method:  http.MethodGet,
        Path:    "/custom",
        Handler: CustomHandler,
    }),
)
if err != nil { panic(err) }
if err := app.Start(context.Background()); err != nil { panic(err) }
```

### 使用自定义处理器

```bash
curl http://localhost:3000/custom
```

**响应**:

```json
{
  "message": "Hello from custom handler"
}
```

## REST 中间件

### 创建 REST 中间件

```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        next.ServeHTTP(w, r)
        
        slog.Info("HTTP request",
            "method", r.Method,
            "path", r.URL.Path,
            "duration", time.Since(start),
        )
    })
}
```

### 注册中间件

```go
import "github.com/codesjoy/yggdrasil/v3/transport/gateway/rest"

middleware.Register("logger", LoggingMiddleware)
```

### 配置中间件

```yaml
yggdrasil:
  rest:
    middleware:
      all:
        - "logger"
      GET:
        - "logger"
      POST:
        - "auth"
        - "logger"
```

## 错误处理

### gRPC 错误映射到 HTTP

Yggdrasil 自动将 gRPC 错误码映射到 HTTP 状态码：

| gRPC Code | HTTP Code | 说明 |
|-----------|-----------|------|
| OK | 200 | 成功 |
| INVALID_ARGUMENT | 400 | 无效参数 |
| UNAUTHENTICATED | 401 | 未认证 |
| PERMISSION_DENIED | 403 | 权限不足 |
| NOT_FOUND | 404 | 资源未找到 |
| ALREADY_EXISTS | 409 | 资源已存在 |
| INTERNAL | 500 | 内部错误 |

### 返回错误

```go
func (s *Server) GetShelf(ctx context.Context, req *pb.GetShelfRequest) (*pb.Shelf, error) {
    shelf, err := s.db.GetShelf(req.Name)
    if err != nil {
        if errors.Is(err, ErrShelfNotFound) {
            return nil, xerror.WrapWithReason(
                err,
                pb.Reason_SHELF_NOT_FOUND,
                "",
                nil,
            )
        }
        return nil, xerror.Wrap(err, code.Code_INTERNAL, "get shelf failed")
    }
    return shelf, nil
}
```

### HTTP 错误响应

**请求**:

```bash
curl http://localhost:3000/v1/shelves/invalid-id
```

**响应**:

```json
{
  "error": {
    "code": 5,
    "message": "shelf not found",
    "status": "NOT_FOUND"
  }
}
```

## REST API 最佳实践

### 1. 遵循 RESTful 规范

- 使用合适的 HTTP 方法
- 使用资源导向的 URL
- 使用标准的状态码

### 2. 版本控制

在 URL 中包含版本号：

```
/v1/shelves
/v2/shelves
```

### 3. 分页

使用分页参数：

```
/v1/shelves?page_size=10&page_token=abc123
```

### 4. 过滤和排序

使用查询参数：

```
/v1/shelves?filter=theme:Fiction&order_by=theme
```

### 5. 字段选择

允许客户端选择需要的字段：

```
/v1/shelves?fields=name,theme
```

## 常见问题

**Q: 如何同时支持 gRPC 和 REST？**

A: 注册两个服务描述：

```go
app, err := yggdrasil.New("advanced-rest-server",
    yggdrasil.WithRPCService(&librarypb.LibraryServiceServiceDesc, ss),
    yggdrasil.WithRESTService(&librarypb.LibraryServiceRestServiceDesc, ss),
)
if err != nil { panic(err) }
if err := app.Start(context.Background()); err != nil { panic(err) }
```

**Q: 如何禁用 REST API？**

A: 在配置文件中禁用：

```yaml
yggdrasil:
  rest:
    enable: false
```

**Q: 如何自定义 REST 路径？**

A: 在 proto 文件中使用 `google.api.http` 选项：

```protobuf
rpc GetShelf(GetShelfRequest) returns (Shelf) {
  option (google.api.http) = {
    get: "/api/v1/shelves/{name}"
  };
}
```

**Q: 如何处理 CORS？**

A: 添加 CORS 中间件：

```go
func CORSMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

**Q: 如何处理认证？**

A: 使用中间件或 gRPC 拦截器：

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

**Q: 如何处理大文件上传？**

A: 使用流式上传：

```protobuf
rpc UploadFile(stream UploadFileRequest) returns (UploadFileResponse) {
  option (google.api.http) = {
    post: "/v1/files:upload"
    body: "*"
  };
}
```

## 相关文档

- [Yggdrasil 主文档](../../../README.md)
- [Sample Server 示例](../../sample/server/)
- [中间件示例](../middleware/)
- [Google API 设计指南](https://cloud.google.com/apis/design)

## 退出

按 `Ctrl+C` 优雅退出服务。
