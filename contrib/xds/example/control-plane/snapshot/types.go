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

package snapshot

import "time"

// XDSConfig holds the xDS configuration
type XDSConfig struct {
	Clusters  []Cluster  `yaml:"clusters"`
	Endpoints []Endpoint `yaml:"endpoints"`
	Listeners []Listener `yaml:"listeners"`
	Routes    []Route    `yaml:"routes"`
}

// Cluster represents a cluster configuration
type Cluster struct {
	Name             string                  `yaml:"name"`
	ConnectTimeout   string                  `yaml:"connectTimeout"`
	Type             string                  `yaml:"type"`
	LbPolicy         string                  `yaml:"lbPolicy"`
	CircuitBreakers  *CircuitBreakersConfig  `yaml:"circuitBreakers,omitempty"`
	OutlierDetection *OutlierDetectionConfig `yaml:"outlierDetection,omitempty"`
	RateLimiting     *RateLimitingConfig     `yaml:"rateLimiting,omitempty"`
}

// CircuitBreakersConfig holds circuit breaker configuration
type CircuitBreakersConfig struct {
	MaxConnections     uint32 `yaml:"maxConnections,omitempty"`
	MaxPendingRequests uint32 `yaml:"maxPendingRequests,omitempty"`
	MaxRequests        uint32 `yaml:"maxRequests,omitempty"`
	MaxRetries         uint32 `yaml:"maxRetries,omitempty"`
}

// OutlierDetectionConfig holds outlier detection configuration
type OutlierDetectionConfig struct {
	Consecutive5xx                 uint32 `yaml:"consecutive5xx,omitempty"`
	ConsecutiveGatewayFailure      uint32 `yaml:"consecutiveGatewayFailure,omitempty"`
	ConsecutiveLocalOriginFailure  uint32 `yaml:"consecutiveLocalOriginFailure,omitempty"`
	Interval                       string `yaml:"interval,omitempty"`
	BaseEjectionTime               string `yaml:"baseEjectionTime,omitempty"`
	MaxEjectionTime                string `yaml:"maxEjectionTime,omitempty"`
	MaxEjectionPercent             uint32 `yaml:"maxEjectionPercent,omitempty"`
	EnforcingConsecutive5xx        uint32 `yaml:"enforcingConsecutive5xx,omitempty"`
	EnforcingSuccessRate           uint32 `yaml:"enforcingSuccessRate,omitempty"`
	SuccessRateMinimumHosts        uint32 `yaml:"successRateMinimumHosts,omitempty"`
	SuccessRateRequestVolume       uint32 `yaml:"successRateRequestVolume,omitempty"`
	SuccessRateStdevFactor         uint32 `yaml:"successRateStdevFactor,omitempty"`
	FailurePercentageThreshold     uint32 `yaml:"failurePercentageThreshold,omitempty"`
	EnforcingFailurePercentage     uint32 `yaml:"enforcingFailurePercentage,omitempty"`
	FailurePercentageMinimumHosts  uint32 `yaml:"failurePercentageMinimumHosts,omitempty"`
	FailurePercentageRequestVolume uint32 `yaml:"failurePercentageRequestVolume,omitempty"`
	SplitExternalLocalOriginErrors bool   `yaml:"splitExternalLocalOriginErrors,omitempty"`
}

// RateLimitingConfig holds rate limiting configuration
type RateLimitingConfig struct {
	MaxTokens     uint32 `yaml:"maxTokens,omitempty"`
	TokensPerFill uint32 `yaml:"tokensPerFill,omitempty"`
	FillInterval  string `yaml:"fillInterval,omitempty"`
}

// Endpoint represents an endpoint configuration
type Endpoint struct {
	ClusterName string            `yaml:"clusterName"`
	Endpoints   []EndpointAddress `yaml:"endpoints"`
}

// EndpointAddress represents an endpoint address
type EndpointAddress struct {
	Address string `yaml:"address"`
	Port    uint32 `yaml:"port"`
}

// Listener represents a listener configuration
type Listener struct {
	Name         string        `yaml:"name"`
	Address      string        `yaml:"address"`
	Port         uint32        `yaml:"port"`
	FilterChains []FilterChain `yaml:"filterChains"`
}

// FilterChain represents a filter chain
type FilterChain struct {
	Filters []Filter `yaml:"filters"`
}

// Filter represents a filter
type Filter struct {
	Name            string `yaml:"name"`
	RouteConfigName string `yaml:"routeConfigName,omitempty"`
}

// Route represents a route configuration
type Route struct {
	Name         string        `yaml:"name"`
	VirtualHosts []VirtualHost `yaml:"virtualHosts"`
}

// VirtualHost represents a virtual host
type VirtualHost struct {
	Name    string       `yaml:"name"`
	Domains []string     `yaml:"domains"`
	Routes  []RouteMatch `yaml:"routes"`
}

// RouteMatch represents a route match
type RouteMatch struct {
	Match RouteMatchCondition `yaml:"match"`
	Route RouteAction         `yaml:"route"`
}

// HeaderMatchCondition represents a header match condition
type HeaderMatchCondition struct {
	Name    string `yaml:"name"`
	Pattern string `yaml:"pattern"`
	Value   string `yaml:"value"`
}

// PathMatchCondition represents a path match condition
type PathMatchCondition struct {
	Prefix   string `yaml:"prefix,omitempty"`
	Path     string `yaml:"path,omitempty"`
	Suffix   string `yaml:"suffix,omitempty"`
	Contains string `yaml:"contains,omitempty"`
	Regex    string `yaml:"regex,omitempty"`
}

// RouteMatchCondition represents route match conditions
type RouteMatchCondition struct {
	Path    *PathMatchCondition    `yaml:"path,omitempty"`
	Headers []HeaderMatchCondition `yaml:"headers,omitempty"`
}

// RouteAction represents a route action
type RouteAction struct {
	Cluster string `yaml:"cluster"`
}

// WeightedCluster represents a weighted cluster
type WeightedCluster struct {
	Name   string `yaml:"name"`
	Weight uint32 `yaml:"weight"`
}

// WeightedRouteAction represents a weighted route action
type WeightedRouteAction struct {
	Clusters []WeightedCluster `yaml:"clusters"`
}

// HeaderMatch represents a header match
type HeaderMatch struct {
	Name     string `yaml:"name"`
	Exact    string `yaml:"exact,omitempty"`
	Prefix   string `yaml:"prefix,omitempty"`
	Suffix   string `yaml:"suffix,omitempty"`
	Contains string `yaml:"contains,omitempty"`
	Regex    string `yaml:"regex,omitempty"`
}

// PathMatch represents a path match
type PathMatch struct {
	Prefix   string `yaml:"prefix,omitempty"`
	Path     string `yaml:"path,omitempty"`
	Suffix   string `yaml:"suffix,omitempty"`
	Contains string `yaml:"contains,omitempty"`
	Regex    string `yaml:"regex,omitempty"`
}

// ParseDuration parses a duration string with a default fallback
func ParseDuration(s string, defaultDuration time.Duration) time.Duration {
	if s == "" {
		return defaultDuration
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultDuration
	}
	return d
}
