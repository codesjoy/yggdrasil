# 00. Yggdrasil v3 Glossary

> This glossary keeps terminology consistent across the English and Chinese documentation. Code identifiers are not translated.

| Term | Meaning | Chinese equivalent |
|---|---|---|
| App | Application-level runtime entry | App / 应用实例 |
| App-local runtime | Runtime state scoped to one App, not process globals | App 本地运行时 |
| process-default facade | Compatibility facade for legacy or single-main-App programs | 进程默认兼容门面 |
| Module Hub | Container for long-lived modules and capabilities | Module Hub / 模块中心 |
| Module | Long-lived carrier of lifecycle behavior and capabilities | Module / 模块 |
| Capability | Typed contract published by modules and resolved by consumers | Capability / 能力 |
| assembly.Spec | Public declarative assembly plan | `assembly.Spec` |
| prepared runtime assembly | Internal in-memory runtime graph | prepared runtime assembly / 已准备运行时装配图 |
| BusinessBundle | Formal output of business composition | `BusinessBundle` |
| Runtime narrow surface | Safe framework surface exposed to business composition | Runtime 窄接口 |
| Staged Reload | Prepare / commit / rollback reload protocol | 分阶段热重载 |
| restart-required | Change cannot be safely hot-reloaded | 需要重启 |
| degraded | Reload rollback failed or runtime diverged | 降级状态 |
| provider | Object that exposes a capability or factory | provider / 提供者 |
| resolver watch | Dynamic service-discovery watch owned by client runtime | resolver watch |
