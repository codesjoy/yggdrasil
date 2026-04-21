// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package settings holds the typed framework configuration schema and resolved views.
package settings

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"

	"github.com/codesjoy/yggdrasil/v2/balancer"
	"github.com/codesjoy/yggdrasil/v2/client"
	"github.com/codesjoy/yggdrasil/v2/config"
	configbootstrap "github.com/codesjoy/yggdrasil/v2/config/bootstrap"
	"github.com/codesjoy/yggdrasil/v2/governor"
	"github.com/codesjoy/yggdrasil/v2/interceptor"
	"github.com/codesjoy/yggdrasil/v2/internal/instance"
	"github.com/codesjoy/yggdrasil/v2/logger"
	xotel "github.com/codesjoy/yggdrasil/v2/otel"
	"github.com/codesjoy/yggdrasil/v2/registry"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/remote/credentials"
	"github.com/codesjoy/yggdrasil/v2/remote/marshaler"
	grpcprotocol "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc"
	protocolhttp "github.com/codesjoy/yggdrasil/v2/remote/protocol/http"
	"github.com/codesjoy/yggdrasil/v2/remote/rest"
	"github.com/codesjoy/yggdrasil/v2/remote/rest/middleware"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/server"
	"github.com/codesjoy/yggdrasil/v2/stats"
)

// Root is the top-level framework configuration schema.
type Root struct {
	Yggdrasil Framework `mapstructure:"yggdrasil"`
}

// Framework contains the framework-owned configuration tree.
type Framework struct {
	Bootstrap  configbootstrap.Settings `mapstructure:"bootstrap"`
	Server     server.Settings          `mapstructure:"server"`
	Transports Transports               `mapstructure:"transports"`
	Clients    Clients                  `mapstructure:"clients"`
	Discovery  Discovery                `mapstructure:"discovery"`
	Balancers  Balancers                `mapstructure:"balancers"`
	Logging    logger.Settings          `mapstructure:"logging"`
	Telemetry  Telemetry                `mapstructure:"telemetry"`
	Admin      Admin                    `mapstructure:"admin"`
}

// Transports contains global transport settings.
type Transports struct {
	GRPC GRPCTransport `mapstructure:"grpc"`
	HTTP HTTPTransport `mapstructure:"http"`
}

// GRPCTransport contains global gRPC transport settings.
type GRPCTransport struct {
	Client      grpcprotocol.Config       `mapstructure:"client"`
	Server      grpcprotocol.ServerConfig `mapstructure:"server"`
	Credentials map[string]map[string]any `mapstructure:"credentials"`
}

// GRPCClientTransport contains service-level gRPC client overrides.
type GRPCClientTransport struct {
	grpcprotocol.Config `mapstructure:",squash"`
	Credentials         map[string]map[string]any `mapstructure:"credentials"`
}

// HTTPTransport contains global HTTP transport settings.
type HTTPTransport struct {
	Client protocolhttp.ClientConfig `mapstructure:"client"`
	Server protocolhttp.ServerConfig `mapstructure:"server"`
	Rest   *rest.Config              `mapstructure:"rest"`
}

// ClientTransports contains service-level transport overrides.
type ClientTransports struct {
	GRPC GRPCClientTransport       `mapstructure:"grpc"`
	HTTP protocolhttp.ClientConfig `mapstructure:"http"`
}

// ClientDefaults contains default client values applied to services.
type ClientDefaults struct {
	client.ServiceConfig `mapstructure:",squash"`
	Transports           ClientTransports `mapstructure:"transports"`
}

// ClientServiceSpec contains one configured client service subtree.
type ClientServiceSpec struct {
	client.ServiceConfig `mapstructure:",squash"`
	Transports           ClientTransports `mapstructure:"transports"`
}

// Clients contains all client settings.
type Clients struct {
	Defaults ClientDefaults               `mapstructure:"defaults"`
	Services map[string]ClientServiceSpec `mapstructure:"services"`
}

// Discovery contains registry and resolver settings.
type Discovery struct {
	Registry  registry.Spec            `mapstructure:"registry"`
	Resolvers map[string]resolver.Spec `mapstructure:"resolvers"`
}

// Balancers contains balancer defaults and per-service overrides.
type Balancers struct {
	Defaults map[string]balancer.Spec            `mapstructure:"defaults"`
	Services map[string]map[string]balancer.Spec `mapstructure:"services"`
}

