# OTLP Example Application

This example demonstrates how to use the Yggdrasil OTLP contrib module to export traces and metrics to an OTLP-compatible backend (such as Jaeger, Tempo, or OpenTelemetry Collector).

## Prerequisites

1. **Yggdrasil Framework**: The main Yggdrasil framework must be installed
2. **OTLP Backend**: You need an OTLP-compatible backend running. Options include:
   - [Jaeger](https://www.jaegertracing.io/) (with OTLP receiver)
   - [Grafana Tempo](https://grafana.com/oss/tempo/)
   - [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
   - [SigNoz](https://signoz.io/)

For development, you can run Jaeger with OTLP support using Docker:

```bash
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 16686:16686 \
  jaegertracing/all-in-one:latest
```

This will:
- Accept OTLP gRPC on port 4317
- Accept OTLP HTTP on port 4318
- Expose the Jaeger UI on port 16686 at http://localhost:16686

## Running the Example

1. **Start the OTLP backend** (if not already running):

```bash
# Using Docker
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 16686:16686 \
  jaegertracing/all-in-one:latest
```

2. **Run the example**:

```bash
# From the contrib/otlp/example directory
go run main.go
```

Or with a custom config file:

```bash
go run main.go -config config.yaml
```

3. **Generate traces and metrics**:

Visit http://localhost:8080 in your browser or use curl:

```bash
curl http://localhost:8080
```

4. **View traces in Jaeger**:

Open http://localhost:16686 in your browser:
- Select "otlp-example" as the service
- Click "Find Traces"
- You should see traces from your HTTP requests

## Configuration

The example uses the configuration in `config.yaml`. Key configuration options:

### Selecting the Exporter

```yaml
yggdrasil:
  tracer: otlp-grpc    # Use gRPC for traces (or otlp-http)
  meter: otlp-grpc     # Use gRPC for metrics (or otlp-http)
```

### OTLP Endpoint

```yaml
yggdrasil:
  otlp:
    trace:
      endpoint: localhost:4317  # Default gRPC endpoint
      # endpoint: localhost:4318  # Default HTTP endpoint
```

### TLS Configuration

For production with TLS:

```yaml
yggdrasil:
  otlp:
    trace:
      tls:
        insecure: false
        enabled: true
        caFile: /path/to/ca.crt
        certFile: /path/to/client.crt
        keyFile: /path/to/client.key
```

## What the Example Does

The example HTTP server:
1. Creates a span for each HTTP request
2. Records a counter metric for each request
3. Adds attributes to both traces and metrics
4. Exports them to the configured OTLP endpoint

You can modify the `handleRequest` function to experiment with:
- Different span types (creating child spans)
- Additional metric types (gauges, histograms)
- Custom attributes
- Baggage propagation

## Troubleshooting

### No traces appearing

1. Check if the OTLP backend is running:
```bash
# Check if Jaeger is accessible
curl http://localhost:16686/api/services
```

2. Verify the endpoint configuration in `config.yaml`

3. Check the application logs for any errors

4. Try switching from gRPC to HTTP (or vice versa) in the config

### Connection refused

- Ensure the OTLP backend is running
- Check the port number (4317 for gRPC, 4318 for HTTP)
- Verify no firewall rules are blocking the connection

### TLS errors

- For development, set `tls.insecure: true` in the config
- For production, provide valid CA and client certificates

## Next Steps

1. **Instrument your own code**: Use the same patterns shown in this example
2. **Configure batch processing**: Adjust batch size and timeout for your workload
3. **Set up resource attributes**: Add service metadata for better traceability
4. **Configure retry logic**: Handle transient network failures
5. **Use HTTP vs gRPC**: Choose based on your infrastructure requirements

For more information, see the main [README](../README.md) in the parent directory.
