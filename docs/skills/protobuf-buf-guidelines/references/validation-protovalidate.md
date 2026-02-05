# Validation with Protovalidate (`buf.validate`)

This reference standardizes validation rules using Protovalidate (a.k.a. `buf.validate`).

## Dependency
If you use validation annotations, add this to `buf.yaml` deps:
- `buf.build/bufbuild/protovalidate`

And import:
```proto
import "buf/validate/validate.proto";
```

## Field validation examples
```proto
import "buf/validate/validate.proto";

message CreateUserRequest {
  string email = 1 [(buf.validate.field).string.email = true];
  string user_id = 2 [(buf.validate.field).string.uuid = true];
  string name = 3 [(buf.validate.field).string = {min_len: 1, max_len: 64}];
}
```

## CEL examples (advanced)
If you need cross-field constraints, use CEL rules on the message:
```proto
import "buf/validate/validate.proto";

message CreateUserRequest {
  string email = 1;
  string password = 2;

  option (buf.validate.message).cel = {
    id: "password_min_len"
    message: "password must be at least 8 characters"
    expression: "size(this.password) >= 8"
  };
}
```

## Runtime integration (keep minimal)
How validation is enforced depends on your language/runtime. This skill standardizes the **annotations**.
Wire the runtime validator in your application framework as needed.
