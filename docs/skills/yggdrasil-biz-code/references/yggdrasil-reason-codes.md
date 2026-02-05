# Yggdrasil Reason Codes (Protobuf + Go Usage)

Yggdrasil supports structured error reasons via:
- proto options in `codesjoy/yggdrasil/reason/reason.proto`
- codegen via `protoc-gen-yggdrasil-reason`

## Define a reason enum in proto
Example pattern (based on `example/proto/library/reason.proto`):
```proto
syntax = "proto3";

package your.org.your_service;

import "codesjoy/yggdrasil/reason/reason.proto";

enum Reason {
  // Mark this enum as the default reason enum for this package/domain.
  option (codesjoy.yggdrasil.reason.default_reason) = 500;

  // Do not use this default value.
  ERROR_REASON_UNSPECIFIED = 0;

  // Map a business reason to a google.rpc.Code (e.g., NOT_FOUND).
  BOOK_NOT_FOUND = 1 [(codesjoy.yggdrasil.reason.code) = NOT_FOUND];
}
```

## Generate reason helpers
Include `protoc-gen-yggdrasil-reason` in your Buf generation, then run:
```bash
buf generate
```

## Return a reasoned error (server)
```go
return nil, status.FromReason(
    errors.New("book not found"),
    pb.Reason_BOOK_NOT_FOUND,
    map[string]string{"book_id": req.BookId},
)
```

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
