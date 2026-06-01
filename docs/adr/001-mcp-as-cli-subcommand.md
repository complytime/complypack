# ADR 001: MCP Server as CLI Subcommand

**Date:** 2026-05-30  
**Status:** Accepted  
**Deciders:** Jennifer Power, Claude

## Context

We need to implement an MCP server for complypack to enable LLM-assisted policy generation. The question is whether to create a standalone binary (like `gemara-mcp`) or integrate it as a subcommand of the existing `complypack` CLI.

## Decision

Implement the MCP server as a subcommand of the `complypack` CLI:

```bash
complypack mcp serve
```

## Rationale

### Why a subcommand?

1. **Single binary simplicity:** Users install one tool, not two
2. **Code sharing:** MCP server reuses catalog pulling logic from the CLI
3. **Different from gemara-mcp:** gemara-mcp is ONLY an MCP server (no separate CLI), so a standalone binary makes sense there. ComplyPack already HAS a CLI.
4. **Reduced distribution complexity:** One binary to build, release, and maintain

### Considered Alternatives

**Standalone binary (`complypack-mcp`):**
- ❌ Two binaries to maintain
- ❌ Duplicate catalog pulling logic or create shared library
- ❌ Two separate release processes
- ✅ Clean separation of concerns
- ✅ Smaller binary for users who only want one tool

**Deferred MCP implementation:**
- ❌ Delays Issue #9 which specifically asks for MCP server
- ✅ Simpler initial implementation
- ✅ Could validate CLI design first

### Trade-offs Accepted

- MCP SDK becomes a dependency of the main CLI (adds ~500KB to binary)
- Users who don't use MCP still get it in their binary
- CLI code has additional complexity for MCP server mode

## Consequences

### Positive

- Single installation story for users
- Easier to share code between CLI commands and MCP resources
- Consistent versioning (CLI and MCP server always in sync)
- Simpler CI/CD pipeline

### Negative

- Slightly larger binary size
- MCP concerns mixed with CLI concerns in same codebase

## Implementation

- Add `cmd/complypack/cli/mcp.go` with `mcp serve` command
- Add `internal/mcp/` package for MCP server logic
- MCP server reuses `internal/registry` for catalog pulling
