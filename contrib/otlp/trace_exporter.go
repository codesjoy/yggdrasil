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

package otlp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/codesjoy/yggdrasil/v2/config"
	xotel "github.com/codesjoy/yggdrasil/v2/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// NewTracerProvider creates a new OTLP tracer provider.
func NewTracerProvider(serviceName string, cfg TraceExporterConfig) (trace.TracerProvider, error) {
	ctx := context.Background()

	var (
		exporter sdktrace.SpanExporter
		err      error
	)

	switch cfg.Protocol {
	case "grpc", "":
		exporter, err = createGRPCTraceExporter(ctx, cfg)
	case "http":
		exporter, err = createHTTPTraceExporter(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s (supported: grpc, http)", cfg.Protocol)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	// Build resource attributes
	resourceAttrs := buildResourceAttributes(serviceName, cfg.Resource)

	// Create batch span processor
	var batchOpts []sdktrace.BatchSpanProcessorOption
	if cfg.Batch.BatchTimeout > 0 {
		batchOpts = append(batchOpts, sdktrace.WithBatchTimeout(cfg.Batch.BatchTimeout))
	} else {
		batchOpts = append(batchOpts, sdktrace.WithBatchTimeout(defaultBatchTimeout))
	}
	if cfg.Batch.MaxQueueSize > 0 {
		batchOpts = append(batchOpts, sdktrace.WithMaxQueueSize(cfg.Batch.MaxQueueSize))
	} else {
		batchOpts = append(batchOpts, sdktrace.WithMaxQueueSize(defaultMaxQueueSize))
	}
	if cfg.Batch.MaxExportBatchSize > 0 {
		batchOpts = append(batchOpts, sdktrace.WithMaxExportBatchSize(cfg.Batch.MaxExportBatchSize))
	} else {
		batchOpts = append(batchOpts, sdktrace.WithMaxExportBatchSize(defaultMaxExportBatchSize))
	}

	bsp := sdktrace.NewBatchSpanProcessor(exporter, batchOpts...)

	// Create tracer provider
	attrs := xotel.ParseAttributes(resourceAttrs)
	res, err := resource.New(ctx,
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	return tp, nil
}

// createGRPCTraceExporter creates a gRPC OTLP trace exporter.
func createGRPCTraceExporter(
	ctx context.Context,
	cfg TraceExporterConfig,
) (sdktrace.SpanExporter, error) {
	opts, err := createGRPCTraceClientOptions(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client options: %w", err)
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC trace exporter: %w", err)
	}

	return exporter, nil
}

// createHTTPTraceExporter creates an HTTP OTLP trace exporter.
func createHTTPTraceExporter(
	ctx context.Context,
	cfg TraceExporterConfig,
) (sdktrace.SpanExporter, error) {
	opts := createHTTPTraceClientOptions(cfg)

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP trace exporter: %w", err)
	}

	return exporter, nil
}

// newGRPCTracerProvider creates a gRPC tracer provider from config.
func newGRPCTracerProvider(serviceName string) trace.TracerProvider {
	cfg := loadTraceConfig()
	cfg.Protocol = "grpc"

	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultGRPCEndpoint
	}

	tp, err := NewTracerProvider(serviceName, cfg)
	if err != nil {
		slog.Warn("failed to create OTLP gRPC tracer provider, using noop",
			slog.String("error", err.Error()))
		return noop.NewTracerProvider()
	}

	return tp
}

// newHTTPTracerProvider creates an HTTP tracer provider from config.
func newHTTPTracerProvider(serviceName string) trace.TracerProvider {
	cfg := loadTraceConfig()
	cfg.Protocol = "http"

	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultHTTPEndpoint
	}

	tp, err := NewTracerProvider(serviceName, cfg)
	if err != nil {
		slog.Warn("failed to create OTLP HTTP tracer provider, using noop",
			slog.String("error", err.Error()))
		return noop.NewTracerProvider()
	}

	return tp
}

// loadTraceConfig loads trace configuration from global config.
func loadTraceConfig() TraceExporterConfig {
	cfgKey := config.Join(config.KeyBase, "otlp", "trace")
	cfgVal := config.Get(cfgKey)

	var cfg TraceExporterConfig
	_ = cfgVal.Scan(&cfg)

	// Apply defaults
	if cfg.Batch.BatchTimeout == 0 {
		cfg.Batch.BatchTimeout = defaultBatchTimeout
	}
	if cfg.Batch.MaxQueueSize == 0 {
		cfg.Batch.MaxQueueSize = defaultMaxQueueSize
	}
	if cfg.Batch.MaxExportBatchSize == 0 {
		cfg.Batch.MaxExportBatchSize = defaultMaxExportBatchSize
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.Retry.InitialDelay == 0 {
		cfg.Retry.InitialDelay = defaultRetryInitialDelay
	}
	if cfg.Retry.MaxDelay == 0 {
		cfg.Retry.MaxDelay = defaultRetryMaxDelay
	}
	if cfg.Retry.MaxAttempts == 0 && cfg.Retry.Enabled {
		cfg.Retry.MaxAttempts = defaultMaxAttempts
	}

	return cfg
}

// buildResourceAttributes builds OpenTelemetry resource attributes.
func buildResourceAttributes(
	serviceName string,
	customAttrs map[string]interface{},
) map[string]any {
	attrs := make(map[string]any)

	// Standard service attributes
	attrs["service.name"] = serviceName

	// Add custom attributes
	for k, v := range customAttrs {
		attrs[k] = v
	}

	return attrs
}

const (
	defaultGRPCEndpoint       = "localhost:4317"
	defaultHTTPEndpoint       = "localhost:4318"
	defaultBatchTimeout       = 5 * 1000 * 1000 * 1000 // 5s in nanoseconds
	defaultMaxQueueSize       = 2048
	defaultMaxExportBatchSize = 512
	defaultTimeout            = 30 * 1000 * 1000 * 1000 // 30s in nanoseconds
	defaultRetryInitialDelay  = 100 * 1000 * 1000       // 100ms in nanoseconds
	defaultRetryMaxDelay      = 5 * 1000 * 1000 * 1000  // 5s in nanoseconds
	defaultMaxAttempts        = 5
)

func init() {
	// Register trace exporters
	xotel.RegisterTracerProviderBuilder("otlp-grpc", newGRPCTracerProvider)
	xotel.RegisterTracerProviderBuilder("otlp-http", newHTTPTracerProvider)
}
