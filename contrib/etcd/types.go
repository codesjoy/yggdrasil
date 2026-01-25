package etcd

import (
	"time"

	"github.com/codesjoy/yggdrasil/v2/config/source"
)

const (
	ConfigSourceModeBlob = "blob"
	ConfigSourceModeKV   = "kv"
)

type ClientConfig struct {
	Endpoints   []string      `mapstructure:"endpoints"`
	DialTimeout time.Duration `mapstructure:"dialTimeout"`
	Username    string        `mapstructure:"username"`
	Password    string        `mapstructure:"password"`
}

type ConfigSourceConfig struct {
	Client ClientConfig  `mapstructure:"client"`
	Key    string        `mapstructure:"key"`
	Prefix string        `mapstructure:"prefix"`
	Mode   string        `mapstructure:"mode"`
	Watch  *bool         `mapstructure:"watch"`
	Format source.Parser `mapstructure:"format"`
	Name   string        `mapstructure:"name"`
}

type RegistryConfig struct {
	Client        ClientConfig  `mapstructure:"client"`
	Prefix        string        `mapstructure:"prefix"`
	TTL           time.Duration `mapstructure:"ttl"`
	KeepAlive     *bool         `mapstructure:"keepAlive"`
	RetryInterval time.Duration `mapstructure:"retryInterval"`
}

type ResolverConfig struct {
	Client    ClientConfig  `mapstructure:"client"`
	Prefix    string        `mapstructure:"prefix"`
	Namespace string        `mapstructure:"namespace"`
	Protocols []string      `mapstructure:"protocols"`
	Debounce  time.Duration `mapstructure:"debounce"`
}

type instanceRecord struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	Region    string            `json:"region"`
	Zone      string            `json:"zone"`
	Campus    string            `json:"campus"`
	Metadata  map[string]string `json:"metadata"`
	Endpoints []endpointRecord  `json:"endpoints"`
}

type endpointRecord struct {
	Scheme   string            `json:"scheme"`
	Address  string            `json:"address"`
	Metadata map[string]string `json:"metadata"`
}
