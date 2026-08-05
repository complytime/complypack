# Agent Instructions

## Architecture

### Thin transport layers

MCP handlers (`internal/mcp/`) and CLI commands (`cmd/complypack/cli/`) are thin wiring: parse input, call a domain function, serialize output. No business logic in these layers.

Business logic belongs in domain packages:
- `internal/requirement/` — policy resolution, triage, delta analysis
- `internal/prepack/` — contract validation
- `internal/evaluator/` — policy evaluation
- `internal/schema/` — schema loading and registry
- `internal/testresult/` — shared per-test outcome types consumed by tester, evaluator, and coverage
- `internal/coverage/` — coverage analysis, gap reporting, and test-to-requirement attribution

When adding a new MCP tool or CLI command, write the logic as an exported function in the appropriate domain package first, then wire it from the transport layer.

### CLI and MCP parity

New capabilities SHOULD be exposed through both the CLI and the MCP server where it makes sense. CLI commands provide deterministic, scriptable access for automation and CI/CD. MCP tools enable conversational, LLM-assisted workflows. Both layers are thin transport wiring over the same domain functions.

### Testing follows the same split

Domain package tests cover logic and edge cases. Transport layer tests only verify wiring: correct input parsing, delegation to the domain function, and response serialization.
