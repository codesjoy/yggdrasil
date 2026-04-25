module github.com/codesjoy/yggdrasil/v3/examples/protogen

go 1.25.7

replace github.com/codesjoy/yggdrasil/v3 => ../../

require (
	github.com/codesjoy/pkg/basic/xerror v0.0.0-20260225033528-924cf61d0622
	github.com/codesjoy/yggdrasil/v3 v3.0.0-00010101000000-000000000000
	github.com/go-chi/chi/v5 v5.2.4
	google.golang.org/genproto/googleapis/api v0.0.0-20260122232226-8e98ce8d340d
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260122232226-8e98ce8d340d
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/codesjoy/pkg/utils v0.0.0-20260227125603-faf7bfdf00a7 // indirect
	github.com/creasty/defaults v1.8.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	google.golang.org/genproto v0.0.0-20251222181119-0a764e51fe1b // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