// Telemetry contains framework telemetry settings.
type Telemetry struct {
	Tracer string         `mapstructure:"tracer"`
	Meter  string         `mapstructure:"meter"`
	Stats  stats.Settings `mapstructure:"stats"`
}

// Validation contains startup validation flags.
type Validation struct {
	Strict bool `mapstructure:"strict"`
	Enable bool `mapstructure:"enable"`
}

// Admin contains framework admin settings.
type Admin struct {
	Application instance.Config `mapstructure:"application"`
	Governor    governor.Config `mapstructure:"governor"`
	Validation  Validation      `mapstructure:"validation"`
}

// Resolved contains normalized settings ready for module configuration.
type Resolved struct {
	Root       Root
	Server     server.Settings
	Logging    logger.Settings
	Discovery  Discovery
	Balancers  Balancers
	Clients    client.Settings
	Transports ResolvedTransports
	Telemetry  Telemetry
	Admin      Admin
}

// ResolvedTransports contains normalized transport settings.
type ResolvedTransports struct {
	GRPC                   grpcprotocol.Settings
	HTTP                   protocolhttp.Settings
	Rest                   *rest.Config
	GRPCCredentials        map[string]map[string]any
	GRPCServiceCredentials map[string]map[string]map[string]any
}

// Catalog provides typed section accessors over a config manager.
type Catalog struct {
	manager *config.Manager
}

// NewCatalog binds a manager to the framework typed catalog.
func NewCatalog(manager *config.Manager) Catalog {
	if manager == nil {
		manager = config.Default()
	}
	return Catalog{manager: manager}
}

// Root returns the root typed section.
func (c Catalog) Root() config.Section[Root] {
	return config.Bind[Root](c.manager)
}

// Server returns the framework server section.
func (c Catalog) Server() config.Section[server.Settings] {
	return config.Bind[server.Settings](c.manager, "yggdrasil", "server")
}

// Transports returns the framework transport section.
func (c Catalog) Transports() config.Section[Transports] {
	return config.Bind[Transports](c.manager, "yggdrasil", "transports")
}

// ClientService returns a single typed client service section.
func (c Catalog) ClientService(name string) config.Section[ClientServiceSpec] {
	return config.Bind[ClientServiceSpec](c.manager, "yggdrasil", "clients", "services", name)
}

// LoggingHandler returns a single typed logging handler section.
func (c Catalog) LoggingHandler(name string) config.Section[logger.HandlerSpec] {
	return config.Bind[logger.HandlerSpec](c.manager, "yggdrasil", "logging", "handlers", name)
}

// LoggingWriter returns a single typed logging writer section.
func (c Catalog) LoggingWriter(name string) config.Section[logger.WriterSpec] {
	return config.Bind[logger.WriterSpec](c.manager, "yggdrasil", "logging", "writers", name)
}

// Registry returns the typed registry section.
func (c Catalog) Registry() config.Section[registry.Spec] {
	return config.Bind[registry.Spec](c.manager, "yggdrasil", "discovery", "registry")
}

// Resolver returns a single typed resolver section.
func (c Catalog) Resolver(name string) config.Section[resolver.Spec] {
	return config.Bind[resolver.Spec](c.manager, "yggdrasil", "discovery", "resolvers", name)
}

// DecodePayload decodes an arbitrary payload map into the provided target.
func DecodePayload(target any, value any) error {
	return config.NewSnapshot(value).Decode(target)
}

