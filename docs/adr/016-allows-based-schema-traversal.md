# ADR 016: Allows-Based Schema Traversal

**Status:** Accepted
**Supersedes:** ADR 010

**Date:** 2026-06-26

**Context:**

ADR 010 introduced a three-tier LookupPath fallback chain for `pathExistsInSchema`:

1. `IncompleteKind() == TopKind` — accept all remaining segments
2. `Str(part).Optional()` — match named/optional fields
3. `AnyString` — match pattern constraints

With the move to upstream CUE registry schemas (#30), the schemas are more complex than the hand-written embedded ones. The `TopKind` check only catches one CUE construct (`_`). It misses disjunctions (`{a: string} | {b: string}` where a field exists in one branch) and other structural allowances that CUE's type system understands.

Verified empirically: `LookupPath` with `Optional()` cannot find fields in disjunction branches. The `TopKind` check doesn't know about disjunctions at all.

**Decision:**

Replace the `TopKind` early-exit check with CUE's `Value.Allows(Selector)` method as the final fallback. The new chain:

1. `Str(part).Optional()` — match named/optional fields (gives a value to continue walking)
2. `AnyString` — match pattern constraints (gives a value to continue walking)
3. `Allows(Str(part))` — CUE-native structural check (accept remaining path, no value to walk)

`Allows` delegates to CUE's own type resolution. It returns true when the schema structurally permits a field — handling top type, disjunctions, and any future CUE constructs the SDK adds.

The `Allows` check is last because it's a terminal: when it fires, we have no `cue.Value` to continue walking, so we accept the entire remaining path. The first two tiers are preferred because they produce a value for deeper traversal.

Test schemas are wrapped in `#Root` definitions to match real-world behavior. CUE registry schemas are always definitions (closed structs), and `Allows` correctly returns false for undefined fields on closed structs.

A fuzz test (`FuzzPathExistsInSchema`) verifies the function never panics on arbitrary input.

**Consequences:**

**Benefits:**

- Correct handling of disjunctions — fields in any branch are accepted
- Future CUE type system features handled by SDK, not by us
- Fuzz test provides robustness guarantee
- Test schemas match real-world closedness (definitions, not open structs)

**Drawbacks:**

- `Allows` on open structs (compiled from bare CUE strings without `#Def`) returns true for any field. This is CUE-correct but means test schemas must use definitions to test rejection behavior.
- `Allows` on definitions with `...` (open definitions) returns false in CUE v0.16 — a known limitation. In practice this doesn't affect upstream schemas since resolved definitions list all fields explicitly.

**Related:**

- ADR 010: LookupPath-Based Schema Traversal (superseded)
- ADR 006: CUE Schema Contract Validation
