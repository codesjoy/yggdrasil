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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/admin/governor"
	"github.com/codesjoy/yggdrasil/v3/config"
)

func TestRegisterGovernorRoutesInstanceIsolation(t *testing.T) {
	govA, err := governor.NewServerWithConfig(governor.Config{}, config.NewManager())
	require.NoError(t, err)
	govB, err := governor.NewServerWithConfig(governor.Config{}, config.NewManager())
	require.NoError(t, err)

	srvA := &server{
		servicesDesc: map[string][]methodInfo{
			"svc.alpha": {
				{MethodName: "Alpha"},
			},
		},
		restRouterDesc: []restRouterInfo{
			{Method: "GET", Path: "/alpha"},
		},
	}
	srvB := &server{
		servicesDesc: map[string][]methodInfo{
			"svc.beta": {
				{MethodName: "Beta"},
			},
		},
		restRouterDesc: []restRouterInfo{
			{Method: "POST", Path: "/beta"},
		},
	}

	RegisterGovernorRoutes(govA, srvA)
	RegisterGovernorRoutes(govB, srvB)

	servicesA := governorRouteBody(t, govA, "/services")
	servicesB := governorRouteBody(t, govB, "/services")
	assert.Contains(t, servicesA, "svc.alpha")
	assert.NotContains(t, servicesA, "svc.beta")
	assert.Contains(t, servicesB, "svc.beta")
	assert.NotContains(t, servicesB, "svc.alpha")

	restA := governorRouteBody(t, govA, "/rest")
	restB := governorRouteBody(t, govB, "/rest")
	assert.Contains(t, restA, "/alpha")
	assert.NotContains(t, restA, "/beta")
	assert.Contains(t, restB, "/beta")
	assert.NotContains(t, restB, "/alpha")
}

func TestRegisterGovernorRoutesIgnoresNil(t *testing.T) {
	assert.NotPanics(t, func() {
		RegisterGovernorRoutes(nil, nil)
	})
}

func governorRouteBody(t *testing.T, gov *governor.Server, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	gov.Handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}
