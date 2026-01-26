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
	"strings"
)

// VirtualHost represents the xDS VirtualHost configuration
type VirtualHost struct {
	Name    string
	Domains []string
	Routes  []*Route
}

// Route represents a single route within a VirtualHost
type Route struct {
	Match  *RouteMatch
	Action *RouteAction
}

// RouteMatch defines how to match a request
type RouteMatch struct {
	Prefix   string
	Path     string
	Suffix   string
	Contains string
	Regex    *regexp.Regexp // Pre-compiled regex
	Headers  []*HeaderMatcher
}

// HeaderMatcher matches HTTP headers
type HeaderMatcher struct {
	Name        string
	ExactMatch  string
	PrefixMatch string
	SuffixMatch string
	RegexMatch  *regexp.Regexp
	Present     bool
}

// RouteAction defines what to do when a route matches
type RouteAction struct {
	Cluster          string
	WeightedClusters *WeightedClusters
}

// WeightedClusters supports traffic splitting
type WeightedClusters struct {
	Clusters    []*WeightedCluster
	TotalWeight uint32
}

// WeightedCluster represents a single cluster in a traffic split
type WeightedCluster struct {
	Name   string
	Weight uint32
}

// MatchRoute finds the matching route action for a given path and headers.
// Currently simplifies VirtualHost selection by using the first available one.
func MatchRoute(vhs []*VirtualHost, path string, headers map[string]string) *RouteAction {
	if len(vhs) == 0 {
		return nil
	}

	// TODO: Implement domain matching. For now, use the first VirtualHost (usually "default" or wildcard).
	vh := vhs[0]

	for _, route := range vh.Routes {
		if route.Match.Matches(path, headers) {
			return route.Action
		}
	}

	return nil
}

// Matches checks if the route match rules apply to the request
func (m *RouteMatch) Matches(path string, headers map[string]string) bool {
	if m == nil {
		return true
	}

	if m.Path != "" {
		if path != m.Path {
			return false
		}
	} else if m.Prefix != "" {
		if !strings.HasPrefix(path, m.Prefix) {
			return false
		}
	} else if m.Suffix != "" {
		if !strings.HasSuffix(path, m.Suffix) {
			return false
		}
	} else if m.Contains != "" {
		if !strings.Contains(path, m.Contains) {
			return false
		}
	} else if m.Regex != nil {
		if !m.Regex.MatchString(path) {
			return false
		}
	}

	for _, h := range m.Headers {
		val, ok := headers[h.Name]
		if !ok {
			return false
		}

		if h.Present {
			continue
		}

		if h.ExactMatch != "" && val != h.ExactMatch {
			return false
		}

		if h.PrefixMatch != "" && !strings.HasPrefix(val, h.PrefixMatch) {
			return false
		}

		if h.SuffixMatch != "" && !strings.HasSuffix(val, h.SuffixMatch) {
			return false
		}

		if h.RegexMatch != nil {
			if !h.RegexMatch.MatchString(val) {
				return false
			}
		}
	}

	return true
}
