# Makefile + Shell Conventions (Reference)

This reference is intentionally short. Treat the repo itself as the source of truth.

## Makefile layout
- Root `Makefile` is a thin entrypoint:
  - Includes a "common" makefile early (often `scripts/make-rules/common.mk`).
  - Includes other modular make-rule files (often `scripts/make-rules/*.mk`).
  - Defines user-facing targets that forward to make-rule targets.
- Targets are documented with `## <target>: <desc>` for `make help`.
- Commands are typically silent (`@...`) and use repo logging helpers:
  - `@$(LOG_INFO) "message"`

### Key repo files to match
- Root `Makefile` entrypoint (often `Makefile`)
- Modular make-rule files (commonly `scripts/make-rules/*.mk` or `make/*.mk`)
- A "common" makefile for shared vars/functions (often `scripts/make-rules/common.mk`)
- Tooling/lint/test make-rules (repo-specific)

## Make-rule modules
- Naming: `<area>.<verb>` (e.g., `go.test`, `tools.install`, `buf.generate`).
- Dependencies: prefer `tools.verify.<tool>` for prerequisites.
- Use `@$(LOG_INFO) ...` at the start of each target.

## Bash scripts
- Shebang: `#!/usr/bin/env bash`
- Header: use the repo's chosen license boilerplate (or pick one and stay consistent).
- Strict mode:
  - `set -o errexit`
  - `set -o nounset`
  - `set -o pipefail`
- Initialize shared env and helpers:
  - Prefer sourcing helpers relative to the script file:
    - `source "$(dirname "${BASH_SOURCE[0]}")/<rel-path>"` (adjust relative path)
- Logging: if the repo provides logger functions, prefer them; otherwise use `echo`/`printf`.

## Buf/proto conventions (generic)
- Buf config typically lives at the repo root or in a dedicated proto workspace directory:
  - `buf.yaml`, `buf.gen.yaml`
- A minimal pattern:
  - `cd $(ROOT_DIR)/<workdir> && buf generate`
  - `cd $(ROOT_DIR)/<workdir> && buf lint`
  - `buf breaking` requires an explicit baseline via `--against` (choose your policy).
