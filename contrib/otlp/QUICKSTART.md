# OTLP Exporter - Quick Start Guide

This guide will help you get started with exporting traces and metrics to OTLP-compatible backends using Yggdrasil.

## 5-Minute Setup

### Step 1: Install Dependencies

```bash
go get github.com/codesjoy/yggdrasil/contrib/otlp/v2
```

### Step 2: Import the Module

```go
import (
    _ "github.com/codesjoy/yggdrasil/contrib/otlp/v2"
    "github.com/codesjoy/yggdrasil/v2"
)
```

### Step 3: Add Configuration

Create or update your `config.yaml`:

```yaml
yggdrasil:
  tracer: otlp-grpc
  meter: otlp-grpc

  otlp:
    trace:
      endpoint: localhost:4317
      tls:
        insecure: true
    metric:
      endpoint: localhost:4317
      tls:
        insecure: true
```

### Step 4: Initialize Yggdrasil

```go
func main() {
    yggdrasil.Init("my-service")
    // Your code here
}
```

That's it! Your traces and metrics are now being exported.

## Running an OTLP Backend

### Option 1: Jaeger (Recommended for Development)

```bash
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 16686:16686 \
  jaegertracing/all-in-one:latest
```

View traces at: http://localhost:16686

### Option 2: OpenTelemetry Collector

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  logging:
    loglevel: debug

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [logging]
```

```bash
otelcol --config otel-collector-config.yaml
```

### Option 3: SigNoz

```bash
docker run -d --name signoz \
  -p 4317:4317 \
  -p 3301:3301 \
  signoz/signoz:latest
```

## Configuration Examples

### Production with TLS

```yaml
yggdrasil:
  otlp:
    trace:
      endpoint: otlp.example.com:4317
      tls:
        insecure: false
        enabled: true
        caFile: /etc/ssl/certs/ca.crt
      headers:
        Authorization: "Bearer ${OTLP_TOKEN}"
      retry:
        enabled: true
        maxAttempts: 5
```

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

### Custom Resource Attributes

```yaml
yggdrasil:
  otlp:
    trace:
      resource:
        deployment.environment: "production"
        service.version: "1.0.0"
        cloud.provider: "aws"
        cloud.region: "us-west-2"
```

## Verification

Send some test traffic:

```bash
curl http://localhost:8080
```

Check Jaeger UI:
1. Open http://localhost:16686
2. Select your service name
3. Click "Find Traces"
4. You should see your traces!

## Troubleshooting

### No traces appearing

1. Check the backend is running:
   ```bash
   curl http://localhost:16686/api/services
   ```

2. Check application logs for errors

3. Verify endpoint configuration

4. Try switching from gRPC to HTTP

### Connection refused

- Ensure OTLP backend is running
- Check port numbers (4317 for gRPC, 4318 for HTTP)
- Verify firewall rules

### TLS errors

For development, use:
```yaml
tls:
  insecure: true
```

## Next Steps

- Read the full [README.md](README.md) for detailed configuration
- Check the [example](example/) directory for a working example
- Configure batch processing for high-throughput scenarios
- Set up custom resource attributes
- Configure retry logic for production environments

## Need Help?

- Check the main [Yggdrasil documentation](../../README.md)
- Open an issue on GitHub
- See [OpenTelemetry documentation](https://opentelemetry.io/docs/instrumentation/go/)
