# Yggdrasil Agent Instructions

These instructions are the execution overlay for coding agents working in this
repository. Read the detailed, self-contained engineering rules in
[`docs/engineering-standards.md`](docs/engineering-standards.md) before making
non-trivial changes. When a rule is not summarized here, follow that document.
Where the two disagree, the engineering standard wins.

## Operating Rules

- Inspect before editing: find the owning package, read its tests and its
  configuration, and check whether the code is generated.
- Preserve unrelated worktree changes. Never reset, restore, or overwrite files
  outside the requested scope.
- Keep the change narrowly scoped, and reuse the interfaces and helpers that
  already exist. Do not add an abstraction that only forwards a standard-library
  call.
- Do not commit secrets, credentials, connection strings, or real tokens, and do
  not print them in output, tests, or documentation.
- Report the changed files, the verification commands actually run, and any
  remaining limitation. Never claim a verification that was not performed.

## Repository Layout

Yggdrasil is a multi-module Go workspace whose root module is a library. There is
no `main` package at the root. `go.work` is a local, untracked convenience.

```text
.                              root library (github.com/codesjoy/yggdrasil/v3)
cmd/protoc-gen-yggdrasil-rest  REST protoc plugin
cmd/protoc-gen-yggdrasil-rpc   RPC protoc plugin
examples                       runnable samples
examples/protogen              generated protobuf
app/                           App, Runtime, BusinessBundle
module/                        Hub, Module, capability, DAG, scope
assembly/                      planner, spec, default selection
config/                        layered configuration and typed views
capabilities/                  built-in capability contracts
transport/                     transport, REST gateway, client/server runtime
rpc/ observability/ discovery/ interceptors, logging/telemetry, registry/resolver
admin/governor/                governor admin HTTP server
internal/                      module-private helpers
docs/engineering-standards.md  normative engineering standard
docs/en/  docs/zh_CN/          descriptive architecture, mirrored pairs
```

`go.work` is local and untracked. The task runner discovers modules from their
`go.mod` files; add a module by adding its `go.mod`, and run
`go work use ./new/module` locally for IDE support.

## Architecture Boundaries

The full rules are in the engineering standard, §2 and §5.

- The Hub holds long-lived modules and capabilities only. Business services,
  handlers, tasks, hooks, and extensions go into `BusinessBundle`, never the Hub.
- A module implements only `Name()`. Everything else is an optional interface with
  a type assertion. Never widen `module.Module` or any existing exported
  interface.
- Declare hard dependencies with `DependsOn()`, never `InitOrder()`.
- `Prepare()` must not listen, serve, accept, or register a service instance.
  `Start()` does the serving. `Stop()` must be idempotent.
- Capability conflicts — cardinality or type mismatch, duplicate `NamedOne` — are
  hard errors, not warnings. Resolve providers through `ResolveExactlyOne`,
  `ResolveOptionalOne`, `ResolveMany`, `ResolveNamed`, or `ResolveOrdered`.
- A `ScopeRuntimeFactory` module must not be registered in the Hub.
- Runtime state is App-local by default. A module needing `slog.Default()` or
  OTel globals must declare `IsolationReporter` and the App must set
  `WithProcessDefaults(true)`.
- A `Capabilities()` callback may run before `Init` and more than once, so it must
  be deterministic and side-effect free.

## Coding Rules

- `golangci-lint` is the single formatting authority (`gofmt`, `gofumpt`,
  `goimports`, `golines`). Never run a standalone formatter. Lines are at most 100
  characters; do not hand-align.
- Exported identifiers need English GoDoc; the `revive` `exported` rule enforces
  it. Acronyms stay capitalised (`ID`, `URL`, `HTTP`, `RPC`, `REST`, `TLS`).
- Errors crossing a service boundary use `github.com/codesjoy/pkg/basic/xerror`.
  Framework and planner errors use `assembly.Error`. Always preserve the cause.
- `context.Context` is the first parameter and is never stored in a long-lived
  struct.
- No side effects in `init()`, no package-level provider registries.
- Generated protobuf is never hand-edited; change the generator and run
  `task proto:generate`.

## Verification Commands

Commands run from the repository root. Bootstrap once per clone with `task setup`;
it pins the tools into `./bin` and installs the git hooks.

```sh
task fmt:check        # verify formatting without rewriting
task go:lint          # run the configured golangci-lint profile per module
task test             # unit tests, examples excluded by default
task coverage         # coverage gate (COVERAGE=80, non-example modules only)
task deps:tidy:check  # verify go.mod/go.sum are tidy
task check:fast       # fmt:check + go:lint + test
task check            # check:fast + coverage + deps:tidy:check
task check:strict     # check + race tests, examples included
task doctor           # toolchain, pinned tools, git hooks
```

A change must pass `task check` before handoff. Add `task check:strict` when the
change touches the runtime hot path or concurrency. Focus a single module with
`task test MODULES=cmd/protoc-gen-yggdrasil-rpc`. `task --list` shows every target.

## Change Workflow

1. Identify the package that owns the behaviour. If no package owns it, that is a
   design question, not an implementation detail.
2. Read the configuration surface, the interfaces, and the generated-file
   boundaries before writing code.
3. Make the smallest coherent change, reusing existing interfaces.
4. Add regression coverage at the narrowest useful layer.
5. Regenerate only when a generator input changed.
6. Run the verification proportional to the risk.
7. Review the diff for scope creep and for files that should not be committed.
8. Summarize with file references and the exact commands that were run.

## Commit and Review Rules

Commit messages, branch names, and the review checklist are specified in the
engineering standard §9. In short:

- Conventional Commits, enforced by gitlint: `feat(scope): subject`. The subject
  is lowercase, at least eight characters, and does not end with a period. The
  title is at most 72 characters.
- Branch names are `main`, `master`, `develop`, or
  `(feature|release|hotfix)/<name>`.
- A breaking change carries `!` and a `BREAKING CHANGE:` footer, and must be
  called out explicitly in the review.
