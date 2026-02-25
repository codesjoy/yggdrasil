---
name: yggdrasil-biz-code
description: Use when implementing or modifying server-side business code with the Yggdrasil framework, implementing proto RPC methods, configuring and starting a yggdrasil service, or when you need examples for REST/interceptors/metadata/error handling.
---

# Yggdrasil Business Development (Server-Side)

## Quick Triage
- Identify whether the request is about server implementation, service registration, config loading, REST support, or cross-cutting concerns (errors/metadata/interceptors).
- If it is only about client calls or test scripts, prefer client-side examples (not the core scope of this skill).

## Primary Workflow (Minimum Viable Path)
1. Confirm proto definitions and code generation approach (see `references/quickstart.md`).
2. Implement the server struct and RPC methods (see `references/service-impl.md`).
3. Prepare and load `config.yaml` (see `references/config.md`).
4. Call `yggdrasil.Init` in `main` to initialize the application name.
5. Register service descriptors with `yggdrasil.Serve` and start the server.

## Common Extensions
- REST support: add `WithRestServiceDesc`, and optionally register custom HTTP routes via `WithRestRawHandleDesc` (see `references/service-impl.md`).
- Structured errors: return errors with a reason via `xerror.WrapWithReason` (see `references/crosscutting.md`).
- Metadata: set response metadata via `metadata.SetHeader/SetTrailer` (see `references/crosscutting.md`).
- Interceptors: enable logging and other interceptors in config (see `references/crosscutting.md`).

## When to Load References
- Proto/codegen and quick start: `references/quickstart.md`
- Buf generation for Yggdrasil plugins: `references/protobuf-codegen.md`
- REST annotations for yggdrasil REST generator: `references/yggdrasil-rest-annotations.md`
- Reason codes integration: `references/yggdrasil-reason-codes.md`
- Service implementation/registration/REST examples: `references/service-impl.md`
- Error handling, metadata, interceptor configuration: `references/crosscutting.md`
- Minimal and common config: `references/config.md`
