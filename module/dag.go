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

package module

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

type dagResult struct {
	order  []Module
	layers map[string]int
}

func buildDAG(modules []Module, index map[string]Module) (dagResult, []string, error) {
	indegree := map[string]int{}
	outgoing := map[string][]string{}
	for _, mod := range modules {
		name := mod.Name()
		indegree[name] = 0
		outgoing[name] = nil
	}

	var dependencyErrors []string
	for _, mod := range modules {
		name := mod.Name()
		deps := dependsOn(mod)
		for _, dep := range deps {
			if _, ok := index[dep]; !ok {
				dependencyErrors = append(
					dependencyErrors,
					fmt.Sprintf("module %q depends on missing module %q", name, dep),
				)
				continue
			}
			indegree[name]++
			outgoing[dep] = append(outgoing[dep], name)
		}
	}
	if len(dependencyErrors) > 0 {
		slices.Sort(dependencyErrors)
		return dagResult{}, dependencyErrors, errors.New(strings.Join(dependencyErrors, "; "))
	}

	ready := make([]Module, 0, len(modules))
	for _, mod := range modules {
		if indegree[mod.Name()] == 0 {
			ready = append(ready, mod)
		}
	}

	visited := 0
	order := make([]Module, 0, len(modules))
	layers := map[string]int{}
	layer := 0
	for len(ready) > 0 {
		slices.SortFunc(ready, compareModules)
		current := ready
		ready = nil
		for _, mod := range current {
			name := mod.Name()
			order = append(order, mod)
			layers[name] = layer
			visited++
			nextList := outgoing[name]
			slices.Sort(nextList)
			for _, next := range nextList {
				indegree[next]--
				if indegree[next] == 0 {
					ready = append(ready, index[next])
				}
			}
		}
		layer++
	}

	if visited != len(modules) {
		cycle := detectCycle(modules, outgoing)
		if len(cycle) == 0 {
			dependencyErrors = append(dependencyErrors, "module dependency cycle detected")
			return dagResult{}, dependencyErrors, fmt.Errorf("module dependency cycle detected")
		}
		dependencyErrors = append(
			dependencyErrors,
			fmt.Sprintf("module dependency cycle detected: %s", strings.Join(cycle, " -> ")),
		)
		return dagResult{}, dependencyErrors, errors.New(strings.Join(dependencyErrors, "; "))
	}

	return dagResult{order: order, layers: layers}, nil, nil
}

func compareModules(a, b Module) int {
	ao := moduleOrder(a)
	bo := moduleOrder(b)
	if ao != bo {
		if ao < bo {
			return -1
		}
		return 1
	}
	if a.Name() < b.Name() {
		return -1
	}
	if a.Name() > b.Name() {
		return 1
	}
	return 0
}

func moduleOrder(m Module) int {
	if item, ok := m.(Ordered); ok {
		return item.InitOrder()
	}
	return 0
}

func dependsOn(m Module) []string {
	if item, ok := m.(Dependent); ok {
		deps := append([]string(nil), item.DependsOn()...)
		out := make([]string, 0, len(deps))
		seen := map[string]struct{}{}
		for _, dep := range deps {
			if dep == "" {
				continue
			}
			if _, ok := seen[dep]; ok {
				continue
			}
			seen[dep] = struct{}{}
			out = append(out, dep)
		}
		slices.Sort(out)
		return out
	}
	return nil
}

func detectCycle(modules []Module, outgoing map[string][]string) []string {
	const (
		unvisited = 0
		visiting  = 1
		done      = 2
	)
	state := map[string]int{}
	stack := make([]string, 0, len(modules))
	var cycle []string
	var dfs func(string) bool
	dfs = func(node string) bool {
		state[node] = visiting
		stack = append(stack, node)
		for _, next := range outgoing[node] {
			switch state[next] {
			case unvisited:
				if dfs(next) {
					return true
				}
			case visiting:
				pos := slices.Index(stack, next)
				if pos >= 0 {
					cycle = append(cycle, stack[pos:]...)
					cycle = append(cycle, next)
				}
				return true
			}
		}
		stack = stack[:len(stack)-1]
		state[node] = done
		return false
	}
	for _, mod := range modules {
		name := mod.Name()
		if state[name] != unvisited {
			continue
		}
		if dfs(name) {
			break
		}
	}
	return cycle
}
