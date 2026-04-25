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

package governor

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/codesjoy/yggdrasil/v3/config"
)

const (
	defaultGovernorBind              = "127.0.0.1"
	defaultGovernorReadHeaderTimeout = 5 * time.Second
	defaultGovernorReadTimeout       = 15 * time.Second
	defaultGovernorWriteTimeout      = 30 * time.Second
	defaultGovernorIdleTimeout       = time.Minute
)

// BasicAuthConfig holds HTTP basic auth credentials for governor routes.
type BasicAuthConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// AuthConfig controls governor authentication.
type AuthConfig struct {
	Token string          `mapstructure:"token"`
	Basic BasicAuthConfig `mapstructure:"basic"`
}

// Enabled reports whether auth is configured.
func (a AuthConfig) Enabled() bool {
	return strings.TrimSpace(a.Token) != "" ||
		strings.TrimSpace(a.Basic.Username) != "" ||
		strings.TrimSpace(a.Basic.Password) != ""
}

// Validate validates auth config consistency.
func (a AuthConfig) Validate() error {
	token := strings.TrimSpace(a.Token)
	username := strings.TrimSpace(a.Basic.Username)
	password := strings.TrimSpace(a.Basic.Password)

	basicConfigured := username != "" || password != ""
	if token != "" && basicConfigured {
		return errors.New("governor auth token and basic auth cannot be configured together")
	}
	if basicConfigured && (username == "" || password == "") {
		return errors.New("governor basic auth requires both username and password")
	}
	return nil
}

// Config governor config
type Config struct {
	// Enabled controls whether governor serve loop is active.
	// Nil means enabled for backward compatibility.
	Enabled *bool `mapstructure:"enabled"`
	// Bind is the preferred bind host for governor.
	Bind string `mapstructure:"bind"`
	// Host is the legacy bind host key kept for compatibility.
	Host              string        `mapstructure:"host"`
	Port              uint64        `mapstructure:"port"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout" default:"5s"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"        default:"15s"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"       default:"30s"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"        default:"1m"`

	ExposePprof      bool       `mapstructure:"expose_pprof"`
	ExposeEnv        bool       `mapstructure:"expose_env"`
	AllowConfigPatch bool       `mapstructure:"allow_config_patch"`
	Advertise        bool       `mapstructure:"advertise"`
	Auth             AuthConfig `mapstructure:"auth"`
}

// Address returns address.
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Bind, c.Port)
}

// IsEnabled reports whether governor should serve.
func (c Config) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// SetDefault sets default values and validates settings.
func (c *Config) SetDefault() error {
	if c.Enabled == nil {
		enabled := true
		c.Enabled = &enabled
	}
	c.Bind = normalizeGovernorBind(c.Bind, c.Host)
	c.Host = c.Bind
	if c.ReadHeaderTimeout <= 0 {
		c.ReadHeaderTimeout = defaultGovernorReadHeaderTimeout
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = defaultGovernorReadTimeout
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = defaultGovernorWriteTimeout
	}
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = defaultGovernorIdleTimeout
	}
	c.Auth.Token = strings.TrimSpace(c.Auth.Token)
	c.Auth.Basic.Username = strings.TrimSpace(c.Auth.Basic.Username)
	c.Auth.Basic.Password = strings.TrimSpace(c.Auth.Basic.Password)
	return c.Auth.Validate()
}

func normalizeGovernorBind(bind, host string) string {
	next := strings.TrimSpace(bind)
	if next == "" {
		next = strings.TrimSpace(host)
	}
	switch next {
	case "", "0.0.0.0", "::", "[::]":
		return defaultGovernorBind
	default:
		return next
	}
}

// Configure is deprecated and no longer used by framework runtime.
// Deprecated: use NewServerWithConfig instead.
func Configure(_ Config, _ *config.Manager) {}
