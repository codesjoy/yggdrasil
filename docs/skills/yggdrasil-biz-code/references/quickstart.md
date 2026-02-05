# Quick Start (proto -> codegen -> serve)

## Minimal steps
1. Define the proto: `service` + request/response messages.
2. Generate code: use Yggdrasil's protoc plugins to generate RPC (and optionally REST).
3. Implement the server: embed `Unimplemented*Server` and implement the RPC methods.
4. Load config: `config.LoadSource(file.NewSource("./config.yaml", false))`.
5. Initialize and serve: `yggdrasil.Init` + `yggdrasil.Serve`.

## Code generation example
```bash
# Generate RPC code
protoc --go_out=. --go_opt=paths=source_relative \
  --yggdrasil-rpc_out=. --yggdrasil-rpc_opt=paths=source_relative \
  your_service.proto

# Generate REST code (optional)
protoc --yggdrasil-rest_out=. --yggdrasil-rest_opt=paths=source_relative \
  your_service.proto
```

## Repo references
- Overview and quick start: `README.md` / `README_CN.md`
- Sample server: `example/sample/server/main.go`
- Sample server guide: `example/sample/server/README.md`
