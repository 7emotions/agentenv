# AgentEnv — AI Agent Instructions

## Identity
- **Module**: `github.com/7emotions/agentenv`
- **Go**: 1.26 (CI runs 1.23.x — do not go below `go 1.23`)
- **Binary**: `agentenv` — single CLI, entrypoint at `cmd/agentenv/main.go`
- **License**: MIT. No telemetry, no phone-home, no Windows support.

## Quick Commands

```bash
make build          # compile → dist/agentenv
make test           # go test ./... -v
make lint           # golangci-lint run
go vet ./...        # CI runs this; Makefile does NOT include vet
go test -race ./... # CI runs with race detector
go mod tidy         # keep go.sum clean; goreleaser runs this before build
```

**GOTCHA**: `make test` runs `go test ./... -v`. CI runs `go test -race ./...`. The difference (`-race`) matters.

Run lints before tests: `make lint && make test`. CI order is `lint → build → vet → test`.

## Package Map

```
cmd/agentenv/        main — CLI entrypoint, 14 cobra commands, 6 test files
pkg/adapter/         4 framework adapters (Claude Code, OpenCode, Cursor, Codex)
                     Each implements 16-method AgentAdapter interface
pkg/types/           Core types: SourceURL, Package, Environment, Lockfile
pkg/parser/          YAML/JSON parsing (agent.yaml, lockfiles)
pkg/envfile/         agent.yaml read/write/validate/merge
pkg/source/          Source fetchers for 6 schemes (github, npm, local, git, file, url)
pkg/resolver/        PubGrub CDCL dependency solver (most complex package)
pkg/store/           Content-addressed store (symlink dedup, GC, cache)
pkg/lockfile/        Lockfile generation and reading
pkg/errors/          Structured errors: UserError / SystemError / NetworkError
internal/shell/      Shell init scripts
```

**No generated code. No proto files. No Docker. No Windows.**

## Code Conventions

### CLI Commands (cobra)
- Each command lives in its own `.go` file in `cmd/agentenv/` (package `main`)
- Pattern: `var <name>Cmd = &cobra.Command{Use: "...", RunE: func(...) error { ... }}`
- Flags are package-level `var` declared at file top
- `init()` registers flags and calls `rootCmd.AddCommand(...)`
- JSON output via `writeJSON(cmd, data)`; errors via `jsonOutput` checks in `main()`

### Error Handling
Always use structured errors from `pkg/errors`:
```go
agentenvError.UserError("what happened", "how to fix it")
agentenvError.SystemError("what happened", "how to fix it").WithCause(err)
agentenvError.NetworkError("what happened", "how to fix it")
```
- Prefer `WithCause(err)` for underlying IO/network errors — preserves the `AgentError` type
- `Wrap(err, msg)` preserves error type if already an `*AgentError`

### Tests
- Test files are `*_test.go` in the same package (package `main` for CLI tests)
- Use `t.TempDir()` and `t.Setenv("HOME", tmp)` for filesystem isolation
- Use table-driven tests (pattern: `tests := []struct{name, wantErr bool}{...}`)
- No mocking frameworks — direct use of interfaces/tempdirs
- 28 test files total. CLI tests call `cmd.RunE(cmd, args)` directly

### JSONC (OpenCode config)
- Comments in `opencode.jsonc` MUST be preserved, NEVER stripped
- Use `github.com/tailscale/hujson` for JSONC parsing (not `encoding/json`)
- Functions in `pkg/adapter/jsonc.go` for comment-preserving operations

### Naming
- `validEnvName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)` — kebab-case, max 64 chars
- Package sources use scheme prefixes: `github:`, `npm:`, `local:`, `git:`, `file:`, `url:`

## Architecture Notes

### Adapter Registry (`pkg/adapter/registry.go`)
- Thread-safe via `sync.RWMutex`
- All adapters registered at compile time via `init()` — no plugin system
- Each adapter knows its framework's config path and format

### PubGrub Resolver (`pkg/resolver/`)
- Most complex package. CDCL algorithm (conflict-driven clause learning)
- `pubgrub_resolver.go` implements the `DependencyResolver` interface
- `pubgrub_adapter.go` bridges `SourceHandler` → pubgrub `Source`
- `namespace.go` handles type-safe name encoding (package names are `type:name`)

### Content-Addressed Store (`pkg/store/`)
- Symlink deduplication with cross-filesystem copy fallback
- Garbage collection for orphaned packages

### Transactional Activation
- Order: backup → validate → apply → rollback on failure
- Never leaves partial state. Backups stored in config directory

## Release
- Triggered by `v*` tags → GoReleaser builds darwin+linux, amd64+arm64 binaries
- Published to GitHub Releases + Homebrew tap (`7emotions/homebrew-agentenv`)
- `install.sh` is the recommended install path: `curl .../install.sh | sh`
- LDFLAGS inject `version`, `commit`, `date` — read from git tags / env
