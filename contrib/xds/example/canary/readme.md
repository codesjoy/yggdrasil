# 金丝雀部署示例

本示例演示如何使用xDS实现渐进式金丝雀部署，安全地将流量从稳定版本逐步切换到新版本。

## 目录结构

```
canary/
├── client/
│   ├── main.go      # 客户端实现，模拟渐进式流量切换
│   └── config.yaml  # 客户端xDS配置
└── server/
    ├── main.go      # 服务端实现，支持稳定版和金丝雀版
    └── config.yaml  # 服务端配置
```

## 功能特性

- **渐进式流量切换**: 从5%逐步增加到100%
- **多阶段验证**: 每个阶段有明确的流量比例和验证时间
- **实时监控**: 每个阶段完成后输出流量分布统计
- **安全回滚**: 可以随时回滚到稳定版本

## 运行步骤

### 1. 启动xDS控制平面

```bash
cd contrib/xds/example/control-plane
set XDS_CONFIG_FILE=canary-xds-config.yaml
go run main.go --config config.yaml
```

### 2. 启动稳定版本服务

```bash
cd contrib/xds/example/canary/server
set DEPLOYMENT_TYPE=stable
go run main.go --config config.yaml
```

### 3. 启动金丝雀版本服务

打开新终端：

```bash
cd contrib/xds/example/canary/server
set DEPLOYMENT_TYPE=canary
set PORT=55556
go run main.go --config config.yaml
```

### 4. 运行客户端

```bash
cd contrib/xds/example/canary/client
go run main.go --config config.yaml
```

## 预期输出

客户端将模拟6个阶段的渐进式流量切换：

```
2025/01/26 10:00:00 INFO Starting canary deployment client...
2025/01/26 10:00:00 INFO Starting canary deployment test with progressive traffic increase...
2025/01/26 10:00:00 INFO Starting stage - stage_name: Stage 1: 5% to stable, canary_percentage: 5
2025/01/26 10:00:00 INFO Stage completed - stage_name: Stage 1: 5% to stable, expected_canary_percentage: 5, actual_canary_percentage: 5.20, total_requests: 100, stable_count: 95, canary_count: 5
2025/01/26 10:00:00 INFO Waiting before next stage... - seconds: 5
2025/01/26 10:00:05 INFO Starting stage - stage_name: Stage 2: 10% to stable, canary_percentage: 10
2025/01/26 10:00:05 INFO Stage completed - stage_name: Stage 2: 10% to stable, expected_canary_percentage: 10, actual_canary_percentage: 9.80, total_requests: 100, stable_count: 90, canary_count: 10
...
2025/01/26 10:00:25 INFO Stage completed - stage_name: Stage 6: 100% to stable, expected_canary_percentage: 100, actual_canary_percentage: 100.00, total_requests: 100, stable_count: 0, canary_count: 100
2025/01/26 10:00:25 INFO Canary deployment test completed successfully
```

## 金丝雀部署策略

### 阶段1：初步验证（5%）
- **目的**: 验证金丝雀版本基本功能正常
- **持续时间**: 5-10分钟
- **关注指标**: 错误率、响应时间

### 阶段2：小范围扩展（10%）
- **目的**: 扩大验证范围，发现潜在问题
- **持续时间**: 10-15分钟
- **关注指标**: 系统资源使用、数据库连接

### 阶段3：中等规模（25%）
- **目的**: 验证中等负载下的表现
- **持续时间**: 15-20分钟
- **关注指标**: 并发处理能力、缓存命中率

### 阶段4：大规模验证（50%）
- **目的**: 验证高负载下的稳定性
- **持续时间**: 20-30分钟
- **关注指标**: 系统瓶颈、依赖服务压力

### 阶段5：接近完成（75%）
- **目的**: 准备全面切换
- **持续时间**: 15-20分钟
- **关注指标**: 所有关键指标

### 阶段6：全面切换（100%）
- **目的**: 完成金丝雀部署
- **持续时间**: 持续监控
- **关注指标**: 长期稳定性

## 配置说明

### xDS配置 (canary-xds-config.yaml)

控制平面配置使用`canary-xds-config.yaml`，其中定义了金丝雀权重：

```yaml
endpoints:
  - clusterName: "library-cluster"
    endpoints:
      - address: "127.0.0.1"
        port: 55555
        weight: 95      # 稳定版本权重95%
      - address: "127.0.0.1"
        port: 55556
        weight: 5       # 金丝雀版本权重5%
```

通过修改权重比例，可以动态调整流量分配。

## 最佳实践

### 1. 监控指标

关键指标包括：
- **错误率**: 金丝雀版本不应高于稳定版
- **响应时间**: P95和P99延迟不应显著增加
- **资源使用**: CPU、内存、网络使用量
- **业务指标**: 订单成功率、用户留存等

### 2. 回滚策略

当以下情况发生时立即回滚：
- 错误率超过阈值（如1%）
- 响应时间显著增加（如>50%）
- 系统资源耗尽
- 业务指标异常

### 3. 部署前检查

- 代码审查完成
- 自动化测试通过
- 性能基准测试
- 安全扫描通过
- 文档更新完成

### 4. 部署后验证

- 检查所有监控指标
- 验证关键业务流程
- 收集用户反馈
- 更新部署文档

## 常见问题

Q: 金丝雀部署需要多长时间？
A: 取决于业务流量和验证需求，通常2-4小时。

Q: 如何决定每个阶段的持续时间？
A: 根据业务流量大小和风险容忍度，高流量服务需要更长的验证时间。

Q: 可以跳过某些阶段吗？
A: 可以，但会降低安全性。建议至少经过小规模、中规模、大规模三个阶段。

Q: 金丝雀版本失败后如何回滚？
A: 将xDS配置中的金丝雀权重设置为0即可快速回滚。

Q: 多个服务同时金丝雀部署？
A: 不推荐。应该逐个服务进行金丝雀部署，避免相互影响。

Q: 如何处理数据迁移？
A: 金丝雀部署期间需要确保数据版本兼容，通常需要实现双写或兼容层。
