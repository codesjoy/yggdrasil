package yggdrasil

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/interceptor"
	xotel "github.com/codesjoy/yggdrasil/v2/otel"
	"github.com/codesjoy/yggdrasil/v2/registry"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/rest"
	"github.com/codesjoy/yggdrasil/v2/rest/marshaler"
	"github.com/codesjoy/yggdrasil/v2/rest/middleware"
)

func validateStartup(opts *options) error {
	validateKey := config.Join(config.KeyBase, "startup", "validate")
	strict := config.GetBool(config.Join(validateKey, "strict"), false)
	enable := strict || config.GetBool(config.Join(validateKey, "enable"), false)
	if !enable {
		return nil
	}

	var multiErr error
	addErr := func(msg string, err error, attrs ...slog.Attr) {
		if err == nil {
			return
		}
		if strict {
			multiErr = errors.Join(multiErr, fmt.Errorf("%s: %w", msg, err))
			return
		}
		attrs = append(attrs, slog.Any("error", err))
		args := make([]any, 0, len(attrs))
		for _, a := range attrs {
			args = append(args, a)
		}
		slog.Warn(msg, args...)
	}

	typeName := config.GetString(config.Join(config.KeyBase, "registry", "type"))
	if typeName != "" && registry.GetBuilder(typeName) == nil {
		addErr(
			"registry builder not found",
			fmt.Errorf("type=%s", typeName),
			slog.String("type", typeName),
		)
	}

	if tracerName := config.GetString(config.Join(config.KeyBase, "tracer")); tracerName != "" {
		if _, ok := xotel.GetTracerProviderBuilder(tracerName); !ok {
			addErr(
				"tracer provider builder not found",
				fmt.Errorf("name=%s", tracerName),
				slog.String("name", tracerName),
			)
		}
	}
	if meterName := config.GetString(config.Join(config.KeyBase, "meter")); meterName != "" {
		if _, ok := xotel.GetMeterProviderBuilder(meterName); !ok {
			addErr(
				"meter provider builder not found",
				fmt.Errorf("name=%s", meterName),
				slog.String("name", meterName),
			)
		}
	}

	if opts != nil &&
		(len(opts.serviceDesc) > 0 || len(opts.restServiceDesc) > 0 || len(opts.restRawHandleDesc) > 0) {
		protocols := config.Get(config.Join(config.KeyBase, "server", "protocol")).StringSlice()
		for _, protocol := range protocols {
			if remote.GetServerBuilder(protocol) == nil {
				addErr(
					"remote server builder not found",
					fmt.Errorf("protocol=%s", protocol),
					slog.String("protocol", protocol),
				)
			}
		}
	}

	unaryNames := config.Get(config.Join(config.KeyBase, "interceptor", "unary_server")).
		StringSlice()
	for _, name := range unaryNames {
		if !interceptor.HasUnaryServerIntBuilder(name) {
			addErr(
				"unary server interceptor not found",
				fmt.Errorf("name=%s", name),
				slog.String("name", name),
			)
		}
	}
	streamNames := config.Get(config.Join(config.KeyBase, "interceptor", "stream_server")).
		StringSlice()
	for _, name := range streamNames {
		if !interceptor.HasStreamServerIntBuilder(name) {
			addErr(
				"stream server interceptor not found",
				fmt.Errorf("name=%s", name),
				slog.String("name", name),
			)
		}
	}

	unaryClientNames := config.Get(config.Join(config.KeyBase, "client", "interceptor", "unary")).
		StringSlice()
	for _, name := range unaryClientNames {
		if !interceptor.HasUnaryClientIntBuilder(name) {
			addErr(
				"unary client interceptor not found",
				fmt.Errorf("name=%s", name),
				slog.String("name", name),
			)
		}
	}
	streamClientNames := config.Get(config.Join(config.KeyBase, "client", "interceptor", "stream")).
		StringSlice()
	for _, name := range streamClientNames {
		if !interceptor.HasStreamClientIntBuilder(name) {
			addErr(
				"stream client interceptor not found",
				fmt.Errorf("name=%s", name),
				slog.String("name", name),
			)
		}
	}

	clientMap := config.Get(config.Join(config.KeyBase, "client")).Map(map[string]any{})
	for appName := range clientMap {
		if appName == "interceptor" {
			continue
		}
		unaryNames := config.Get(config.Join(config.KeyBase, "client", appName, "interceptor", "unary")).
			StringSlice()
		for _, name := range unaryNames {
			if !interceptor.HasUnaryClientIntBuilder(name) {
				addErr(
					"unary client interceptor not found",
					fmt.Errorf("name=%s", name),
					slog.String("name", name),
					slog.String("app", appName),
				)
			}
		}
		streamNames := config.Get(config.Join(config.KeyBase, "client", appName, "interceptor", "stream")).
			StringSlice()
		for _, name := range streamNames {
			if !interceptor.HasStreamClientIntBuilder(name) {
				addErr(
					"stream client interceptor not found",
					fmt.Errorf("name=%s", name),
					slog.String("name", name),
					slog.String("app", appName),
				)
			}
		}
	}

	if config.GetBool(config.Join(config.KeyBase, "rest", "enable"), false) {
		cfg := &rest.Config{}
		if err := config.Get(config.Join(config.KeyBase, "rest")).Scan(cfg); err != nil {
			addErr("rest config invalid", err)
		} else {
			for _, name := range cfg.Middleware.All {
				if !middleware.HasBuilder(name) {
					addErr("rest middleware not found", fmt.Errorf("name=%s", name), slog.String("name", name))
				}
			}
			for _, name := range cfg.Middleware.RPC {
				if !middleware.HasBuilder(name) {
					addErr("rest middleware not found", fmt.Errorf("name=%s", name), slog.String("name", name))
				}
			}
			for _, name := range cfg.Middleware.Web {
				if !middleware.HasBuilder(name) {
					addErr("rest middleware not found", fmt.Errorf("name=%s", name), slog.String("name", name))
				}
			}
			if !middleware.HasBuilder("marshaler") {
				addErr("rest middleware not found", fmt.Errorf("name=marshaler"), slog.String("name", "marshaler"))
			}
		}

		schemesKey := config.Join(config.KeyBase, "rest", "marshaler", "support")
		schemes := config.GetStringSlice(schemesKey, []string{"jsonpb"})
		for _, scheme := range schemes {
			if !marshaler.HasMarshallerBuilder(scheme) {
				addErr(
					"rest marshaler builder not found",
					fmt.Errorf("scheme=%s", scheme),
					slog.String("scheme", scheme),
				)
			}
		}
	}

	return multiErr
}
