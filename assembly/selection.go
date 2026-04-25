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

package assembly

import (
	"fmt"
	"sort"
	"strings"

	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/internal/settings"
	"github.com/codesjoy/yggdrasil/v3/module"
)

const (
	stagePlan = "plan"

	capLoggerHandler = "observability.logger.handler"
	capLoggerWriter  = "observability.logger.writer"
	capTracer        = "observability.otel.tracer_provider"
	capMeter         = "observability.otel.meter_provider"
	capStatsHandler  = "observability.stats.handler"
	capSecurity      = "security.profile.provider"
	capMarshaler     = "marshaler.scheme"
	capServerTrans   = "transport.server.provider"
	capClientTrans   = "transport.client.provider"
	capUnaryServer   = "rpc.interceptor.unary_server"
	capStreamServer  = "rpc.interceptor.stream_server"
	capUnaryClient   = "rpc.interceptor.unary_client"
	capStreamClient  = "rpc.interceptor.stream_client"
	capRESTMW        = "transport.rest.middleware"
	capRegistry      = "discovery.registry.provider"
	capResolver      = "discovery.resolver.provider"
	capBalancer      = "transport.balancer.provider"

	chainUnaryServer  = capUnaryServer
	chainStreamServer = capStreamServer
	chainUnaryClient  = capUnaryClient
	chainStreamClient = capStreamClient
	chainRESTAll      = "middleware.rest_all"
	chainRESTRPC      = "middleware.rest_rpc"
	chainRESTWeb      = "middleware.rest_web"
)

var requiredModules = map[string]struct{}{
	"foundation.capabilities":   {},
	"connectivity.capabilities": {},
	"foundation.runtime":        {},
	"connectivity.runtime":      {},
}

var frameworkModules = map[string]struct{}{
	"foundation.capabilities":   {},
	"connectivity.capabilities": {},
	"foundation.runtime":        {},
	"connectivity.runtime":      {},
	"telemetry.stats.otel":      {},
}

var businessInputPaths = []string{
	"yggdrasil.clients",
	"yggdrasil.discovery",
	"yggdrasil.extensions",
	"yggdrasil.logging",
	"yggdrasil.server",
	"yggdrasil.telemetry",
	"yggdrasil.transports",
}

type templateSelection struct {
	Name    string
	Version string
}

type modeDefinition struct {
	Name      string
	Profile   string
	Bundle    string
	Defaults  map[string]string
	Templates map[string]templateSelection
}

type plannerRegistry struct {
	modes     map[string]modeDefinition
	templates map[string]Template
}

type moduleDefaultCandidate struct {
	Module   string
	Provider string
	Source   string
	Score    int
}

func newDefaultPlannerRegistry() plannerRegistry {
	registry := plannerRegistry{
		modes:     map[string]modeDefinition{},
		templates: map[string]Template{},
	}
	registry.registerBuiltInTemplates()
	registry.registerBuiltInModes()
	return registry
}

func (r plannerRegistry) mode(name string) (modeDefinition, bool) {
	mode, ok := r.modes[name]
	return mode, ok
}

func (r plannerRegistry) template(name, version string) (Template, bool) {
	template, ok := r.templates[templateKey(name, version)]
	return template, ok
}

func (r plannerRegistry) registerMode(def modeDefinition) {
	if def.Name == "" {
		return
	}
	r.modes[def.Name] = def
}

func (r plannerRegistry) registerTemplate(template Template) {
	if template.Name == "" || template.Version == "" {
		return
	}
	r.templates[templateKey(template.Name, template.Version)] = template
}

func (r plannerRegistry) registerBuiltInModes() {
	r.registerMode(modeDefinition{
		Name:    "dev",
		Profile: "dev",
		Bundle:  "server-basic",
		Defaults: map[string]string{
			capLoggerHandler: "text",
			capLoggerWriter:  "console",
			capRegistry:      "multi_registry",
		},
		Templates: defaultObservableTemplates(true),
	})
	r.registerMode(modeDefinition{
		Name:    "prod-grpc",
		Profile: "prod",
		Bundle:  "grpc-server",
		Defaults: map[string]string{
			capLoggerHandler: "json",
			capLoggerWriter:  "console",
			capTracer:        "otel",
			capMeter:         "otel",
			capRegistry:      "multi_registry",
		},
		Templates: defaultObservableTemplates(false),
	})
	r.registerMode(modeDefinition{
		Name:    "prod-http-gateway",
		Profile: "prod",
		Bundle:  "http-gateway",
		Defaults: map[string]string{
			capLoggerHandler: "json",
			capLoggerWriter:  "console",
			capTracer:        "otel",
			capMeter:         "otel",
			capRegistry:      "multi_registry",
		},
		Templates: defaultObservableTemplates(true),
	})
}

