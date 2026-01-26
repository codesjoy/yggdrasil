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

package polaris

import (
	"sort"
	"strings"
	"sync"

	polaris "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/api"
	polariscfg "github.com/polarismesh/polaris-go/pkg/config"
)

func applyTokenToConfig(cfg polariscfg.Configuration, token string) {
	if cfg == nil || token == "" {
		return
	}
	global := cfg.GetGlobal()
	if global != nil {
		global.GetServerConnector().SetToken(token)
	}
	configFile := cfg.GetConfigFile()
	if configFile != nil {
		configFile.GetConfigConnectorConfig().SetToken(token)
	}
}

func applyConfigAddressesToConfig(cfg polariscfg.Configuration, addresses []string) {
	if cfg == nil || len(addresses) == 0 {
		return
	}
	configFile := cfg.GetConfigFile()
	if configFile == nil {
		return
	}
	connector := configFile.GetConfigConnectorConfig()
	if connector == nil {
		return
	}
	connector.SetAddresses(addresses)
}

type sdkHolder struct {
	sdkName         string
	addresses       []string
	configAddresses []string

	ctxOnce sync.Once
	ctx     api.SDKContext
	ctxErr  error

	providerOnce sync.Once
	provider     providerAPI
	providerErr  error

	consumerOnce sync.Once
	consumer     consumerAPI
	consumerErr  error

	configOnce sync.Once
	config     configAPI
	configErr  error

	limitOnce sync.Once
	limit     limitAPI
	limitErr  error

	cbOnce sync.Once
	cb     circuitBreakerAPI
	cbErr  error

	routerOnce sync.Once
	router     routerAPI
	routerErr  error
}

func (h *sdkHolder) initContext() {
	cfg := LoadSDKConfig(h.sdkName)
	if cfg.ConfigFile != "" {
		c, err := polariscfg.LoadConfigurationByFile(cfg.ConfigFile)
		if err != nil {
			h.ctxErr = err
			return
		}
		h.ctx, h.ctxErr = polaris.NewSDKContextByConfig(c)
		return
	}

	addresses := h.addresses
	if len(addresses) == 0 {
		addresses = cfg.Addresses
	}
	configAddresses := h.configAddresses
	if len(configAddresses) == 0 {
		configAddresses = cfg.ConfigAddress
	}
	if cfg.Token != "" || len(configAddresses) > 0 {
		var c *polariscfg.ConfigurationImpl
		if len(addresses) > 0 {
			c = polariscfg.NewDefaultConfiguration(addresses)
		} else {
			c = polariscfg.NewDefaultConfigurationWithDomain()
		}
		applyTokenToConfig(c, cfg.Token)
		applyConfigAddressesToConfig(c, configAddresses)
		h.ctx, h.ctxErr = polaris.NewSDKContextByConfig(c)
		return
	}
	if len(addresses) > 0 {
		h.ctx, h.ctxErr = polaris.NewSDKContextByAddress(addresses...)
		return
	}

	h.ctx, h.ctxErr = polaris.NewSDKContext()
}

func (h *sdkHolder) getContext() (api.SDKContext, error) {
	h.ctxOnce.Do(h.initContext)
	return h.ctx, h.ctxErr
}

func (h *sdkHolder) getProvider() (providerAPI, error) {
	h.providerOnce.Do(func() {
		ctx, err := h.getContext()
		if err != nil {
			h.providerErr = err
			return
		}
		h.provider = polaris.NewProviderAPIByContext(ctx)
	})
	return h.provider, h.providerErr
}

func (h *sdkHolder) getConsumer() (consumerAPI, error) {
	h.consumerOnce.Do(func() {
		ctx, err := h.getContext()
		if err != nil {
			h.consumerErr = err
			return
		}
		h.consumer = polaris.NewConsumerAPIByContext(ctx)
	})
	return h.consumer, h.consumerErr
}

func (h *sdkHolder) getConfig() (configAPI, error) {
	h.configOnce.Do(func() {
		ctx, err := h.getContext()
		if err != nil {
			h.configErr = err
			return
		}
		h.config = polaris.NewConfigAPIByContext(ctx)
	})
	return h.config, h.configErr
}

func (h *sdkHolder) getLimit() (limitAPI, error) {
	h.limitOnce.Do(func() {
		ctx, err := h.getContext()
		if err != nil {
			h.limitErr = err
			return
		}
		h.limit = polaris.NewLimitAPIByContext(ctx)
	})
	return h.limit, h.limitErr
}

func (h *sdkHolder) getCircuitBreaker() (circuitBreakerAPI, error) {
	h.cbOnce.Do(func() {
		ctx, err := h.getContext()
		if err != nil {
			h.cbErr = err
			return
		}
		h.cb = polaris.NewCircuitBreakerAPIByContext(ctx)
	})
	return h.cb, h.cbErr
}

func (h *sdkHolder) getRouter() (routerAPI, error) {
	h.routerOnce.Do(func() {
		ctx, err := h.getContext()
		if err != nil {
			h.routerErr = err
			return
		}
		h.router = polaris.NewRouterAPIByContext(ctx)
	})
	return h.router, h.routerErr
}

var (
	sdkMu      sync.Mutex
	sdkHolders = map[string]*sdkHolder{}
)

func getSDKHolder(sdkName string, addresses []string, configAddresses []string) *sdkHolder {
	cp := append([]string(nil), addresses...)
	sort.Strings(cp)
	ccp := append([]string(nil), configAddresses...)
	sort.Strings(ccp)

	key := sdkName + "|" + strings.Join(cp, ",") + "|" + strings.Join(ccp, ",")
	sdkMu.Lock()
	defer sdkMu.Unlock()
	if h, ok := sdkHolders[key]; ok {
		return h
	}
	h := &sdkHolder{sdkName: sdkName, addresses: cp, configAddresses: ccp}
	sdkHolders[key] = h
	return h
}