// Compile normalizes the raw framework root into per-module resolved settings.
func Compile(root Root) (Resolved, error) {
	fw := root.Yggdrasil
	resolved := Resolved{
		Root:      root,
		Server:    fw.Server,
		Logging:   fw.Logging,
		Discovery: fw.Discovery,
		Balancers: fw.Balancers,
		Telemetry: fw.Telemetry,
		Admin:     fw.Admin,
		Clients: client.Settings{
			Services: map[string]client.ServiceConfig{},
		},
		Transports: ResolvedTransports{
			GRPC: grpcprotocol.Settings{
				Client:         fw.Transports.GRPC.Client,
				ClientServices: map[string]grpcprotocol.Config{},
				Server:         fw.Transports.GRPC.Server,
			},
			HTTP: protocolhttp.Settings{
				Client:         fw.Transports.HTTP.Client,
				ClientServices: map[string]protocolhttp.ClientConfig{},
				Server:         fw.Transports.HTTP.Server,
			},
			Rest:                   fw.Transports.HTTP.Rest,
			GRPCCredentials:        cloneNestedMap(fw.Transports.GRPC.Credentials),
			GRPCServiceCredentials: map[string]map[string]map[string]any{},
		},
	}

	if resolved.Logging.Handlers == nil {
		resolved.Logging.Handlers = map[string]logger.HandlerSpec{}
	}
	if resolved.Logging.Writers == nil {
		resolved.Logging.Writers = map[string]logger.WriterSpec{}
	}
	if resolved.Logging.Interceptors == nil {
		resolved.Logging.Interceptors = map[string]map[string]any{}
	}
	defaultHandler := resolved.Logging.Handlers["default"]
	if defaultHandler.Type == "" {
		defaultHandler.Type = "text"
	}
	if defaultHandler.Writer == "" {
		defaultHandler.Writer = "default"
	}
	resolved.Logging.Handlers["default"] = defaultHandler
	if _, ok := resolved.Logging.Writers["default"]; !ok {
		resolved.Logging.Writers["default"] = logger.WriterSpec{Type: "console"}
	}
	if resolved.Logging.RemoteLevel == "" {
		resolved.Logging.RemoteLevel = "error"
	}

	if resolved.Discovery.Resolvers == nil {
		resolved.Discovery.Resolvers = map[string]resolver.Spec{}
	}
	if resolved.Balancers.Defaults == nil {
		resolved.Balancers.Defaults = map[string]balancer.Spec{}
	}
	if resolved.Balancers.Services == nil {
		resolved.Balancers.Services = map[string]map[string]balancer.Spec{}
	}
	if fw.Clients.Services == nil {
		fw.Clients.Services = map[string]ClientServiceSpec{}
	}

	resolved.Server.RestEnabled = resolved.Transports.Rest != nil
	for serviceName, spec := range fw.Clients.Services {
		resolved.Clients.Services[serviceName] = mergeClientServiceConfig(fw.Clients.Defaults.ServiceConfig, spec.ServiceConfig)
		resolved.Transports.GRPC.ClientServices[serviceName] = mergeGRPCClientConfig(
			fw.Transports.GRPC.Client,
			spec.Transports.GRPC.Config,
		)
		resolved.Transports.HTTP.ClientServices[serviceName] = mergeHTTPClientConfig(
			fw.Transports.HTTP.Client,
			spec.Transports.HTTP,
		)
		if len(spec.Transports.GRPC.Credentials) != 0 {
			resolved.Transports.GRPCServiceCredentials[serviceName] = cloneNestedMap(spec.Transports.GRPC.Credentials)
		}
	}

	return resolved, nil
}

