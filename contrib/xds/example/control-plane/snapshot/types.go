package snapshot

import "time"

type XDSConfig struct {
	Clusters  []Cluster  `yaml:"clusters"`
	Endpoints []Endpoint `yaml:"endpoints"`
	Listeners []Listener `yaml:"listeners"`
	Routes    []Route    `yaml:"routes"`
}

type Cluster struct {
	Name             string                  `yaml:"name"`
	ConnectTimeout   string                  `yaml:"connectTimeout"`
	Type             string                  `yaml:"type"`
	LbPolicy         string                  `yaml:"lbPolicy"`
	CircuitBreakers  *CircuitBreakersConfig  `yaml:"circuitBreakers,omitempty"`
	OutlierDetection *OutlierDetectionConfig `yaml:"outlierDetection,omitempty"`
	RateLimiting     *RateLimitingConfig     `yaml:"rateLimiting,omitempty"`
}

type CircuitBreakersConfig struct {
	MaxConnections     uint32 `yaml:"maxConnections,omitempty"`
	MaxPendingRequests uint32 `yaml:"maxPendingRequests,omitempty"`
	MaxRequests        uint32 `yaml:"maxRequests,omitempty"`
	MaxRetries         uint32 `yaml:"maxRetries,omitempty"`
}

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

type RateLimitingConfig struct {
	MaxTokens     uint32 `yaml:"maxTokens,omitempty"`
	TokensPerFill uint32 `yaml:"tokensPerFill,omitempty"`
	FillInterval  string `yaml:"fillInterval,omitempty"`
}

type Endpoint struct {
	ClusterName string            `yaml:"clusterName"`
	Endpoints   []EndpointAddress `yaml:"endpoints"`
}

type EndpointAddress struct {
	Address string `yaml:"address"`
	Port    uint32 `yaml:"port"`
}

type Listener struct {
	Name         string        `yaml:"name"`
	Address      string        `yaml:"address"`
	Port         uint32        `yaml:"port"`
	FilterChains []FilterChain `yaml:"filterChains"`
}

type FilterChain struct {
	Filters []Filter `yaml:"filters"`
}

type Filter struct {
	Name            string `yaml:"name"`
	RouteConfigName string `yaml:"routeConfigName,omitempty"`
}

type Route struct {
	Name         string        `yaml:"name"`
	VirtualHosts []VirtualHost `yaml:"virtualHosts"`
}

type VirtualHost struct {
	Name    string       `yaml:"name"`
	Domains []string     `yaml:"domains"`
	Routes  []RouteMatch `yaml:"routes"`
}

type RouteMatch struct {
	Match RouteMatchCondition `yaml:"match"`
	Route RouteAction         `yaml:"route"`
}

type HeaderMatchCondition struct {
	Name    string `yaml:"name"`
	Pattern string `yaml:"pattern"`
	Value   string `yaml:"value"`
}

type PathMatchCondition struct {
	Prefix   string `yaml:"prefix,omitempty"`
	Path     string `yaml:"path,omitempty"`
	Suffix   string `yaml:"suffix,omitempty"`
	Contains string `yaml:"contains,omitempty"`
	Regex    string `yaml:"regex,omitempty"`
}

type RouteMatchCondition struct {
	Path    *PathMatchCondition    `yaml:"path,omitempty"`
	Headers []HeaderMatchCondition `yaml:"headers,omitempty"`
}

type RouteAction struct {
	Cluster string `yaml:"cluster"`
}

type WeightedCluster struct {
	Name   string `yaml:"name"`
	Weight uint32 `yaml:"weight"`
}

type WeightedRouteAction struct {
	Clusters []WeightedCluster `yaml:"clusters"`
}

type HeaderMatch struct {
	Name     string `yaml:"name"`
	Exact    string `yaml:"exact,omitempty"`
	Prefix   string `yaml:"prefix,omitempty"`
	Suffix   string `yaml:"suffix,omitempty"`
	Contains string `yaml:"contains,omitempty"`
	Regex    string `yaml:"regex,omitempty"`
}

type PathMatch struct {
	Prefix   string `yaml:"prefix,omitempty"`
	Path     string `yaml:"path,omitempty"`
	Suffix   string `yaml:"suffix,omitempty"`
	Contains string `yaml:"contains,omitempty"`
	Regex    string `yaml:"regex,omitempty"`
}

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
