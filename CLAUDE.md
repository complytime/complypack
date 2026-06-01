# Claude Code Instructions

## Git Commit Protocol

**CRITICAL: Always run gitleaks before committing:**

```bash
gitleaks detect --config ~/.gitleaks.toml --source . -v
```

If gitleaks finds leaks, **STOP** and fix them before proceeding.

### Commit Steps

1. **Pre-commit check**: Run gitleaks (above)
2. **Stage changes**: `git add <files>`
3. **Commit**: `git commit -S -s -m "message"`
4. **Push**: Only after user approval

### Commit Message Format

```
<type>: <subject>

<body>

Assisted-by: Claude (Anthropic, Claude 3.5 Sonnet 4.5)
```

**Types**: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `build`, `ci`

### Never Commit

- Secrets, API keys, tokens
- ESS references (blocked by gitleaks)
- Temporary files, build artifacts
- Large binaries without approval

## Testing

- Run tests before committing: `go test -race ./...`
- Run acceptance tests: `ginkgo -v acceptance/`
- Verify builds: `go build ./...`
