<a name="v3.0.0-rc.5"></a>
## [v3.0.0-rc.5](https://github.com/codesjoy/yggdrasil/compare/v3.0.0-rc.4...v3.0.0-rc.5) (2026-09-28)

### Features
- add governor routes and routable listen defaults

### Fixes
- **transport:** serialize builtin codec registration

<a name="v3.0.0-rc.4"></a>
## [v3.0.0-rc.4](https://github.com/codesjoy/yggdrasil/compare/v3.0.0-rc.3...v3.0.0-rc.4) (2026-08-12)

### Features
- expose xerror carrier semantics

### Fixes
- preserve run context values during shutdown
- **transport:** preserve certificate verification callbacks
- **transport:** cancel stream context after handling

<a name="v3.0.0-rc.3"></a>
## [v3.0.0-rc.3](https://github.com/codesjoy/yggdrasil/compare/v3.0.0-rc.2...v3.0.0-rc.3) (2026-07-26)

### Fixes
- **rest:** preserve query binding compatibility

<a name="v3.0.0-rc.2"></a>
## [v3.0.0-rc.2](https://github.com/codesjoy/yggdrasil/compare/v3.0.0-rc.1...v3.0.0-rc.2) (2026-04-28)

### Features
- add root signal handling controls
- **config:** support bootstrap config sources

### Refactor
- require explicit app names

### BREAKING CHANGE

app names are no longer read from configuration or WithAppName; callers must pass the name to app.New, yggdrasil.New, or yggdrasil.Run explicitly.

<a name="v3.0.0-rc.1"></a>
## [v3.0.0-rc.1](https://github.com/codesjoy/yggdrasil/compare/v2.0.0...v3.0.0-rc.1) (2026-04-27)

### Features
- **app:** make runtime state app-local by default
- **app:** add provider-only capability registrations
- **assembly:** add business bundle bootstrap flow
- **config:** add env placeholder interpolation for file sources
- **examples:** add custom service cron example
- **governor:** harden admin surface and decouple lifecycle
- **module:** introduce hub foundation and provider runtime
- **transport:** replace credentials with security profiles

### Fixes
- harden lifecycle shutdown and startup cleanup
- **app:** run app cleanup through lifecycle
- **client:** correct fast_fail override and tighten state/retry paths
- **client:** recover from transient endpoint failures
- **credentials:** validate configs and local handshakes
- **http:** normalize marshaler negotiation
- **lifecycle:** harden startup and runtime failure handling
- **logger:** harden slog encoding and builder concurrency
- **observability:** harden telemetry stats behavior
- **rest:** handle structured path bindings and mixed request population
- **rest:** omit unused rest import
- **rpc:** fix streaming codegen and imports
- **server:** harden lifecycle and isolate governor metadata routes
- **settings:** validate resolved config before applying globals

### Refactor
- remove legacy process-default facades
- **app:** tidy capability registration plumbing
- **app:** rename Open to New and require explicit managers
- **app:** extract app and config internals into focused packages
- **client:** finish runtime extraction and cleanup
- **config:** replace legacy config with layered manager model
- **example:** migrate examples to bundle bootstrap API
- **example:** migrate examples to the v3 app API
- **grpc:** rewrite protocol runtime on official grpc-go APIs
- **layout:** regroup public packages by domain
- **layout:** normalize settings and type filenames
- **logger:** switch handlers to slog defaults
- **observability:** nest framework logging and telemetry config
- **public:** promote status and transport support helpers
- **remote:** internalize logger and drop legacy shims
- **runtime:** switch root and server to app-scoped runtime
- **transport:** split transport packages by layer

### BREAKING CHANGE

remove deprecated root instance helpers and governor compatibility entrypoints.

- remove legacy process-default instance facade exports
- remove governor package-level compatibility registration
- rename internal process-default instance helpers
- move remaining root package tests into yggdrasil_test.go

remove remote/logger and legacy exported helpers from remote/credentials

<a name="v2.0.0"></a>
## [v2.0.0](https://github.com/codesjoy/yggdrasil/compare/v2.0.0-rc.8...v2.0.0) (2026-04-27)

### Fixes
- harden lifecycle shutdown and startup cleanup
- **client:** recover from transient endpoint failures
- **credentials:** validate configs and local handshakes
- **http:** normalize marshaler negotiation
- **lifecycle:** harden startup and runtime failure handling
- **logger:** harden slog encoding and builder concurrency
- **rest:** omit unused rest import
- **rest:** handle structured path bindings and mixed request population
- **rpc:** fix streaming codegen and imports

<a name="v2.0.0-rc.8"></a>
## [v2.0.0-rc.8](https://github.com/codesjoy/yggdrasil/compare/v2.0.0-rc.7...v2.0.0-rc.8) (2026-03-24)

### Fixes
- **client:** report successful pick results

<a name="v2.0.0-rc.7"></a>
## [v2.0.0-rc.7](https://github.com/codesjoy/yggdrasil/compare/v2.0.0-rc.6...v2.0.0-rc.7) (2026-03-21)

### Features
- **grpc:** add jsonraw passthrough codec
- **grpc:** add raw codec call options

<a name="v2.0.0-rc.6"></a>
## [v2.0.0-rc.6](https://github.com/codesjoy/yggdrasil/compare/v2.0.0-rc.5...v2.0.0-rc.6) (2026-03-04)

