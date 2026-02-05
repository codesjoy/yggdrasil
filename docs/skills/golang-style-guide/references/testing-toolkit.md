# Testing Toolkit (table tests + testify + gomock)

## Table-driven tests (default)
Use table tests for multiple cases:
```go
func TestParseEmail(t *testing.T) {
    tests := []struct {
        name    string
        in      string
        wantErr bool
    }{
        {"ok", "a@b.com", false},
        {"bad", "not-an-email", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := ParseEmail(tt.in)
            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

## `testify` conventions
- Prefer `require` for fail-fast assertions (most unit tests).
- Use `assert` only when you intentionally want multiple failures collected.

## `gomock` conventions
- Mock **ports** (interfaces) at the application layer, not concrete adapters.
- Avoid over-mocking: if a fake is simpler, use a hand-written fake.

### Placement
Pick one:
- `internal/<service>/service/mocks/` for service-layer ports
- `internal/<service>/test/mocks/` if you want a centralized test folder

### Setup helpers
Prefer a helper that returns both controller and mocks:
```go
func newMocks(t *testing.T) (*gomock.Controller, *MockOrderRepository) {
    t.Helper()
    ctrl := gomock.NewController(t)
    repo := NewMockOrderRepository(ctrl)
    return ctrl, repo
}
```

### Expectations style
- Keep expectations close to the act/assert sections.
- Use strict expectations only when behavior matters; do not lock down incidental calls.

## Integration tests
- Use build tags if they require external dependencies:
  - `//go:build integration`
- Keep integration tests separate from fast unit tests in CI pipelines if needed.
