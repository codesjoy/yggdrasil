# Yggdrasil v3 中文文档集

本目录包含优化后的 Yggdrasil v3 中文文档集。本文档集面向业务应用开发者、模块作者、框架维护者和运维人员。

## 文档状态

| 文档 | 状态 | 说明 |
|---|---|---|
| 00. 术语表 | Stable | 中英文术语对齐 |
| 01. 架构总览与设计原则 | Stable | 长期架构基线 |
| 02. 模块中心 Hub 与 Capability 模型 | Stable | Hub、DAG、Capability、Scope 和 diagnostics |
| 03. Bootstrap 自动装配与规划系统 | Design Baseline | 规划、默认选择、链模板、Spec |
| 04. 应用生命周期与业务组合 | Design Baseline | Prepare、Compose、Install、Start、Stop |
| 05. 配置、声明式装配与热重载 | Design Baseline | 配置快照、Planner、Diff、Staged Reload |
| 06. 传输、服务发现与可观测性 | Design Baseline | transport、registry、resolver、balancer、observability |
| 07. 开发者实践与扩展指南 | Guide | 模板、checklist、反模式、排障 |
| 08. 实施边界与优化补充 | Guide | 测试、错误格式、reload 边界、生产语义 |

## 推荐阅读路径

| 角色 | 推荐顺序 |
|---|---|
| 业务应用开发者 | 00 -> 07 -> 04 -> 05 -> 06 |
| 模块作者 | 00 -> 02 -> 03 -> 07 -> 05 |
| 框架维护者 | 00 -> 01 -> 02 -> 03 -> 04 -> 05 -> 06 -> 08 |
| 运维人员 | 00 -> 05 -> 07 -> 02 -> 08 |

## 文档列表

1. [术语表](00-术语表.md)
2. [架构总览与设计原则](01-架构总览与设计原则.md)
3. [模块中心 Hub 与 Capability 模型](02-模块中心Hub与Capability模型.md)
4. [Bootstrap 自动装配与规划系统](03-Bootstrap自动装配与规划系统.md)
5. [应用生命周期与业务组合](04-应用生命周期与业务组合.md)
6. [配置、声明式装配与热重载](05-配置声明式装配与热重载.md)
7. [传输、服务发现与可观测性](06-传输服务发现与可观测性.md)
8. [开发者实践与扩展指南](07-开发者实践与扩展指南.md)
9. [实施边界与优化补充](08-实施边界与优化补充.md)

## 维护规则

- 中英文文档应同步更新。
- 新增设计概念时先更新术语表。
- 所有 proposal-level API 必须标注实现状态。
- 所有默认选择、reload 分类、错误语义都应有 explain / diagnostics 支撑。
