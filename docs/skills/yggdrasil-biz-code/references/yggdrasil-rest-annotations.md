# REST Annotations for the Yggdrasil REST Generator

This reference is specific to `protoc-gen-yggdrasil-rest`.

## Dependency
The generator consumes `google.api.http` annotations:
- proto import: `google/api/annotations.proto`
- Buf dep (if using Buf): `buf.build/googleapis/googleapis`

## Minimal example
```proto
import "google/api/annotations.proto";

service GreeterService {
  rpc SayHello(SayHelloRequest) returns (SayHelloReply) {
    option (google.api.http) = {
      post: "/v1/hello"
      body: "*"
    };
  }
}
```

## Generator constraints (enforced)
Derived from `cmd/protoc-gen-yggdrasil-rest/rest.go`:
- **GET/DELETE**: `body` must be empty (do not declare a body).
- **POST/PUT/PATCH**: `body` must be declared (either `*` or a field name).
- `additional_bindings` is supported.

### Practical patterns
- Whole request as body:
  - `body: "*"`
- Single field as body:
  - `body: "resource"`
  - Ensure the field exists in the request message.

## Example with `additional_bindings`
```proto
rpc GetThing(GetThingRequest) returns (Thing) {
  option (google.api.http) = {
    get: "/v1/{name=things/*}"
    additional_bindings { get: "/v1/{name=projects/*/things/*}" }
  };
}
```

## Repo references
- Generator implementation: `cmd/protoc-gen-yggdrasil-rest/rest.go`
- REST usage example: `example/advanced/rest/server/main.go`
