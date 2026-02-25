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
	"context"
	"fmt"
	"time"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/interceptor"
	"github.com/codesjoy/yggdrasil/v2/status"
	"github.com/codesjoy/yggdrasil/v2/utils/xmap"
	"github.com/mitchellh/mapstructure"
	polaris "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
	"google.golang.org/genproto/googleapis/rpc/code"
)

func init() {
	interceptor.RegisterUnaryClientIntBuilder("polaris_ratelimit", buildPolarisRateLimitUnary)
	interceptor.RegisterUnaryClientIntBuilder(
		"polaris_circuitbreaker",
		buildPolarisCircuitBreakerUnary,
	)
}

type governanceConfig struct {
	Addresses       []string `mapstructure:"addresses"`
	SDK             string   `mapstructure:"sdk"`
	Namespace       string   `mapstructure:"namespace"`
	CallerService   string   `mapstructure:"callerService"`
	CallerNamespace string   `mapstructure:"callerNamespace"`

	RateLimit rateLimitConfig `mapstructure:"rateLimit"`

	CircuitBreaker circuitBreakerConfig `mapstructure:"circuitBreaker"`

	Routing routingConfig `mapstructure:"routing"`
}

type rateLimitConfig struct {
	Enable     bool              `mapstructure:"enable"`
	Token      uint32            `mapstructure:"token"`
	Timeout    time.Duration     `mapstructure:"timeout"`
	RetryCount int               `mapstructure:"retryCount"`
	Arguments  map[string]string `mapstructure:"arguments"`
	Release    bool              `mapstructure:"release"`
}

type circuitBreakerConfig struct {
	Enable bool `mapstructure:"enable"`
}

type routingConfig struct {
	Enable     bool              `mapstructure:"enable"`
	RecoverAll bool              `mapstructure:"recover_all"`
	Routers    []string          `mapstructure:"routers"`
	Timeout    time.Duration     `mapstructure:"timeout"`
	RetryCount int               `mapstructure:"retryCount"`
	LbPolicy   string            `mapstructure:"lbPolicy"`
	Arguments  map[string]string `mapstructure:"arguments"`
}

func loadGovernanceConfig(serviceName string) governanceConfig {
	m := loadGovernanceConfigMap(serviceName)
	return decodeGovernanceConfig(m)
}

func loadGovernanceConfigMap(serviceName string) map[string]any {
	serviceKey := fmt.Sprintf("{%s}", serviceName)
	base := config.Get(config.Join(config.KeyBase, "polaris", "governance", "config")).
		Map(map[string]any{})
	svc := config.Get(config.Join(config.KeyBase, "polaris", "governance", serviceKey, "config")).
		Map(map[string]any{})
	xmap.MergeStringMap(base, svc)
	xmap.CoverInterfaceMapToStringMap(base)
	return base
}

func decodeGovernanceConfig(m map[string]any) governanceConfig {
	var out governanceConfig
	decoder, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.TextUnmarshallerHookFunc(),
			mapstructure.StringToTimeDurationHookFunc(),
		),
		Result: &out,
	})
	if decoder != nil {
		_ = decoder.Decode(m)
	}
	return out
}

