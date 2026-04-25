# Documentation

This directory contains the public documentation set for the Yggdrasil v3 framework.

## Architecture

- [Architecture Overview](overview.md) — framework layers, package map, and module hub summary

## Modularity

- [Module System Design](module-system.md) — module interfaces, hub lifecycle, DAG ordering, and capability model
- [Declarative Assembly Planning](assembly-planning.md) — config-driven module selection, overrides, and spec diffing

## Lifecycle

- [App Lifecycle & Business Composition](app-lifecycle.md) — state machine, hot reload, two-phase commit, and business bundle pattern

## Infrastructure

- [Layered Configuration System](configuration.md) — immutable snapshots, layered merging, and config change detection
- [Transport & Service Discovery](transport-and-discovery.md) — transport providers, security profiles, service registry, and load balancing

## Schemas

- [Module Hub Diagnostics Schema](schemas/module-hub-diagnostics.schema.json)
