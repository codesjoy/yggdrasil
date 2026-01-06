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
	"log/slog"
	"sync/atomic"

	"github.com/codesjoy/yggdrasil/v2/application"
	"github.com/codesjoy/yggdrasil/v2/client"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/governor"
	"github.com/codesjoy/yggdrasil/v2/internal/instance"
	"github.com/codesjoy/yggdrasil/v2/logger"
	xotel "github.com/codesjoy/yggdrasil/v2/otel"
	"github.com/codesjoy/yggdrasil/v2/registry"
	logger2 "github.com/codesjoy/yggdrasil/v2/remote/logger"
	"github.com/codesjoy/yggdrasil/v2/server"

	"go.opentelemetry.io/otel"
)

var (
	app, _      = application.New()
	appRunning  atomic.Bool
	initialized atomic.Bool
	opts        = &options{
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
func Init(appName string, ops ...Option) error {
	if !initialized.CompareAndSwap(false, true) {
		return nil
	}
	if err := initLogger(); err != nil {
		slog.Error("fault to initialize logger", slog.Any("error", err))
		return err
	}

	initInstanceInfo(appName)
	if err := applyOpt(opts, ops...); err != nil {
		slog.Error("fault to initialize yggdrasil", slog.Any("error", err))
		return err
	}
	if err := initGovernor(opts); err != nil {
		slog.Error("fault to initialize governor", slog.Any("error", err))
		return err
	}
	initRegistry(opts)
	initTracer()
	initMeter()
	return nil
}

// Serve serves the application.
func Serve(ops ...Option) error {
	if !appRunning.CompareAndSwap(false, true) {
		return errors.New("application had already running")
	}
	if !initialized.Load() {
		return errors.New("please initialize the yggdrasil before serve")
	}
	if err := applyOpt(opts, ops...); err != nil {
		slog.Error("fault to initialize yggdrasil", slog.Any("error", err))
		return err
	}
	if err := initServer(opts); err != nil {
		slog.Error("fault to initialize yggdrasil", slog.Any("error", err))
		return err
	}
	if err := app.Init(opts.getAppOpts()...); err != nil {
		return err
	}
	if err := app.Run(); err != nil {
		slog.Error("the application was ended forcefully", slog.Any("error", err))
		return err
	}
	return nil
}

// Run runs the application.
func Run(appName string, ops ...Option) error {
	if !appRunning.CompareAndSwap(false, true) {
		return errors.New("application had already running")
	}
	if err := Init(appName, ops...); err != nil {
		return err
	}
	if err := initServer(opts); err != nil {
		slog.Error("fault to initialize yggdrasil", slog.Any("error", err))
		return err
	}
	if err := app.Init(opts.getAppOpts()...); err != nil {
		return err
	}
	if err := app.Run(); err != nil {
		slog.Error("fault to run yggdrasil application", slog.Any("error", err))
		return err
	}
	return nil
}

// Stop stops the application.
func Stop() error {
	if err := app.Stop(); err != nil {
		slog.Error("fault to stop yggdrasil application", slog.Any("error", err))
		return err
	}
	return nil
}

func initRegistry(opts *options) {
	name := config.GetString(config.Join(config.KeyBase, "registry"))
	if len(name) == 0 {
		return
	}
	f := registry.GetBuilder(name)
	if f == nil {
		slog.Warn("not found registry", slog.String("name", name))
		return
	}
	opts.registry = f()
}

func initTracer() {
	if tracerName := config.GetString(config.Join(config.KeyBase, "tracer")); len(tracerName) > 0 {
		constructor, ok := xotel.GetTracerProviderBuilder(tracerName)
		if !ok {
			slog.Warn("not found tracer provider", slog.String("name", tracerName))
			return
		}
		otel.SetTracerProvider(constructor(InstanceName()))
	}
}

func initMeter() {
	if meterName := config.GetString(config.Join(config.KeyBase, "meter")); len(meterName) > 0 {
		constructor, ok := xotel.GetMeterProviderBuilder(meterName)
		if !ok {
			slog.Warn("not found meter provider", slog.String("name", meterName))
			return
		}
		otel.SetMeterProvider(constructor(InstanceName()))
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
		writerType := config.GetString(config.Join("yggdrasil", "logger", "writer", "default", "type"))
		if writerType == "" {
			err := config.Set(config.Join("yggdrasil", "logger", "writer", "default", "type"), "console")
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
	remoteLoggerLvStr := config.GetString(config.Join(config.KeyBase, "remote", "logger_level"), "error")
	var remoteLoggerLv slog.Level
	if err = remoteLoggerLv.UnmarshalText([]byte(remoteLoggerLvStr)); err != nil {
		return err
	}
	logger2.InitLogger(remoteLoggerLv, h)
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
	if len(opts.serviceDesc) == 0 && len(opts.restServiceDesc) == 0 && len(opts.restRawHandleDesc) == 0 {
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