func (r plannerRegistry) registerBuiltInTemplates() {
	r.registerTemplate(Template{
		Name:    "default-observable",
		Version: "v1",
		Items:   []string{"logging"},
	})
	r.registerTemplate(Template{
		Name:    "default-client-safe",
		Version: "v1",
		Items:   []string{"logging"},
	})
	r.registerTemplate(Template{
		Name:    "default-rest-observable",
		Version: "v1",
		Items:   []string{"logger"},
	})
}

func defaultObservableTemplates(includeREST bool) map[string]templateSelection {
	templates := map[string]templateSelection{
		chainUnaryServer:  {Name: "default-observable", Version: "v1"},
		chainStreamServer: {Name: "default-observable", Version: "v1"},
		chainUnaryClient:  {Name: "default-client-safe", Version: "v1"},
		chainStreamClient: {Name: "default-client-safe", Version: "v1"},
	}
	if includeREST {
		templates[chainRESTAll] = templateSelection{Name: "default-rest-observable", Version: "v1"}
	}
	return templates
}

func (p *planner) resolveDefaults() error {
	for _, capability := range []string{
		capLoggerHandler,
		capLoggerWriter,
		capTracer,
		capMeter,
		capRegistry,
	} {
		value, source, err := p.selectDefault(capability)
		if err != nil {
			return err
		}
		if value == "" {
			continue
		}
		p.selectedDefaults[capability] = value
		p.defaultSources[capability] = source
		p.decisions = append(p.decisions, Decision{
			Kind:   "default",
			Target: capability,
			Value:  value,
			Source: source,
			Reason: "selected capability default",
			Stage:  stagePlan,
		})
	}
	return nil
}

func (p *planner) selectDefault(capability string) (string, string, error) {
	if forced, ok := p.codeOverrides.ForcedDefaults[capability]; ok {
		if err := p.requireProvider(capability, forced, ErrUnknownExplicitBinding); err != nil {
			return "", "", err
		}
		p.decisions = append(p.decisions, Decision{
			Kind:   "override.force_default",
			Target: capability,
			Value:  forced,
			Source: "code_override",
			Reason: "forced default",
			Stage:  stagePlan,
		})
		return forced, "code_override", nil
	}
	if forced, ok := p.configOverrides.ForcedDefaults[capability]; ok {
		if err := p.requireProvider(capability, forced, ErrUnknownExplicitBinding); err != nil {
			return "", "", err
		}
		p.decisions = append(p.decisions, Decision{
			Kind:   "override.force_default",
			Target: capability,
			Value:  forced,
			Source: "config_override",
			Reason: "forced default",
			Stage:  stagePlan,
		})
		return forced, "config_override", nil
	}

	if explicit, ok := explicitDefaultValue(p.input.Resolved, p.input.Snapshot, capability); ok {
		if err := p.requireProvider(capability, explicit, ErrUnknownExplicitBinding); err != nil {
			return "", "", err
		}
		return explicit, "explicit_config", nil
	}

	if _, disabled := p.codeOverrides.DisabledAuto[capability]; !disabled {
		if _, disabled = p.configOverrides.DisabledAuto[capability]; !disabled {
			if preferred, ok := p.mode.Defaults[capability]; ok {
				if providerAvailable(p.availableProviders[capability], preferred) {
					return preferred, "mode:" + p.mode.Name, nil
				}
			}
			if fallback, source, candidates, err := p.selectModuleFallbackDefault(capability); err != nil {
				return "", "", err
			} else if fallback != "" {
				p.defaultCandidates[capability] = candidates
				return fallback, source, nil
			}
			if fallback, ok := fallbackDefault(capability, p.availableProviders[capability]); ok {
				return fallback, "fallback", nil
			}
			if len(p.availableProviders[capability]) > 1 && isDefaultSelectable(capability) {
				return "", "", newError(
					ErrAmbiguousDefault,
					stagePlan,
					fmt.Sprintf(
						"capability %q has multiple candidates %v and no deterministic default",
						capability,
						p.availableProviders[capability],
					),
					nil,
					map[string]string{"capability": capability},
				)
			}
		}
	}
	return "", "", nil
}

