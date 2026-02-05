# API Annotations (Tooling-focused)

This reference standardizes a small set of common annotations as **tooling building blocks**.
It does not prescribe any particular “Google API design” approach.

## HTTP mapping: `google.api.http` (optional)
If you expose REST endpoints, you can map RPCs to HTTP routes:
```proto
import "google/api/annotations.proto";

rpc GetBook(GetBookRequest) returns (Book) {
  option (google.api.http) = { get: "/v1/{name=books/*}" };
}
```

Notes:
- Use `additional_bindings` for multiple routes.
- Only add HTTP annotations when you actually need REST exposure.

Buf dependency (only if you import these protos):
- `buf.build/googleapis/googleapis`

## Field behavior: `google.api.field_behavior`
Use `REQUIRED` to mark required fields (tooling and humans can consume it):
```proto
import "google/api/field_behavior.proto";

message GetBookRequest {
  string name = 1 [(google.api.field_behavior) = REQUIRED];
}
```

## Resources (optional): `google.api.resource` / `google.api.resource_reference`
If you model resource identifiers, you can annotate messages/fields:
```proto
import "google/api/resource.proto";

message Book {
  option (google.api.resource) = {
    type: "example.googleapis.com/Book"
    pattern: "books/{book}"
  };
  string name = 1;
}
```

```proto
import "google/api/resource.proto";

message GetBookRequest {
  string name = 1 [(google.api.resource_reference).type = "example.googleapis.com/Book"];
}
```

## Method signature: `google.api.method_signature` (optional)
Use it if your tooling benefits from “primary fields” for method calls:
```proto
import "google/api/client.proto";

rpc GetBook(GetBookRequest) returns (Book) {
  option (google.api.method_signature) = "name";
}
```
