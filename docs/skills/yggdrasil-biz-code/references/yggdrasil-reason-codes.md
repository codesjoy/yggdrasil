# Yggdrasil Reason Codes (Protobuf + Go Usage)

Yggdrasil supports structured error reasons via:
- proto options in `codesjoy/reason/v1/reason.proto`
- codegen via `protoc-gen-codesjoy-reason`

## Define a reason enum in proto
Example pattern (based on `example/proto/library/reason.proto`):
```proto
syntax = "proto3";

package your.org.your_service;

import "codesjoy/reason/v1/reason.proto";

enum Reason {
  // Mark this enum as the default reason enum for this package/domain.
  option (codesjoy.reason.v1.default_reason) = 500;

  // Do not use this default value.
  ERROR_REASON_UNSPECIFIED = 0;

  // Map a business reason to a google.rpc.Code (e.g., NOT_FOUND).
  BOOK_NOT_FOUND = 1 [(codesjoy.reason.v1.code) = NOT_FOUND];
}
```

## Generate reason helpers
Include `protoc-gen-codesjoy-reason` in your Buf generation, then run:
```bash
buf generate
```

## Return a reasoned error (server)
```go
return nil, xerror.WrapWithReason(
    errors.New("book not found"),
    pb.Reason_BOOK_NOT_FOUND,
    "",
    map[string]string{"book_id": req.BookId},
)
```
Generated reason enums satisfy `xerror.Reason` directly.

## Extract structured error info (client)
```go
st := status.FromError(err)
_ = st.Code()
_ = st.HTTPCode()
_ = st.ErrorInfo().Reason
```

## Repo references
- Reason enum example: `example/proto/library/reason.proto`
- Server-side usage: `example/sample/server/main.go`
- Client-side extraction: `example/sample/client/main.go`
