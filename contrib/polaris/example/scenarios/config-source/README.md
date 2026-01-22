# Config Source（从 Polaris 配置中心加载并订阅）

该示例演示：把 Polaris 配置中心的一个配置文件作为 Yggdrasil 的 `config source` 加载，并订阅变更事件，驱动 `config.AddWatcher` 回调输出。

## 你会看到什么

- 启动后会阻塞运行（`select {}`）。
- 当你在 Polaris 控制台修改并发布配置文件后，终端会输出类似：

```text
type: update version: <n> value: <int>
```

其中 `value` 来自你配置文件中被监听的 key（默认 key 为 `dd`）。

## 前置条件

1. 可访问的 Polaris Server（配置中心 gRPC 端口默认 `8093`，注册发现 gRPC 端口默认 `8091`）。
2. 已准备好命名空间（示例用 `default`）。
3. （可选）如果开启了鉴权，准备 token（`yggdrasil.polaris.default.token`）。

## 启动方式

1. 修改本目录 [config.yaml](file:///Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/contrib/polaris/example/scenarios/config-source/config.yaml)：
   - `yggdrasil.polaris.default.addresses`：naming 地址（通常是 `host:8091`）
   - `yggdrasil.polaris.default.config_addresses`：config 地址（通常是 `host:8093`）
   - `yggdrasil.polaris.default.token`：可选
2. 启动：

```bash
cd contrib/polaris/example/scenarios/config-source
go run ./
```

## Polaris 控制台操作（创建并发布配置文件）

目标是创建一个配置文件：`namespace=default`、`fileGroup=app`、`fileName=service.yaml`，内容里包含被监听的 key（默认 `dd`）。

1. 打开控制台：`http://127.0.0.1:8080`
2. 进入“配置中心/配置文件”（不同版本控制台菜单名可能略有差异）
3. 选择命名空间 `default`
4. 创建配置文件：
   - 分组（FileGroup）：`app`
   - 文件名（FileName）：`service.yaml`
5. 编辑内容（示例）并发布：

```yaml
dd: 1
```

6. 回到示例进程输出窗口：应当会打印一次初始值（或首次变更值）。
7. 修改为其他值并再次发布：

```yaml
dd: 2
```

进程会打印变更事件。

## 配置说明（示例 config.yaml 的 example 段）

`main.go` 会从 `yggdrasil.example.config_source` 读取 `polaris.ConfigSourceConfig`：

- `sdk`：使用哪套 Polaris SDK（对应 `yggdrasil.polaris.<name>`）
- `namespace`：配置文件所在 namespace（默认会回退到 `yggdrasil.InstanceNamespace()`）
- `fileGroup` / `fileName`：配置文件定位
- `subscribe`：是否订阅变更（示例为 true）
- `fetchTimeout`：拉取超时

监听的 key 由 `yggdrasil.example.watched_key` 指定，示例为 `dd`：
- 注意：这里监听的是“配置内容里的 key”，不是 Polaris 的文件名/分组。

## 常见问题

- 没有任何输出：
  - 确认已在 Polaris 控制台“发布”配置文件（仅保存未发布通常不会下发）。
  - 确认配置文件内容里包含 `dd` 这个 key，并且值是整数（示例里 `Value().Int()`）。
- 连接失败：
  - 确认 `config_addresses` 指向 `8093`，不要填成 `8091`。
  - 如果开启鉴权，确认 token 有权限读取配置中心。

