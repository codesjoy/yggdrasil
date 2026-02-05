# Buf Management (Framework-neutral)

Use Buf as the single source of truth for:
- module configuration (`buf.yaml`)
- lint rules (`buf lint`)
- breaking-change checks (`buf breaking`)
- code generation (`buf generate`)

This reference intentionally avoids framework-specific plugins. If you need Yggdrasil plugin wiring,
see `$yggdrasil-biz-code` (in this repo: `docs/skills/yggdrasil-biz-code/`).

## Recommended layout
```
proto/
  <org-or-domain>/
    <service>/
      v1/
        service.proto
      v1/
        types.proto
gen/                      # generated outputs (pick one directory and keep stable)
```

## `buf.yaml` (v2) template
```yaml
version: v2

modules:
  - path: ./proto

lint:
  use:
    - STANDARD

breaking:
  use:
    - FILE

deps:
  # Optional: only if you import google/api/annotations.proto (HTTP annotations, etc.).
  - buf.build/googleapis/googleapis

  # Optional: only if you use Protovalidate (buf.validate).
  - buf.build/bufbuild/protovalidate
```

## `buf.gen.yaml` (v2) template (Go)
```yaml
version: v2

managed:
  enabled: true
  override:
    # Adjust to your module prefix.
    - file_option: go_package_prefix
      value: github.com/your-org

plugins:
  - remote: buf.build/protocolbuffers/go
    out: ./gen/go
    opt: paths=source_relative

  # Optional: generate gRPC stubs.
  - remote: buf.build/grpc/go
    out: ./gen/go
    opt: paths=source_relative
```

## Commands (local + CI)
- Generate:
  - `buf generate`
- Lint:
  - `buf lint`
- Breaking checks (PR gate):
  - `buf breaking --against <baseline>`
    - Example baselines:
      - default branch: `--against '.git#branch=main'`
      - last release tag: `--against '.git#tag=v1.2.3'`

## CI recommendation
For every proto change, run:
1. `buf lint`
2. `buf breaking --against <baseline>`
3. `buf generate` (and fail if generated code drift is detected)

## References
- Buf docs: <https://buf.build/docs/>
