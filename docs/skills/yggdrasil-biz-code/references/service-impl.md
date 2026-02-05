# Service Implementation and Registration

## Implementation skeleton
```go
import (
    "context"

    "github.com/codesjoy/yggdrasil/v2"
    pb "your_module/api/your_service/v1"
)

type ServiceImpl struct {
    pb.UnimplementedYourServiceServer
}

func (s *ServiceImpl) YourMethod(
    ctx context.Context,
    req *pb.YourRequest,
) (*pb.YourReply, error) {
    // TODO: implement business logic
    return &pb.YourReply{}, nil
}
```

## Register and start the server
```go
ss := &ServiceImpl{}
if err := yggdrasil.Serve(
    yggdrasil.WithServiceDesc(&pb.YourServiceServiceDesc, ss),
); err != nil {
    // handle error
}
```

## REST support (optional)
```go
if err := yggdrasil.Serve(
    yggdrasil.WithServiceDesc(&pb.YourServiceServiceDesc, ss),
    yggdrasil.WithRestServiceDesc(&pb.YourServiceRestServiceDesc, ss),
); err != nil {
    // handle error
}
```

## Custom HTTP handler (optional)
```go
import "github.com/codesjoy/yggdrasil/v2/server"

yggdrasil.WithRestRawHandleDesc(&server.RestRawHandlerDesc{
    Method:  http.MethodGet,
    Path:    "/healthz",
    Handler: YourHandler,
})
```

## Multiple services
You can pass multiple `WithServiceDesc(...)` options to `yggdrasil.Serve(...)` to register multiple services.

## Repo references
- Entry example: `example/sample/server/main.go`
- REST example: `example/advanced/rest/server/main.go`
- Detailed guide: `example/sample/server/README.md`
