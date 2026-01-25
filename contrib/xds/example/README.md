# xDS Examples

本目录包含yggdrasil v2框架与xDS协议集成的完整示例，展示了各种服务发现、负载均衡和流量管理场景。

## 目录结构

```
example/
├── README.md                    # 本文档
├── basic/                      # 基础集成示例
├── canary/                     # 金丝雀部署示例
├── load-balancing/             # 负载均衡示例
├── traffic-splitting/           # 流量分割示例
├── multi-service/              # 多服务示例
└── control-plane/              # xDS控制平面
```

## 示例概览

### 1. Basic - 基础集成示例

**目录**: [basic/](./basic)

**场景**: 演示xDS协议的基础集成，包括服务发现、连接建立和基本通信。

**功能特性**:
- xDS服务发现
- 动态配置更新
- 完整的RPC服务实现
- 元数据传递
- 结构化日志

**运行时间**: ~10秒

**适合人群**: 初学者，第一次接触xDS协议的开发者

**快速开始**:
```bash
cd control-plane
go run main.go --config config.yaml

cd basic/server
go run main.go --config config.yaml

cd basic/client
go run main.go --config config.yaml
```

**测试**: `./test.ps1`

---

### 2. Canary - 金丝雀部署示例

**目录**: [canary/](./canary)

**场景**: 演示如何使用xDS实现渐进式金丝雀部署，安全地将流量从稳定版本逐步切换到新版本。

**功能特性**:
- 渐进式流量切换（5% → 100%）
- 多阶段验证
- 实时监控
- 安全回滚

**运行时间**: ~1分钟（模拟6个阶段）

**适合人群**: DevOps工程师、需要安全发布流程的团队

**快速开始**:
```bash
cd control-plane
set XDS_CONFIG_FILE=canary-xds-config.yaml
go run main.go --config config.yaml

cd canary/server
set DEPLOYMENT_TYPE=stable
go run main.go --config config.yaml

set DEPLOYMENT_TYPE=canary
set PORT=55556
go run main.go --config config.yaml

cd canary/client
go run main.go --config config.yaml
```

**测试**: `./test.ps1`

**业务价值**: 降低发布风险，快速发现和修复问题

---

### 3. Load Balancing - 负载均衡示例

**目录**: [load-balancing/](./load-balancing)

**场景**: 演示不同的负载均衡策略，包括轮询、随机和最少请求。

**功能特性**:
- 多实例支持
- 流量统计
- 多种负载均衡策略
- 实时监控

**运行时间**: ~20秒（30个请求）

**适合人群**: 需要优化服务性能、提高资源利用率的团队

**快速开始**:
```bash
cd control-plane
set XDS_CONFIG_FILE=load-balancing-xds-config.yaml
go run main.go --config config.yaml

cd load-balancing/server
set SERVER_ID=1
go run main.go --config config.yaml

set SERVER_ID=2
set PORT=55556
go run main.go --config config.yaml

set SERVER_ID=3
set PORT=55557
go run main.go --config config.yaml

cd load-balancing/client
go run main.go --config config.yaml
```

**测试**: `./test.ps1`

**策略对比**:
- **Round Robin**: 公平分配，适合性能相似的实例
- **Random**: 简单高效，适合实例数量多的场景
- **Least Request**: 动态适应，适合请求处理时间差异大的场景

---

### 4. Traffic Splitting - 流量分割示例

**目录**: [traffic-splitting/](./traffic-splitting)

**场景**: 演示如何将流量按权重分配到不同的后端服务。

**功能特性**:
- 流量分割（按权重分配）
- 实时统计
- 灵活配置
- 多后端支持

**运行时间**: ~15秒（20个请求）

**适合人群**: 需要A/B测试、灰度发布、容量管理的团队

**快速开始**:
```bash
cd control-plane
set XDS_CONFIG_FILE=traffic-splitting-xds-config.yaml
go run main.go --config config.yaml

cd traffic-splitting/server
set BACKEND_ID=1
go run main.go --config config.yaml

set BACKEND_ID=2
set PORT=55556
go run main.go --config config.yaml

cd traffic-splitting/client
go run main.go --config config.yaml
```

**测试**: `./test.ps1`

**应用场景**:
- A/B测试：50% / 50%
- 灰度发布：90% / 10% → 0% / 100%
- 容量管理：根据后端容量分配流量
- 多租户：不同租户使用不同后端

---

### 5. Multi-Service - 多服务示例

**目录**: [multi-service/](./multi-service)

