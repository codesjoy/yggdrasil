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

package client

import (
	"slices"

	"github.com/codesjoy/yggdrasil/v3/rpc/interceptor"
)

func (c *client) initInterceptor() error {
	cfg := c.runtime.ClientSettings(c.appName)
	unaryNames := append([]string(nil), cfg.Interceptors.Unary...)
	unaryNames = dedupStableStrings(
		slices.DeleteFunc(unaryNames, func(s string) bool { return s == "" }),
	)
	streamNames := append([]string(nil), cfg.Interceptors.Stream...)
	streamNames = dedupStableStrings(
		slices.DeleteFunc(streamNames, func(s string) bool { return s == "" }),
	)
	if strict, ok := c.runtime.(interface {
		BuildUnaryClientInterceptorE(
			string,
			[]string,
		) (interceptor.UnaryClientInterceptor, error)
		BuildStreamClientInterceptorE(
			string,
			[]string,
		) (interceptor.StreamClientInterceptor, error)
	}); ok {
		unary, err := strict.BuildUnaryClientInterceptorE(c.appName, unaryNames)
		if err != nil {
			return err
		}
		streamInterceptor, err := strict.BuildStreamClientInterceptorE(
			c.appName,
			streamNames,
		)
		if err != nil {
			return err
		}
		c.unaryInterceptor = unary
		c.streamInterceptor = streamInterceptor
		return nil
	}
	c.unaryInterceptor = c.runtime.BuildUnaryClientInterceptor(c.appName, unaryNames)
	c.streamInterceptor = c.runtime.BuildStreamClientInterceptor(c.appName, streamNames)
	return nil
}

func dedupStableStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	i := 0
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values[i] = value
		i++
	}
	return values[:i]
}
