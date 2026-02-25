# Cross-Cutting Concerns (Errors / Metadata / Interceptors)

## Errors (Reason + Code)
Use `xerror.WrapWithReason` to return an error with a reason:
```go
return nil, xerror.WrapWithReason(
    errors.New("business error"),
    pb.Reason_INVALID_INPUT,
    "",
    map[string]string{"field": "name"},
)
```
- `Reason_*` typically comes from generated code (reason enum).
- Generated reason enums implement `xerror.Reason` directly.

## Metadata (Header / Trailer)
```go
_ = metadata.SetHeader(ctx, metadata.Pairs("request-id", "xxx"))
_ = metadata.SetTrailer(ctx, metadata.Pairs("trace", "abc"))
```

## Interceptors (logging example)
Enable in `config.yaml`:
```yaml
yggdrasil:
  interceptor:
    unary_server: "logging"
    stream_server: "logging"
    config:
      logging:
        print_req_and_res: true
```

## Repo references
- Error handling example: `example/advanced/error-handling/server/main.go`
- Metadata example: `example/advanced/metadata/server/main.go`
- Sample server config: `example/sample/server/config.yaml`
