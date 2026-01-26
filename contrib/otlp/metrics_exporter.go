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
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// NewMeterProvider creates a new OTLP meter provider.
func NewMeterProvider(
	serviceName string,
	cfg MetricExporterConfig,
) (*sdkmetric.MeterProvider, error) {
	ctx := context.Background()

	var (
		exporter sdkmetric.Exporter
		err      error
	)

	switch cfg.Protocol {
	case "grpc", "":
		exporter, err = createGRPCMeterExporter(ctx, cfg)
	case "http":
		exporter, err = createHTTPMeterExporter(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s (supported: grpc, http)", cfg.Protocol)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	// Build resource attributes
	resourceAttrs := buildResourceAttributes(serviceName, cfg.Resource)

	// Create resource
	attrs := xotel.ParseAttributes(resourceAttrs)
	res, err := resource.New(ctx,
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create periodic reader
	var readerOpts []sdkmetric.PeriodicReaderOption
	if cfg.ExportInterval > 0 {
		readerOpts = append(readerOpts, sdkmetric.WithInterval(cfg.ExportInterval))
	} else {
		readerOpts = append(readerOpts, sdkmetric.WithInterval(defaultExportInterval))
	}
	if cfg.ExportTimeout > 0 {
		readerOpts = append(readerOpts, sdkmetric.WithTimeout(cfg.ExportTimeout))
	} else {
		readerOpts = append(readerOpts, sdkmetric.WithTimeout(defaultExportTimeout))
	}

	reader := sdkmetric.NewPeriodicReader(exporter, readerOpts...)

	// Create meter provider
	// Note: Temporality configuration is not directly supported on periodic readers.
	// Default cumulative temporality is used. For delta temporality, use views.
	var providerOpts []sdkmetric.Option
	providerOpts = append(providerOpts, sdkmetric.WithResource(res))
	providerOpts = append(providerOpts, sdkmetric.WithReader(reader))

	mp := sdkmetric.NewMeterProvider(providerOpts...)

	return mp, nil
}

// createGRPCMeterExporter creates a gRPC OTLP metric exporter.
func createGRPCMeterExporter(
	ctx context.Context,
	cfg MetricExporterConfig,
) (sdkmetric.Exporter, error) {
	opts, err := createGRPCMeterClientOptions(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client options: %w", err)
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC metric exporter: %w", err)
	}

	return exporter, nil
}

// createHTTPMeterExporter creates an HTTP OTLP metric exporter.
func createHTTPMeterExporter(
	ctx context.Context,
	cfg MetricExporterConfig,
) (sdkmetric.Exporter, error) {
	opts := createHTTPMeterClientOptions(cfg)

	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP metric exporter: %w", err)
	}

	return exporter, nil
}

// newGRPCMeterProvider creates a gRPC meter provider from config.
func newGRPCMeterProvider(serviceName string) metric.MeterProvider {
	cfg := loadMetricConfig()
	cfg.Protocol = "grpc"

	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultGRPCEndpoint
	}

	mp, err := NewMeterProvider(serviceName, cfg)
	if err != nil {
		slog.Warn("failed to create OTLP gRPC meter provider, using noop",
			slog.String("error", err.Error()))
		// Create noop meter provider directly
		return sdkmetric.NewMeterProvider()
	}

	return mp
}

// newHTTPMeterProvider creates an HTTP meter provider from config.
func newHTTPMeterProvider(serviceName string) metric.MeterProvider {
	cfg := loadMetricConfig()
	cfg.Protocol = "http"

	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultHTTPEndpoint
	}

	mp, err := NewMeterProvider(serviceName, cfg)
	if err != nil {
		slog.Warn("failed to create OTLP HTTP meter provider, using noop",
			slog.String("error", err.Error()))
		// Create noop meter provider directly
		return sdkmetric.NewMeterProvider()
	}

	return mp
}

// loadMetricConfig loads metric configuration from global config.
func loadMetricConfig() MetricExporterConfig {
	cfgKey := config.Join(config.KeyBase, "otlp", "metric")
	cfgVal := config.Get(cfgKey)

	var cfg MetricExporterConfig
	_ = cfgVal.Scan(&cfg)

	// Apply defaults
	if cfg.ExportInterval == 0 {
		cfg.ExportInterval = defaultExportInterval
	}
	if cfg.ExportTimeout == 0 {
		cfg.ExportTimeout = defaultExportTimeout
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

const (
	defaultExportInterval = 60 * 1000 * 1000 * 1000 // 60s in nanoseconds
	defaultExportTimeout  = 30 * 1000 * 1000 * 1000 // 30s in nanoseconds
)

func init() {
	// Register metrics exporters
	xotel.RegisterMeterProviderBuilder("otlp-grpc", newGRPCMeterProvider)
	xotel.RegisterMeterProviderBuilder("otlp-http", newHTTPMeterProvider)
}
