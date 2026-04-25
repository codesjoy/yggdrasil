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
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/memory"
)

// HandleFunc registers a new route with this governor instance.
func (s *Server) HandleFunc(pattern string, handler http.HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.routes {
		if item == pattern {
			return
		}
	}
	s.mux.HandleFunc(pattern, handler)
	s.routes = append(s.routes, pattern)
}

// ErrResponse represents an error response.
type ErrResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type setConfigReq struct {
	Paths [][]string `json:"paths"`
	Data  []any      `json:"data"`
}

func (s *Server) installDefaultRoutes() {
	s.HandleFunc("/", s.routesHandle)
	s.HandleFunc("/routes", s.routesHandle)
	if s.cfg.ExposePprof {
		s.installPprofRoutes()
	}
	if s.cfg.ExposeEnv {
		s.HandleFunc("/env", s.envHandle)
	}
	s.HandleFunc("/configs", s.configHandle)
	if info, ok := debug.ReadBuildInfo(); ok {
		s.HandleFunc("/build_info", s.newBuildInfoHandle(info))
	}
}

func (s *Server) routesHandle(resp http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/routes" {
		http.NotFound(resp, r)
		return
	}
	s.mu.Lock()
	routes := append([]string(nil), s.routes...)
	s.mu.Unlock()
	_ = json.NewEncoder(resp).Encode(routes)
}

func respErr(w http.ResponseWriter, code int, err error) {
	w.WriteHeader(code)
	data, _ := json.Marshal(&ErrResponse{
		Code: code,
		Msg:  err.Error(),
	})
	_, _ = w.Write(data)
}

func respSuccess(w http.ResponseWriter, r *http.Request, data interface{}) {
	encoder := json.NewEncoder(w)
	if r.URL.Query().Get("pretty") == "true" {
		encoder.SetIndent("", "    ")
	}
	w.WriteHeader(http.StatusOK)
	if data != nil {
		_ = encoder.Encode(data)
	}
}

func respNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setConfig(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.AllowConfigPatch {
		respErr(w, http.StatusForbidden, errors.New("governor config patch is disabled"))
		return
	}
	cfg := &setConfigReq{}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(cfg); err != nil {
		respErr(w, http.StatusBadRequest, err)
		return
	}
	if err := validateConfigPatchRequest(cfg.Paths, cfg.Data); err != nil {
		respErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.applyConfigPatch(cfg.Paths, cfg.Data); err != nil {
		respErr(w, http.StatusBadRequest, err)
		return
	}
	respNoContent(w)
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	if s.manager == nil {
		respSuccess(w, r, json.RawMessage([]byte("{}")))
		return
	}
	respSuccess(w, r, json.RawMessage(s.manager.Bytes()))
}

func (s *Server) configHandle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getConfig(w, r)
	case http.MethodPut, http.MethodPost:
		s.setConfig(w, r)
	default:
		w.Header().Set("Allow", "GET, POST, PUT")
		respErr(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}

func (s *Server) envHandle(w http.ResponseWriter, r *http.Request) {
	respSuccess(w, r, os.Environ())
}

func (s *Server) newBuildInfoHandle(info *debug.BuildInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respSuccess(w, r, info)
	}
}

func (s *Server) applyConfigPatch(paths [][]string, values []any) error {
	s.configPatchMu.Lock()
	defer s.configPatchMu.Unlock()
	for i := range paths {
		setNestedValue(s.configPatchData, paths[i], values[i])
	}
	if s.manager == nil {
		return errors.New("config manager is not configured")
	}
	return s.manager.LoadLayer(
		"governor.config_patch",
		config.PriorityOverride,
		memory.NewSource("governor.config_patch", s.configPatchData),
	)
}

func validateConfigPatchRequest(paths [][]string, values []any) error {
	if len(paths) != len(values) {
		return errors.New("the quantity of path and value does not match")
	}
	if len(paths) == 0 {
		return errors.New("paths is required")
	}
	for i := range paths {
		if len(paths[i]) == 0 {
			return fmt.Errorf("paths[%d] is empty", i)
		}
		for j := range paths[i] {
			paths[i][j] = strings.TrimSpace(paths[i][j])
			if paths[i][j] == "" {
				return fmt.Errorf("paths[%d][%d] is empty", i, j)
			}
		}
	}
	return nil
}

func setNestedValue(dst map[string]any, path []string, val any) {
	if len(path) == 0 {
		return
	}
	config.SetPath(dst, val, path...)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	if !s.cfg.Auth.Enabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.authorize(r) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.TrimSpace(s.cfg.Auth.Token) != "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
		} else {
			w.Header().Set("WWW-Authenticate", `Basic realm="governor"`)
		}
		respErr(w, http.StatusUnauthorized, errors.New("unauthorized"))
	})
}

func (s *Server) authorize(r *http.Request) bool {
	if token := strings.TrimSpace(s.cfg.Auth.Token); token != "" {
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
			return false
		}
		received := strings.TrimSpace(header[len("Bearer "):])
		return secureEqual(received, token)
	}
	username := strings.TrimSpace(s.cfg.Auth.Basic.Username)
	password := strings.TrimSpace(s.cfg.Auth.Basic.Password)
	if username == "" && password == "" {
		return true
	}
	u, p, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return secureEqual(strings.TrimSpace(u), username) &&
		secureEqual(strings.TrimSpace(p), password)
}

func secureEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
