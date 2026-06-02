# ADR 007: OPA SDK In-Process vs Subprocess Execution

**Status:** Accepted

**Date:** 2026-06-01

**Context:**

ComplyPack needs to validate, test, and evaluate OPA Rego policies. There are two architectural approaches:

1. **In-process via OPA Go SDK** (`github.com/open-policy-agent/opa`)
2. **Subprocess via `opa` CLI** (shell out to `opa test`, `opa eval`, etc.)

**Subprocess Approach:**

```go
func Validate(filename, src string) []error {
    cmd := exec.Command("opa", "check", filename)
    output, err := cmd.CombinedOutput()
    // Parse stdout/stderr for errors
}
```

**In-Process Approach:**

```go
func Validate(filename, src string) []error {
    mod, err := ast.ParseModuleWithOpts(filename, src, ...)
    compiler := ast.NewCompiler()
    compiler.Compile(map[string]*ast.Module{filename: mod})
    return compiler.Errors
}
```

**Decision:**

We use the **OPA Go SDK in-process** for all policy operations:

- **Validation**: `ast.ParseModuleWithOpts` + `ast.NewCompiler`
- **Testing**: `tester.NewRunner().RunTests()`
- **Evaluation**: `rego.New().PrepareForEval()`
- **AST inspection**: `ast.WalkRefs` for contract validation

**Only exception**: Regal linting (optional, best-effort subprocess call).

**Rationale:**

| Criterion | In-Process SDK | Subprocess CLI |
|-----------|----------------|----------------|
| **Performance** | ✅ No process spawn overhead | ❌ ~50-100ms per invocation |
| **Error handling** | ✅ Typed errors, structured data | ❌ Parse stdout/stderr strings |
| **Determinism** | ✅ Same Go version = same behavior | ❌ Depends on `opa` in `$PATH` |
| **Testability** | ✅ Unit tests, no mocks needed | ❌ Requires `opa` binary in CI |
| **AST access** | ✅ Direct AST for contract validation | ❌ Would need `opa parse --format json` |
| **Cancellation** | ✅ `context.Context` propagation | ❌ Manual `cmd.Cancel()` handling |
| **Memory control** | ✅ Share memory pool | ❌ Each process allocates separately |
| **Versioning** | ✅ `go.mod` locks OPA version | ❌ Runtime dependency on system `opa` |

**Key Benefits:**

1. **Fast feedback loops**: Validation in <10ms vs 50-100ms subprocess overhead
2. **Structured errors**: `compiler.Errors` is `[]ast.Error` with location/code, not string parsing
3. **AST introspection**: Contract validation needs `ast.WalkRefs` to find `input.*` references (not exposed via CLI)
4. **Single binary**: No external dependencies - `complypack` binary is self-contained
5. **Reproducibility**: `go.mod` locks `github.com/open-policy-agent/opa v1.16.2` - same behavior across environments

**Implementation:**

- **`internal/validator/rego.go`**: Uses `ast.ParseModuleWithOpts`, `ast.NewCompiler`
- **`internal/validator/contract.go`**: Uses `ast.WalkRefs` to walk policy AST
- **`internal/tester/runner.go`**: Uses `tester.NewRunner().RunTests()`
- **`internal/tester/fixture.go`**: Uses `rego.New().PrepareForEval()` for policy evaluation
- **`internal/evaluator/opa.go`**: Wires everything together via Evaluator interface

**Exception: Regal Linting**

Regal (<https://github.com/StyraInc/regal>) is OPA's official linter, but:

- No stable Go API (CLI-only)
- Frequent releases (pinning version is hard)
- Optional quality tool (not blocking)

We shell out to `regal` with **graceful degradation**:

```go
func Lint(filename, src string) ([]LintWarning, error) {
    if _, err := exec.LookPath("regal"); err != nil {
        return nil, nil  // Not installed, silently skip
    }
    // Run regal lint --format json
}
```

This allows users with `regal` installed to get style warnings without forcing it as a dependency.

**Consequences:**

**Benefits:**

- **Speed**: 5-10x faster validation (no subprocess overhead)
- **Reliability**: No dependency on external binaries
- **Developer experience**: Simpler debugging (in-process stack traces)
- **CI/CD simplicity**: Just `go test`, no `opa` installation step

**Drawbacks:**

- **OPA version coupling**: If OPA SDK introduces breaking changes, we must update code (mitigated by `go.mod` version pinning)
- **Binary size**: Includes OPA SDK (~10-15MB) in `complypack` binary (acceptable for a compliance tool)
- **No Regal in-process**: Must shell out for linting (acceptable for optional tooling)

**Future Considerations:**

- **WebAssembly policies**: If we add Wasm evaluation, OPA SDK already supports it (`opa.WithWasmRuntime`)
- **Remote bundles**: OPA SDK supports bundle downloading/caching natively
- **Custom built-ins**: Could register Go functions as Rego built-ins via SDK

**Alternatives Considered:**

1. **Hybrid approach** (SDK for validation, subprocess for testing):
   - **Rejected**: Adds complexity, forces CLI dependency anyway

2. **Always subprocess** (like `conftest` wrapper):
   - **Rejected**: Contract validation needs AST access (CLI doesn't expose it)

3. **Embed `opa` binary** (package CLI in our binary):
   - **Rejected**: Worse than SDK (still subprocess overhead + binary bloat)

**Related:**

- ADR 005: Evaluator Interface Pattern
- ADR 006: CUE Schema Contract Validation
- `go.mod`: OPA SDK version pinned at `v1.16.2`
