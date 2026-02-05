# DDD with a Kratos-inspired (adapted) Clean Architecture

## Core idea
- Keep the **domain model** independent of frameworks and infrastructure.
- Make business behavior testable via **ports** (interfaces) and **use cases**.

## Layering (recommended mapping)
```
internal/
  core/          # domain core: entities, value objects, invariants + ports (interfaces)
  service/       # application + handler layer: orchestrate core, implement APIs
  data/          # infra adapters: DB/cache/external clients + port implementations
  conf/          # configuration structs (often generated)

cmd/<app>/        # composition root: create servers, register handlers, start app
```

Dependency direction:
- `core` <- `service` <- `cmd`
- `core` <- `data` <- `cmd`
- `conf` is typically imported by `cmd` and `data`/`service` as needed

## Responsibilities (what goes where)
- `internal/core`
  - Domain entities/value objects
  - Domain invariants (constructors/methods)
  - Port interfaces (e.g., repositories) that the domain/application depends on
- `internal/service`
  - Application orchestration (use cases)
  - API/DTO mapping and API implementation
  - Calls into `core` and ports; avoids infra details
- `internal/data`
  - Implements `core` ports (repositories)
  - Owns DB/cache/external client code
  - Owns persistence mapping (PO <-> domain objects)
- `cmd/<app>`
  - Dependency wiring and server creation
  - Register handlers/servers and start the process

## Typical port interfaces (in `internal/core`)

Repository port:
```go
type OrderRepository interface {
    Get(ctx context.Context, id string) (*core.Order, error)
    Save(ctx context.Context, o *core.Order) error
}
```

Clock port (avoid time coupling in domain tests):
```go
type Clock interface {
    Now() time.Time
}
```

Transaction boundary (optional):
```go
type TxManager interface {
    InTx(ctx context.Context, fn func(ctx context.Context) error) error
}
```

## Use case structure (in `internal/service`)
- Validate input at the boundary (DTO/request).
- Load domain entities via ports (interfaces defined in `core`).
- Apply domain behavior/invariants (methods in `core`).
- Persist via ports (implemented in `data`).
- Return a minimal response DTO.

## Domain rules
- `core` must not import DB/HTTP/logging packages.
- Domain invariants should be enforced in constructors/methods.
- Prefer value objects for constrained values (Email, Money, OrderID).

## Adapter rules
- `data` translates between external representations (DB rows, JSON) and domain types.
- Keep mapping code localized to the adapter layer.

## Testing implication
- Unit-test `core` and `service` without real DB/network.
- Integration-test `data` separately.

## Minimal skeleton (port -> impl -> service -> cmd wiring)

Port (core):
```go
// internal/core/order_repo.go
type OrderRepo interface {
    Get(ctx context.Context, id string) (*core.Order, error)
}
```

Implementation (data):
```go
// internal/data/order_repo.go
type OrderRepoImpl struct { /* db client */ }

func (r *OrderRepoImpl) Get(ctx context.Context, id string) (*core.Order, error) {
    // query + map PO -> core.Order
    return nil, nil
}
```

Use case / handler (service):
```go
// internal/service/order_service.go
type OrderService struct {
    repo core.OrderRepo
}
```

Composition root (cmd):
```go
// cmd/<app>/main.go
repo := &data.OrderRepoImpl{/* ... */}
svc := &service.OrderService{repo: repo}
// create servers + register svc
```
