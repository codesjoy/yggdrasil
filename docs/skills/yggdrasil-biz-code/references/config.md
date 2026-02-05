# Server Configuration (`config.yaml`)

## Minimal example
```yaml
yggdrasil:
  server:
    protocol:
      - "grpc"

  remote:
    protocol:
      grpc:
        address: "127.0.0.1:55879"
```

## Common fields
```yaml
yggdrasil:
  rest:
    enable: true
    port: 3000
    middleware:
      all:
        - "logger"

  interceptor:
    unary_server: "logging"
    stream_server: "logging"
    config:
      logging:
        print_req_and_res: true

  logger:
    handler:
      default:
        type: "console"
        level: "debug"
```

## Repo references
- Example config: `example/sample/server/config.yaml`
- Config explanation: `example/sample/server/README.md`