func (p *planner) selectModuleFallbackDefault(
	capability string,
) (string, string, []DefaultCandidate, error) {
	rawCandidates := p.moduleDefaultCandidates(capability)
	if len(rawCandidates) == 0 {
		return "", "", nil, nil
	}
	sort.Slice(rawCandidates, func(i, j int) bool {
		if rawCandidates[i].Score != rawCandidates[j].Score {
			return rawCandidates[i].Score > rawCandidates[j].Score
		}
		if rawCandidates[i].Module != rawCandidates[j].Module {
			return rawCandidates[i].Module < rawCandidates[j].Module
		}
		return rawCandidates[i].Provider < rawCandidates[j].Provider
	})
	candidates := make([]DefaultCandidate, 0, len(rawCandidates))
	for _, item := range rawCandidates {
		candidates = append(candidates, DefaultCandidate{
			Module:   item.Module,
			Provider: item.Provider,
			Source:   item.Source,
			Score:    item.Score,
		})
	}
	if len(rawCandidates) > 1 && rawCandidates[0].Score == rawCandidates[1].Score {
		return "", "", candidates, newError(
			ErrAmbiguousDefault,
			stagePlan,
			fmt.Sprintf("capability %q has ambiguous module fallback defaults", capability),
			nil,
			map[string]string{"capability": capability},
		)
	}
	candidates[0].Selected = true
	return rawCandidates[0].Provider, rawCandidates[0].Source, candidates, nil
}

func (p *planner) moduleDefaultCandidates(capability string) []moduleDefaultCandidate {
	if !isDefaultSelectable(capability) {
		return nil
	}
	providers := make([]moduleDefaultCandidate, 0)
	for _, mod := range p.modules {
		state, ok := p.autoModules[mod.Name()]
		if !ok {
			continue
		}
		if state.spec.DefaultPolicy == nil ||
			!defaultPolicyMatches(state.spec.DefaultPolicy, p.mode.Profile) {
			continue
		}
		name, ok := uniqueCapabilityProviderName(mod, capability)
		if !ok || !providerAvailable(p.availableProviders[capability], name) {
			continue
		}
		providers = append(providers, moduleDefaultCandidate{
			Module:   mod.Name(),
			Provider: name,
			Source:   "module_fallback",
			Score:    state.spec.DefaultPolicy.Score,
		})
	}
	return providers
}

func defaultPolicyMatches(policy *module.DefaultPolicy, profile string) bool {
	if policy == nil || len(policy.Profiles) == 0 || profile == "" {
		return true
	}
	for _, item := range policy.Profiles {
		if strings.TrimSpace(item) == profile {
			return true
		}
	}
	return false
}

func uniqueCapabilityProviderName(mod module.Module, capability string) (string, bool) {
	provider, ok := mod.(module.CapabilityProvider)
	if !ok {
		return "", false
	}
	names := make([]string, 0, 1)
	for _, cap := range provider.Capabilities() {
		if cap.Spec.Name != capability {
			continue
		}
		name := cap.Name
		if name == "" {
			name = mod.Name()
		}
		names = append(names, name)
	}
	names = dedupStrings(names)
	if len(names) != 1 {
		return "", false
	}
	return names[0], true
}

func capabilityConfigPaths(capability string) []string {
	switch capability {
	case capLoggerHandler:
		return []string{"yggdrasil.logging.handlers.default.type"}
	case capLoggerWriter:
		return []string{"yggdrasil.logging.writers.default.type"}
	case capTracer:
		return []string{"yggdrasil.telemetry.tracer"}
	case capMeter:
		return []string{"yggdrasil.telemetry.meter"}
	case capRegistry:
		return []string{"yggdrasil.discovery.registry.type"}
	default:
		return nil
	}
}

func fallbackDefault(capability string, available []string) (string, bool) {
	var preferred []string
	switch capability {
	case capLoggerHandler:
		preferred = []string{"text", "json"}
	case capLoggerWriter:
		preferred = []string{"console", "file"}
	case capTracer, capMeter:
		preferred = []string{"otel"}
	case capRegistry:
		preferred = []string{"multi_registry"}
	}
	for _, candidate := range preferred {
		if providerAvailable(available, candidate) {
			return candidate, true
		}
	}
	switch len(available) {
	case 0:
		return "", false
	case 1:
		return available[0], true
	default:
		return "", false
	}
}

