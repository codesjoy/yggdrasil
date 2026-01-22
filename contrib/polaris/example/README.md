# Polaris Example

目录结构：

- `sample/server`：启动一个 RPC 服务并注册到 Polaris（见 sample/server/README.md）
- `sample/client`：通过 Polaris 发现服务并发起 RPC 调用（见 sample/client/README.md）
- `scenarios/governance`：路由标签示例（client+server，见 scenarios/governance/*/README.md）
- `scenarios/config-source`：配置中心 source 示例（见 scenarios/config-source/README.md）
- `scenarios/instance-metadata`：实例元信息注册示例（见 scenarios/instance-metadata/server/README.md）
- `scenarios/multi-sdk`：多 SDKContext（以 name 区分）配置说明（见 scenarios/multi-sdk/README.md）

运行方式（假设 Polaris Server 已可访问，默认 `127.0.0.1:8091`）：

- 该示例默认使用 `yggdrasil.polaris.default.addresses` 初始化 SDKContext；也可改为在 `yggdrasil.polaris.default.config_file` 指定 Polaris 原生配置文件路径，并通过 `yggdrasil.polaris.default.token` 配置 token。
- 如果你的 Polaris 配置中心地址与注册发现地址不同，可配置 `yggdrasil.polaris.default.config_addresses`（通常为 `host:8093`）。

```bash
cd contrib/polaris/example/sample/server
go run ./
```

另开一个终端：

```bash
cd contrib/polaris/example/sample/client
go run ./
```

治理路由标签示例：

```bash
cd contrib/polaris/example/scenarios/governance/server
go run ./
```

另开一个终端：

```bash
cd contrib/polaris/example/scenarios/governance/client
go run ./
```

配置中心 source 示例：

```bash
cd contrib/polaris/example/scenarios/config-source
go run ./
```
