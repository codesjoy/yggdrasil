# 03 Diagnostics Reload

## 体现的框架能力

- 使用 watchable config source 触发自动 reload，而不是手工重启示例。
- 通过 governor 的 `/diagnostics` 和 `/module-hub` 观察 `AssemblySpec`、plan hash、spec diff 和 reload 错误。
- 用一个最小示例演示业务 bundle 已安装时，配置变化为什么会被分类成 `restart-required`。

## 启动方式

```bash
cd /Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/03-diagnostics-reload
go run .
```

触发一次可观察的 reload：

```bash
perl -0pi -e 's/mode: dev/mode: prod-grpc/' reload.yaml
curl http://127.0.0.1:56032/diagnostics?pretty=true
curl http://127.0.0.1:56032/module-hub?pretty=true
```

## 观察点

- `main.go` 仍走 root `yggdrasil.Run(...)`，只是额外通过 `yggdrasil.WithConfigSource(...)` 注入了可 watch 的 `reload.yaml`。
- `config.yaml` 是基础配置，`reload.yaml` 作为附加 layer 加载并 watch。
- 当 `reload.yaml` 把 `mode` 从 `dev` 改到 `prod-grpc` 时，framework 会重新规划 assembly，并在 diagnostics 中记录新的 spec hash 与 diff。

## 关键源码入口

- 生命周期入口：[main.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/03-diagnostics-reload/main.go)
- 业务组合：[business/compose.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/03-diagnostics-reload/business/compose.go)
- reload smoke test：[smoke_test.go](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/03-diagnostics-reload/smoke_test.go)

## 下一步看什么

- 如果你要看更聚焦的 REST 行为，读 [10-rest-gateway](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/10-rest-gateway)。
- 如果你要看 capability provider-only 扩展，读 [20-capability-registration](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/20-capability-registration)。
