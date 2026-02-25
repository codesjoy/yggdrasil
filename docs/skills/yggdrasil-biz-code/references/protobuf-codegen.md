# Protobuf Code Generation for Yggdrasil (Buf-based)

This reference focuses on generating the Protobuf outputs that Yggdrasil consumes:
- RPC descriptors for server/client wiring (`*ServiceDesc`)
- REST descriptors/handlers (`*RestServiceDesc`, `*_rest.pb.go`) when using `google.api.http`
- Reason-code helpers (`*_reason.pb.go`) when using yggdrasil reason enums

## What Yggdrasil consumes from generated code
On the server side, you register generated descriptors with `yggdrasil.Serve(...)`:
- RPC: `yggdrasil.WithServiceDesc(&pb.YourServiceServiceDesc, impl)`
- REST (optional): `yggdrasil.WithRestServiceDesc(&pb.YourServiceRestServiceDesc, impl)`

See repo example:
- `example/sample/server/main.go`

## Generate with Buf (recommended)
Use Buf as the single source of truth:
- `buf.yaml`: module + deps + lint/breaking rules
- `buf.gen.yaml`: codegen plugins and output directories

Repo examples:
- `example/buf.yaml`
- `example/buf.gen.yaml`

Run:
```bash
buf generate
```

## Minimal Yggdrasil-flavored `buf.gen.yaml` snippet
This is aligned with the repo example and assumes the plugins are installed in `PATH`.
```yaml
version: v2

plugins:
  - local: protoc-gen-go
    out: ./protogen
    opt: paths=source_relative

  # Shared reason plugin from codesjoy/pkg (optional)
  - local: protoc-gen-codesjoy-reason
    out: ./protogen
    opt: paths=source_relative

  # Yggdrasil: RPC stubs
  - local: protoc-gen-yggdrasil-rpc
    out: ./protogen
    opt: paths=source_relative

  # Yggdrasil: REST handlers from google.api.http (optional)
  - local: protoc-gen-yggdrasil-rest
    out: ./protogen
    opt: paths=source_relative
```

If you do not use REST or reason codes, remove the corresponding plugin blocks.

## Tooling prerequisites
- Install Yggdrasil protoc plugins (in this repo: `make install`)
- Ensure `protoc-gen-go` is available in `PATH`

## Common pitfalls
- REST codegen requires `google/api/annotations.proto` in your module deps and valid `google.api.http` rules.
- Keep `out` directories stable to avoid churn in imports and builds.
