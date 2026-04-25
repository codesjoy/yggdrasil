# 流式通信示例

本示例演示 Yggdrasil 框架中四种 gRPC 流式通信模式的使用方法。

## 你会得到什么

- 完整的四种流式通信示例
- Unary RPC（一元调用）
- Client Streaming（客户端流）
- Server Streaming（服务端流）
- Bidirectional Streaming（双向流）

## 目录结构

```
streaming/
├── server/
│   ├── main.go      # 服务端实现
│   └── config.yaml  # 服务端配置
├── client/
│   ├── main.go      # 客户端实现
│   └── config.yaml  # 客户端配置
└── README.md        # 本文档
```

## 流式通信模式

### 1. Unary RPC（一元调用）

**特点**: 最简单的 RPC 模式，客户端发送一个请求，服务端返回一个响应。

**适用场景**: 简单的查询操作、CRUD 操作。

**示例**: `SayHello` 方法

```protobuf
rpc SayHello(SayHelloRequest) returns (SayHelloResponse);
```

**服务端实现**:

```go
func (s *GreeterServer) SayHello(ctx context.Context, req *helloworldpb.SayHelloRequest) (*helloworldpb.SayHelloResponse, error) {
    return &helloworldpb.SayHelloResponse{
        Message: fmt.Sprintf("Hello %s!", req.Name),
    }, nil
}
```

**客户端调用**:

```go
resp, err := client.SayHello(ctx, &helloworldpb.SayHelloRequest{Name: "World"})
```

### 2. Client Streaming（客户端流）

**特点**: 客户端发送多个请求，服务端返回一个响应。

**适用场景**: 文件上传、批量数据处理、日志收集。

**示例**: `SayHelloClientStream` 方法

```protobuf
rpc SayHelloClientStream(stream SayHelloClientStreamRequest) returns (SayHelloClientStreamResponse);
```

**服务端实现**:

```go
func (s *GreeterServer) SayHelloClientStream(stream helloworldpb.GreeterService_SayHelloClientStreamServer) error {
    var names []string
    for {
        req, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
        names = append(names, req.Name)
    }
    
    return stream.SendAndClose(&helloworldpb.SayHelloClientStreamResponse{
        Message: fmt.Sprintf("Hello %v!", names),
    })
}
```

**客户端调用**:

```go
stream, err := client.SayHelloClientStream(ctx)
if err != nil {
    return err
}

for _, name := range names {
    if err := stream.Send(&helloworldpb.SayHelloClientStreamRequest{Name: name}); err != nil {
        return err
    }
}

resp, err := stream.CloseAndRecv()
```

### 3. Server Streaming（服务端流）

**特点**: 客户端发送一个请求，服务端返回多个响应。

**适用场景**: 数据订阅、实时通知、分页查询。

**示例**: `SayHelloServerStream` 方法

```protobuf
rpc SayHelloServerStream(SayHelloServerStreamRequest) returns (stream SayHelloServerStreamResponse);
```

**服务端实现**:

```go
func (s *GreeterServer) SayHelloServerStream(req *helloworldpb.SayHelloServerStreamRequest, stream helloworldpb.GreeterService_SayHelloServerStreamServer) error {
    for i := 0; i < 5; i++ {
        resp := &helloworldpb.SayHelloServerStreamResponse{
            Message: fmt.Sprintf("Hello %s! (message %d)", req.Name, i+1),
        }
        if err := stream.Send(resp); err != nil {
            return err
        }
        time.Sleep(500 * time.Millisecond)
    }
    return nil
}
```

**客户端调用**:

```go
stream, err := client.SayHelloServerStream(ctx, &helloworldpb.SayHelloServerStreamRequest{Name: "Grace"})
if err != nil {
    return err
}

for {
    resp, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    fmt.Println(resp.Message)
}
```

### 4. Bidirectional Streaming（双向流）

**特点**: 客户端和服务端都可以发送多个消息，读写顺序独立。

**适用场景**: 聊天应用、实时协作、游戏通信。

**示例**: `SayHelloStream` 方法

```protobuf
rpc SayHelloStream(stream SayHelloStreamRequest) returns (stream SayHelloStreamResponse);
```

**服务端实现**:

```go
func (s *GreeterServer) SayHelloStream(stream helloworldpb.GreeterService_SayHelloStreamServer) error {
    for {
        req, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }
        
        resp := &helloworldpb.SayHelloStreamResponse{
            Message: fmt.Sprintf("Hello %s!", req.Name),
        }
        if err := stream.Send(resp); err != nil {
            return err
        }
    }
}
```

**客户端调用**:

```go
stream, err := client.SayHelloStream(ctx)
if err != nil {
    return err
}

errChan := make(chan error, 1)

go func() {
    defer close(errChan)
    for _, name := range names {
        if err := stream.Send(&helloworldpb.SayHelloStreamRequest{Name: name}); err != nil {
            errChan <- err
            return
        }
    }
    stream.CloseSend()
}()

for {
    resp, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    fmt.Println(resp.Message)
}
```

## 前置条件

- Go 1.24 或更高版本
- 已安装 Yggdrasil 框架
- 已生成 protobuf 代码（在 `../../protogen/` 目录）

## 启动方式

### 1. 启动服务端

```bash
cd example/advanced/streaming/server
go run main.go
```

服务将在 `127.0.0.1:55880` 端口启动。

### 2. 运行客户端

打开新终端：

```bash
cd example/advanced/streaming/client
go run main.go
```

## 预期输出

### 服务端日志

```
time=2025-01-26T10:00:00.000Z level=INFO msg="SayHello called" name=World
time=2025-01-26T10:00:00.100Z level=INFO msg="SayHelloStream started"
time=2025-01-26T10:00:00.200Z level=INFO msg="SayHelloStream received" name=Alice
time=2025-01-26T10:00:00.300Z level=INFO msg="SayHelloStream received" name=Bob
time=2025-01-26T10:00:00.400Z level=INFO msg="SayHelloClientStream started"
time=2025-01-26T10:00:00.500Z level=INFO msg="SayHelloClientStream received" name=David
time=2025-01-26T10:00:00.600Z level=INFO msg="SayHelloServerStream started" name=Grace
```

### 客户端日志

```
time=2025-01-26T10:00:00.000Z level=INFO msg="=== Testing Unary RPC ==="
time=2025-01-26T10:00:00.050Z level=INFO msg="Calling SayHello..."
time=2025-01-26T10:00:00.100Z level=INFO msg="SayHello response" message="Hello World!"
time=2025-01-26T10:00:00.200Z level=INFO msg="=== Testing Bidirectional Streaming ==="
time=2025-01-26T10:00:00.250Z level=INFO msg="Calling SayHelloStream..."
time=2025-01-26T10:00:00.300Z level=INFO msg="Sent message" index=0 name=Alice
time=2025-01-26T10:00:00.350Z level=INFO msg="Received message" message="Hello Alice!"
time=2025-01-26T10:00:00.450Z level=INFO msg="Sent message" index=1 name=Bob
time=2025-01-26T10:00:00.500Z level=INFO msg="Received message" message="Hello Bob!"
time=2025-01-26T10:00:00.600Z level=INFO msg="=== Testing Client Streaming ==="
time=2025-01-26T10:00:00.650Z level=INFO msg="Calling SayHelloClientStream..."
time=2025-01-26T10:00:00.700Z level=INFO msg="Sent message" index=0 name=David
time=2025-01-26T10:00:00.800Z level=INFO msg="SayHelloClientStream response" message="Hello [David Eve Frank]!"
time=2025-01-26T10:00:00.900Z level=INFO msg="=== Testing Server Streaming ==="
time=2025-01-26T10:00:00.950Z level=INFO msg="Calling SayHelloServerStream..."
time=2025-01-26T10:00:01.000Z level=INFO msg="Received message" message="Hello Grace! (message 1)"
time=2025-01-26T10:00:01.500Z level=INFO msg="Received message" message="Hello Grace! (message 2)"
time=2025-01-26T10:00:02.000Z level=INFO msg="Received message" message="Hello Grace! (message 3)"
time=2025-01-26T10:00:02.500Z level=INFO msg="Received message" message="Hello Grace! (message 4)"
time=2025-01-26T10:00:03.000Z level=INFO msg="Received message" message="Hello Grace! (message 5)"
time=2025-01-26T10:00:03.100Z level=INFO msg="All streaming tests completed successfully!"
```

