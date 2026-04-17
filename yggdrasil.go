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

// Package yggdrasil integrate core modules into the yggdrasil entry package
package yggdrasil

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"

	"github.com/codesjoy/yggdrasil/v2/application"
	"github.com/codesjoy/yggdrasil/v2/client"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/governor"
	"github.com/codesjoy/yggdrasil/v2/internal/instance"
	"github.com/codesjoy/yggdrasil/v2/internal/remotelog"
	"github.com/codesjoy/yggdrasil/v2/logger"
	xotel "github.com/codesjoy/yggdrasil/v2/otel"
	"github.com/codesjoy/yggdrasil/v2/registry"
	"github.com/codesjoy/yggdrasil/v2/server"

	"go.opentelemetry.io/otel"
)

type lifecycleState uint32

const (
	lifecycleStateNew lifecycleState = iota
	lifecycleStateInitialized
	lifecycleStateRunning
	lifecycleStateStopped
)

var (
	errApplicationAlreadyRunning = errors.New("application is already running")
	errApplicationNotInitialized = errors.New("please initialize yggdrasil before serve")
	errRestartUnsupported        = errors.New("restarting yggdrasil in the same process is not supported")
)

var (
	app, _ = application.New()
	initMu sync.Mutex
	state  = lifecycleStateNew
	opts   = &options{
		serviceDesc:     map[*server.ServiceDesc]interface{}{},
		restServiceDesc: map[*server.RestServiceDesc]restServiceDesc{},
	}
)

// NewClient create a new client
func NewClient(name string) (client.Client, error) {
	cli, err := client.NewClient(context.Background(), name)
	if err != nil {
		slog.Error("fault to new client", slog.String("name", name), slog.Any("error", err))
		return nil, err
	}
	return cli, nil
}

// Init initialize application.
func Init(appName string, ops ...Option) (err error) {
	initMu.Lock()
	defer initMu.Unlock()

	return initLocked(appName, ops...)
}

func initLocked(appName string, ops ...Option) (err error) {
	switch state {
	case lifecycleStateInitialized, lifecycleStateRunning:
		return nil
	case lifecycleStateStopped:
		return errRestartUnsupported
	}
	if err = applyOpt(opts, ops...); err != nil {
		slog.Error("fault to initialize yggdrasil", slog.Any("error", err))
		return err
	}
	if err = initConfigChain(opts); err != nil {
		slog.Error("fault to load startup config", slog.Any("error", err))
		return err
	}
	if err = initLogger(); err != nil {
		slog.Error("fault to initialize logger", slog.Any("error", err))
		return err
	}

	initInstanceInfo(appName)
	if err = validateStartup(opts); err != nil {
		slog.Error("startup validation failed", slog.Any("error", err))
		return err
	}
	if err = initGovernor(opts); err != nil {
		slog.Error("fault to initialize governor", slog.Any("error", err))
		return err
	}
	initRegistry(opts)
	initTracer(opts)
	initMeter(opts)
	state = lifecycleStateInitialized
	return nil
}

// Serve serves the application.
func Serve(ops ...Option) (err error) {
	initMu.Lock()
	switch state {
	case lifecycleStateNew:
		initMu.Unlock()
		return errApplicationNotInitialized
	case lifecycleStateRunning:
		initMu.Unlock()
		return errApplicationAlreadyRunning
	case lifecycleStateStopped:
		initMu.Unlock()
		return errRestartUnsupported
	}
	if err = applyOpt(opts, ops...); err != nil {
		initMu.Unlock()
		slog.Error("fault to initialize yggdrasil", slog.Any("error", err))
		return err
	}
	dropServeStageConfigSources(opts)
	if err = validateStartup(opts); err != nil {
		initMu.Unlock()
		slog.Error("startup validation failed", slog.Any("error", err))
		return err
	}
	if err = initServer(opts); err != nil {
		initMu.Unlock()
		slog.Error("fault to initialize yggdrasil", slog.Any("error", err))
		return err
	}
	if err = app.Init(opts.getAppOpts()...); err != nil {
		initMu.Unlock()
		return err
	}
	state = lifecycleStateRunning
	initMu.Unlock()

	if err = app.Run(); err != nil {
		initMu.Lock()
		state = lifecycleStateStopped
		initMu.Unlock()
		slog.Error("the application was ended forcefully", slog.Any("error", err))
		return err
	}
	initMu.Lock()
	state = lifecycleStateStopped
	initMu.Unlock()
	return nil
}

// Run runs the application.
func Run(appName string, ops ...Option) (err error) {
	initMu.Lock()
	switch state {
	case lifecycleStateRunning:
		initMu.Unlock()
		return errApplicationAlreadyRunning
	case lifecycleStateStopped:
		initMu.Unlock()
		return errRestartUnsupported
	}
	if state == lifecycleStateNew {
		if err = initLocked(appName, ops...); err != nil {
			initMu.Unlock()
			return err
		}
	}
	if err = initServer(opts); err != nil {
		initMu.Unlock()
		slog.Error("fault to initialize yggdrasil", slog.Any("error", err))
		return err
	}
	if err = app.Init(opts.getAppOpts()...); err != nil {
		initMu.Unlock()
		return err
	}
	state = lifecycleStateRunning
	initMu.Unlock()

	if err = app.Run(); err != nil {
		initMu.Lock()
		state = lifecycleStateStopped
		initMu.Unlock()
		slog.Error("fault to run yggdrasil application", slog.Any("error", err))
		return err
	}
	initMu.Lock()
	state = lifecycleStateStopped
	initMu.Unlock()
	return nil
}

