# etcd 示例

本目录包含 etcd contrib 模块的完整示例，展示如何使用 etcd 作为配置中心、注册中心和服务发现。

## 快速开始

### 前置条件

1. 安装 etcd（推荐使用 Docker）
2. 安装 Go 1.19+

### 启动 etcd

```bash
docker run -d --name etcd \
  -p 2379:2379 \
  -p 2380:2380 \
  -e ALLOW_NONE_AUTHENTICATION=yes \
  bitnami/etcd:latest
```

验证 etcd 是否正常运行：
```bash
etcdctl --endpoints=127.0.0.1:2379 endpoint health
```

## 示例列表

| 示例 | 说明 | 难度 | 预计时间 |
|------|------|--------|----------|
| [allinone](allinone/) | 完整示例：配置源 + 注册中心 + 服务发现 | ⭐⭐ | 15 分钟 |
| [config-source/blob](config-source/blob/) | 配置源 blob 模式 | ⭐ | 10 分钟 |
| [config-source/kv](config-source/kv/) | 配置源 kv 模式 | ⭐⭐ | 15 分钟 |
| [registry](registry/) | 服务注册中心 | ⭐⭐ | 10 分钟 |
| [resolver](resolver/) | 服务发现 | ⭐⭐ | 10 分钟 |

## 学习路径

### 路径 1：快速上手（推荐新手）

1. **[allinone](allinone/)**：运行完整示例，了解整体流程
2. **[config-source/blob](config-source/blob/)**：学习配置源基础用法
3. **[registry](registry/)**：学习服务注册基础用法
4. **[resolver](resolver/)**：学习服务发现基础用法

预计时间：45 分钟

### 路径 2：深入学习（推荐有经验的开发者）

1. **[allinone](allinone/)**：了解完整流程
2. **[config-source/blob](config-source/blob/)**：掌握 blob 模式
3. **[config-source/kv](config-source/kv/)**：掌握 kv 模式
4. **[registry](registry/)**：深入理解注册中心
5. **[resolver](resolver/)**：深入理解服务发现

预计时间：60 分钟

### 路径 3：按需学习

根据你的需求选择对应的示例：

- 需要配置管理 → [config-source/blob](config-source/blob/) 或 [config-source/kv](config-source/kv/)
- 需要服务注册 → [registry](registry/)
- 需要服务发现 → [resolver](resolver/)
- 需要完整功能 → [allinone](allinone/)

## 示例对比

### 配置源示例

| 特性 | blob 模式 | kv 模式 |
|------|-----------|---------|
| 存储 | 单个 key | 多个 key |
| 适合场景 | 配置较大、更新不频繁 | 配置较小、需要细粒度更新 |
| 更新粒度 | 整体更新 | 可单独更新某个配置项 |
| 示例 | [config-source/blob](config-source/blob/) | [config-source/kv](config-source/kv/) |

### 注册中心与服务发现

| 组件 | 功能 | 示例 |
|------|------|------|
| 注册中心 | 将服务实例注册到 etcd，维持心跳 | [registry](registry/) |
| 服务发现 | 从 etcd 发现服务实例，监听变更 | [resolver](resolver/) |
| 组合使用 | 同时使用注册中心和服务发现 | [allinone](allinone/) |

## 运行示例

### 基本步骤

每个示例的运行步骤基本一致：

```bash
# 1. 进入示例目录
cd contrib/etcd/example/<example-name>

# 2. 修改配置（如果需要）
vim config.yaml

# 3. 运行示例
go run <main-file>
```

### 多示例同时运行

你可以同时运行多个示例来观察它们之间的交互：

```bash
# 终端 1：启动注册中心示例
cd contrib/etcd/example/registry
go run server.go

# 终端 2：启动服务发现示例
cd contrib/etcd/example/resolver
go run client.go
```

## 常见问题

### Q: 示例运行失败怎么办？

A: 检查以下几点：
1. 确认 etcd 正在运行：`etcdctl --endpoints=127.0.0.1:2379 endpoint health`
2. 确认端口没有被占用：`netstat -an | grep 2379`
3. 查看错误日志，根据错误信息排查

### Q: 如何修改 etcd 地址？

A: 修改示例目录下的 `config.yaml` 文件：

```yaml
etcd:
  client:
    endpoints:
      - "your-etcd-endpoint:2379"
```

### Q: 示例需要依赖吗？

A: 大部分示例只需要 etcd 客户端库，会自动下载。运行前确保网络连接正常。

### Q: 如何停止示例？

A: 按 `Ctrl+C`，示例会优雅退出。

## 进阶用法

### 修改配置

你可以在示例运行时动态修改 etcd 中的配置：

```bash
# 查看当前配置
etcdctl --endpoints=127.0.0.1:2379 get /config/app --prefix

# 更新配置
etcdctl --endpoints=127.0.0.1:2379 put /config/app "new config"
```

### 监控 etcd

使用 `etcdctl` 监控 etcd 的状态：

```bash
# 查看所有 key
etcdctl --endpoints=127.0.0.1:2379 get "" --prefix --keys-only

# 查看成员状态
etcdctl --endpoints=127.0.0.1:2379 member list

# 查看集群健康状态
etcdctl --endpoints=127.0.0.1:2379 endpoint health --cluster
```

### 清理测试数据

测试完成后，清理 etcd 中的测试数据：

```bash
# 删除配置
etcdctl --endpoints=127.0.0.1:2379 del /config --prefix

# 删除注册信息
etcdctl --endpoints=127.0.0.1:2379 del /yggdrasil/registry --prefix

# 删除所有数据（谨慎使用）
etcdctl --endpoints=127.0.0.1:2379 del "" --prefix
```

## 架构图

```
┌─────────────────────────────────────────────────────────┐
│                   Application                      │
│                                                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │
│  │ Config      │  │ Registry    │  │ Resolver    │ │
│  │ Source      │  │             │  │             │ │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘ │
│         │                │                │          │
└─────────┼────────────────┼────────────────┼──────────┘
          │                │                │
          ▼                ▼                ▼
┌─────────────────────────────────────────────────────┐
│                    etcd                          │
│                                                     │
│  /config/app        (配置源）                     │
│  /registry/...      (注册中心/发现）               │
└─────────────────────────────────────────────────────┘
```

## 生产环境建议

### etcd 集群

生产环境建议使用 etcd 集群（3 或 5 节点）：

```yaml
etcd:
  client:
    endpoints:
      - "etcd-1:2379"
      - "etcd-2:2379"
      - "etcd-3:2379"
```

### TLS 加密

生产环境建议启用 TLS 加密：

```yaml
etcd:
  client:
    tls:
      certFile: "/path/to/cert.pem"
      keyFile: "/path/to/key.pem"
      caFile: "/path/to/ca.pem"
```

### 认证授权

生产环境建议启用认证：

```yaml
etcd:
  client:
    username: "your-username"
    password: "your-password"
```

### 监控告警

建议监控以下指标：
- etcd 连接状态
- 配置更新频率
- 注册/反注册次数
- 服务发现延迟
- 实例数量变化

## 参考文档

- [etcd 主文档](../readme.md)
- [etcd 官方文档](https://etcd.io/docs/latest/)
- [Yggdrasil 框架文档](../../README.md)
- [其他 contrib 模块](../../)

## 反馈与贡献

如果你在使用过程中遇到问题或有改进建议，欢迎：

1. 提交 Issue
2. 提交 Pull Request
3. 在社区讨论

## 许可证

本示例遵循 Yggdrasil 项目的许可证。
