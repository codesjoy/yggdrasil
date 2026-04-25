// Copyright 2026 The codesjoy Authors.
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

// Package governor provides a simple HTTP server for monitoring and debugging.
package governor

import (
	"log/slog"
	"net/http"
	"net/http/pprof"
	"sync"
)

var (
	compatMu     sync.RWMutex
	compatServer *Server
)

func setCompatServer(server *Server) {
	compatMu.Lock()
	defer compatMu.Unlock()
	compatServer = server
}

// HandleFunc registers a new route with the default compatibility server.
//
// Deprecated: use (*Server).HandleFunc instead.
func HandleFunc(pattern string, handler http.HandlerFunc) {
	compatMu.RLock()
	server := compatServer
	compatMu.RUnlock()
	if server == nil {
		slog.Warn(
			"governor compatibility HandleFunc ignored because no default server exists",
			"pattern",
			pattern,
		)
		return
	}
	server.HandleFunc(pattern, handler)
}

func (s *Server) installPprofRoutes() {
	s.HandleFunc("/debug/pprof/", pprof.Index)
	s.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	s.HandleFunc("/debug/pprof/profile", pprof.Profile)
	s.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	s.HandleFunc("/debug/pprof/trace", pprof.Trace)
}
