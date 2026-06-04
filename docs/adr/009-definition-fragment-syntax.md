# ADR 009: Definition Fragment Syntax for CUE Module Sources

**Status:** Accepted

**Date:** 2026-06-03

**Context:**

CUE registry modules (e.g., `cue.dev/x/githubactions@v0`) expose definitions (`#Workflow`, `#Job`) at the top level rather than regular fields. The contract validator receives a `cue.Value` with no traversable fields, causing all `input.*` paths to fail validation.

The validator needs to know which definition to use as the root for path traversal. Two sub-problems:

1. **How does the user specify the definition?** A hardcoded platform-to-definition mapping (e.g., `ci` → `#Workflow`) doesn't work because complypack can't know at build time which definitions third-party modules expose.
2. **What happens when no definition is specified?** Embedded schemas have regular fields at the top level and work without definition selection. Registry modules with only definitions need explicit selection.

**Decision:**

Extend the `source` URL syntax with a `#Definition` fragment:

```
cue://cue.dev/x/githubactions@v0#Workflow
```

The fragment is parsed from the source string and resolved via `cue.Def()` after module loading. The `#` character does not appear in CUE module paths or semver strings, so parsing is unambiguous.

When no fragment is provided:
- If the loaded value has regular (non-definition) fields: use as-is (backward compatible with embedded schemas)
- If the loaded value has only definitions: return an error listing available definitions

**Consequences:**

**Benefits:**

- Explicit: user declares exactly which definition to validate against
- No build-time coupling between platforms and module internals
- Backward compatible: embedded schemas and file-based sources work unchanged
- Natural syntax: `#` mirrors CUE's own definition notation

**Drawbacks:**

- Users must know the definition name for registry modules (mitigated by the error message listing available definitions)
- Adds parsing complexity to the schema source URL

**Related:**

- ADR 006: CUE Schema Contract Validation
- ADR 008: Pre-Pack Validation Gates
- ADR 010: LookupPath-Based Schema Traversal
