# Layout, Packages, and `go_package`

This reference defines **repository structure** and **importable Go packages** for protos.

## Directory layout patterns

### Pattern A: single-service repo
```
proto/
  your/org/yourservice/v1/service.proto
  your/org/yourservice/v1/types.proto
```

### Pattern B: monorepo / multi-service
```
proto/
  your/org/service_a/v1/service.proto
  your/org/service_b/v1/service.proto
```

Pick one pattern and keep it consistent across services.

## `package` conventions
- Keep `package` aligned with the directory structure.
- Use lowercase segments.
- Put version as the last segment (e.g., `.v1`, `.v2`) when you need breaking version bumps.

Example:
```proto
package your.org.yourservice.v1;
```

## `go_package` conventions
`go_package` should be importable and stable:
- Use your module/repo path prefix.
- Include the semicolon alias suffix to control the Go package name if needed.

Example:
```proto
option go_package = "github.com/your-org/your-repo/gen/go/your/org/yourservice/v1;v1";
```

## Versioning approach
- Use `v1` for the initial stable API.
- Use `v2` only for breaking changes that cannot be made compatible.
- Avoid mixing incompatible versions in the same package.
