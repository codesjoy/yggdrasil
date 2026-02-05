---
name: protobuf-buf-guidelines
description: Use when defining, standardizing, or reviewing Protobuf governance (directory layout, package/go_package, naming, annotations, validation) and Buf-based management (buf.yaml/buf.gen.yaml, lint/breaking, codegen). Framework-neutral; refer to $yggdrasil-biz-code for Yggdrasil-specific integration.
---

# Protobuf Governance and Buf Management

## Scope
- Define or refactor `.proto` repository layout, package naming, and `go_package`.
- Standardize naming (files/messages/fields/enums/services/methods).
- Standardize API annotations as tooling (HTTP mapping, required fields, resources).
- Standardize validation using Protovalidate (`buf.validate`).
- Use Buf as the single source of truth for module config, lint/breaking, and code generation.

## Workflow (Definition -> Governance)
1. **Decide layout and packages**: define directory structure, `package`, and `go_package` (see `references/layout-and-packages.md`).
2. **Apply naming conventions**: enforce consistent naming for symbols and files (see `references/naming.md`).
3. **Apply API annotations (optional)**: use a small, standardized set of annotation protos (see `references/api-annotations.md`).
4. **Add validation (optional)**: use Protovalidate (`buf.validate`) for constraints (see `references/validation-protovalidate.md`).
5. **Configure Buf**: create `buf.yaml` + `buf.gen.yaml` as the authoritative config (see `references/buf-management.md`).
6. **Gate changes**: enforce `buf lint` and `buf breaking` in CI for every proto change (see `references/buf-management.md`).

## Output Requirements (Checklist)
- `package` and `go_package` are consistent and importable.
- Protos are organized in a stable layout; file and symbol names follow your conventions.
- If annotations are used, imports and deps are correctly configured (Buf deps).
- If validation is used, Protovalidate annotations are consistent and lintable.
- Buf lint/breaking passes (or has explicit and justified exceptions).

## When to Load References
- Layout and packages: `references/layout-and-packages.md`
- Naming conventions: `references/naming.md`
- API annotations: `references/api-annotations.md`
- Validation (Protovalidate): `references/validation-protovalidate.md`
- Buf config/templates/CI commands: `references/buf-management.md`