### Features
- migrate to xerror-first error model and reason codegen
- **config:** integrate bootstrap config chain with source builders

### Fixes
- **logger:** preserve slog attrs/groups and isolate pooled encoders
- **runtime:** harden lifecycle, registration, and reconnect paths

### Refactor
- **core:** migrate utils to pkg/utils and consolidate helpers
- **example:** use framework bootstrap config loading
- **lint:** resolve strict lint debt and stabilize examples

### BREAKING CHANGE

removed status business APIs and local reason generator.

Use xerror for local error creation and classification.

<a name="v2.0.0-rc.5"></a>
## [v2.0.0-rc.5](https://github.com/codesjoy/yggdrasil/compare/v2.0.0-rc.4...v2.0.0-rc.5) (2026-02-07)

### Fixes
- **etcd:** ensure Register cleans up resources on initial put failure
- **k8s:** add missing ObjectMeta to Endpoints in test
- **xds:** use endpoint address as key when available in balancer

### Refactor
- extract reason.proto as independent status module

<a name="v2.0.0-rc.2"></a>
## [v2.0.0-rc.2](https://github.com/codesjoy/yggdrasil/compare/v2.0.0-rc.1...v2.0.0-rc.2) (2026-01-26)

### Features
- automatically apply default settings to plugins
- integrate xDS protocol for Service Mesh and control plane support
- integrate etcd as service registry, resolver and config source
- integrate Kubernetes-native service discovery and configuration
- add HTTP as new remote transport protocol
- implement credential validation and enable TLS
- refactor to support multi-registry
- set default values for instance hierarchy fields
- extract actual listen address for gRPC server
- **contrib-otlp:** add OTLP export support
- **contrib-polaris:** initialize independent module for examples
- **contrib-polaris:** support discovery, config, and governance

### Fixes
- resolve nil pointer panic in REST code generator
- resolve issue preventing graceful shutdown
- **contrib-xds:** correct module path to include v2 suffix in example

### Refactor
- reorganize rest and marshal directory structure
- unify Close/Stop conventions and add startup validation
- add Type() and update Name() semantics in config/source.Source
- support multiple instances of the same balancer type
- unify plugin type identification logic
- **contrib-xds:** polish implementation and enrich examples
- **example:** refactor error-handling implementation
- **example:** refactor error-handling implementation

<a name="v2.0.0-rc.1"></a>
## v2.0.0-rc.1 (2026-01-08)

### Features
- support UnmarshalText in mapstructure decoder
- add gRPC support
- add remote credentials module
- add Protobuf definitions and implementation demo
- implement Stringer interface for State type
- create scoped logger for independent level control
- add protobuf generators for yggdrasil (reason, rpc, rest)
- integrate core modules into the yggdrasil entry package
- add application module
- add reason proto
- add registry module
- add server module for unified lifecycle and server abstraction
- add governor and REST server modules
- implement core request invocation logic
- support direct assignment in Scan for same types
- add the stats module
- add the instance
- add the stream and status module
- add xgo and xmap utils
- add the interceptor module
- add the opentelemetry module
- add the metadata module
- add the logger module
- add the config module

### Fixes
- correct mapstructure tag typo
- incorrect output format in console logger handler
- add mutex lock to RegisterBuilder and GetBuilder for thread-safety
- precise watcher removal to avoid deleting wrong handles
- resolve error when renaming files during file watch
- correct remote client state transition logic
- return ErrNoAvailableInstance when endpoint list is empty
- handle empty string for TimeEncoder and TimeHandler config
- correct default value logic for config.Scan
- fix formatting in boilerplate.go.txt
- handle empty module arrays in bash script
- **example:** align sample yaml config with latest schema

### Refactor
- define OnStateChange type
- reorganize framwork directory structure
- rename StreamHandler to Handler and StreamDesc to Desc
- refactor and optimize the status module


[v3.0.0-rc.5]: https://github.com/codesjoy/yggdrasil/compare/v3.0.0-rc.4...v3.0.0-rc.5
[v3.0.0-rc.4]: https://github.com/codesjoy/yggdrasil/compare/v3.0.0-rc.3...v3.0.0-rc.4
[v3.0.0-rc.3]: https://github.com/codesjoy/yggdrasil/compare/v3.0.0-rc.2...v3.0.0-rc.3
[v3.0.0-rc.2]: https://github.com/codesjoy/yggdrasil/compare/v3.0.0-rc.1...v3.0.0-rc.2
[v3.0.0-rc.1]: https://github.com/codesjoy/yggdrasil/compare/v2.0.0...v3.0.0-rc.1
[v2.0.0]: https://github.com/codesjoy/yggdrasil/compare/v2.0.0-rc.8...v2.0.0
[v2.0.0-rc.8]: https://github.com/codesjoy/yggdrasil/compare/v2.0.0-rc.7...v2.0.0-rc.8
[v2.0.0-rc.7]: https://github.com/codesjoy/yggdrasil/compare/v2.0.0-rc.6...v2.0.0-rc.7
[v2.0.0-rc.6]: https://github.com/codesjoy/yggdrasil/compare/v2.0.0-rc.5...v2.0.0-rc.6
[v2.0.0-rc.5]: https://github.com/codesjoy/yggdrasil/compare/v2.0.0-rc.4...v2.0.0-rc.5
[v2.0.0-rc.2]: https://github.com/codesjoy/yggdrasil/compare/v2.0.0-rc.1...v2.0.0-rc.2
