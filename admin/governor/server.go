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
	"maps"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/codesjoy/yggdrasil/v3/config"
)

// ServerInfo contains server info.
type ServerInfo struct {
	Address string
	Scheme  string
	Attr    map[string]string
}

// Server is a governor server.
type Server struct {
	*http.Server

	mu       sync.Mutex
	listener net.Listener
	cfg      Config
	manager  *config.Manager

	mux    *http.ServeMux
	routes []string

	configPatchMu   sync.Mutex
	configPatchData map[string]any

	infoMu sync.RWMutex
	info   ServerInfo

	startedMu   sync.Mutex
	startedErr  error
	startedOnce sync.Once
	startedCh   chan struct{}
}

// NewServer creates a new governor server with default config.
func NewServer() (*Server, error) {
	return NewServerWithConfig(Config{}, config.Default())
}

// NewServerWithConfig creates a new governor server with explicit config and manager.
func NewServerWithConfig(cfg Config, manager *config.Manager) (*Server, error) {
	if err := cfg.SetDefault(); err != nil {
		return nil, err
	}

	s := &Server{
		cfg:             cfg,
		manager:         manager,
		mux:             http.NewServeMux(),
		routes:          make([]string, 0, 16),
		configPatchData: map[string]any{},
		startedCh:       make(chan struct{}),
	}
	s.Server = &http.Server{
		Addr:              cfg.Address(),
		Handler:           s.authMiddleware(s.mux),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
	s.setInfo(ServerInfo{
		Address: cfg.Address(),
		Scheme:  "http",
		Attr:    map[string]string{},
	})
	s.installDefaultRoutes()
	setCompatServer(s)
	return s, nil
}

// Serve starts the governor server.
func (s *Server) Serve() error {
	if !s.cfg.IsEnabled() {
		s.markStarted(nil)
		return nil
	}
	s.warnIfUnauthenticatedExposure()
	listener, err := s.listen()
	if err != nil {
		s.markStarted(err)
		return err
	}

	host, portStr, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		_ = listener.Close()
		s.markStarted(err)
		return err
	}
	port, err := strconv.ParseUint(portStr, 10, 64)
	if err != nil {
		_ = listener.Close()
		s.markStarted(err)
		return err
	}

	s.mu.Lock()
	s.listener = listener
	s.cfg.Bind = host
	s.cfg.Host = host
	s.cfg.Port = port
	s.Addr = s.cfg.Address()
	s.mu.Unlock()
	s.setInfo(ServerInfo{
		Address: fmt.Sprintf("%s:%d", host, port),
		Scheme:  "http",
		Attr:    map[string]string{},
	})

	info := s.Info()
	slog.Info("governor start", "endpoint", fmt.Sprintf("%s://%s", info.Scheme, info.Address))
	s.markStarted(nil)

	err = s.Server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Stop stops the governor server.
func (s *Server) Stop() error {
	return s.Shutdown(context.TODO())
}

// Shutdown gracefully stops the governor server.
func (s *Server) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if s.Server == nil {
		return nil
	}
	err := s.Server.Shutdown(ctx)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Info returns the server info.
func (s *Server) Info() ServerInfo {
	s.infoMu.RLock()
	defer s.infoMu.RUnlock()
	return ServerInfo{
		Address: s.info.Address,
		Scheme:  s.info.Scheme,
		Attr:    maps.Clone(s.info.Attr),
	}
}

// WaitStarted waits for the first serve attempt to finish initial startup.
func (s *Server) WaitStarted(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.startedCh:
		s.startedMu.Lock()
		defer s.startedMu.Unlock()
		return s.startedErr
	}
}

// IsEnabled reports whether governor is enabled.
func (s *Server) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.IsEnabled()
}

// ShouldAdvertise reports whether governor endpoint should be registered.
func (s *Server) ShouldAdvertise() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.IsEnabled() && s.cfg.Advertise
}

func (s *Server) listen() (net.Listener, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return nil, errors.New("governor already serve")
	}
	lc := net.ListenConfig{}
	return lc.Listen(context.Background(), "tcp4", s.cfg.Address())
}

func (s *Server) markStarted(err error) {
	s.startedMu.Lock()
	s.startedErr = err
	s.startedMu.Unlock()
	s.startedOnce.Do(func() {
		close(s.startedCh)
	})
}

func (s *Server) setInfo(info ServerInfo) {
	s.infoMu.Lock()
	defer s.infoMu.Unlock()
	s.info = info
	if s.info.Attr == nil {
		s.info.Attr = map[string]string{}
	}
}

func (s *Server) warnIfUnauthenticatedExposure() {
	if s.cfg.Auth.Enabled() {
		return
	}
	exposed := make([]string, 0, 3)
	if s.cfg.ExposePprof {
		exposed = append(exposed, "pprof")
	}
	if s.cfg.ExposeEnv {
		exposed = append(exposed, "env")
	}
	if s.cfg.AllowConfigPatch {
		exposed = append(exposed, "config_patch")
	}
	if len(exposed) == 0 {
		return
	}
	slog.Warn(
		"governor high-risk routes are exposed without authentication",
		"routes",
		strings.Join(exposed, ","),
		"suggestion",
		"configure governor.auth.token or governor.auth.basic credentials",
	)
}