// Stop stops the application.
func Stop() error {
	initMu.Lock()
	if state != lifecycleStateNew {
		state = lifecycleStateStopped
	}
	initMu.Unlock()

	var err error
	if stopErr := app.Stop(); stopErr != nil {
		err = errors.Join(err, stopErr)
		slog.Error("fault to stop yggdrasil application", slog.Any("error", err))
	}
	if closeErr := closeManagedConfigSources(opts); closeErr != nil {
		err = errors.Join(err, closeErr)
		slog.Error("fault to close config sources", slog.Any("error", closeErr))
	}
	return err
}

func initRegistry(opts *options) {
	typeName := config.GetString(config.Join(config.KeyBase, "registry", "type"))
	if typeName == "" {
		return
	}
	r, err := registry.Get()
	if err != nil {
		slog.Warn(
			"fault to initialize registry",
			slog.String("type", typeName),
			slog.Any("error", err),
		)
		return
	}
	opts.registry = r
	if c, ok := r.(io.Closer); ok {
		opts.appOpts = append(
			opts.appOpts,
			application.WithCleanup("registry", func(context.Context) error {
				return c.Close()
			}),
		)
	}
}

func initTracer(opts *options) {
	if tracerName := config.GetString(config.Join(config.KeyBase, "tracer")); len(tracerName) > 0 {
		constructor, ok := xotel.GetTracerProviderBuilder(tracerName)
		if !ok {
			slog.Warn("not found tracer provider", slog.String("name", tracerName))
			return
		}
		tp := constructor(InstanceName())
		otel.SetTracerProvider(tp)
		if opts != nil {
			if s, ok := tp.(interface{ Shutdown(context.Context) error }); ok {
				opts.appOpts = append(opts.appOpts, application.WithCleanup("tracer", s.Shutdown))
			} else if c, ok := tp.(io.Closer); ok {
				opts.appOpts = append(opts.appOpts, application.WithCleanup("tracer", func(context.Context) error {
					return c.Close()
				}))
			}
		}
	}
}

func initMeter(opts *options) {
	if meterName := config.GetString(config.Join(config.KeyBase, "meter")); len(meterName) > 0 {
		constructor, ok := xotel.GetMeterProviderBuilder(meterName)
		if !ok {
			slog.Warn("not found meter provider", slog.String("name", meterName))
			return
		}
		mp := constructor(InstanceName())
		otel.SetMeterProvider(mp)
		if opts != nil {
			if s, ok := mp.(interface{ Shutdown(context.Context) error }); ok {
				opts.appOpts = append(opts.appOpts, application.WithCleanup("meter", s.Shutdown))
			} else if c, ok := mp.(io.Closer); ok {
				opts.appOpts = append(opts.appOpts, application.WithCleanup("meter", func(context.Context) error {
					return c.Close()
				}))
			}
		}
	}
}

func initInstanceInfo(appName string) {
	instance.InitInstanceInfo(appName)
}

func initLogger() error {
	loggerKeyBase := config.Join(config.KeyBase, "logger")
	vals := config.ValueToValues(config.Get(config.Join(loggerKeyBase, "handler", "default")))
	typeName := vals.Get("type").String("console")
	writerName := vals.Get("writer").String("default")
	if writerName == "default" {
		writerType := config.GetString(
			config.Join("yggdrasil", "logger", "writer", "default", "type"),
		)
		if writerType == "" {
			err := config.Set(
				config.Join("yggdrasil", "logger", "writer", "default", "type"),
				"console",
			)
			if err != nil {
				return err
			}
		}
	}

	handlerBuilder, err := logger.GetHandlerBuilder(typeName)
	if err != nil {
		return err
	}
	h, err := handlerBuilder(writerName, vals.Get("config"))
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(h))
	remoteLoggerLvStr := config.GetString(
		config.Join(config.KeyBase, "remote", "logger_level"),
		"error",
	)
	var remoteLoggerLv slog.Level
	if err = remoteLoggerLv.UnmarshalText([]byte(remoteLoggerLvStr)); err != nil {
		return err
	}
	remotelog.Init(remoteLoggerLv, h)
	return nil
}

func initGovernor(opts *options) error {
	svr, err := governor.NewServer()
	if err != nil {
		return err
	}
	opts.governor = svr
	return nil
}

func initServer(opts *options) error {
	if len(opts.serviceDesc) == 0 && len(opts.restServiceDesc) == 0 &&
		len(opts.restRawHandleDesc) == 0 {
		return nil
	}
	svr, err := server.NewServer()
	if err != nil {
		return err
	}
	for k, v := range opts.serviceDesc {
		svr.RegisterService(k, v)
	}
	for k, v := range opts.restServiceDesc {
		svr.RegisterRestService(k, v.ss, v.Prefix...)
	}

	if len(opts.restRawHandleDesc) > 0 {
		svr.RegisterRestRawHandlers(opts.restRawHandleDesc...)
	}
	opts.server = svr
	return nil
}

func applyOpt(opts *options, ops ...Option) error {
	for _, f := range ops {
		if err := f(opts); err != nil {
			return err
		}
	}
	return nil
}