// Validate validates the resolved framework configuration.
func Validate(resolved Resolved) error {
	strict := resolved.Admin.Validation.Strict
	enable := strict || resolved.Admin.Validation.Enable
	if !enable {
		return nil
	}

	var multiErr error
	addErr := func(msg string, err error, attrs ...slog.Attr) {
		if err == nil {
			return
		}
		if strict {
			multiErr = errors.Join(multiErr, fmt.Errorf("%s: %w", msg, err))
			return
		}
		attrs = append(attrs, slog.Any("error", err))
		args := make([]any, 0, len(attrs))
		for _, a := range attrs {
			args = append(args, a)
		}
		slog.Warn(msg, args...)
	}

	if typeName := resolved.Discovery.Registry.Type; typeName != "" && registry.GetBuilder(typeName) == nil {
		addErr("registry builder not found", fmt.Errorf("type=%s", typeName), slog.String("type", typeName))
	}
	for name, spec := range resolved.Discovery.Resolvers {
		if spec.Type == "" {
			continue
		}
		if !resolver.HasBuilder(spec.Type) {
			addErr("resolver builder not found", fmt.Errorf("type=%s", spec.Type), slog.String("name", name))
		}
	}
	if tracerName := resolved.Telemetry.Tracer; tracerName != "" {
		if _, ok := xotel.GetTracerProviderBuilder(tracerName); !ok {
			addErr("tracer provider builder not found", fmt.Errorf("name=%s", tracerName), slog.String("name", tracerName))
		}
	}
	if meterName := resolved.Telemetry.Meter; meterName != "" {
		if _, ok := xotel.GetMeterProviderBuilder(meterName); !ok {
			addErr("meter provider builder not found", fmt.Errorf("name=%s", meterName), slog.String("name", meterName))
		}
	}
	for _, protocol := range resolved.Server.Transports {
		if remote.GetServerBuilder(protocol) == nil {
			addErr("remote server builder not found", fmt.Errorf("protocol=%s", protocol), slog.String("protocol", protocol))
		}
	}
	validateCredential := func(protoName, serviceName string, client bool, key string) {
		if protoName == "" {
			return
		}
		builder := credentials.GetBuilder(protoName)
		if builder == nil {
			addErr(
				"remote credentials builder not found",
				fmt.Errorf("name=%s", protoName),
				slog.String("name", protoName),
				slog.String("key", key),
			)
			return
		}
		if builder(serviceName, client) == nil {
			addErr(
				"remote credentials config invalid",
				fmt.Errorf("name=%s", protoName),
				slog.String("name", protoName),
				slog.String("key", key),
			)
		}
	}
	validateCredential(
		resolved.Transports.GRPC.Server.CredsProto,
		"",
		false,
		"yggdrasil.transports.grpc.server.creds_proto",
	)
	validateCredential(
		resolved.Transports.GRPC.Client.Transport.CredsProto,
		"",
		true,
		"yggdrasil.transports.grpc.client.transport.creds_proto",
	)
	for serviceName, cfg := range resolved.Transports.GRPC.ClientServices {
		validateCredential(
			cfg.Transport.CredsProto,
			serviceName,
			true,
			fmt.Sprintf("yggdrasil.clients.services.%s.transports.grpc.transport.creds_proto", serviceName),
		)
	}
	for _, name := range resolved.Server.Interceptors.Unary {
		if !interceptor.HasUnaryServerIntBuilder(name) {
			addErr("unary server interceptor not found", fmt.Errorf("name=%s", name), slog.String("name", name))
		}
	}
	for _, name := range resolved.Server.Interceptors.Stream {
		if !interceptor.HasStreamServerIntBuilder(name) {
			addErr("stream server interceptor not found", fmt.Errorf("name=%s", name), slog.String("name", name))
		}
	}
	for _, name := range resolved.Root.Yggdrasil.Clients.Defaults.Interceptors.Unary {
		if !interceptor.HasUnaryClientIntBuilder(name) {
			addErr("unary client interceptor not found", fmt.Errorf("name=%s", name), slog.String("name", name))
		}
	}
	for _, name := range resolved.Root.Yggdrasil.Clients.Defaults.Interceptors.Stream {
		if !interceptor.HasStreamClientIntBuilder(name) {
			addErr("stream client interceptor not found", fmt.Errorf("name=%s", name), slog.String("name", name))
		}
	}
	for serviceName, cfg := range resolved.Clients.Services {
		for _, name := range cfg.Interceptors.Unary {
			if !interceptor.HasUnaryClientIntBuilder(name) {
				addErr(
					"unary client interceptor not found",
					fmt.Errorf("name=%s", name),
					slog.String("name", name),
					slog.String("app", serviceName),
				)
			}
		}
		for _, name := range cfg.Interceptors.Stream {
			if !interceptor.HasStreamClientIntBuilder(name) {
				addErr(
					"stream client interceptor not found",
					fmt.Errorf("name=%s", name),
					slog.String("name", name),
					slog.String("app", serviceName),
				)
			}
		}
	}
	if resolved.Transports.Rest != nil {
		for _, name := range resolved.Transports.Rest.Middleware.All {
			if !middleware.HasBuilder(name) {
				addErr("rest middleware not found", fmt.Errorf("name=%s", name), slog.String("name", name))
			}
		}
		for _, name := range resolved.Transports.Rest.Middleware.RPC {
			if !middleware.HasBuilder(name) {
				addErr("rest middleware not found", fmt.Errorf("name=%s", name), slog.String("name", name))
			}
		}
		for _, name := range resolved.Transports.Rest.Middleware.Web {
			if !middleware.HasBuilder(name) {
				addErr("rest middleware not found", fmt.Errorf("name=%s", name), slog.String("name", name))
			}
		}
		if !middleware.HasBuilder("marshaler") {
			addErr("rest middleware not found", fmt.Errorf("name=marshaler"), slog.String("name", "marshaler"))
		}
		schemes := resolved.Transports.Rest.Marshaler.Support
		if len(schemes) == 0 {
			schemes = []string{marshaler.SchemeJSONPb}
		}
		for _, scheme := range schemes {
			if !marshaler.HasMarshallerBuilder(scheme) {
				addErr(
					"rest marshaler builder not found",
					fmt.Errorf("scheme=%s", scheme),
					slog.String("scheme", scheme),
				)
			}
		}
	}
	return multiErr
}

