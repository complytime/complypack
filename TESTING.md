# Testing Guidelines

## Test Strategy

ComplyPack uses a layered testing approach:

### Library Tests (pkg/complypack/)

**Framework:** `testify/assert` and `testify/require`

**Style:** Table-driven tests with testify assertions

**Example:**
```go
func TestPackErrors(t *testing.T) {
    tests := []struct {
        name    string
        cfg     Config
        content io.Reader
        wantErr error
    }{
        {
            name: "empty content",
            cfg:  validConfig(),
            content: bytes.NewReader([]byte{}),
            wantErr: ErrEmptyContent,
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := Pack(ctx, store, tt.cfg, tt.content)
            assert.ErrorIs(t, err, tt.wantErr)
        })
    }
}
```

**Key patterns:**
- Use `require` for setup/preconditions (failures stop the test)
- Use `assert` for actual test assertions (failures continue, report all issues)
- Prefer `assert.Equal(t, expected, actual)` over manual comparisons
- Use `assert.ErrorIs(t, err, sentinel)` for error checking
- Use `require.NoError(t, err)` when error would make rest of test meaningless

### CLI Tests (cmd/, internal/)

**Framework:** TBD - Consider Ginkgo/Gomega for BDD-style behavioral specs

**Rationale for BDD consideration:**
- CLIs are inherently behavior-driven (user actions → outcomes)
- Complex integration scenarios (registry auth, network errors, etc.)
- Aligns with cloud-native tooling patterns (Kubernetes, ORAS)

**Current:** Using testify for CLI unit tests

**Future:** Evaluate Ginkgo/Gomega when adding integration tests

### Integration Tests

**Framework:** TBD - Likely Ginkgo + testcontainers

**Scope:**
- Full command execution
- Mock/test OCI registries
- E2E workflows

## Migration Status

**Completed:**
- ✅ `pkg/complypack/pack_test.go` - Refactored to testify
- ✅ `pkg/complypack/config_test.go` - Refactored to testify

**Pending:**
- `pkg/complypack/errors_test.go`
- `pkg/complypack/mediatype_test.go`
- `pkg/complypack/options_test.go`
- `pkg/complypack/integration_test.go`
- `pkg/complypack/unpack_test.go`
- `pkg/complypack/sign_test.go`
- `pkg/complypack/verify_test.go`

**Pattern:** Refactor incrementally as files are touched. No need for mass refactoring.

## Running Tests

```bash
# All tests
go test -race ./...

# With coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Specific package
go test -race github.com/complytime/complypack/pkg/complypack

# Specific test
go test -run TestPackErrors github.com/complytime/complypack/pkg/complypack
```

## Test Organization

```
pkg/complypack/
  ├── pack.go
  ├── pack_test.go          # Unit tests for Pack function
  ├── integration_test.go   # Integration tests (round-trips, stores)
  └── ...

cmd/complypack/cli/
  ├── catalog_pull.go
  ├── catalog_pull_test.go  # Unit tests for command structure
  └── ...

internal/registry/
  ├── client.go
  ├── client_test.go        # Unit tests for registry helpers
  └── ...
```

## Future Considerations

### Ginkgo/Gomega Evaluation Criteria

**Adopt if:**
- Adding complex integration test suites
- Need better scenario organization
- Team prefers BDD-style specs

**Skip if:**
- Tests remain simple unit tests
- Stdlib + testify sufficient
- Want to minimize dependencies

**Decision point:** When implementing `complypack pack/validate/test` commands (issue #1)