func providerAvailable(available []string, target string) bool {
	for _, item := range available {
		if item == target {
			return true
		}
	}
	return false
}

func (p *planner) requireProvider(capability, name string, code ErrorCode) error {
	if name == "" {
		return nil
	}
	available := p.availableProviders[capability]
	if providerAvailable(available, name) {
		return nil
	}
	if len(available) > 1 && isDefaultSelectable(capability) {
		return newError(
			ErrAmbiguousDefault,
			stagePlan,
			fmt.Sprintf(
				"capability %q has no deterministic default among %v",
				capability,
				available,
			),
			nil,
			map[string]string{"capability": capability},
		)
	}
	return newError(
		code,
		stagePlan,
		fmt.Sprintf("capability %q provider %q is not available", capability, name),
		nil,
		map[string]string{"capability": capability, "provider": name},
	)
}

func isDefaultSelectable(capability string) bool {
	switch capability {
	case capLoggerHandler, capLoggerWriter, capTracer, capMeter, capRegistry:
		return true
	default:
		return false
	}
}

func (p *planner) resolveChains() error {
	paths := []string{
		chainUnaryServer,
		chainStreamServer,
		chainUnaryClient,
		chainStreamClient,
		chainRESTAll,
		chainRESTRPC,
		chainRESTWeb,
	}
	for _, path := range paths {
		chain, source, err := p.selectChain(path)
		if err != nil {
			return err
		}
		if len(chain.Items) == 0 && chain.Template == "" && chain.Version == "" {
			continue
		}
		p.selectedChains[path] = chain
		p.chainSources[path] = source
		p.decisions = append(p.decisions, Decision{
			Kind:   "chain",
			Target: path,
			Value:  strings.Join(chain.Items, ","),
			Source: source,
			Reason: "resolved chain selection",
			Stage:  stagePlan,
		})
	}
	return nil
}

func (p *planner) selectChain(path string) (Chain, string, error) {
	if forced, ok := p.codeOverrides.ForcedTemplates[path]; ok {
		return p.expandTemplate(path, forced, "code_override")
	}
	if forced, ok := p.configOverrides.ForcedTemplates[path]; ok {
		return p.expandTemplate(path, forced, "config_override")
	}
	if selection, ok, err := explicitTemplateValue(p.input.Snapshot, path); err != nil {
		return Chain{}, "", err
	} else if ok {
		return p.expandTemplate(path, selection, "explicit_config")
	}

	if items, ok := explicitChainValue(p.input.Resolved, p.input.Snapshot, path); ok {
		if err := p.validateChainItems(path, items); err != nil {
			return Chain{}, "", err
		}
		return Chain{Items: append([]string(nil), items...)}, "explicit_config", nil
	}

	if _, disabled := p.codeOverrides.DisabledAuto[path]; !disabled {
		if _, disabled = p.configOverrides.DisabledAuto[path]; !disabled {
			if template, ok := p.mode.Templates[path]; ok {
				return p.expandTemplate(path, template, "mode:"+p.mode.Name)
			}
		}
	}
	return Chain{}, "", nil
}

func (p *planner) expandTemplate(
	path string,
	selection templateSelection,
	source string,
) (Chain, string, error) {
	template, ok := p.registry.template(selection.Name, selection.Version)
	if !ok {
		if selection.Version == "" {
			return Chain{}, "", newError(
				ErrTemplateVersionNotFound,
				stagePlan,
				fmt.Sprintf("template %q requires one version", selection.Name),
				nil,
				map[string]string{"path": path, "template": selection.Name},
			)
		}
		return Chain{}, "", newError(
			ErrUnknownTemplate,
			stagePlan,
			fmt.Sprintf("template %q version %q not found", selection.Name, selection.Version),
			nil,
			map[string]string{
				"path":     path,
				"template": selection.Name,
				"version":  selection.Version,
			},
		)
	}
	if err := p.validateChainItems(path, template.Items); err != nil {
		return Chain{}, "", err
	}
	return Chain{
		Template: template.Name,
		Version:  template.Version,
		Items:    append([]string(nil), template.Items...),
	}, source, nil
}

func (p *planner) validateChainItems(path string, items []string) error {
	specName := capabilityForChain(path)
	for _, item := range items {
		if err := p.requireProvider(specName, item, ErrUnknownExplicitBinding); err != nil {
			return err
		}
	}
	return nil
}

