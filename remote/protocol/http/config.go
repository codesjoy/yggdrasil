package http

import "time"

type MarshalerConfig struct {
	Type   string            `mapstructure:"type"`
	Config *JSONPbConfigOpts `mapstructure:"config"`
}

type JSONPbConfigOpts struct {
	MarshalOptions struct {
		Multiline         bool   `mapstructure:"multiline"`
		Indent            string `mapstructure:"indent"`
		AllowPartial      bool   `mapstructure:"allow_partial"`
		UseProtoNames     bool   `mapstructure:"use_proto_names"`
		UseEnumNumbers    bool   `mapstructure:"use_enum_numbers"`
		EmitUnpopulated   bool   `mapstructure:"emit_unpopulated"`
		EmitDefaultValues bool   `mapstructure:"emit_default_values"`
	} `mapstructure:"marshal_options"`
	UnmarshalOptions struct {
		AllowPartial   bool `mapstructure:"allow_partial"`
		DiscardUnknown bool `mapstructure:"discard_unknown"`
		RecursionLimit int  `mapstructure:"recursion_limit"`
	} `mapstructure:"unmarshal_options"`
}

type ClientConfig struct {
	Timeout   time.Duration       `mapstructure:"timeout" default:"10s"`
	Marshaler *MarshalerConfigSet `mapstructure:"marshaler"`
}

type MarshalerConfigSet struct {
	Inbound  *MarshalerConfig `mapstructure:"inbound"`
	Outbound *MarshalerConfig `mapstructure:"outbound"`
}

type ServerConfig struct {
	Network      string              `mapstructure:"network" default:"tcp"`
	Address      string              `mapstructure:"address" default:":0"`
	ReadTimeout  time.Duration       `mapstructure:"read_timeout" default:"0s"`
	WriteTimeout time.Duration       `mapstructure:"write_timeout" default:"0s"`
	IdleTimeout  time.Duration       `mapstructure:"idle_timeout" default:"0s"`
	MaxBodyBytes int64               `mapstructure:"max_body_bytes" default:"4194304"`
	Marshaler    *MarshalerConfigSet `mapstructure:"marshaler"`
	Attr         map[string]string   `mapstructure:"attr"`
}
