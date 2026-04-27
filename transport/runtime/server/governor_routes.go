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

package server

import (
	"encoding/json"
	"net/http"

	"github.com/codesjoy/yggdrasil/v3/admin/governor"
	internalidentity "github.com/codesjoy/yggdrasil/v3/internal/identity"
)

// RegisterGovernorRoutes registers service and rest metadata routes into governor.
func RegisterGovernorRoutes(gov *governor.Server, app Server, identity internalidentity.Identity) {
	if gov == nil || app == nil {
		return
	}
	s, ok := app.(*server)
	if !ok {
		return
	}
	gov.HandleFunc("/services", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		encoder := json.NewEncoder(w)
		if r.URL.Query().Get("pretty") == "true" {
			encoder.SetIndent("", "    ")
		}
		result := map[string]interface{}{
			"appName":  identity.AppName,
			"services": s.serviceDescSnapshot(),
		}
		_ = encoder.Encode(result)
	})
	gov.HandleFunc("/rest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		encoder := json.NewEncoder(w)
		if r.URL.Query().Get("pretty") == "true" {
			encoder.SetIndent("", "    ")
		}
		result := map[string]interface{}{
			"appName": identity.AppName,
			"routers": s.restRouteSnapshot(),
		}
		_ = encoder.Encode(result)
	})
}
