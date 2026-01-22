# Instance Metadata（namespace/version/metadata 注册示例）

该示例演示：通过 `yggdrasil.application` 设置应用维度的 `namespace/version/metadata`，并在注册到 Polaris 时把这些信息带到实例上。

## 你会得到什么

- 服务：`github.com.codesjoy.yggdrasil.contrib.polaris.example.instance_metadata.server`
- 注册到 namespace：`prod`（示例配置）
- 实例 metadata（示例）：
  - `env=prod`
  - `region=gz`
  - `lane=blue`
  - `version=v2026.01`

## 启动方式

1. 修改 [config.yaml](file:///Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/contrib/polaris/example/scenarios/instance-metadata/server/config.yaml)：
   - `yggdrasil.polaris.default.addresses`：Polaris naming 地址（通常 `host:8091`）
   - `yggdrasil.application.namespace/version/metadata`：按你的环境调整
   - `yggdrasil.registry.config.namespace`：建议与 application.namespace 保持一致（示例为 `prod`）
   - `yggdrasil.remote.protocol.grpc.address`：本地监听地址（示例 `127.0.0.1:55881`）
2. 启动：

```bash
cd contrib/polaris/example/scenarios/instance-metadata/server
go run ./
```

## 配置说明（重点）

### 1) yggdrasil.application

```yaml
yggdrasil:
  application:
    namespace: "prod"
    version: "v2026.01"
    metadata:
      env: "prod"
      region: "gz"
      lane: "blue"
```

- `namespace`：作为应用默认 namespace；当 registry 未显式指定 namespace 时会回退使用。
- `version`/`metadata`：会参与实例 metadata 的注册，用于路由/灰度/分组、以及控制台展示检索。

### 2) registry namespace

示例里显式指定注册到 `prod`：

```yaml
yggdrasil:
  registry:
    config:
      namespace: "prod"
```

## Polaris 控制台核验

1. 打开控制台：`http://127.0.0.1:8080`
2. 切换命名空间为 `prod`（如果控制台支持多 namespace）
3. 在“服务管理/服务列表”搜索服务：
   - `github.com.codesjoy.yggdrasil.contrib.polaris.example.instance_metadata.server`
4. 打开服务详情的“实例”页：
   - 确认实例端口为 `55881`
   - 查看实例 metadata 是否包含 `env/region/lane/version`

## 常见问题

- 控制台看不到服务：
  - 确认你切换到了正确 namespace（示例为 `prod`）。
  - 确认 `registry.config.namespace` 与 `application.namespace` 没有冲突。
