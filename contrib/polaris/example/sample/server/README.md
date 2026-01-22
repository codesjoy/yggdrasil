# Sample Server（注册到 Polaris）

该示例启动一个 gRPC 服务，并通过 Yggdrasil 的 `registry=polaris` 把实例注册到 Polaris。

## 你会得到什么

- 一个服务：`github.com.codesjoy.yggdrasil.contrib.polaris.example.server`
- 一个实例：在 Polaris “服务列表/实例列表”里可见（host/port/protocol）

## 前置条件

1. 可访问的 Polaris Server（示例默认使用 gRPC 注册发现端口 `8091`）。
2. 已准备好命名空间（示例用 `default`）。
3. 可选：如果你的 Polaris 开启了鉴权，需要准备 token（对应 `yggdrasil.polaris.default.token`）与 service token（对应 `yggdrasil.registry.config.serviceToken`）。

### 快速启动 Polaris（可选）

开发测试可以用官方单机版镜像启动（会同时包含 console/server/limiter/prometheus 等组件）：

```bash
docker run -d --privileged=true --name polaris-standalone \
  -p 8080:8080 \
  -p 8090:8090 \
  -p 8091:8091 \
  -p 8093:8093 \
  polarismesh/polaris-standalone:latest
```

- 控制台：`http://127.0.0.1:8080`
- 注册发现 gRPC：`127.0.0.1:8091`
- 配置中心 gRPC：`127.0.0.1:8093`

## 启动方式

1. 修改本目录的 [config.yaml](file:///Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/contrib/polaris/example/sample/server/config.yaml)：
   - 把 `yggdrasil.polaris.default.addresses` 改成你的 Polaris Server 地址（例如 `127.0.0.1:8091`）。
   - 如果不需要鉴权：删除或留空 `yggdrasil.polaris.default.token`，并把 `yggdrasil.registry.config.serviceToken` 置空。
2. 启动服务：

```bash
cd contrib/polaris/example/sample/server
go run ./
```

启动成功后，服务会监听在 `yggdrasil.remote.protocol.grpc.address`（默认 `:55879`）。

## 配置说明（核心字段）

### 1) Polaris SDK（连接 Polaris）

- `yggdrasil.polaris.default.addresses`：Polaris 注册发现地址列表（通常是 `host:8091`）。
- `yggdrasil.polaris.default.token`：Polaris 访问 token（可选）。

### 2) Registry（把实例注册到 Polaris）

- `yggdrasil.registry.schema: polaris`
- `yggdrasil.registry.config.sdk: default`：使用哪套 SDK 配置（对应 `yggdrasil.polaris.<name>`）。
- `yggdrasil.registry.config.namespace`：注册到哪个 namespace（未填会回退到应用 namespace，再回退 `default`）。
- `yggdrasil.registry.config.serviceToken`：服务级 token（可选，取决于 Polaris 是否开启鉴权）。
- `ttl/autoHeartbeat`：是否启用心跳与实例 TTL。

### 3) 应用监听地址

- `yggdrasil.remote.protocol.grpc.address`：本地 gRPC 监听地址，注册到 Polaris 的实例 host/port 会基于这里派生。

## Polaris 控制台操作（建议流程）

1. 打开控制台：`http://127.0.0.1:8080`
2. 选择/创建命名空间：
   - 确认存在 `default`（或你在配置里填的 namespace）。
3. 查看服务是否出现：
   - 进入“服务管理/服务列表”，搜索服务名 `github.com.codesjoy.yggdrasil.contrib.polaris.example.server`。
4. 查看实例是否在线：
   - 打开服务详情的“实例”页，确认有 1 条实例记录，端口为 `55879`，协议为 `grpc`。
5. （可选）如果你需要 service token：
   - 在服务详情里找到 token 配置/展示位置，把 token 填回 `serviceToken` 后重启服务。

## 常见问题

- 服务注册了但 client 发现不到：
  - 确认 server/client 使用同一个 namespace。
  - 确认 server 注册的实例 protocol 是 `grpc`（示例会把 endpoint scheme 填为 `grpc`）。
- 认证失败：
  - 分别确认 `yggdrasil.polaris.default.token`（平台 token）与 `serviceToken`（服务 token）的来源与权限。
