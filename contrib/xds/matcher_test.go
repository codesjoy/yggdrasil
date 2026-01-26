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

package xds

import (
	"regexp"
	"testing"
)

func TestMatchRoute(t *testing.T) {
	vhs := []*VirtualHost{
		{
			Name: "default",
			Routes: []*Route{
				{
					Match: &RouteMatch{
						Path: "/exact",
					},
					Action: &RouteAction{Cluster: "cluster-exact"},
				},
				{
					Match: &RouteMatch{
						Prefix: "/prefix",
						Headers: []*HeaderMatcher{
							{Name: "x-version", ExactMatch: "v2"},
						},
					},
					Action: &RouteAction{Cluster: "cluster-prefix-header"},
				},
				{
					Match: &RouteMatch{
						Prefix: "/prefix",
					},
					Action: &RouteAction{Cluster: "cluster-prefix"},
				},
				{
					Match: &RouteMatch{
						Regex: regexp.MustCompile(`^/regex/\d+$`),
					},
					Action: &RouteAction{Cluster: "cluster-regex"},
				},
				{
					Match: &RouteMatch{
						Prefix: "/", // Catch-all
					},
					Action: &RouteAction{Cluster: "cluster-default"},
				},
			},
		},
	}

	tests := []struct {
		name    string
		path    string
		headers map[string]string
		want    string
	}{
		{"exact match", "/exact", nil, "cluster-exact"},
		{"prefix match without header", "/prefix/foo", nil, "cluster-prefix"},
		{
			"prefix match with header",
			"/prefix/foo",
			map[string]string{"x-version": "v2"},
			"cluster-prefix-header",
		},
		{
			"prefix match with wrong header",
			"/prefix/foo",
			map[string]string{"x-version": "v1"},
			"cluster-prefix",
		},
		{"regex match", "/regex/123", nil, "cluster-regex"},
		{"regex no match", "/regex/abc", nil, "cluster-default"},
		{"default catch-all", "/other", nil, "cluster-default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchRoute(vhs, tt.path, tt.headers)
			if got == nil {
				t.Errorf("MatchRoute() returned nil")
				return
			}
			if got.Cluster != tt.want {
				t.Errorf("MatchRoute() = %v, want %v", got.Cluster, tt.want)
			}
		})
	}
}
