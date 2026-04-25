# Credentials To Security Profiles

`transport/support/credentials` has been replaced by `transport/support/security`.

## Configuration changes

- Remove `yggdrasil.transports.grpc.credentials`
- Remove service-level `yggdrasil.clients.services.<svc>.transports.grpc.credentials`
- Replace all `creds_proto` fields with `security_profile`
- Define reusable profiles under `yggdrasil.transports.security.profiles`

Example:

```yaml
yggdrasil:
  transports:
    security:
      profiles:
        insecure-default:
          type: insecure
        internal-mtls:
          type: tls
          config:
            min_version: "1.3"
    grpc:
      server:
        security_profile: internal-mtls
      client:
        transport:
          security_profile: insecure-default
    http:
      server:
        security_profile: internal-mtls
      client:
        security_profile: insecure-default
```

## Runtime changes

- Security providers are now resolved through the `security.profile.provider` capability.
- gRPC consumes compiled security profiles and converts them into gRPC transport credentials.
- `rpchttp` consumes the same profiles through `http.Transport`, `tls.Listener`, and per-request authentication.
