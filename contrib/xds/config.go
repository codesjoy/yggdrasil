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

package xds

import (
	"fmt"
	"strings"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
)

// ResolverConfig holds the configuration for xDS resolver
type ResolverConfig struct {
	Server     ServerConfig
	Node       NodeConfig
	ServiceMap map[string]string
	Protocol   string
	MaxRetries int
	Health     HealthConfig
	Retry      RetryConfig
}

// ServerConfig holds the xDS server connection configuration
type ServerConfig struct {
	Address string
	Timeout time.Duration
	TLS     TLSConfig
}

// TLSConfig holds TLS configuration for xDS server connection
type TLSConfig struct {
	Enable   bool
	CertFile string
	KeyFile  string
	CAFile   string
}

// NodeConfig holds the node identification information
type NodeConfig struct {
	ID       string
	Cluster  string
	Metadata map[string]string
	Locality *Locality
}

// Locality holds the node locality information
type Locality struct {
	Region  string
	Zone    string
	SubZone string
}

// HealthConfig holds health check configuration
type HealthConfig struct {
	HealthyOnly    bool
	IgnoreStatuses []string
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries int
	Backoff    time.Duration
}

// LoadResolverConfig loads xDS resolver configuration from the config source
func LoadResolverConfig(resolverName string) ResolverConfig {
	cfg := ResolverConfig{
		Server: ServerConfig{
			Address: "127.0.0.1:18000",
			Timeout: 5 * time.Second,
			TLS: TLSConfig{
				Enable: false,
			},
		},
		Node: NodeConfig{
			ID:       "yggdrasil-node",
			Cluster:  "yggdrasil-cluster",
			Metadata: make(map[string]string),
		},
		ServiceMap: make(map[string]string),
		Protocol:   "grpc",
		Health: HealthConfig{
			HealthyOnly:    true,
			IgnoreStatuses: []string{},
		},
		Retry: RetryConfig{
			MaxRetries: 3,
			Backoff:    100 * time.Millisecond,
		},
	}

	sdkNameKey := config.Join(config.KeyBase, "resolver", resolverName, "config", "name")
	sdkName := config.Get(sdkNameKey).String("default")

	base := config.Join(config.KeyBase, "xds", sdkName, "config")

	serverAddress := config.GetString(config.Join(base, "server", "address"))
	if serverAddress != "" {
		cfg.Server.Address = serverAddress
	}

	serverTimeout := config.GetString(config.Join(base, "server", "timeout"))
	if serverTimeout != "" {
		if d, err := time.ParseDuration(serverTimeout); err == nil {
			cfg.Server.Timeout = d
		}
	}

	tlsEnable := config.GetString(config.Join(base, "server", "tls", "enable"))
	if tlsEnable != "" {
		cfg.Server.TLS.Enable = strings.ToLower(tlsEnable) == "true"
	}

	if cfg.Server.TLS.Enable {
		certFile := config.GetString(config.Join(base, "server", "tls", "cert_file"))
		if certFile != "" {
			cfg.Server.TLS.CertFile = certFile
		}
		keyFile := config.GetString(config.Join(base, "server", "tls", "key_file"))
		if keyFile != "" {
			cfg.Server.TLS.KeyFile = keyFile
		}
		caFile := config.GetString(config.Join(base, "server", "tls", "ca_file"))
		if caFile != "" {
			cfg.Server.TLS.CAFile = caFile
		}
	}

	nodeID := config.GetString(config.Join(base, "node", "id"))
	if nodeID != "" {
		cfg.Node.ID = nodeID
	}

	nodeCluster := config.GetString(config.Join(base, "node", "cluster"))
	if nodeCluster != "" {
		cfg.Node.Cluster = nodeCluster
	}

	nodeMetadata := config.GetStringMap(config.Join(base, "node", "metadata"))
	if len(nodeMetadata) > 0 {
		cfg.Node.Metadata = nodeMetadata
	}

	nodeRegion := config.GetString(config.Join(base, "node", "locality", "region"))
	nodeZone := config.GetString(config.Join(base, "node", "locality", "zone"))
	nodeSubZone := config.GetString(config.Join(base, "node", "locality", "sub_zone"))
	if nodeRegion != "" || nodeZone != "" || nodeSubZone != "" {
		cfg.Node.Locality = &Locality{
			Region:  nodeRegion,
			Zone:    nodeZone,
			SubZone: nodeSubZone,
		}
	}

	protocol := config.GetString(config.Join(base, "protocol"))
	if protocol != "" {
		cfg.Protocol = protocol
	}

	serviceMap := config.GetStringMap(config.Join(base, "service_map"))
	if len(serviceMap) > 0 {
		cfg.ServiceMap = serviceMap
	}

	healthyOnly := config.GetString(config.Join(base, "health", "healthy_only"))
	if healthyOnly != "" {
		cfg.Health.HealthyOnly = strings.ToLower(healthyOnly) == "true"
	}

	ignoreStatuses := config.GetStringSlice(config.Join(base, "health", "ignore_statuses"))
	if len(ignoreStatuses) > 0 {
		cfg.Health.IgnoreStatuses = ignoreStatuses
	}

	maxRetries := config.GetInt(config.Join(base, "retry", "max_retries"))
	if maxRetries > 0 {
		cfg.Retry.MaxRetries = maxRetries
	}

	backoff := config.GetString(config.Join(base, "retry", "backoff"))
	if backoff != "" {
		if d, err := time.ParseDuration(backoff); err == nil {
			cfg.Retry.Backoff = d
		}
	}

	adsMaxRetries := config.GetInt(config.Join(base, "max_retries"))
	if adsMaxRetries > 0 {
		cfg.MaxRetries = adsMaxRetries
	}

	return cfg
}

// LoadBalancerConfig loads xDS balancer configuration from the config source
func LoadBalancerConfig(_ string) BalancerConfig {
	return BalancerConfig{}
}

// BalancerConfig holds xDS balancer configuration
type BalancerConfig struct{}

func (b *BalancerConfig) String() string {
	return fmt.Sprintf("%+v", *b)
}
