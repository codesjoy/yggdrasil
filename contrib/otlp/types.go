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

// Package otlp provides configuration for OTLP exporters.
package otlp

import "time"

// Config is the top-level configuration for OTLP exporters.
type Config struct {
	Trace  TraceExporterConfig  `mapstructure:"trace"`
	Metric MetricExporterConfig `mapstructure:"metric"`
}

// TraceExporterConfig is the configuration for OTLP trace exporter.
type TraceExporterConfig struct {
	Protocol    string                 `mapstructure:"protocol"`    // grpc or http
	Endpoint    string                 `mapstructure:"endpoint"`    // OTLP endpoint
	TLS         TLSConfig              `mapstructure:"tls"`         // TLS configuration
	Headers     map[string]string      `mapstructure:"headers"`     // Custom headers (e.g., auth)
	Timeout     time.Duration          `mapstructure:"timeout"`     // Request timeout
	Compression string                 `mapstructure:"compression"` // Compression type (gzip, none)
	Retry       RetryConfig            `mapstructure:"retry"`       // Retry configuration
	Batch       BatchConfig            `mapstructure:"batch"`       // Batch processing config
	Resource    map[string]interface{} `mapstructure:"resource"`    // Resource attributes
}

// MetricExporterConfig is the configuration for OTLP metrics exporter.
type MetricExporterConfig struct {
	Protocol       string                 `mapstructure:"protocol"`       // grpc or http
	Endpoint       string                 `mapstructure:"endpoint"`       // OTLP endpoint
	TLS            TLSConfig              `mapstructure:"tls"`            // TLS configuration
	Headers        map[string]string      `mapstructure:"headers"`        // Custom headers
	Timeout        time.Duration          `mapstructure:"timeout"`        // Request timeout
	Compression    string                 `mapstructure:"compression"`    // Compression type
	Retry          RetryConfig            `mapstructure:"retry"`          // Retry configuration
	Temporality    string                 `mapstructure:"temporality"`    // cumulative or delta
	Resource       map[string]interface{} `mapstructure:"resource"`       // Resource attributes
	ExportInterval time.Duration          `mapstructure:"exportInterval"` // Metrics export interval
	ExportTimeout  time.Duration          `mapstructure:"exportTimeout"`  // Metrics export timeout
}

// TLSConfig is the TLS configuration for OTLP clients.
type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`  // Whether TLS is enabled
	Insecure bool   `mapstructure:"insecure"` // Skip TLS verification (for development)
	CAFile   string `mapstructure:"caFile"`   // Path to CA certificate
	CertFile string `mapstructure:"certFile"` // Path to client certificate
	KeyFile  string `mapstructure:"keyFile"`  // Path to client key
}

// RetryConfig is the retry configuration for OTLP exporters.
type RetryConfig struct {
	Enabled      bool          `mapstructure:"enabled"`      // Enable retry
	MaxAttempts  int           `mapstructure:"maxAttempts"`  // Maximum retry attempts
	InitialDelay time.Duration `mapstructure:"initialDelay"` // Initial delay before retry
	MaxDelay     time.Duration `mapstructure:"maxDelay"`     // Maximum delay between retries
}

// BatchConfig is the batch configuration for trace exporters.
type BatchConfig struct {
	BatchTimeout       time.Duration `mapstructure:"batchTimeout"`       // Time to wait before exporting
	MaxQueueSize       int           `mapstructure:"maxQueueSize"`       // Maximum queue size
	MaxExportBatchSize int           `mapstructure:"maxExportBatchSize"` // Maximum batch size
}
