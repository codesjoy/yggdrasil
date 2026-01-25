# 基础集成示例

本示例演示如何使用yggdrasil v2框架实现xDS协议的基础集成，包括服务发现、连接建立和基本通信。

## 目录结构

```
basic/
├── client/
│   ├── main.go      # 客户端实现
│   └── config.yaml  # 客户端配置
└── server/
    ├── main.go      # 服务端实现
    └── config.yaml  # 服务端配置
```

## 功能特性

- **xDS服务发现**: 通过xDS控制平面发现服务端点
- **动态配置**: 支持运行时动态更新服务配置
- **完整服务实现**: 实现Library服务的所有RPC方法
- **元数据传递**: 使用metadata传递服务标识信息
- **结构化日志**: 完整的请求/响应日志记录

## 运行步骤

### 1. 启动xDS控制平面

```bash
cd contrib/xds/example/control-plane
go run main.go --config config.yaml
```

控制平面将在18000端口启动。

### 2. 启动服务端

```bash
cd contrib/xds/example/basic/server
go run main.go --config config.yaml
```

服务端将在55555端口启动。

### 3. 运行客户端

```bash
cd contrib/xds/example/basic/client
go run main.go --config config.yaml
```

## 预期输出

### 服务端日志

```
time=2025-01-26T10:00:00.000Z level=INFO msg="GetShelf called" name=shelves/1
time=2025-01-26T10:00:00.100Z level=INFO msg="CreateShelf called" name=shelves/2 theme=History
time=2025-01-26T10:00:00.200Z level=INFO msg="ListShelves called" parent=""
```

### 客户端日志

```
time=2025-01-26T10:00:00.000Z level=INFO msg="Starting xDS basic client..."
time=2025-01-26T10:00:00.050Z level=INFO msg="Calling GetShelf..."
time=2025-01-26T10:00:00.150Z level=INFO msg="GetShelf response" name=shelves/1 theme="Basic Service Theme"
time=2025-01-26T10:00:00.150Z level=INFO msg="Response trailer" trailer="map[server:basic-server]"
time=2025-01-26T10:00:00.150Z level=INFO msg="Response header" header="map[server:basic-server]"
time=2025-01-26T10:00:00.200Z level=INFO msg="Calling CreateShelf..."
time=2025-01-26T10:00:00.300Z level=INFO msg="CreateShelf response" name=shelves/2 theme=History
time=2025-01-26T10:00:00.350Z level=INFO msg="Calling ListShelves..."
time=2025-01-26T10:00:00.450Z level=INFO msg="ListShelves response" count=2
time=2025-01-26T10:00:00.450Z level=INFO msg="Shelf" index=0 name=shelves/1 theme="Basic Service Theme 1"
time=2025-01-26T10:00:00.450Z level=INFO msg="Shelf" index=1 name=shelves/2 theme="Basic Service Theme 2"
time=2025-01-26T10:00:00.500Z level=INFO msg="xDS basic client completed successfully"
```

## 配置说明

### 客户端配置 (config.yaml)

```yaml
yggdrasil:
  resolver:
    xds:
      type: "xds"
      config:
        name: "default"

  client:
    github.com.codesjoy.yggdrasil.example.sample:
      resolver: "xds"
      balancer: "xds"

  xds:
    default:
      server:
        address: "127.0.0.1:18000"  # xDS控制平面地址
      node:
        id: "basic-client"              # 客户端节点ID
        cluster: "test-cluster"          # 集群名称
      protocol: "grpc"
```

### 服务端配置 (config.yaml)

```yaml
yggdrasil:
  server:
    protocol:
      - "grpc"
  remote:
    protocol:
      grpc:
        address: "127.0.0.1:55555"  # gRPC服务端口
```

## 技术要点

### 1. yggdrasil初始化

```go
if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
    os.Exit(1)
}

if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.xds.example.basic.client"); err != nil {
    os.Exit(1)
}
```

### 2. 创建客户端

```go
cli, err := yggdrasil.NewClient("github.com.codesjoy.yggdrasil.example.sample")
if err != nil {
    os.Exit(1)
}
defer cli.Close()
```

### 3. 使用metadata

```go
ctx := metadata.WithStreamContext(context.Background())
client := librarypb.NewLibraryServiceClient(cli)
```

### 4. 启动服务端

```go
ss := &LibraryImpl{}
if err := yggdrasil.Serve(
    yggdrasil.WithServiceDesc(&librarypb2.LibraryServiceServiceDesc, ss),
); err != nil {
    os.Exit(1)
}
```

## 支持的RPC方法

| 方法 | 描述 |
|------|------|
| CreateShelf | 创建书架 |
| GetShelf | 获取书架信息 |
| ListShelves | 列出所有书架 |
| DeleteShelf | 删除书架 |
| MergeShelves | 合并书架 |
| CreateBook | 创建书籍 |
| GetBook | 获取书籍信息 |
| ListBooks | 列出书架中的书籍 |
| DeleteBook | 删除书籍 |
| UpdateBook | 更新书籍信息 |
| MoveBook | 移动书籍到其他书架 |

## 调试技巧

### 启用详细日志

修改配置文件中的日志级别：

```yaml
yggdrasil:
  logger:
    handler:
      default:
        level: "debug"
```

### 查看xDS通信

在控制平面日志中查看请求：

```bash
[xDS Control Plane] Request from node: basic-client, type: type.googleapis.com/envoy.config.listener.v3.Listener
```

### 验证服务发现

检查客户端是否正确连接到服务：

```bash
grpcurl -plaintext 127.0.0.1:55555 list
```

## 常见问题

Q: 客户端无法连接到xDS控制平面？
A: 检查控制平面是否在18000端口启动，客户端配置的address是否正确。

Q: 服务端没有收到请求？
A: 确认xDS控制平面已正确配置服务端点信息，检查服务端是否在55555端口启动。

Q: 如何切换到其他xDS控制平面？
A: 修改客户端配置中的`xds.default.server.address`。

Q: 如何添加新的服务方法？
A: 在proto文件中定义方法，重新生成代码，然后在`LibraryImpl`中实现新方法。
