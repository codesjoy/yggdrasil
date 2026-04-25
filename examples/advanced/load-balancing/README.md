# Load Balancing Example

This example demonstrates load balancing with multiple service instances in the Yggdrasil framework.

## What You'll Get

- Multiple service instances
- Round-robin load balancing
- Request distribution tracking
- Load balancer statistics

## Setup

### 1. Start Multiple Server Instances

Open three terminals and start three instances:

```bash
# Terminal 1 - Instance 1
cd example/advanced/load-balancing
go run main.go --port 55884

# Terminal 2 - Instance 2
cd example/advanced/load-balancing
go run main.go --port 55885

# Terminal 3 - Instance 3
cd example/advanced/load-balancing
go run main.go --port 55886
```

### 2. Run the Client

```bash
cd example/advanced/load-balancing
go run main.go
```

## Expected Output

### Server Logs (each instance)

```
time=2025-01-26T10:00:00.000Z level=INFO msg="SayHello called" name=User-0 instance=lb-server-55884
time=2025-01-26T10:00:00.100Z level=INFO msg="SayHello called" name=User-1 instance=lb-server-55885
time=2025-01-26T10:00:00.200Z level=INFO msg="SayHello called" name=User-2 instance=lb-server-55886
time=2025-01-26T10:00:00.300Z level=INFO msg="SayHello called" name=User-3 instance=lb-server-55884
```

### Client Logs

```
time=2025-01-26T10:00:00.000Z level=INFO msg="=== Testing Load Balancing ==="
time=2025-01-26T10:00:00.050Z level=INFO msg="Testing unary RPC load balancing..."
time=2025-01-26T10:00:00.100Z level=INFO msg="request succeeded" index=0 message="Hello User-0! from lb-server-55884"
time=2025-01-26T10:00:00.150Z level=INFO msg="served by instance" instance=lb-server-55884
time=2025-01-26T10:00:00.250Z level=INFO msg="request succeeded" index=1 message="Hello User-1! from lb-server-55885"
time=2025-01-26T10:00:00.350Z level=INFO msg="served by instance" instance=lb-server-55885
time=2025-01-26T10:00:00.450Z level=INFO msg="request succeeded" index=2 message="Hello User-2! from lb-server-55886"
time=2025-01-26T10:00:00.550Z level=INFO msg="served by instance" instance=lb-server-55886
```

### Load Balancer Statistics

```
time=2025-01-26T10:00:01.000Z level=INFO msg="=== Load Balancer Statistics ==="
time=2025-01-26T10:00:01.010Z level=INFO msg="Request distribution" stats=map[lb-server-55884:4 lb-server-55885:3 lb-server-55886:3]
time=2025-01-26T10:00:01.020Z level=INFO msg="Response distribution" stats=map[lb-server-55884:4 lb-server-55885:3 lb-server-55886:3]
```

## How It Works

### 1. Multiple Instances

Each server instance has a unique instance ID:
```go
instanceID := fmt.Sprintf("%s-%d", hostname, port)
```

### 2. Client Configuration

The client is configured with multiple endpoints:
```yaml
endpoints:
  - address: "127.0.0.1:55884"
    protocol: "grpc"
  - address: "127.0.0.1:55885"
    protocol: "grpc"
  - address: "127.0.0.1:55886"
    protocol: "grpc"
```

### 3. Load Balancer

Yggdrasil automatically distributes requests across instances using round-robin.

### 4. Statistics Tracking

The client tracks which instance handled each request:
```go
if trailer, ok := metadata.FromTrailerCtx(ctx); ok {
    if instanceID, ok := trailer["server"]; ok {
        stats.RecordRequest(instanceID)
        stats.RecordResponse(instanceID)
    }
}
```

## Testing

### Test Load Balancing

Send multiple requests and verify they're distributed:

```bash
# Send 10 requests
for i in {1..10}; do
  grpcurl -plaintext -d "{\"name\":\"User-$i\"}" \
    127.0.0.1:55884 \
    GreeterService/SayHello
done
```

### Check Distribution

Verify that requests are evenly distributed across instances.

## Common Issues

**Q: Requests are not distributed evenly?**

A: Check that all instances are running and that the client can establish connections to them. This example uses built-in round-robin over the configured endpoints.

**Q: How to handle instance failure or a dependency starting late?**

A: The balancer only picks clients in the `READY` state. In practice this relies on the transport reporting connection state changes, which the built-in gRPC client does and will reconnect in the background after transient connect failures. HTTP clients are connectionless and do not self-demote on request failures, so this example uses gRPC endpoints and does not configure a separate health-check or weighted-balancing policy.
