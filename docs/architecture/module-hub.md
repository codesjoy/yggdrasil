# Module Hub Architecture

The module hub remains the center of runtime composition in Yggdrasil v3.

## Responsibilities

- register modules
- validate dependencies
- resolve named and ordered capabilities
- coordinate initialization, start, stop, and reload phases
- surface diagnostics and conflict information

## Core Concepts

- `module.Module`: minimum identity contract.
- `module.Dependent`: declares hard dependencies.
- `module.Initializable`, `Startable`, `Stoppable`: lifecycle hooks.
- `module.Reloadable`, `ReloadCommitter`, `ReloadReporter`: staged reload protocol.
- `module.Capability`, `module.CapabilitySpec`: typed extension surface owned by the relevant subsystem.

## Relationship to `app`

`app.App` owns the runtime lifecycle, while `module.Hub` owns capability and dependency orchestration.
`app` compiles configuration, selects modules, builds the runtime snapshot, and delegates capability resolution to the hub.

## Diagnostics

The diagnostics schema used by tooling lives at:

- [`docs/schemas/module-hub-diagnostics.schema.json`](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/docs/schemas/module-hub-diagnostics.schema.json)
