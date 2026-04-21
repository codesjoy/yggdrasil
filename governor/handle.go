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
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"runtime/debug"
	"sync"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/memory"
)

var (
	defaultServeMux = http.NewServeMux()
	routes          []string
	configPatchMu   sync.Mutex
	configPatchData = map[string]any{}
)

// HandleFunc registers a new route with the default ServeMux.
func HandleFunc(pattern string, handler http.HandlerFunc) {
	defaultServeMux.HandleFunc(pattern, handler)
	routes = append(routes, pattern)
}

// ErrResponse represents an error response
type ErrResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type setConfigReq struct {
	Paths [][]string    `json:"paths"`
	Data  []interface{} `json:"data"`
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

func setConfig(w http.ResponseWriter, r *http.Request) {
	cfg := &setConfigReq{}
	if err := json.NewDecoder(r.Body).Decode(cfg); err != nil {
		respErr(w, http.StatusBadRequest, err)
		return
	}
	if err := applyConfigPatch(cfg.Paths, cfg.Data); err != nil {
		respErr(w, http.StatusBadRequest, err)
		return
	}
	respNoContent(w)
}

func getConfig(w http.ResponseWriter, r *http.Request) {
	manager := currentManager()
	if manager == nil {
		respSuccess(w, r, json.RawMessage([]byte("{}")))
		return
	}
	respSuccess(w, r, json.RawMessage(manager.Bytes()))
}

func configHandle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getConfig(w, r)
	case http.MethodPut, http.MethodPost:
		setConfig(w, r)
	}
}

func envHandle(w http.ResponseWriter, r *http.Request) {
	respSuccess(w, r, os.Environ())
}

func newBuildInfoHandle(info *debug.BuildInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respSuccess(w, r, info)
	}
}

func applyConfigPatch(paths [][]string, values []interface{}) error {
	if len(paths) != len(values) {
		return errors.New("the quantity of path and value does not match")
	}

	configPatchMu.Lock()
	defer configPatchMu.Unlock()
	for i := range paths {
		setNestedValue(configPatchData, paths[i], values[i])
	}
	manager := currentManager()
	if manager == nil {
		return errors.New("config manager is not configured")
	}
	return manager.LoadLayer("governor.config_patch", config.PriorityOverride, memory.NewSource("governor.config_patch", configPatchData))
}

func setNestedValue(dst map[string]any, path []string, val any) {
	if len(path) == 0 {
		return
	}
	config.SetPath(dst, val, path...)
}
