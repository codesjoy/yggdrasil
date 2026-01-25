module github.com/codesjoy/yggdrasil/contrib/xds/v2

go 1.24.0

toolchain go1.24.6

require (
	github.com/codesjoy/yggdrasil/v2 v2.0.0
	github.com/envoyproxy/go-control-plane v0.14.0
	github.com/envoyproxy/go-control-plane/envoy v1.36.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260122232226-8e98ce8d340d
	google.golang.org/grpc v1.78.0
	google.golang.org/protobuf v1.36.11
)

require (
	cel.dev/expr v0.24.0 // indirect
	github.com/cncf/xds/go v0.0.0-20251022180443-0feb69152e9f // indirect
	github.com/creasty/defaults v1.8.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260122232226-8e98ce8d340d // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/codesjoy/yggdrasil/v2 => ../..
