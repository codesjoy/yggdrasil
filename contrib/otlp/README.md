# Yggdrasil OTLP Exporter Integration

This module provides [OpenTelemetry Protocol (OTLP)](https://opentelemetry.io/docs/reference/specification/protocol/otlp/) exporters for traces and metrics in the Yggdrasil framework. It supports both gRPC and HTTP protocols for sending telemetry data to OTLP-compatible backends.

## Features

- **Dual Protocol Support**: Export via gRPC or HTTP
- **Traces Export**: Automatic trace export with configurable batch processing
- **Metrics Export**: Periodic metrics export with configurable intervals
- **TLS Configuration**: Secure connections with custom certificates
- **Retry Logic**: Automatic retry with configurable backoff
- **Resource Attributes**: Custom resource metadata for better traceability
- **Compression**: Optional gzip compression for reduced bandwidth
- **Fail-Safe Design**: Falls back to noop providers on errors

## Installation

```bash
go get github.com/codesjoy/yggdrasil/contrib/otlp/v2
```

## Quick Start

### 1. Import the Module

```go
import (
    _ "github.com/codesjoy/yggdrasil/contrib/otlp/v2"
    "github.com/codesjoy/yggdrasil/v2"
)
```

### 2. Configure OTLP Exporters

Add to your `config.yaml`:

```yaml
yggdrasil:
  tracer: otlp-grpc    # Select OTLP trace exporter
  meter: otlp-grpc     # Select OTLP metrics exporter

  otlp:
    trace:
      protocol: grpc            # grpc or http
      endpoint: localhost:4317  # OTLP endpoint
      tls:
        insecure: true          # true for development
      timeout: 30s
      compression: gzip

    metric:
      protocol: grpc
      endpoint: localhost:4317
      tls:
        insecure: true
      timeout: 30s
      compression: gzip
      exportInterval: 60s
```

### 3. Initialize Yggdrasil

```go
func main() {
    yggdrasil.Init("my-service")
    // Traces and metrics are now exported to OTLP
    yggdrasil.Serve(/*...*/)
}
```

## Configuration Reference

### Tracer Configuration

```yaml
yggdrasil:
  otlp:
    trace:
      # Protocol: grpc or http (default: grpc)
      protocol: grpc

      # OTLP endpoint
      # - For gRPC: localhost:4317 (default)
      # - For HTTP: localhost:4318 (default)
      endpoint: localhost:4317

      # TLS configuration
      tls:
        insecure: true          # Skip TLS verification (dev only)
        enabled: false          # Enable TLS
        caFile: /path/to/ca.crt
        certFile: /path/to/client.crt
        keyFile: /path/to/client.key

      # Headers (e.g., for authentication)
      headers:
        Authorization: "Bearer token"

      # Request timeout
      timeout: 30s

      # Compression: gzip, none, or empty
      compression: gzip

      # Retry configuration
      retry:
        enabled: true
        maxAttempts: 5
        initialDelay: 100ms
        maxDelay: 5s

      # Batch processing
      batch:
        batchTimeout: 5s
        maxQueueSize: 2048
        maxExportBatchSize: 512

      # Resource attributes
      resource:
        deployment.environment: "production"
        service.version: "1.0.0"
```

### Metrics Configuration

```yaml
yggdrasil:
  otlp:
    metric:
      # Protocol: grpc or http (default: grpc)
      protocol: grpc

      # OTLP endpoint
      endpoint: localhost:4317

      # TLS configuration (same as traces)
      tls:
        insecure: true

      # Request timeout
      timeout: 30s

      # Compression
      compression: gzip

      # Retry configuration (same as traces)
      retry:
        enabled: true

      # Export interval (how often to export)
      exportInterval: 60s

      # Export timeout
      exportTimeout: 30s

      # Temporality: cumulative or delta
      # Note: Delta temporality requires views configuration
      temporality: cumulative

      # Resource attributes
      resource:
        deployment.environment: "production"
```

## Protocol Selection

### gRPC Protocol (Recommended)

```yaml
yggdrasil:
  tracer: otlp-grpc
  meter: otlp-grpc
  otlp:
    trace:
      protocol: grpc
      endpoint: localhost:4317
```

**Benefits:**
- More efficient (binary protocol)
- Lower latency
- Better for high-throughput scenarios

### HTTP Protocol

```yaml
yggdrasil:
  tracer: otlp-http
  meter: otlp-http
  otlp:
    trace:
      protocol: http
      endpoint: localhost:4318
```

**Benefits:**
- Easier debugging (human-readable)
- Better firewall compatibility
- Simpler proxy configuration

## OTLP Backends

This module works with any OTLP-compatible backend:

### Jaeger

```bash
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 16686:16686 \
  jaegertracing/all-in-one:latest
```

### OpenTelemetry Collector

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

exporters:
  jaeger:
    endpoint: jaeger:14250
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [jaeger]
```

### Grafana Tempo

```yaml
# tempo.yaml
server:
  http_listen_port: 3200

distributor:
  receivers:
    otlp:
      protocols:
        grpc:
          endpoint: 0.0.0.0:4317
        http:
          endpoint: 0.0.0.0:4318
```

## Advanced Configuration

### Custom Headers

Useful for authentication:

```yaml
yggdrasil:
  otlp:
    trace:
      headers:
        X-Custom-Header: "value"
        Authorization: "Bearer ${TOKEN}"
```

### TLS Configuration

For production environments:

```yaml
yggdrasil:
  otlp:
    trace:
      tls:
        insecure: false
        enabled: true
        caFile: /etc/ssl/certs/ca.crt
        certFile: /etc/ssl/certs/client.crt
        keyFile: /etc/ssl/certs/client.key
```

### Batch Processing Tuning

For high-throughput services:

```yaml
yggdrasil:
  otlp:
    trace:
      batch:
        batchTimeout: 10s
        maxQueueSize: 10000
        maxExportBatchSize: 1000
```

### Retry Configuration

For unreliable networks:

```yaml
yggdrasil:
  otlp:
    trace:
      retry:
        enabled: true
        maxAttempts: 10
        initialDelay: 50ms
        maxDelay: 10s
```

## Architecture

```
┌─────────────────┐
│   Yggdrasil     │
│   Application   │
└────────┬────────┘
         │
         ├─────────────────────┐
         │                     │
    ┌────▼─────┐         ┌────▼─────┐
    │  Tracer  │         │   Meter  │
    │ Provider │         │ Provider │
    └────┬─────┘         └────┬─────┘
         │                     │
         │              ┌──────┴──────┐
         │              │  Periodic   │
         │              │    Reader   │
         │              └──────┬──────┘
         │                     │
         └──────────┬──────────┘
                    │
            ┌───────▼───────┐
            │ OTLP Exporter │
            │  (gRPC/HTTP)  │
            └───────┬───────┘
                    │
        ┌───────────▼───────────┐
        │  OTLP Backend         │
        │  (Jaeger/Tempo/etc.)  │
        └───────────────────────┘
```

## Troubleshooting

### No Traces Appearing

1. **Check configuration:**
   ```yaml
   yggdrasil:
     tracer: otlp-grpc  # Must be set
   ```

2. **Verify endpoint:**
   ```bash
   # Test gRPC connection
   grpcurl -plaintext localhost:4317 list

   # Test HTTP connection
   curl http://localhost:4318/v1/traces
   ```

3. **Check logs:**
   ```
   grep "OTLP" /var/log/myapp.log
   ```

### Connection Refused

- Ensure OTLP backend is running
- Check firewall rules
- Verify correct port (4317 for gRPC, 4318 for HTTP)

### TLS Errors

For development, use:
```yaml
tls:
  insecure: true
```

For production, ensure certificates are valid.

### High Memory Usage

Reduce batch size:
```yaml
batch:
  maxQueueSize: 512
  maxExportBatchSize: 256
```

## Examples

See the [example](./example/) directory for a complete working example.

## Contributing

Contributions are welcome! Please see the main Yggdrasil repository for contribution guidelines.

## License

Copyright 2022 The codesjoy Authors

Licensed under the Apache License, Version 2.0
