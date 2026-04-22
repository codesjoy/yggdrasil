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
	"time"

	"github.com/codesjoy/yggdrasil/v3/balancer"
	"github.com/codesjoy/yggdrasil/v3/client"
	"github.com/codesjoy/yggdrasil/v3/config"
	configbootstrap "github.com/codesjoy/yggdrasil/v3/config/bootstrap"
	"github.com/codesjoy/yggdrasil/v3/governor"
	"github.com/codesjoy/yggdrasil/v3/internal/instance"
	"github.com/codesjoy/yggdrasil/v3/logger"
	"github.com/codesjoy/yggdrasil/v3/registry"
	grpcprotocol "github.com/codesjoy/yggdrasil/v3/remote/transport/grpc"
	rpchttp "github.com/codesjoy/yggdrasil/v3/remote/transport/rpchttp"
	"github.com/codesjoy/yggdrasil/v3/resolver"
	"github.com/codesjoy/yggdrasil/v3/server"
	"github.com/codesjoy/yggdrasil/v3/server/rest"
	"github.com/codesjoy/yggdrasil/v3/stats"

	gkeepalive "google.golang.org/grpc/keepalive"
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
	Extensions Extensions               `mapstructure:"extensions"`
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
	Config      grpcClientConfigOverlay   `mapstructure:",squash"`
	Credentials map[string]map[string]any `mapstructure:"credentials"`
}

// HTTPTransport contains global HTTP transport settings.
type HTTPTransport struct {
	Client rpchttp.ClientConfig `mapstructure:"client"`
	Server rpchttp.ServerConfig `mapstructure:"server"`
	Rest   *rest.Config         `mapstructure:"rest"`
}

// ClientTransports contains service-level transport overrides.
type ClientTransports struct {
	GRPC GRPCClientTransport `mapstructure:"grpc"`
	HTTP HTTPClientTransport `mapstructure:"http"`
}

// ClientDefaults contains default client values applied to services.
type ClientDefaults struct {
	client.ServiceSettings `mapstructure:",squash"`
	Transports             ClientTransports `mapstructure:"transports"`
}

// clientServiceConfigOverlay keeps service-level override presence information.
type clientServiceConfigOverlay struct {
	FastFail     *bool                     `mapstructure:"fast_fail"`
	Resolver     *string                   `mapstructure:"resolver"`
	Balancer     *string                   `mapstructure:"balancer"`
	Backoff      *backoffConfigOverlay     `mapstructure:"backoff"`
	Remote       *remoteConfigOverlay      `mapstructure:"remote"`
	Interceptors *interceptorConfigOverlay `mapstructure:"interceptors"`
}

// ClientServiceSpec contains one configured client service subtree.
type ClientServiceSpec struct {
	ServiceSettings clientServiceConfigOverlay `mapstructure:",squash"`
	Transports      ClientTransports           `mapstructure:"transports"`
}

// HTTPClientTransport contains service-level HTTP client overrides.
type HTTPClientTransport struct {
	Timeout   *time.Duration              `mapstructure:"timeout"`
	Marshaler *rpchttp.MarshalerConfigSet `mapstructure:"marshaler"`
}

type backoffConfigOverlay struct {
	BaseDelay  *time.Duration `mapstructure:"baseDelay"`
	Multiplier *float64       `mapstructure:"multiplier"`
	Jitter     *float64       `mapstructure:"jitter"`
	MaxDelay   *time.Duration `mapstructure:"maxDelay"`
}

type remoteConfigOverlay struct {
	Endpoints  *[]resolver.BaseEndpoint `mapstructure:"endpoints"`
	Attributes *map[string]any          `mapstructure:"attributes"`
}

type interceptorConfigOverlay struct {
	Unary  *[]string `mapstructure:"unary"`
	Stream *[]string `mapstructure:"stream"`
}

type grpcClientConfigOverlay struct {
	WaitConnTimeout   *time.Duration                    `mapstructure:"wait_conn_timeout"`
	ConnectTimeout    *time.Duration                    `mapstructure:"connect_timeout"`
	MaxSendMsgSize    *int                              `mapstructure:"max_send_msg_size"`
	MaxRecvMsgSize    *int                              `mapstructure:"max_recv_msg_size"`
	Compressor        *string                           `mapstructure:"compressor"`
	BackOffMaxDelay   *time.Duration                    `mapstructure:"back_off_max_delay"`
	MinConnectTimeout *time.Duration                    `mapstructure:"min_connect_timeout"`
	Network           *string                           `mapstructure:"network"`
	Transport         grpcClientTransportOptionsOverlay `mapstructure:"transport"`
}

type grpcClientTransportOptionsOverlay struct {
	UserAgent             *string                      `mapstructure:"user_agent"`
	CredsProto            *string                      `mapstructure:"creds_proto"`
	Authority             *string                      `mapstructure:"authority"`
	KeepaliveParams       *gkeepalive.ClientParameters `mapstructure:"keepalive_params"`
	InitialWindowSize     *int32                       `mapstructure:"initial_window_size"`
	InitialConnWindowSize *int32                       `mapstructure:"initial_conn_window_size"`
	WriteBufferSize       *int                         `mapstructure:"write_buffer_size"`
	ReadBufferSize        *int                         `mapstructure:"read_buffer_size"`
	MaxHeaderListSize     *uint32                      `mapstructure:"max_header_list_size"`
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

// Extensions contains chain-oriented extension references.
type Extensions struct {
	Interceptors ExtensionInterceptors `mapstructure:"interceptors"`
	Middleware   ExtensionMiddleware   `mapstructure:"middleware"`
}

// ExtensionInterceptors contains ordered interceptor names.
type ExtensionInterceptors struct {
	UnaryServer  []string `mapstructure:"unary_server"`
	StreamServer []string `mapstructure:"stream_server"`
	UnaryClient  []string `mapstructure:"unary_client"`
	StreamClient []string `mapstructure:"stream_client"`
}

// ExtensionMiddleware contains ordered middleware names.
type ExtensionMiddleware struct {
	RestAll []string `mapstructure:"rest_all"`
	RestRPC []string `mapstructure:"rest_rpc"`
	RestWeb []string `mapstructure:"rest_web"`
}

// OrderedExtensions is the compiled ordered extension name lists.
type OrderedExtensions struct {
	UnaryServer  []string
	StreamServer []string
	UnaryClient  []string
	StreamClient []string
	RestAll      []string
	RestRPC      []string
	RestWeb      []string
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
	Extensions Extensions

	OrderedExtensions  OrderedExtensions
	ModuleViews        map[string]string
	CapabilityBindings map[string][]string
}

// ResolvedTransports contains normalized transport settings.
type ResolvedTransports struct {
	GRPC                   grpcprotocol.Settings
	HTTP                   rpchttp.Settings
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