func mergeClientServiceConfig(base, overlay client.ServiceConfig) client.ServiceConfig {
	out := base
	if overlay.FastFail {
		out.FastFail = true
	}
	if overlay.Resolver != "" {
		out.Resolver = overlay.Resolver
	}
	if overlay.Balancer != "" {
		out.Balancer = overlay.Balancer
	}
	if !reflect.DeepEqual(overlay.Backoff, reflect.Zero(reflect.TypeOf(overlay.Backoff)).Interface()) {
		out.Backoff = overlay.Backoff
	}
	if len(overlay.Remote.Endpoints) > 0 {
		out.Remote.Endpoints = overlay.Remote.Endpoints
	}
	if out.Remote.Attributes == nil {
		out.Remote.Attributes = map[string]any{}
	}
	for key, value := range overlay.Remote.Attributes {
		out.Remote.Attributes[key] = value
	}
	out.Interceptors.Unary = dedupStrings(append(append([]string{}, base.Interceptors.Unary...), overlay.Interceptors.Unary...))
	out.Interceptors.Stream = dedupStrings(append(append([]string{}, base.Interceptors.Stream...), overlay.Interceptors.Stream...))
	return out
}

func mergeHTTPClientConfig(base, overlay protocolhttp.ClientConfig) protocolhttp.ClientConfig {
	out := base
	if overlay.Timeout > 0 {
		out.Timeout = overlay.Timeout
	}
	if overlay.Marshaler != nil {
		out.Marshaler = overlay.Marshaler
	}
	return out
}

func mergeGRPCClientConfig(base, overlay grpcprotocol.Config) grpcprotocol.Config {
	out := base
	if overlay.WaitConnTimeout > 0 {
		out.WaitConnTimeout = overlay.WaitConnTimeout
	}
	if overlay.ConnectTimeout > 0 {
		out.ConnectTimeout = overlay.ConnectTimeout
	}
	if overlay.MaxSendMsgSize > 0 {
		out.MaxSendMsgSize = overlay.MaxSendMsgSize
	}
	if overlay.MaxRecvMsgSize > 0 {
		out.MaxRecvMsgSize = overlay.MaxRecvMsgSize
	}
	if overlay.Compressor != "" {
		out.Compressor = overlay.Compressor
	}
	if overlay.BackOffMaxDelay > 0 {
		out.BackOffMaxDelay = overlay.BackOffMaxDelay
	}
	if overlay.MinConnectTimeout > 0 {
		out.MinConnectTimeout = overlay.MinConnectTimeout
	}
	if overlay.Network != "" {
		out.Network = overlay.Network
	}
	out.Transport = mergeGRPCTransportConfig(base.Transport, overlay.Transport)
	return out
}

func mergeGRPCTransportConfig(base, overlay grpcprotocol.ClientTransportOptions) grpcprotocol.ClientTransportOptions {
	out := base
	if overlay.UserAgent != "" {
		out.UserAgent = overlay.UserAgent
	}
	if overlay.CredsProto != "" {
		out.CredsProto = overlay.CredsProto
	}
	if overlay.Authority != "" {
		out.Authority = overlay.Authority
	}
	if !reflect.ValueOf(overlay.KeepaliveParams).IsZero() {
		out.KeepaliveParams = overlay.KeepaliveParams
	}
	if overlay.InitialWindowSize != 0 {
		out.InitialWindowSize = overlay.InitialWindowSize
	}
	if overlay.InitialConnWindowSize != 0 {
		out.InitialConnWindowSize = overlay.InitialConnWindowSize
	}
	if overlay.WriteBufferSize != 0 {
		out.WriteBufferSize = overlay.WriteBufferSize
	}
	if overlay.ReadBufferSize != 0 {
		out.ReadBufferSize = overlay.ReadBufferSize
	}
	if overlay.MaxHeaderListSize != nil {
		out.MaxHeaderListSize = overlay.MaxHeaderListSize
	}
	return out
}

func cloneNestedMap(src map[string]map[string]any) map[string]map[string]any {
	if src == nil {
		return map[string]map[string]any{}
	}
	out := make(map[string]map[string]any, len(src))
	for key, value := range src {
		cloned := make(map[string]any, len(value))
		for k, v := range value {
			cloned[k] = v
		}
		out[key] = cloned
	}
	return out
}

func dedupStrings(values []string) []string {
	values = slices.DeleteFunc(values, func(item string) bool { return item == "" })
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, item := range values {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
