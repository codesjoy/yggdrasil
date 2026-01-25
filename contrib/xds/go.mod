module github.com/codesjoy/yggdrasil/contrib/xds/v2

go 1.24.0

toolchain go1.24.6

require (
	github.com/codesjoy/yggdrasil/v2 v2.0.0
	github.com/envoyproxy/go-control-plane v0.13.1
	google.golang.org/grpc v1.69.4
	google.golang.org/protobuf v1.36.11
)

require (
	cel.dev/expr v0.16.2 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cncf/xds/go v0.0.0-20240905190251-b4127c9b8d78 // indirect
	github.com/creasty/defaults v1.8.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20241015192408-796eee8c2d53 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/codesjoy/yggdrasil/v2 => ../..
