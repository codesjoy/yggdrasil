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
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	otlpmetricgrpc "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otlpmetrichttp "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// createGRPCTraceClientOptions creates gRPC client options for trace exporter.
func createGRPCTraceClientOptions(cfg TraceExporterConfig) ([]otlptracegrpc.Option, error) {
	var opts []otlptracegrpc.Option

	// Set endpoint
	if cfg.Endpoint != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
	}

	// Set headers
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
	}

	// Set timeout
	if cfg.Timeout > 0 {
		opts = append(opts, otlptracegrpc.WithTimeout(cfg.Timeout))
	}

	// Set compression
	switch cfg.Compression {
	case "gzip":
		opts = append(opts, otlptracegrpc.WithCompressor("gzip"))
	case "none", "":
		// Default (no compression)
	default:
		// Unknown compression, use default
	}

	// Configure TLS/retry options
	grpcOpts, err := createGRPCDialOptions(cfg.TLS, cfg.Retry)
	if err != nil {
		return nil, err
	}
	if len(grpcOpts) > 0 {
		opts = append(opts, otlptracegrpc.WithDialOption(grpcOpts...))
	}

	// Configure retry
	if cfg.Retry.Enabled {
		backoff := otlptracegrpc.RetryConfig{
			Enabled:         true,
			InitialInterval: cfg.Retry.InitialDelay,
			MaxInterval:     cfg.Retry.MaxDelay,
			MaxElapsedTime:  cfg.Retry.MaxDelay * time.Duration(cfg.Retry.MaxAttempts),
		}
		opts = append(opts, otlptracegrpc.WithRetry(backoff))
	}

	return opts, nil
}

// createHTTPTraceClientOptions creates HTTP client options for trace exporter.
func createHTTPTraceClientOptions(cfg TraceExporterConfig) []otlptracehttp.Option {
	var opts []otlptracehttp.Option

	// Set endpoint
	if cfg.Endpoint != "" {
		opts = append(opts, otlptracehttp.WithEndpoint(cfg.Endpoint))
	}

	// Set headers
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
	}

	// Set timeout
	if cfg.Timeout > 0 {
		opts = append(opts, otlptracehttp.WithTimeout(cfg.Timeout))
	}

	// Set compression
	switch cfg.Compression {
	case "gzip":
		opts = append(opts, otlptracehttp.WithCompression(otlptracehttp.GzipCompression))
	case "none", "":
		opts = append(opts, otlptracehttp.WithCompression(otlptracehttp.NoCompression))
	default:
		opts = append(opts, otlptracehttp.WithCompression(otlptracehttp.NoCompression))
	}

	// Configure TLS
	tlsClientOpt := createHTTPClientTLSOption(cfg.TLS)
	if tlsClientOpt != nil {
		opts = append(opts, tlsClientOpt)
	}

	// Configure retry
	if cfg.Retry.Enabled {
		backoff := otlptracehttp.RetryConfig{
			Enabled:         true,
			InitialInterval: cfg.Retry.InitialDelay,
			MaxInterval:     cfg.Retry.MaxDelay,
			MaxElapsedTime:  cfg.Retry.MaxDelay * time.Duration(cfg.Retry.MaxAttempts),
		}
		opts = append(opts, otlptracehttp.WithRetry(backoff))
	}

	return opts
}

// createGRPCMeterClientOptions creates gRPC client options for metrics exporter.
func createGRPCMeterClientOptions(cfg MetricExporterConfig) ([]otlpmetricgrpc.Option, error) {
	var opts []otlpmetricgrpc.Option

	// Set endpoint
	if cfg.Endpoint != "" {
		opts = append(opts, otlpmetricgrpc.WithEndpoint(cfg.Endpoint))
	}

	// Set headers
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Headers))
	}

	// Set timeout
	if cfg.Timeout > 0 {
		opts = append(opts, otlpmetricgrpc.WithTimeout(cfg.Timeout))
	}

	// Set compression
	switch cfg.Compression {
	case "gzip":
		opts = append(opts, otlpmetricgrpc.WithCompressor("gzip"))
	case "none", "":
		// Default
	default:
		// Unknown compression, use default
	}

	// Configure TLS/retry options
	grpcOpts, err := createGRPCDialOptions(cfg.TLS, cfg.Retry)
	if err != nil {
		return nil, err
	}
	if len(grpcOpts) > 0 {
		opts = append(opts, otlpmetricgrpc.WithDialOption(grpcOpts...))
	}

	// Configure retry
	if cfg.Retry.Enabled {
		backoff := otlpmetricgrpc.RetryConfig{
			Enabled:         true,
			InitialInterval: cfg.Retry.InitialDelay,
			MaxInterval:     cfg.Retry.MaxDelay,
			MaxElapsedTime:  cfg.Retry.MaxDelay * time.Duration(cfg.Retry.MaxAttempts),
		}
		opts = append(opts, otlpmetricgrpc.WithRetry(backoff))
	}

	return opts, nil
}

