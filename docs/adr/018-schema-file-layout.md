# ADR-018: JSON Schema File Layout

**Status:** accepted

**Date:** 2026-07-10

## Context

The `complypack.yaml` config file is validated at runtime against a JSON
Schema (draft 2020-12). The schema must be:

1. **Embeddable by Go** — compiled into the binary via `//go:embed`
2. **Discoverable by editors** — at a stable path for `yaml-language-server`
   and future SchemaStore registration

Go's `//go:embed` directive has two restrictions that conflict with a
single-file layout:

- Patterns cannot contain `..` (no embedding from a parent or sibling
  directory)
- Symlinks are rejected ("cannot embed irregular file")

The validation code lives in `schemas/jsonschema/` (a sub-package of
`schemas/`). It cannot live directly in `schemas/` because that would
create an import cycle: `schemas/index.go` imports `internal/config`,
and `internal/config/complypack.go` calls `ValidateConfig`.

## Decision

The real schema file lives at `schemas/jsonschema/complypack.schema.json`
where Go can embed it. The external-facing path
`schemas/json-schema/complypack.schema.json` is a symlink pointing to
`../jsonschema/complypack.schema.json`.

```
schemas/
  jsonschema/
    complypack.schema.json    ← real file, embedded by validate.go
    validate.go
    unknown.go
  json-schema/
    complypack.schema.json    ← symlink → ../jsonschema/complypack.schema.json
```

## Consequences

- One copy of the schema. No drift risk.
- Editors and CI tools referencing `schemas/json-schema/` continue to work
  through the symlink.
- The `$id` field (`https://complytime.github.io/complypack/schemas/complypack.schema.json`)
  is stable regardless of the local file layout.
- Windows users cloning the repo need `git config core.symlinks true`
  (Git for Windows defaults to false). This affects development only,
  not the compiled binary.
