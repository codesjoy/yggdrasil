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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/utils/xnet"
)

// Config governor config
type Config struct {
	Host              string        `mapstruct:"host"`
	Port              uint64        `mapstruct:"port"`
	ReadHeaderTimeout time.Duration `mapstruct:"read_header_timeout" default:"5s"`
	ReadTimeout       time.Duration `mapstruct:"read_timeout"        default:"15s"`
	WriteTimeout      time.Duration `mapstruct:"write_timeout"       default:"30s"`
	IdleTimeout       time.Duration `mapstruct:"idle_timeout"        default:"1m"`
}

// Address returns address
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// SetDefault sets default values
func (c *Config) SetDefault() (err error) {
	c.Host, err = xnet.Extract(c.Host)
	return
}

// ServerInfo contains server info
type ServerInfo struct {
	Address string
	Scheme  string
	Attr    map[string]string
}

// Server is a governor server
type Server struct {
	*http.Server
	listener net.Listener
	*Config
	info ServerInfo
}

// NewServer creates a new governor server
func NewServer() (*Server, error) {
	cfg := &Config{}
	if err := config.Get(config.Join(config.KeyBase, "governor")).Scan(cfg); err != nil {
		return nil, err
	}
	if err := cfg.SetDefault(); err != nil {
		return nil, err
	}

	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp4", cfg.Address())
	if err != nil {
		return nil, err
	}
	cfg.Host, cfg.Port = xnet.GetHostAndPortByAddr(listener.Addr())
	s := &Server{
		Server: &http.Server{
			Addr:              cfg.Address(),
			Handler:           defaultServeMux,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
		listener: listener,
		Config:   cfg,
		info: ServerInfo{
			Address: fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Scheme:  "http",
			Attr:    map[string]string{},
		},
	}
	return s, nil
}

// Serve starts the governor server
func (s *Server) Serve() error {
	info := s.Info()
	slog.Info("governor start", "endpoint", fmt.Sprintf("%s://%s", info.Scheme, info.Address))
	err := s.Server.Serve(s.listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Stop stops the governor server
func (s *Server) Stop() error {
	return s.Shutdown(context.TODO())
}

// Info returns the server info
func (s *Server) Info() ServerInfo {
	return s.info
}