func chainConfigPaths(path string) []string {
	switch path {
	case chainUnaryServer:
		return []string{
			"yggdrasil.server.interceptors.unary",
			"yggdrasil.extensions.interceptors.unary_server",
		}
	case chainStreamServer:
		return []string{
			"yggdrasil.server.interceptors.stream",
			"yggdrasil.extensions.interceptors.stream_server",
		}
	case chainUnaryClient:
		return []string{
			"yggdrasil.clients.defaults.interceptors.unary",
			"yggdrasil.extensions.interceptors.unary_client",
		}
	case chainStreamClient:
		return []string{
			"yggdrasil.clients.defaults.interceptors.stream",
			"yggdrasil.extensions.interceptors.stream_client",
		}
	case chainRESTAll:
		return []string{
			"yggdrasil.transports.http.rest.middleware.all",
			"yggdrasil.extensions.middleware.rest_all",
		}
	case chainRESTRPC:
		return []string{
			"yggdrasil.transports.http.rest.middleware.rpc",
			"yggdrasil.extensions.middleware.rest_rpc",
		}
	case chainRESTWeb:
		return []string{
			"yggdrasil.transports.http.rest.middleware.web",
			"yggdrasil.extensions.middleware.rest_web",
		}
	default:
		return nil
	}
}

func extensionTemplateConfigPath(path string) string {
	switch path {
	case chainUnaryServer:
		return "yggdrasil.extensions.interceptors.unary_server"
	case chainStreamServer:
		return "yggdrasil.extensions.interceptors.stream_server"
	case chainUnaryClient:
		return "yggdrasil.extensions.interceptors.unary_client"
	case chainStreamClient:
		return "yggdrasil.extensions.interceptors.stream_client"
	case chainRESTAll:
		return "yggdrasil.extensions.middleware.rest_all"
	case chainRESTRPC:
		return "yggdrasil.extensions.middleware.rest_rpc"
	case chainRESTWeb:
		return "yggdrasil.extensions.middleware.rest_web"
	default:
		return ""
	}
}

func capabilityForChain(path string) string {
	switch path {
	case chainRESTAll, chainRESTRPC, chainRESTWeb:
		return capRESTMW
	default:
		return path
	}
}

func explicitDefaultValue(
	resolved settings.Resolved,
	snap config.Snapshot,
	capability string,
) (string, bool) {
	switch capability {
	case capLoggerHandler:
		if !pathExists(snap, "yggdrasil.logging.handlers.default.type") {
			return "", false
		}
		value := normalizedHandlerType(resolved.Logging.Handlers["default"].Type)
		return value, value != ""
	case capLoggerWriter:
		if !pathExists(snap, "yggdrasil.logging.writers.default.type") {
			return "", false
		}
		value := resolved.Logging.Writers["default"].Type
		return value, value != ""
	case capTracer:
		if !pathExists(snap, "yggdrasil.telemetry.tracer") {
			return "", false
		}
		return resolved.Telemetry.Tracer, resolved.Telemetry.Tracer != ""
	case capMeter:
		if !pathExists(snap, "yggdrasil.telemetry.meter") {
			return "", false
		}
		return resolved.Telemetry.Meter, resolved.Telemetry.Meter != ""
	case capRegistry:
		if !pathExists(snap, "yggdrasil.discovery.registry.type") {
			return "", false
		}
		return resolved.Discovery.Registry.Type, resolved.Discovery.Registry.Type != ""
	default:
		return "", false
	}
}

