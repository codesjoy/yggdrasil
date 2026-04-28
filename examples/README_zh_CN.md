# Yggdrasil Examples 文档

本目录是 Yggdrasil v3 examples 的中文优化版文档。示例按学习路径组织，而不是简单分成 sample / advanced。

## 阅读路径

### 入门主线

1. [01 快速开始](01-quickstart/README_zh_CN.md)：最短端到端路径，服务端使用 `yggdrasil.Run(ctx, appName, ...)`，客户端使用 standalone `app.New(appName, ...)->NewClient(...)`。
2. [02 Runtime Bundle](02-runtime-bundle/README_zh_CN.md)：理解 root facade 背后的 `Runtime` 与 `BusinessBundle` 安装面。
3. [03 Diagnostics Reload](03-diagnostics-reload/README_zh_CN.md)：观察 watchable config、diagnostics、spec hash/diff 与 restart-required reload 行为。

### 功能示例

- [10 REST Gateway](10-rest-gateway/README_zh_CN.md)：通过 `RESTBinding` 安装 HTTP/JSON 接口。
- [11 RPC Streaming](11-rpc-streaming/README_zh_CN.md)：unary、client-streaming、server-streaming、bidirectional-streaming。
- [12 Transport Metadata](12-transport-metadata/README_zh_CN.md)：metadata、header、trailer 传递。
- [13 Error Reason](13-error-reason/README_zh_CN.md)：结构化错误 reason、gRPC code、HTTP code、metadata。
- [14 Client Load Balancing](14-client-load-balancing/README_zh_CN.md)：一个 service target 对多个 endpoint 的请求分发。

### 扩展示例

- [20 Capability Registration](20-capability-registration/README_zh_CN.md)：provider-only capability registration。
- [21 Custom Service Cron](21-custom-service-cron/README_zh_CN.md)：业务方自定义 `BusinessInstallable`，集成第三方后台调度器。

### Recipes

- [Config Layers Recipe](90-recipes/config-layers.md)：配置分层、typed section、watchable overlay。
- [Raw gRPC Recipe](90-recipes/raw-grpc.md)：通过 `raw` content subtype 直接收发 `[]byte`。
- [JSON Raw gRPC Recipe](90-recipes/jsonraw-grpc.md)：通过 `jsonraw` content subtype 传递 JSON 文本字节。

## 文档约定

- 服务端示例优先使用 root `yggdrasil.Run(ctx, appName, ...)`。
- standalone client、provider-heavy 或需要低层控制的场景使用 `app.New(appName, ...)`。
- 正式业务安装边界始终是 `BusinessBundle`。
- 所有链接使用仓库相对路径，不使用本地机器绝对路径。
