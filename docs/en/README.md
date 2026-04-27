# Yggdrasil v3 Documentation

This directory contains the optimized English documentation set for Yggdrasil v3. It targets application developers, module authors, framework maintainers, and operators.

## Document Status

| Document | Status | Notes |
|---|---|---|
| 00. Glossary | Stable | Terminology alignment |
| 01. Architecture Overview and Design Principles | Stable | Long-term architecture baseline |
| 02. Module Hub and Capability Model | Stable | Hub, DAG, Capability, Scope, diagnostics |
| 03. Bootstrap, Auto Assembly, and Planning | Design Baseline | Planning, default selection, chain templates, Spec |
| 04. Application Lifecycle and Business Composition | Design Baseline | Prepare, Compose, Install, Start, Stop |
| 05. Configuration, Declarative Assembly, and Hot Reload | Design Baseline | Snapshots, Planner, Diff, Staged Reload |
| 06. Transport, Service Discovery, and Observability | Design Baseline | transport, registry, resolver, balancer, observability |
| 07. Developer Practices and Extension Guide | Guide | Templates, checklists, anti-patterns, troubleshooting |
| 08. Implementation Boundaries and Optimization Notes | Guide | Tests, error shape, reload boundary, production semantics |

## Recommended Reading Paths

| Audience | Recommended order |
|---|---|
| Application developers | 00 -> 07 -> 04 -> 05 -> 06 |
| Module authors | 00 -> 02 -> 03 -> 07 -> 05 |
| Framework maintainers | 00 -> 01 -> 02 -> 03 -> 04 -> 05 -> 06 -> 08 |
| Operators | 00 -> 05 -> 07 -> 02 -> 08 |

## Documents

1. [Glossary](00-glossary.md)
2. [Architecture Overview and Design Principles](01-architecture-overview-and-design-principles.md)
3. [Module Hub and Capability Model](02-module-hub-and-capability-model.md)
4. [Bootstrap, Auto Assembly, and Planning](03-bootstrap-auto-assembly-and-planning.md)
5. [Application Lifecycle and Business Composition](04-application-lifecycle-and-business-composition.md)
6. [Configuration, Declarative Assembly, and Hot Reload](05-configuration-declarative-assembly-and-hot-reload.md)
7. [Transport, Service Discovery, and Observability](06-transport-discovery-and-observability.md)
8. [Developer Practices and Extension Guide](07-developer-practices-and-extension-guide.md)
9. [Implementation Boundaries and Optimization Notes](08-implementation-boundaries-and-optimization-notes.md)

## Maintenance Rules

- English and Chinese docs should be updated together.
- Update the glossary before adding new design terminology.
- Proposal-level APIs must explicitly state implementation status.
- Default selection, reload classification, and error semantics should be explainable through diagnostics.