## 技术要点

### 1. 流式方法签名

服务端流方法签名：

```go
func (s *GreeterServer) MethodName(req *pb.Request, stream pb.Service_MethodNameServer) error
```

客户端流方法签名：

```go
func (s *GreeterServer) MethodName(stream pb.Service_MethodNameServer) error
```

双向流方法签名：

```go
func (s *GreeterServer) MethodName(stream pb.Service_MethodNameServer) error
```

### 2. 流式方法操作

**发送消息**:

```go
if err := stream.Send(&pb.Response{}); err != nil {
    return err
}
```

**接收消息**:

```go
resp, err := stream.Recv()
if err == io.EOF {
    return nil
}
if err != nil {
    return err
}
```

**关闭流**:

- 客户端流: `stream.SendAndClose()`
- 其他模式: `stream.CloseSend()`

### 3. 并发处理

双向流通常需要并发处理发送和接收：

```go
errChan := make(chan error, 1)

go func() {
    defer close(errChan)
    for _, item := range items {
        if err := stream.Send(&pb.Request{Data: item}); err != nil {
            errChan <- err
            return
        }
    }
    stream.CloseSend()
}()

for {
    resp, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    process(resp)
}

return <-errChan
```

### 4. 错误处理

流式方法的错误处理需要特别注意：

```go
for {
    req, err := stream.Recv()
    if err == io.EOF {
        return nil
    }
    if err != nil {
        return err
    }
}
```

### 5. 上下文使用

流式方法可以访问上下文：

```go
func (s *GreeterServer) MethodName(stream pb.Service_MethodNameServer) error {
    ctx := stream.Context()
    
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
}
```

## 常见问题

**Q: 什么时候使用流式 RPC？**

A: 
- **Unary RPC**: 简单查询、CRUD 操作
- **Client Streaming**: 文件上传、批量处理
- **Server Streaming**: 数据订阅、实时通知
- **Bidirectional Streaming**: 聊天、实时协作

**Q: 流式 RPC 性能如何？**

A: gRPC 使用 HTTP/2，支持多路复用，流式通信性能优秀。建议使用连接池复用连接。

**Q: 如何控制流式通信的并发？**

A: 使用 context 控制超时和取消：

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

stream, err := client.MethodName(ctx)
```

**Q: 如何处理流式通信中的错误？**

A: 在每次 Send 和 Recv 后检查错误：

```go
if err := stream.Send(req); err != nil {
    return err
}

resp, err := stream.Recv()
if err != nil {
    if err == io.EOF {
        return nil
    }
    return err
}
```

**Q: 如何实现背压（Backpressure）？**

A: 控制发送速率，避免缓冲区溢出：

```go
for i := 0; i < len(items); i++ {
    if err := stream.Send(&pb.Request{Data: items[i]}); err != nil {
        return err
    }
    time.Sleep(10 * time.Millisecond)
}
```

**Q: 流式通信可以传递大量数据吗？**

A: 可以，但需要注意：
1. 分批发送，避免单次发送过大
2. 实现流量控制，避免服务器过载
3. 使用合适的超时设置

**Q: 如何在流式通信中传递元数据？**

A: 使用 metadata：

```go
ctx := metadata.NewContext(context.Background(),
    metadata.Pairs("user-id", "12345"),
)

stream, err := client.MethodName(ctx)
```

## 最佳实践

1. **错误处理**: 每次 Send 和 Recv 后检查错误
2. **资源清理**: 使用 defer 确保流被正确关闭
3. **并发控制**: 使用 goroutine 和 channel 处理双向流
4. **超时控制**: 使用 context.WithTimeout 设置合理的超时
5. **流量控制**: 实现背压机制，避免服务器过载
6. **日志记录**: 记录流式通信的关键事件
7. **元数据传递**: 使用 metadata 传递上下文信息
8. **连接复用**: 复用客户端连接，避免频繁创建连接

## 相关文档

- [Yggdrasil 主文档](../../../README.md)
- [Sample Server 示例](../../sample/server/)
- [中间件示例](../middleware/)
- [元数据传递示例](../metadata/)

## 退出

服务端按 `Ctrl+C` 优雅退出。