func explicitChainValue(
	resolved settings.Resolved,
	snap config.Snapshot,
	path string,
) ([]string, bool) {
	switch path {
	case chainUnaryServer:
		if !isExplicitOrderListPath(snap, "yggdrasil.server.interceptors.unary") &&
			!isExplicitOrderListPath(snap, "yggdrasil.extensions.interceptors.unary_server") {
			return nil, false
		}
		return append([]string(nil), resolved.Server.Interceptors.Unary...), true
	case chainStreamServer:
		if !isExplicitOrderListPath(snap, "yggdrasil.server.interceptors.stream") &&
			!isExplicitOrderListPath(snap, "yggdrasil.extensions.interceptors.stream_server") {
			return nil, false
		}
		return append([]string(nil), resolved.Server.Interceptors.Stream...), true
	case chainUnaryClient:
		if !isExplicitOrderListPath(snap, "yggdrasil.clients.defaults.interceptors.unary") &&
			!isExplicitOrderListPath(snap, "yggdrasil.extensions.interceptors.unary_client") {
			return nil, false
		}
		return append(
			[]string(nil),
			resolved.Root.Yggdrasil.Clients.Defaults.Interceptors.Unary...), true
	case chainStreamClient:
		if !isExplicitOrderListPath(snap, "yggdrasil.clients.defaults.interceptors.stream") &&
			!isExplicitOrderListPath(snap, "yggdrasil.extensions.interceptors.stream_client") {
			return nil, false
		}
		return append(
			[]string(nil),
			resolved.Root.Yggdrasil.Clients.Defaults.Interceptors.Stream...), true
	case chainRESTAll:
		if resolved.Transports.Rest == nil {
			return nil, false
		}
		if !isExplicitOrderListPath(snap, "yggdrasil.transports.http.rest.middleware.all") &&
			!isExplicitOrderListPath(snap, "yggdrasil.extensions.middleware.rest_all") {
			return nil, false
		}
		return append([]string(nil), resolved.Transports.Rest.Middleware.All...), true
	case chainRESTRPC:
		if resolved.Transports.Rest == nil {
			return nil, false
		}
		if !isExplicitOrderListPath(snap, "yggdrasil.transports.http.rest.middleware.rpc") &&
			!isExplicitOrderListPath(snap, "yggdrasil.extensions.middleware.rest_rpc") {
			return nil, false
		}
		return append([]string(nil), resolved.Transports.Rest.Middleware.RPC...), true
	case chainRESTWeb:
		if resolved.Transports.Rest == nil {
			return nil, false
		}
		if !isExplicitOrderListPath(snap, "yggdrasil.transports.http.rest.middleware.web") &&
			!isExplicitOrderListPath(snap, "yggdrasil.extensions.middleware.rest_web") {
			return nil, false
		}
		return append([]string(nil), resolved.Transports.Rest.Middleware.Web...), true
	default:
		return nil, false
	}
}

func explicitTemplateValue(snap config.Snapshot, path string) (templateSelection, bool, error) {
	configPath := extensionTemplateConfigPath(path)
	if configPath == "" {
		return templateSelection{}, false, nil
	}
	section := snap.Section(splitDotPath(configPath)...)
	if section.Empty() {
		return templateSelection{}, false, nil
	}
	switch value := section.Value().(type) {
	case string:
		name, version := parseTemplateReference(value)
		if name == "" {
			return templateSelection{}, false, nil
		}
		if version == "" {
			return templateSelection{}, false, newError(
				ErrTemplateVersionNotFound,
				stagePlan,
				fmt.Sprintf("template %q requires one version", name),
				nil,
				map[string]string{"path": path, "template": name},
			)
		}
		return templateSelection{Name: name, Version: version}, true, nil
	case map[string]any:
		rawTemplate, _ := value["template"].(string)
		rawVersion, _ := value["version"].(string)
		template := strings.TrimSpace(rawTemplate)
		version := strings.TrimSpace(rawVersion)
		if template == "" && version == "" {
			return templateSelection{}, false, nil
		}
		if version == "" {
			return templateSelection{}, false, newError(
				ErrTemplateVersionNotFound,
				stagePlan,
				fmt.Sprintf("template %q requires one version", template),
				nil,
				map[string]string{"path": path, "template": template},
			)
		}
		return templateSelection{Name: template, Version: version}, true, nil
	default:
		return templateSelection{}, false, nil
	}
}

func isExplicitOrderListPath(snap config.Snapshot, dotPath string) bool {
	section := snap.Section(splitDotPath(dotPath)...)
	if section.Empty() {
		return false
	}
	switch section.Value().(type) {
	case []any, []string:
		return true
	default:
		return false
	}
}

func parseTemplateReference(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	name, version, ok := strings.Cut(raw, "@")
	if !ok {
		return raw, ""
	}
	return strings.TrimSpace(name), strings.TrimSpace(version)
}

func templateKey(name, version string) string {
	return name + "@" + version
}

func pathExists(snap config.Snapshot, dotPath string) bool {
	return !snap.Section(splitDotPath(dotPath)...).Empty()
}

func splitDotPath(path string) []string {
	raw := strings.Split(path, ".")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}