func buildPolarisRateLimitUnary(serviceName string) interceptor.UnaryClientInterceptor {
	cfg := loadGovernanceConfig(serviceName)
	if !cfg.RateLimit.Enable {
		return func(ctx context.Context, method string, req, reply any, invoker interceptor.UnaryInvoker) error {
			return invoker(ctx, method, req, reply)
		}
	}

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = "default"
	}
	sdkName := resolveSDKName(serviceName, cfg.SDK)
	addresses := resolveSDKAddresses(serviceName, cfg.SDK, cfg.Addresses)
	api, initErr := getSDKHolder(sdkName, addresses, nil).getLimit()

	return func(ctx context.Context, method string, req, reply any, invoker interceptor.UnaryInvoker) error {
		if initErr != nil {
			return initErr
		}

		qr := polaris.NewQuotaRequest()
		qr.SetNamespace(namespace)
		qr.SetService(serviceName)
		qr.SetMethod(method)
		if cfg.RateLimit.Token > 0 {
			qr.SetToken(cfg.RateLimit.Token)
		}
		if cfg.RateLimit.Timeout > 0 {
			qr.SetTimeout(cfg.RateLimit.Timeout)
		}
		if cfg.RateLimit.RetryCount > 0 {
			qr.SetRetryCount(cfg.RateLimit.RetryCount)
		}
		for k, v := range cfg.RateLimit.Arguments {
			qr.AddArgument(model.BuildCustomArgument(k, v))
		}

		future, err := api.GetQuota(qr)
		if err != nil {
			return err
		}
		if cfg.RateLimit.Release {
			defer future.Release()
		}
		resp := future.GetImmediately()
		if resp == nil {
			return xerror.New(code.Code_UNKNOWN, "polaris rate limit: empty response")
		}
		if resp.Code != model.QuotaResultOk {
			msg := resp.Info
			if msg == "" {
				msg = "polaris rate limit exceeded"
			}
			return xerror.New(code.Code_RESOURCE_EXHAUSTED, msg)
		}
		if resp.WaitMs > 0 {
			t := time.NewTimer(time.Duration(resp.WaitMs) * time.Millisecond)
			defer t.Stop()
			select {
			case <-ctx.Done():
				switch ctx.Err() {
				case context.DeadlineExceeded:
					return xerror.Wrap(ctx.Err(), code.Code_DEADLINE_EXCEEDED, "")
				case context.Canceled:
					return xerror.Wrap(ctx.Err(), code.Code_CANCELLED, "")
				default:
					return xerror.Wrap(ctx.Err(), code.Code_UNKNOWN, "")
				}
			case <-t.C:
			}
		}
		return invoker(ctx, method, req, reply)
	}
}

func buildPolarisCircuitBreakerUnary(serviceName string) interceptor.UnaryClientInterceptor {
	cfg := loadGovernanceConfig(serviceName)
	if !cfg.CircuitBreaker.Enable {
		return func(ctx context.Context, method string, req, reply any, invoker interceptor.UnaryInvoker) error {
			return invoker(ctx, method, req, reply)
		}
	}

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = "default"
	}
	callerNamespace := cfg.CallerNamespace
	if callerNamespace == "" {
		callerNamespace = namespace
	}
	callerService := cfg.CallerService
	if callerService == "" {
		callerService = "unknown"
	}

	addresses := resolveSDKAddresses(serviceName, cfg.SDK, cfg.Addresses)
	sdkName := resolveSDKName(serviceName, cfg.SDK)
	api, initErr := getSDKHolder(sdkName, addresses, nil).getCircuitBreaker()
	dst := &model.ServiceKey{Namespace: namespace, Service: serviceName}
	src := &model.ServiceKey{Namespace: callerNamespace, Service: callerService}

	return func(ctx context.Context, method string, req, reply any, invoker interceptor.UnaryInvoker) error {
		if initErr != nil {
			return initErr
		}

		res, err := model.NewMethodResource(dst, src, method)
		if err != nil {
			return err
		}
		cr, err := api.Check(res)
		if err != nil {
			return err
		}
		if cr != nil && !cr.Pass {
			msg := "polaris circuit breaker open"
			if cr.RuleName != "" {
				msg = msg + ": " + cr.RuleName
			}
			return xerror.New(code.Code_UNAVAILABLE, msg)
		}

		start := time.Now()
		invokeErr := invoker(ctx, method, req, reply)
		retStatus := model.RetSuccess
		retCode := "0"
		if invokeErr != nil {
			retStatus = model.RetFail
			retCode = status.FromError(invokeErr).Code().String()
		}
		_ = api.Report(&model.ResourceStat{
			Resource:  res,
			RetCode:   retCode,
			Delay:     time.Since(start),
			RetStatus: retStatus,
		})
		return invokeErr
	}
}
