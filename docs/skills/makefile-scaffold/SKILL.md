---
name: makefile-scaffold
description: Scaffold and standardize Makefile targets, modular make-rule .mk files, and bash scripts using a common pattern (thin root Makefile, help-style ## comments, .PHONY, structured logging via $(LOG_INFO) or similar, forwarding to make-rule modules, strict-mode bash with init.sh + log::*). Use when creating/modifying Makefile targets, adding a new make-rule module, writing scripts/*.sh, or wiring buf/proto generate/lint/breaking into Makefile in any repo that already follows (or wants to adopt) this pattern.
---

# Makefile Scaffold

## Installation (User Environment)
This skill is meant to be installed into the user's tooling environment, not used only inside this repo.

To install, copy this skill folder into the skills directory used by your tool/platform.

Example (use any copy method you prefer):
```bash
SKILL_SRC="/path/to/makefile-scaffold"   # folder that contains SKILL.md
SKILL_DEST="/path/to/your/skills/makefile-scaffold"
rm -rf "$SKILL_DEST" && mkdir -p "$(dirname "$SKILL_DEST")"
cp -R "$SKILL_SRC" "$SKILL_DEST"
```

## Source Of Truth (Read First)

Align output with the current repo patterns by reading these files before drafting:
- Root Makefile entrypoint (often `Makefile`)
- Any included modular make-rule files (for example: `scripts/make-rules/*.mk` or `make/*.mk`)
- Shared shell helpers (for example: `scripts/lib/init.sh`, `scripts/lib/logger.sh`, etc.)
- Buf workspace configs if relevant (commonly `buf.yaml`, `buf.gen.yaml`)

If the repo does not have an established pattern yet, decide on the target layout first and then scaffold consistently.

## Minimal Requirements
- `make` (GNU Make recommended)
- `bash` (for generated scripts)
- `python3` (only required if you want to run the generator)

## Makefile Conventions (Top-Level)
- Keep root `Makefile` thin:
  - Put shared constants, vars, and helper functions in a "common" included makefile (example: `scripts/make-rules/common.mk`).
  - If you adopt the "common.mk-first" convention, keep it as the first include line.
  - Add other includes in modular make-rule files (e.g., `scripts/make-rules/*.mk`).
  - Define user-facing targets that forward to make-rule targets: `@$(MAKE) <area>.<verb> -j$(nproc)`
- Help style:
  - Document targets with `## <target>: <desc>` so `make help` can extract them.
  - Ensure each public target is `.PHONY`.
- Logging:
  - If the repo has logging helpers (for example `$(LOG_INFO)`), prefer them; otherwise use `@echo`.
- Tools:
  - If the repo has tooling make-rules, use `tools.verify.<tool>` / `tools.install.<tool>`; otherwise fall back to `command -v <tool>`.

## Make-Rule Module Conventions (`scripts/make-rules/<area>.mk`)
- Use the standard file header separators (`# ==============================================================================`).
- Name targets as `<area>.<verb>` (e.g., `buf.generate`, `buf.lint`).
- Use tool verification targets for prerequisites (e.g., `buf.*: tools.verify.buf`).
- Log each target with `@$(LOG_INFO) "..."`.

## Bash Script Conventions (`scripts/*.sh`)
- Start with:
  - `#!/usr/bin/env bash`
  - License header comments (use the repo's existing boilerplate, or choose a license and stay consistent)
- Strict mode:
  - `set -o errexit`
  - `set -o nounset`
  - `set -o pipefail`
- Always source project init and logging:
  - Prefer sourcing helpers relative to the script file: `source "$(dirname "${BASH_SOURCE[0]}")/<path>"` (adjust path as needed)
  - If the repo provides logger functions, use them; otherwise use `echo`/`printf`.

## Buf/Proto Default Targets
Recommended make-rule targets to scaffold:
- `buf.generate`: run `buf generate`
- `buf.lint`: run `buf lint`
- `buf.breaking`: run `buf breaking --against <baseline>`

Default directory strategy:
- Run Buf commands in the repo's Buf workspace (configure `--workdir` in the generator; default is `.`):
  - `cd $(ROOT_DIR)/<workdir> && buf generate`
  - `cd $(ROOT_DIR)/<workdir> && buf lint`
  - For breaking checks, choose a baseline (often the default branch or a tag).

## Go Tooling (Optional)
If you need minimal templates for Go formatting/linting/testing/tidying and CI gates, read `references/go-tooling-templates.md`.

## Scaffold Generator (Optional)
Use the bundled generator to produce a starting point, then edit to fit your module/layout.

From the skill directory (the folder that contains this `SKILL.md`):

Generate a make-rule module:
```bash
python3 scripts/scaffold.py mk --area buf --verbs generate,lint,breaking --workdir . --include-snippet
```

If your repo uses a different make-rule directory, override the suggested include path:
```bash
python3 scripts/scaffold.py mk --area buf --verbs generate,lint,breaking --include-snippet --include-path "make/{area}.mk"
```

Generate a bash script skeleton:
```bash
python3 scripts/scaffold.py sh --name proto-gen --rel-init-sh lib/init.sh --purpose "Generate code from protos"
```

When you need detailed conventions, read `references/conventions.md`.
