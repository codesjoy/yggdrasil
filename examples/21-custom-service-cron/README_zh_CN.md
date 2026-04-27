# 21 Custom Service Cron

## 体现的框架能力

- 展示业务方如何把第三方后台服务封装成 `BusinessInstallable`。
- 展示 `BusinessInstallable.Install(...)` 如何通过 `InstallContext.AddTask(...)` 把自定义服务交给 Yggdrasil 生命周期管理。
- 展示 `robfig/cron/v3` 这类后台调度器如何跟随 app 启停，并在 stop 时等待 cron job graceful shutdown。

## 启动方式

```bash
cd examples/21-custom-service-cron
go run .
```

可选观察：

```bash
curl http://127.0.0.1:56024/diagnostics?pretty=true
```

## 观察点

- `business.Compose(...)` 不直接注册 `Tasks`，而是返回一个 `Extensions` 项，模拟真实业务里封装出的自定义服务集成。
- `cronIntegration.Install(...)` 在 install 阶段校验 cron 表达式并注册 managed background task。
- `cronTask.Stop(...)` 使用 `cron.Stop()` 返回的 context 等待已触发 job 完成，同时尊重框架传入的 shutdown context。

## 关键源码入口

- 生命周期入口：[main.go](main.go)
- 自定义 cron 集成：[business/compose.go](business/compose.go)
- 集成测试：[business/compose_test.go](business/compose_test.go)

## 下一步看什么

- 如果你想先理解 `BusinessBundle` 的完整安装面，读 [02-runtime-bundle](../02-runtime-bundle/README_zh_CN.md)。
- 如果你想看 provider-only 扩展，而不是业务侧自定义服务，读 [20-capability-registration](../20-capability-registration/README_zh_CN.md)。