// createHTTPMeterClientOptions creates HTTP client options for metrics exporter.
func createHTTPMeterClientOptions(cfg MetricExporterConfig) []otlpmetrichttp.Option {
	var opts []otlpmetrichttp.Option

	// Set endpoint
	if cfg.Endpoint != "" {
		opts = append(opts, otlpmetrichttp.WithEndpoint(cfg.Endpoint))
	}

	// Set headers
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlpmetrichttp.WithHeaders(cfg.Headers))
	}

	// Set timeout
	if cfg.Timeout > 0 {
		opts = append(opts, otlpmetrichttp.WithTimeout(cfg.Timeout))
	}

	// Set compression
	switch cfg.Compression {
	case "gzip":
		opts = append(opts, otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression))
	case "none", "":
		opts = append(opts, otlpmetrichttp.WithCompression(otlpmetrichttp.NoCompression))
	default:
		opts = append(opts, otlpmetrichttp.WithCompression(otlpmetrichttp.NoCompression))
	}

	// Configure TLS
	tlsClientOpt := createHTTPMetricClientTLSOption(cfg.TLS)
	if tlsClientOpt != nil {
		opts = append(opts, tlsClientOpt)
	}

	// Configure retry
	if cfg.Retry.Enabled {
		backoff := otlpmetrichttp.RetryConfig{
			Enabled:         true,
			InitialInterval: cfg.Retry.InitialDelay,
			MaxInterval:     cfg.Retry.MaxDelay,
			MaxElapsedTime:  cfg.Retry.MaxDelay * time.Duration(cfg.Retry.MaxAttempts),
		}
		opts = append(opts, otlpmetrichttp.WithRetry(backoff))
	}

	return opts
}

// createGRPCDialOptions creates gRPC dial options based on TLS and retry config.
func createGRPCDialOptions(tlsCfg TLSConfig, _ RetryConfig) ([]grpc.DialOption, error) {
	var opts []grpc.DialOption

	if tlsCfg.Insecure {
		// Skip TLS verification (for development)
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if tlsCfg.Enabled {
		// TLS with custom certificates
		tlsConfig := &tls.Config{} // nolint:gosec

		if tlsCfg.CAFile != "" {
			caCert, err := os.ReadFile(tlsCfg.CAFile)
			if err != nil {
				return nil, err
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsConfig.RootCAs = caCertPool
		}

		if tlsCfg.CertFile != "" && tlsCfg.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		// Default to insecure for development
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return opts, nil
}

// createHTTPClientTLSOption creates TLS option for HTTP clients.
func createHTTPClientTLSOption(tlsCfg TLSConfig) otlptracehttp.Option {
	if tlsCfg.Insecure {
		return otlptracehttp.WithInsecure()
	}

	if tlsCfg.Enabled {
		tlsConfig := &tls.Config{} // nolint:gosec

		if tlsCfg.CAFile != "" {
			caCert, err := os.ReadFile(tlsCfg.CAFile)
			if err == nil {
				caCertPool := x509.NewCertPool()
				caCertPool.AppendCertsFromPEM(caCert)
				tlsConfig.RootCAs = caCertPool
			}
		}

		if tlsCfg.CertFile != "" && tlsCfg.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
			if err == nil {
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}

		return otlptracehttp.WithTLSClientConfig(tlsConfig)
	}

	// Default to insecure for development
	return otlptracehttp.WithInsecure()
}

// createHTTPMetricClientTLSOption creates TLS option for HTTP metric clients.
func createHTTPMetricClientTLSOption(tlsCfg TLSConfig) otlpmetrichttp.Option {
	if tlsCfg.Insecure {
		return otlpmetrichttp.WithInsecure()
	}

	if tlsCfg.Enabled {
		tlsConfig := &tls.Config{} // nolint:gosec

		if tlsCfg.CAFile != "" {
			caCert, err := os.ReadFile(tlsCfg.CAFile)
			if err == nil {
				caCertPool := x509.NewCertPool()
				caCertPool.AppendCertsFromPEM(caCert)
				tlsConfig.RootCAs = caCertPool
			}
		}

		if tlsCfg.CertFile != "" && tlsCfg.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
			if err == nil {
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}

		return otlpmetrichttp.WithTLSClientConfig(tlsConfig)
	}

	// Default to insecure for development
	return otlpmetrichttp.WithInsecure()
}

// getMetricTemporality converts string to metric temporality.
func getMetricTemporality(temporality string) string {
	switch temporality {
	case "delta":
		return "delta"
	case "cumulative", "":
		return "cumulative"
	default:
		return "cumulative"
	}
}
