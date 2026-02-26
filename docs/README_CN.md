# 文档入口

本目录是 Yggdrasil 文档入口（索引页）。

## 从这里开始

- 项目主文档（英文）：[../README.md](../README.md)
- 项目主文档（中文）：[../README_CN.md](../README_CN.md)
- 示例总览：[../example/README.md](../example/README.md)

## 仓库常用命令

- 安装开发工具（buf、golangci-lint、go-junit-report）：`make tools`
- 安装仓库内的二进制（含 protoc 插件）：`make install`
- 运行稳定测试（默认不带 race）：`make test`
- 运行 race 测试：`make test.race`
- 运行 lint（稳定档）：`make lint`
- 运行完整稳定门禁（CI 默认）：`make check`
- 运行严格门禁（含 examples/race/严格 lint）：`make check.strict`
- 检查依赖 tidy 漂移：`make go.mod.tidy.check`

默认会排除 `example/` 模块的 lint/test/coverage；如需纳入请加 `INCLUDE_EXAMPLES=1`。

Go 版本要求以 `../go.mod` 为准。

## 代码生成

### 生成示例的 protogen

`example/` 目录使用 Buf：

```bash
cd example
buf generate
```

### 生成 Reason 错误示例的代码

```bash
cd example/proto/error-handling
make generate
```

## 示例

- 示例学习路径与总览：[../example/README.md](../example/README.md)
- Sample Server：[../example/sample/server/README.md](../example/sample/server/README.md)
- Sample Client：[../example/sample/client/README.md](../example/sample/client/README.md)

## Contrib 模块

- etcd：[yggdrasil-ecosystem/integrations/etcd](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/etcd/README.md)
- Kubernetes：[yggdrasil-ecosystem/integrations/k8s](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/k8s/README.md)
- OpenTelemetry 导出器：[yggdrasil-ecosystem/integrations/otlp](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/otlp/README.md)、[OTLP Quickstart](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/otlp/QUICKSTART.md)
- xDS：[yggdrasil-ecosystem/integrations/xds](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/xds/README.md)
- Polaris：[yggdrasil-ecosystem/integrations/polaris](https://github.com/codesjoy/yggdrasil-ecosystem/tree/main/integrations/polaris/README.md)
