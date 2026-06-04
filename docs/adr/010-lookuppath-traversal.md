# ADR 010: LookupPath-Based Schema Traversal with Fallback Chain

**Status:** Accepted

**Date:** 2026-06-03

**Context:**

The contract validator's `pathExistsInSchema` function uses `Fields(cue.All())` to iterate fields and match path segments by label. This approach has three blind spots:

1. Top type (`_`) returns zero fields from `Fields()`, so any sub-path fails
2. Pattern constraints (`[string]: T`) are not matched by label comparison
3. The deprecated `iter.Label()` API requires a `//nolint` suppression

These produce false positives for valid policies referencing standard fields like `input.on.push.branches` or `input.jobs.build`.

**Decision:**

Replace the field iteration loop with CUE's `LookupPath` API using a per-segment fallback chain:

1. Check `IncompleteKind() == cue.TopKind` — accept all remaining segments
2. Try `Str(part).Optional()` — matches named and optional fields
3. Try `AnyString` — matches `[string]: T` pattern constraints

This delegates CUE type system semantics to the CUE SDK rather than reimplementing them.

**Consequences:**

**Benefits:**

- Correct handling of top type, pattern constraints, and optional fields
- Eliminates deprecated API usage (`iter.Label()`)
- Simpler code (~20 lines vs ~35 lines)
- Future CUE type system features handled by SDK upgrades

**Drawbacks:**

- Behavior change: paths that previously failed now pass (this is the bug fix)
- `AnyString` fallback is permissive — a path like `input.jobs.anything` will pass if `jobs` has a pattern constraint, even if `anything` isn't a real job name (this is correct: the schema says any string key is valid)

**Related:**

- ADR 006: CUE Schema Contract Validation
- ADR 009: Definition Fragment Syntax
