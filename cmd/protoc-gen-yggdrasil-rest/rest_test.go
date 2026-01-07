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

package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func buildPathVars1(path string) string {
	subPattern0 := regexp.MustCompile(`(?i)^{params[0-9]+}$`)
	if subPattern0.MatchString(path) {
		path = fmt.Sprintf(
			`%sURLParam(r, "%s")`,
			"chi.",
			strings.TrimRight(strings.TrimLeft(path, "{"), "}"),
		)
		return path
	}
	pathValPattern1 := regexp.MustCompile(`(?i)/{(params[0-9]+)}/`)
	path = pathValPattern1.ReplaceAllStringFunc(path, func(subStr string) string {
		params := pathPattern.FindStringSubmatch(subStr)
		return fmt.Sprintf(`/"+%sURLParam(r, "%s")+"/`, "chi.", params[1])
	})
	pathValPattern2 := regexp.MustCompile(`(?i)/{(params[0-9]+)}`)
	path = pathValPattern2.ReplaceAllStringFunc(path, func(subStr string) string {
		params := pathPattern.FindStringSubmatch(subStr)
		return fmt.Sprintf(`/"+%sURLParam(r,"%s")+"`, "chi.", params[1])
	})
	path = fmt.Sprintf(`"%s"`, path)
	path = strings.TrimRight(path, `+""`) // nolint:staticcheck
	return path
}

func Test_buildPathVars(t *testing.T) {
	// data, _ := json.Marshal(buildPathVars1("/v1/{parent=pools/*}/users"))
	// fmt.Println(string(data))
	// data, _ = json.Marshal(buildPathVars1("/v1/{name=pools/*/users/*}"))
	// fmt.Println(string(data))
	// data, _ = json.Marshal(buildPathVars1("/v1/poos/{pool}/{name=pools/*/users/*}"))
	// fmt.Println(string(data))
	// return
	{
		path, nameVars := buildPathVars("/v1/{pool}/users")
		nameVarsData, _ := json.Marshal(nameVars)
		fmt.Println(path, string(nameVarsData))
		for k, v := range nameVars {
			fmt.Println(k, buildPathVars1(v))
		}
	}
	fmt.Println()
	{
		path, nameVars := buildPathVars("/v1/{parent=pools/*}/users")
		nameVarsData, _ := json.Marshal(nameVars)
		fmt.Println(path, string(nameVarsData))
		for k, v := range nameVars {
			fmt.Println(k, buildPathVars1(v))
		}
	}
	fmt.Println()
	{
		path, nameVars := buildPathVars("/v1/{name=pools/*/users/*}")
		nameVarsData, _ := json.Marshal(nameVars)
		fmt.Println(path, string(nameVarsData))
		for k, v := range nameVars {
			fmt.Println(k, buildPathVars1(v))
		}
	}
	fmt.Println()
	{
		path, nameVars := buildPathVars("/v1/poos/{pool}/{name=pools/*/users/*}")
		nameVarsData, _ := json.Marshal(nameVars)
		fmt.Println(path, string(nameVarsData))
		for k, v := range nameVars {
			fmt.Println(k, buildPathVars1(v))
		}
	}
}