**场景**: 演示如何在多服务场景下实现服务发现和通信。

**功能特性**:
- 多服务支持
- 统一管理
- 服务发现
- 元数据传递
- 灵活配置

**运行时间**: ~10秒（20个请求，交替调用2个服务）

**适合人群**: 微服务架构开发者、需要集成多个服务的团队

**快速开始**:
```bash
cd control-plane
set XDS_CONFIG_FILE=multi-service-xds-config.yaml
go run main.go --config config.yaml

cd multi-service/server
go run main.go --config config.yaml

cd multi-service/client
go run main.go --config config.yaml
```

**测试**: `./test.ps1`

**应用场景**:
- 微服务架构：多个独立服务协同工作
- 网关模式：统一的服务管理和配置
- 多租户系统：不同租户使用不同服务实例
- 混合部署：本地服务 + 远程服务

---

## Control Plane - xDS控制平面

**目录**: [control-plane/](./control-plane)

**功能**: 提供xDS配置管理和服务发现功能。

**特性**:
- 模块化架构（server/、snapshot/、watcher/）
- SnapshotCache实现（Envoy官方推荐）
- 文件监控和热重载（fsnotify）
- YAML配置驱动
- 优雅关闭和信号处理
- 回调系统和监控

**配置文件**:
- `config.yaml`: 控制平面服务器配置
- `xds-config.yaml`: 默认xDS配置
- `canary-xds-config.yaml`: 金丝雀部署配置
- `load-balancing-xds-config.yaml`: 负载均衡配置
- `traffic-splitting-xds-config.yaml`: 流量分割配置
- `multi-service-xds-config.yaml`: 多服务配置

**端口**: 18000

**启动方式**:
```bash
cd control-plane
# 使用默认配置
go run main.go --config config.yaml

# 使用特定配置
set XDS_CONFIG_FILE=canary-xds-config.yaml
go run main.go --config config.yaml
```

---

## 快速开始指南

### 1. 环境准备

确保已安装：
- Go 1.21+
- PowerShell 5.1+

### 2. 选择示例

根据你的需求选择合适的示例：

| 需求 | 推荐示例 |
|------|----------|
| 学习xDS基础 | Basic |
| 安全发布新版本 | Canary |
| 优化服务性能 | Load Balancing |
| A/B测试或灰度发布 | Traffic Splitting |
| 集成多个服务 | Multi-Service |

### 3. 运行示例

每个示例都包含详细的README文档，按照文档步骤运行即可。

### 4. 运行测试

每个示例都提供了自动化测试脚本：

```powershell
cd <example-directory>
.\test.ps1
```

---

## 常见问题

### Q: 控制平面必须在18000端口运行吗？

A: 不是。可以在`control-plane/config.yaml`中修改`server.port`配置。

### Q: 可以同时运行多个示例吗？

A: 不建议。每个示例使用不同的xDS配置，同时运行可能导致冲突。建议一次运行一个示例。

### Q: 示例中的服务端点地址可以修改吗？

A: 可以。在相应的xDS配置文件（如`canary-xds-config.yaml`）中修改endpoints的address和port。

### Q: 如何调试xDS通信问题？

A: 启用详细日志：

```yaml
yggdrasil:
  logger:
    handler:
      default:
        level: "debug"
```

### Q: 测试脚本失败了怎么办？

A: 检查以下几点：
1. 控制平面是否正在运行
2. 服务端是否正在运行
3. 端口是否被占用
4. 查看测试脚本输出的日志文件（*.log）

### Q: 可以在生产环境使用这些示例吗？

A: 这些示例主要用于学习和演示。在生产环境使用前，需要：
1. 添加完善的错误处理和重试机制
2. 实现健康检查和故障转移
3. 添加监控和告警
4. 实现配置版本管理
5. 添加安全性配置（TLS、认证等）

---

## 技术栈

- **框架**: yggdrasil v2
- **协议**: gRPC, xDS v3
- **配置**: YAML
- **日志**: slog (Go 1.21+)
- **文件监控**: fsnotify
- **xDS库**: envoy go-control-plane v3

---

## 扩展阅读

- [yggdrasil文档](https://github.com/codesjoy/yggdrasil)
- [Envoy xDS协议文档](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol)
- [gRPC文档](https://grpc.io/docs/)
- [Go控制平面库](https://github.com/envoyproxy/go-control-plane)

---

## 贡献

欢迎提交Issue和Pull Request来改进这些示例。

---

## 许可证

与yggdrasil项目保持一致。
