module github.com/codesjoy/yggdrasil/contrib/xds/example

go 1.24.6

replace (
	github.com/codesjoy/yggdrasil/contrib/xds/example/control-plane/server => ./control-plane
	github.com/codesjoy/yggdrasil/contrib/xds/example/control-plane/snapshot => ./control-plane
	github.com/codesjoy/yggdrasil/contrib/xds/example/control-plane/watcher => ./control-plane
	github.com/codesjoy/yggdrasil/contrib/xds/v2 => ../
	github.com/codesjoy/yggdrasil/v2 => ../../../
	github.com/codesjoy/yggdrasil/v2/example/protogen => ../../../example/protogen
)

require (
	github.com/codesjoy/yggdrasil/v2 v2.0.0-00010101000000-000000000000
	github.com/codesjoy/yggdrasil/v2/example/protogen v0.0.0-00010101000000-000000000000
	github.com/envoyproxy/go-control-plane v0.14.0
	github.com/envoyproxy/go-control-plane/envoy v1.36.0
	github.com/fsnotify/fsnotify v1.9.0
	google.golang.org/grpc v1.78.0
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cel.dev/expr v0.24.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20251022180443-0feb69152e9f // indirect
	github.com/creasty/defaults v1.8.0 // indirect
	github.com/envoyproxy/go-control-plane/ratelimit v0.1.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/go-chi/chi/v5 v5.2.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.39.0 // indirect
	go.opentelemetry.io/otel/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/trace v1.39.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260122232226-8e98ce8d340d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260122232226-8e98ce8d340d // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)
