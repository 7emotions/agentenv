# agentenv MVP v0.1.0 — Implementation Plan

## TL;DR

> **Quick Summary**: Build the "conda for agent development" CLI tool in Go — isolated environments that manage skills, MCP servers, and agent configs with declarative `agent.yaml` definitions and lockfiles.
>
> **Deliverables**:
> - Go CLI binary (`agentenv`) distributed via brew + shell script
> - 11 subcommands: create, activate, deactivate, add, remove, list, list-packages, info, delete, lock, install
> - 2 package types: skill + mcp
> - 1 agent framework adapter: Claude Code
> - agent.yaml and agent.lock formats
> - Shell integration for zsh + bash
>
> **Estimated Effort**: Large
> **Parallel Execution**: YES — 4 waves
> **Critical Path**: Task 1 → Task 5 → Task 8 → Task 12 → Task 15 → Task 18 → Final

---

## Context

### Original Request
Build agentenv — a `conda`-style environment manager for AI agent development. Users need isolated environments where they can install skills, MCP servers, agents, tools, hooks, and prompts without polluting their global agent configuration.

### Interview Summary
**Key Discussions**:
- Confirmed market gap: no existing tool combines environment isolation + declarative definitions + dependency management + lockfiles
- MVP targets Claude Code only; Go 1.22+ with cobra/yaml.v3/go-git/goreleaser
- Dependency resolver uses topological sort + backtrack (not SAT solver) — interface designed for future swap
- Skill installation delegates to `npx skills` for registry access; agentenv manages file placement
- Shell integration via `eval "$(agentenv init zsh)"` pattern — shell functions call the binary

### Metis Review
**Critical Findings** (incorporated into plan):
1. Shell Integration MUST be designed first — activates/deactivates are shell functions, not direct binary calls
2. Ownership model for `~/.claude/` MUST use manifest tracking (not snapshot/restore) to coexist with npx skills
3. Activation MUST be transactional: backup → validate → apply → rollback on failure
4. DependencyResolver interface MUST support algorithm swap (topological → PubGrub/SAT later)
5. Explicit limits: max dependency depth 3, max packages 50 per env, no SAT solver, no Windows
6. Missing acceptance criteria added: concurrent safety, lock determinism, offline grace, store GC

---

## Work Objectives

### Core Objective
Deliver a working agentenv v0.1.0 that lets developers create, activate, and share isolated agent development environments on macOS and Linux, with Claude Code as the first supported framework.

### Concrete Deliverables
- `agentenv` Go binary: single-file distribution via brew + `install.sh`
- `~/.agentenv/` directory tree: envs/, store/, cache/, bin/
- agent.yaml v1 format (schema + parser + writer)
- agent.lock v1 format (schema + generator + reader)
- Claude Code adapter (skills + MCP + agent install/remove)
- Shell init scripts for zsh and bash
- 11 CLI subcommands with `--json` output support
- Built-in help, man page, and completions

### Definition of Done
- [ ] `brew install agentenv` successfully installs on macOS
- [ ] `agentenv create test && agentenv activate test && agentenv deactivate` end-to-end works
- [ ] `agentenv add skill code-review` installs via npx skills and registers in agent.lock
- [ ] `agentenv lock` produces deterministic, byte-identical output on repeat runs
- [ ] Activating env B while env A is active: env A is cleanly deactivated, env B activated
- [ ] All commands produce valid `--json` output
- [ ] All tests pass: `go test ./...` on macOS and Linux

### Must Have
- Transactional activation: no partial state on failure
- Manifest-based cleanup (not snapshot/restore)
- Shell eval integration: `eval "$(agentenv init zsh)"`
- Deterministic lockfile generation
- Clear, actionable error messages

### Must NOT Have (Guardrails)
- No SAT solver or PubGrub (v0.1.0 = topological + backtrack only)
- No telemetry, no phone-home, no analytics
- No Windows support
- No auto-activation on `cd`
- No Cursor/OpenCode/Codex adapters
- No partial MCP config merging (full overwrite with backup)
- No git submodule support
- No env var auto-injection without explicit declaration
- Max dependency depth: 3 (hard error beyond)
- Max packages per environment: 50 (hard error beyond)

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: NO — this is greenfield
- **Automated tests**: YES (tests-after-implementation, not TDD due to interface volatility)
- **Framework**: Go standard `testing` + `testify`
- **CI**: GitHub Actions, macOS + Linux matrix

### QA Policy
Every task MUST include agent-executed QA scenarios using the specified tool types. Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation + types + shell):
├── Task 1: Go project scaffolding + module init + Makefile [quick]
├── Task 2: Core types: Package, Environment, Lockfile, MCPConfig [quick]
├── Task 3: agent.yaml + agent.lock schema + parser [unspecified-high]
├── Task 4: Shell init generator (zsh + bash) [quick]
└── Task 5: Store abstraction (download, cache, dedup) [unspecified-high]

Wave 2 (After Wave 1 — resolvers + package ops):
├── Task 6: Source handlers (github, npm, local) [unspecified-high]
├── Task 7: Version constraint parser + semver matching [unspecified-high]
├── Task 8: Dependency resolver (topological + backtrack + conflict detection) [unspecified-high]
├── Task 9: Lockfile generator + reader [quick]
└── Task 10: agent.yaml reader + validator [unspecified-high]

Wave 3 (After Wave 2 — framework adapter + activation):
├── Task 11: Claude Code adapter (skills install/remove) [quick]
├── Task 12: Claude Code adapter (MCP config write/read/backup) [unspecified-high]
├── Task 13: Claude Code adapter (agent subagent support) [quick]
├── Task 14: Environment create command [quick]
└── Task 15: Environment activate/deactivate flow (transactional) [deep]

Wave 4 (After Wave 3 — all remaining CLI + polish):
├── Task 16: Package add/remove commands [quick]
├── Task 17: List/info/delete commands [quick]
├── Task 18: Lock + install commands (full loop) [unspecified-high]
├── Task 19: Error handling + actionable messages [quick]
├── Task 20: --json output + completions + help text [quick]
└── Task 21: Release engineering (goreleaser, brew formula, install.sh) [quick]

Wave FINAL (After ALL — parallel review + QA):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
```

**Critical Path**: T1 → T5 → T8 → T12 → T15 → T18 → F1-F4

### Agent Dispatch Summary
- **Wave 1**: 5 tasks — T1→quick, T2→quick, T3→unspecified-high, T4→quick, T5→unspecified-high
- **Wave 2**: 5 tasks — T6→unspecified-high, T7→unspecified-high, T8→unspecified-high, T9→quick, T10→unspecified-high
- **Wave 3**: 5 tasks — T11→quick, T12→unspecified-high, T13→quick, T14→quick, T15→deep
- **Wave 4**: 6 tasks — T16→quick, T17→quick, T18→unspecified-high, T19→quick, T20→quick, T21→quick
- **Wave FINAL**: 4 tasks — F1→oracle, F2→unspecified-high, F3→unspecified-high, F4→deep

---

## TODOs

### Wave 1: Foundation + Types + Shell

- [x] 1. Go project scaffolding + module init + Makefile

  **What to do**:
  - Initialize Go module: `go mod init github.com/agentenv/agentenv`
  - Create directory structure: `cmd/agentenv/`, `pkg/`, `internal/`
  - Write `Makefile` with targets: `build`, `test`, `lint`, `clean`, `install`, `release-dry-run`
  - Add `.gitignore` (ignore `/dist/`, `.agentenv-local-test/`)
  - Add `.goreleaser.yaml` skeleton
  - Add `.golangci.yml` with linters: errcheck, gosimple, govet, ineffassign, staticcheck, unused, misspell
  - Write `cmd/agentenv/main.go` with cobra root command (placeholder subcommands)
  - Set up GitHub Actions `.github/workflows/ci.yml`: test on ubuntu-latest + macos-latest, Go 1.22

  **Must NOT do**:
  - Do NOT add telemetry or analytics setup
  - Do NOT create Windows CI runners
  - Do NOT add docker-related build targets

  **Recommended Agent Profile**:
  - **Category**: `quick` — standard project scaffolding, toolchain setup
  - **Skills**: [`commit-helper`]
    - `commit-helper`: For conventional commit message formatting

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3, 4)
  - **Blocks**: All tasks (project root)
  - **Blocked By**: None

  **References**:
  - `cobra` docs: https://github.com/spf13/cobra — CLI framework patterns
  - `goreleaser` docs: https://goreleaser.com/quick-start/ — Release config
  - Study: `cli/cli` (GitHub CLI) for Go project layout conventions

  **Acceptance Criteria**:
  - [ ] `go build ./cmd/agentenv` produces binary
  - [ ] `go test ./...` passes (0 tests initially is OK)
  - [ ] `go vet ./...` passes with no warnings
  - [ ] `make build` works
  - [ ] GitHub Actions CI passes on push

  **QA Scenarios**:

  ```
  Scenario: Happy path — build from source
    Tool: Bash
    Preconditions: Go 1.22+ installed, GOPATH/bin in PATH
    Steps:
      1. git clone <repo> && cd agentenv
      2. make build
      3. ./dist/agentenv --version
    Expected Result: Prints "agentenv version 0.1.0" (or dev)
    Failure Indicators: Build error, missing binary, wrong version string
    Evidence: .sisyphus/evidence/task-1-build.txt

  Scenario: CI passes
    Tool: Bash (simulating CI)
    Steps:
      1. go vet ./...
      2. go test ./...
    Expected Result: Both exit 0, no warnings
    Failure Indicators: vet warnings, test failures
    Evidence: .sisyphus/evidence/task-1-ci.txt
  ```

  **Commit**: YES (first commit)
  - Message: `feat: initialize agentenv project scaffolding`
  - Files: go.mod, go.sum, cmd/agentenv/main.go, Makefile, .gitignore, .goreleaser.yaml, .golangci.yml, .github/workflows/ci.yml

---

- [x] 2. Core types: Package, Environment, Lockfile, MCPConfig

  **What to do**:
  - Define `pkg/types/package.go`: `PackageType` enum (Skill, MCP, Agent, Tool, Hook, Prompt), `Package` struct (Name, Version, Source, SHA256, Dependencies, RuntimeReqs)
  - Define `pkg/types/environment.go`: `Environment` struct (Name, Path, Active, Agent, Packages)
  - Define `pkg/types/lockfile.go`: `Lockfile` struct (Version, Generated, Packages array, EnvironmentSnapshot)
  - Define `pkg/types/mcpconfig.go`: `UniversalMCPConfig` struct (Servers map), `UniversalMCPServer` (Name, Command, Args, Env, URL, Transport, Headers, Description)
  - Define `pkg/types/agent.go`: `AgentSpec` struct (Name, Version, Description, Dependencies, Conflicts, Provides, Runtime)
  - Define `pkg/types/source.go`: `SourceURL` struct with scheme parsing (github, npm, local, git, file, url)
  - All types MUST have JSON and YAML struct tags
  - Add Go doc comments for every exported type

  **Must NOT do**:
  - Do NOT add database/sql struct tags
  - Do NOT add validation logic here (that's in the parser tasks)
  - Do NOT add Windows path handling

  **Recommended Agent Profile**:
  - **Category**: `quick` — pure type definitions, no logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3, 4)
  - **Blocks**: Tasks 3, 5, 6, 8, 9 (all depend on types)
  - **Blocked By**: Task 1 (needs module structure)

  **References**:
  - Existing spec: `.sisyphus/drafts/agentenv-spec.md` §3 (Package types), §4 (agent.yaml), §5 (agent.lock), Appendix D (Universal MCP Config)
  - `semver` package: https://github.com/Masterminds/semver — Version representation in Go

  **Acceptance Criteria**:
  - [ ] All types are in `pkg/types/`
  - [ ] Each file has package doc comment
  - [ ] Each exported type has a Go doc comment
  - [ ] Types round-trip through JSON marshal/unmarshal
  - [ ] Types round-trip through YAML marshal/unmarshal
  - [ ] `go vet ./pkg/types` passes

  **QA Scenarios**:

  ```
  Scenario: Happy path — JSON round-trip for all types
    Tool: Bash (go test)
    Steps:
      1. Create a Package with all fields populated
      2. json.Marshal → bytes
      3. json.Unmarshal → Package
      4. Assert all fields equal original
    Expected Result: Deep equal, no field loss
    Evidence: .sisyphus/evidence/task-2-types-json.txt

  Scenario: Source URL parsing
    Tool: Bash (go test)
    Steps:
      1. Parse "github:owner/repo/path/to/skill"
      2. Assert Scheme=="github", Owner=="owner", Repo=="repo", SubPath=="path/to/skill"
      3. Parse "npm:@scope/package"
      4. Assert Scheme=="npm", Scope=="scope", Package=="package"
      5. Parse "local:./my-skill"
      6. Assert Scheme=="local", Path=="./my-skill"
    Expected Result: All parse correctly, invalid URLs error
    Evidence: .sisyphus/evidence/task-2-source-url.txt
  ```

  **Commit**: YES (grouped with T1)
  - Message: `feat: add core type definitions`
  - Files: pkg/types/package.go, environment.go, lockfile.go, mcpconfig.go, agent.go, source.go

---

- [x] 3. agent.yaml + agent.lock schema + parser

  **What to do**:
  - Implement `pkg/parser/agent_yaml.go`: Parse agent.yaml into `types.Environment` struct
  - Implement `pkg/parser/agent_yaml.go`: Validate required fields (name, description), validate version constraint syntax
  - Implement `pkg/parser/agent_yaml.go`: Support all source schemes (github, npm, local, git, file, url)
  - Implement `pkg/parser/lockfile.go`: Parse agent.lock into `types.Lockfile` struct
  - Implement `pkg/parser/lockfile.go`: Write Lockfile struct to YAML
  - Implement `pkg/parser/agentpkg.go`: Parse agentpkg.yaml → `types.AgentSpec`
  - Handle YAML parsing errors with line numbers
  - Handle missing optional fields gracefully

  **Must NOT do**:
  - Do NOT add schema validation beyond required field checks (JSON Schema validation is v0.3.0)
  - Do NOT support environment variable interpolation in parser (that's runtime, Task 15)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high` — moderate complexity, YAML parsing with custom types
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4)
  - **Blocks**: Tasks 8, 9, 10, 14, 18
  - **Blocked By**: Task 2 (needs types)

  **References**:
  - Existing spec: `.sisyphus/drafts/agentenv-spec.md` §4 (agent.yaml schema), §5 (agent.lock schema)
  - `yaml.v3`: https://github.com/go-yaml/yaml — Go YAML parser
  - `Masterminds/semver`: https://github.com/Masterminds/semver — Version constraint parsing

  **Acceptance Criteria**:
  - [ ] Parse a minimal agent.yaml (name + description only) → valid Environment
  - [ ] Parse full agent.yaml with skills, mcps, agents, tools, hooks, prompts → valid Environment
  - [ ] Parse agent.lock → valid Lockfile
  - [ ] Write Lockfile → valid YAML, byte-compatible with spec
  - [ ] Parse agentpkg.yaml with dependencies section → valid AgentSpec
  - [ ] Invalid YAML → error with line number
  - [ ] Missing required field → error with field name

  **QA Scenarios**:

  ```
  Scenario: Happy path — parse complete agent.yaml
    Tool: Bash (go test)
    Steps:
      1. Create agent.yaml with 2 skills, 2 mcps, 1 agent, nested configs
      2. Parse → Environment struct
      3. Assert len(Environment.Skills) == 2, len(Environment.MCPs) == 2
      4. Assert skill version constraints are parsed
    Expected Result: All fields correctly populated
    Evidence: .sisyphus/evidence/task-3-parse.yaml

  Scenario: Error — missing required name field
    Tool: Bash (go test)
    Steps:
      1. Create agent.yaml with description only, no name
      2. Parse
    Expected Result: Error containing "name" and line number
    Evidence: .sisyphus/evidence/task-3-error.txt

  Scenario: Version constraint parsing edge cases
    Tool: Bash (go test)
    Steps:
      1. Parse version constraints: "^1.2.3", "~1.2.3", ">=1.0 <2.0", "1.2.3", "latest", "*"
      2. Assert all parse without error
    Expected Result: All constraint types parse
    Evidence: .sisyphus/evidence/task-3-version.txt
  ```

  **Commit**: YES (grouped with T2)
  - Message: `feat: add YAML parser for agent.yaml, agent.lock, and agentpkg.yaml`
  - Files: pkg/parser/agent_yaml.go, lockfile.go, agentpkg.go, and tests

---

- [x] 4. Shell init generator (zsh + bash)

  **What to do**:
  - Implement `internal/shell/init.go`: `GenerateInitScript(shell string) string`
  - Generate `agentenv()` shell function that wraps the binary
  - Implement `agentenv activate` as a shell function (NOT a cobra command) that:
    1. Calls `agentenv _internal_activate <name> --json` to get env changes as JSON
    2. Parses JSON with `jq` or built-in (provide fallback)
    3. Sets AGENTENV_ACTIVE, modifies PATH, updates PS1
  - Implement `agentenv deactivate` as a shell function that restores previous state
  - Store pre-activation state (PS1, AGENTENV_ACTIVE) so deactivation restores it
  - Support zsh and bash with correct syntax differences
  - `agentenv init zsh --json` outputs JSON (for shell-agnostic consumption)
  - `agentenv init bash --json` same
  - Handle `set -euo pipefail` compatibility

  **Must NOT do**:
  - Do NOT attempt to set env vars from Go binary directly (impossible)
  - Do NOT generate fish or powershell init scripts
  - Do NOT add auto-activation on cd

  **Recommended Agent Profile**:
  - **Category**: `quick` — template generation with careful escaping
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3)
  - **Blocks**: Task 15 (activation flow depends on shell init)
  - **Blocked By**: Task 1 (needs cobra root command)

  **References**:
  - `pyenv` init script for reference: https://github.com/pyenv/pyenv/blob/master/libexec/pyenv-init
  - `conda` shell init for reference
  - Go `text/template`: standard library — for generating shell scripts safely
  - Pro-tip: test with `zsh -n` (syntax check) and `bash -n` before claiming it works

  **Acceptance Criteria**:
  - [ ] `eval "$(./dist/agentenv init zsh)"` works in zsh
  - [ ] `eval "$(./dist/agentenv init bash)"` works in bash
  - [ ] After init, `agentenv activate <name>` is available as shell function
  - [ ] Shell function calls the binary, parses JSON, sets env vars
  - [ ] `agentenv deactivate` restores previous PS1 and unsets AGENTENV_ACTIVE
  - [ ] `agentenv init zsh` output passes `zsh -n`
  - [ ] `agentenv init bash` output passes `bash -n`

  **QA Scenarios**:

  ```
  Scenario: Happy path — eval init then activate in zsh
    Tool: interactive_bash (tmux with zsh)
    Preconditions: Binary built at ./dist/agentenv, test env created
    Steps:
      1. zsh
      2. eval "$(./dist/agentenv init zsh)"
      3. echo $AGENTENV_ACTIVE → should be empty
      4. agentenv activate test-env
      5. echo $AGENTENV_ACTIVE → should be "test-env"
      6. echo $PS1 → should contain "(agentenv:test-env)"
      7. agentenv deactivate
      8. echo $AGENTENV_ACTIVE → should be empty
    Expected Result: Activate sets vars, deactivate restores, prompt changes
    Failure Indicators: AGENTENV_ACTIVE not set, PS1 corrupted, deactivate doesn't restore
    Evidence: .sisyphus/evidence/task-4-shell-activate.txt

  Scenario: set -euo pipefail compatibility
    Tool: interactive_bash (tmux)
    Steps:
      1. bash
      2. set -euo pipefail
      3. eval "$(./dist/agentenv init bash)"
      4. agentenv activate test-env
    Expected Result: No "unbound variable" or "pipefail" errors
    Evidence: .sisyphus/evidence/task-4-set-e.txt

  Scenario: init --json output
    Tool: Bash
    Steps:
      1. ./dist/agentenv init zsh --json
    Expected Result: Valid JSON with "functions" array, "env_vars" map, "prompt_template"
    Evidence: .sisyphus/evidence/task-4-json.txt
  ```

  **Commit**: YES (grouped with T1-T3)
  - Message: `feat: add shell init generator for zsh and bash`
  - Files: internal/shell/init.go, and test files

---

- [x] 5. Store abstraction (download, cache, dedup)

  **What to do**:
  - Implement `pkg/store/store.go`: `Store` struct managing `~/.agentenv/store/`
  - `Store.Put(pkg ResolvedPackage, data []byte) error` — store package by (type, source, version)
  - `Store.Get(pkg ResolvedPackage) ([]byte, error)` — retrieve from store
  - `Store.Exists(pkg ResolvedPackage) bool` — check if cached
  - `Store.Link(envPath string, pkg ResolvedPackage) error` — create symlink from env to store
  - `Store.Unlink(envPath string, pkgName string) error` — remove symlink
  - Implement `pkg/store/cache.go`: `Cache` struct managing `~/.agentenv/cache/`
  - `Cache.Download(url string) (string, error)` — download to temp, move to cache, return path
  - Implement `Store.GC() ([]string, error)` — garbage collect unreferenced store entries
  - `Store.ReferencedBy(pkgID string) []string` — list environments referencing a package
  - `Store.Size() (int64, error)` — total store disk usage

  **Must NOT do**:
  - Do NOT implement content-addressing (no IPFS, no CAS) — use (type, source, version) as key
  - Do NOT add compression or dedup beyond symlink sharing
  - Do NOT add network retry logic here (that's in source handlers, Task 6)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high` — filesystem operations with careful error handling
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (after T1, concurrent with T2, T3, T4)
  - **Blocks**: Tasks 6, 14, 15, 18 (all need store)
  - **Blocked By**: Task 1 (needs project root), Task 2 (needs types)

  **References**:
  - Go `os` and `path/filepath` standard library
  - Go `io` and `os` for file copy operations
  - Go `os.Symlink` for symlink creation
  - Go `testing/fstest` for test filesystem

  **Acceptance Criteria**:
  - [ ] `Store.Put` writes package to correct path: `store/<type>/<source-slug>/<version>/`
  - [ ] `Store.Get` reads and returns stored data
  - [ ] `Store.Exists` correctly identifies present/missing packages
  - [ ] `Store.Link` creates valid symlink, `Store.Unlink` removes it without affecting store
  - [ ] `Store.GC` removes only packages with zero environment references
  - [ ] `Store.Size` returns accurate byte count
  - [ ] Store operations work across filesystem boundaries (symlink-to-copy fallback)

  **QA Scenarios**:

  ```
  Scenario: Happy path — put, get, link, unlink cycle
    Tool: Bash (go test)
    Steps:
      1. Store.Put(skillPackage, skillData)
      2. Store.Exists(skillPackage) → true
      3. Store.Get(skillPackage) → same as skillData
      4. Store.Link(envPath, skillPackage) → symlink created
      5. os.Readlink(envLinkPath) → points to store
      6. Store.Unlink(envPath, skillPackage) → symlink removed
      7. Store.Get(skillPackage) → still works (store intact)
    Expected Result: Full cycle works, store data preserved after unlink
    Evidence: .sisyphus/evidence/task-5-store-cycle.txt

  Scenario: Garbage collection
    Tool: Bash (go test)
    Steps:
      1. Store.Put(pkg1) and Store.Put(pkg2)
      2. Link pkg1 to envA, Link pkg2 to envB
      3. Unlink pkg2 from envB (now unreferenced)
      4. Store.GC()
      5. Store.Exists(pkg1) → true (still referenced by envA)
      6. Store.Exists(pkg2) → false (GC'd)
      7. Store.Exists(pkg1) → still true
    Expected Result: Only unreferenced packages removed
    Evidence: .sisyphus/evidence/task-5-gc.txt

  Scenario: Cross-filesystem fallback
    Tool: Bash (go test with tmpfs or different mount)
    Steps:
      1. Create store on /tmp (tmpfs)
      2. Create env on disk
      3. Store.Link(envPath, pkg) → falls back to copy (symlink would cross fs)
      4. os.Lstat(envLinkPath) → regular file (not symlink)
      5. Store.Unlink successfully removes the copy
    Expected Result: Copy fallback works, unlink cleans up
    Evidence: .sisyphus/evidence/task-5-cross-fs.txt
  ```

  **Commit**: YES
  - Message: `feat: add store abstraction with download, cache, dedup, and GC`
  - Files: pkg/store/store.go, pkg/store/cache.go, and tests

### Wave 2: Resolvers + Package Operations

- [x] 6. Source handlers (github, npm, local)

  **What to do**:
  - Implement `pkg/source/source.go`: `SourceHandler` interface — `ListVersions(src SourceURL) ([]SemVer, error)`, `Fetch(src SourceURL, version SemVer) ([]byte, string, error)` (data, sha256, error)
  - Implement `pkg/source/github.go`: `GitHubSource` — GitHub API for tags/releases, download tarballs
  - Implement `pkg/source/npm.go`: `NPMSource` — npm registry API for versions, download tarballs
  - Implement `pkg/source/local.go`: `LocalSource` — read local filesystem
  - Implement `pkg/source/git.go`: `GitSource` — go-git clone and extract
  - Source registry: map scheme → SourceHandler
  - GitHub MUST use `GITHUB_TOKEN` env var for rate limit, work without it
  - All handlers MUST timeout at 30s

  **Must NOT do**: No git submodule support. No file:// scheme. No API response caching.

  **Recommended Agent Profile**: `unspecified-high` — HTTP clients, API integration, error handling
  **Parallelization**: Wave 2 (with T7, T9, T10); Blocks T8, T14, T16, T18; Blocked By T5, T2

  **Acceptance Criteria**:
  - [ ] GitHub handler: list versions from tags, fetch tarball, return data + sha256
  - [ ] npm handler: list versions from registry, fetch tarball, extract skills/ subdir
  - [ ] local handler: read version from agentpkg.yaml, copy directory
  - [ ] All handlers timeout at 30s, unknown scheme → clear error
  - [ ] Works without GITHUB_TOKEN (lower rate limit)

  **QA Scenarios**:

  ```
  Scenario: Happy path — fetch from GitHub
    Tool: Bash (go test with mock HTTP)
    Steps: Mock API returning 3 tags; handler.ListVersions → sorted; handler.Fetch → data+sha256
    Expected Result: Correct version list, correct data
    Evidence: .sisyphus/evidence/task-6-github.txt

  Scenario: Rate limit exceeded
    Tool: Bash (go test)
    Steps: Mock 403 + rate limit headers; handler.ListVersions
    Expected Result: Error with "rate limit" and "GITHUB_TOKEN"
    Evidence: .sisyphus/evidence/task-6-ratelimit.txt

  Scenario: Network timeout
    Tool: Bash (go test)
    Steps: Mock server sleeping 35s; handler.Fetch
    Expected Result: Error with "timeout" after 30s
    Evidence: .sisyphus/evidence/task-6-timeout.txt
  ```

  **Commit**: YES (grouped T6-T10)
  - Message: `feat: add source handlers for github, npm, local, and git`
  - Files: pkg/source/*.go + tests

---

- [x] 7. Version constraint parser + semver matching

  **What to do**:
  - Parse constraint strings → `VersionConstraint` struct: `^x.y.z`, `~x.y.z`, `>=x <y`, `x.y.z` (exact), `latest`, `*`
  - `VersionConstraint.Satisfies(version SemVer) bool`
  - Pre-release handling: `1.0.0-alpha < 1.0.0`, constraints only match pre-releases if explicitly requested
  - Version sorting: highest non-pre-release first

  **Must NOT do**: No `||` (OR) constraints. No hyphen range notation. Don't reimplement semver — use `Masterminds/semver`.

  **Recommended Agent Profile**: `unspecified-high` — algorithmic, edge cases
  **Parallelization**: Wave 2 (with T6, T9, T10); Blocks T8; Blocked By: none (types from T2)

  **References**: `Masterminds/semver` library, npm semver docs, SemVer 2.0 spec

  **Acceptance Criteria**:
  - [ ] `^1.2.3` satisfies 1.9.0 but not 2.0.0
  - [ ] `~1.2.3` satisfies 1.2.9 but not 1.3.0
  - [ ] `>=1.0 <2.0` satisfies 1.5.0 but not 2.0.0
  - [ ] `1.2.3` exact match only
  - [ ] Pre-release sorting correct
  - [ ] Invalid constraint → error, not panic

  **QA Scenarios**:
  ```
  Scenario: Caret constraint matching
    Tool: Bash (go test) — table-driven with 10+ cases
    Expected: Correct per semver.org caret rules
    Evidence: .sisyphus/evidence/task-7-caret.txt

  Scenario: Pre-release edge case
    Tool: Bash (go test) — test 1.0.0-alpha < 1.0.0, constraint ^1.0.0 excludes 1.0.0-alpha
    Evidence: .sisyphus/evidence/task-7-prerelease.txt
  ```

  **Commit**: YES (grouped T6-T10)
  - Message: `feat: add version constraint parser with caret, tilde, range support`
  - Files: pkg/resolver/version.go + tests

---

- [x] 8. Dependency resolver (topological + backtrack + conflict detection)

  **What to do**:
  - Define `DependencyResolver` interface (designed for future algorithm swap)
  - Implement `TopologicalBacktrackResolver`: BFS traversal, fetch + parse agentpkg.yaml, queue deps, version matching, backtrack on conflict
  - Cycle detection: report cycle path clearly (`A → B → A`)
  - Version conflict detection: report constraint intersection failure with source info
  - Conflict levels: HARD (cycle/version → fail), WARNING (tool name collision → auto-prefix), ADVISORY (overlap → note)
  - Enforce limits: max depth 3, max packages 50, hard error beyond

  **Must NOT do**: No SAT solver. No PubGrub. No parallel resolution. No constraint propagation beyond direct deps.

  **Recommended Agent Profile**: `unspecified-high` — algorithmic complexity, state management
  **Parallelization**: Sequential (depends T6+T7+T3); Blocks T14, T16, T18; Blocked By T6+T7+T3

  **Acceptance Criteria**:
  - [ ] Single pkg → resolves itself
  - [ ] Pkg with 2 deps → resolves all 3 transitively
  - [ ] Circular dep → clear error with cycle path
  - [ ] Version conflict → clear error with conflicting sources
  - [ ] Tool collision → warning + auto-prefix
  - [ ] Max depth exceeded → clear error
  - [ ] Compatible constraints from 2 parents → single resolution
  - [ ] Incompatible constraints → conflict error

  **QA Scenarios**:
  ```
  Scenario: Transitive resolution
    Tool: Bash (go test with mock sources)
    Steps: deploy-to-aws depends on git-ops; resolve → both resolved
    Evidence: .sisyphus/evidence/task-8-transitive.txt

  Scenario: Circular dependency
    Tool: Bash (go test) — A→B→A → error with cycle path
    Evidence: .sisyphus/evidence/task-8-cycle.txt

  Scenario: Max depth
    Tool: Bash (go test) — depth 4 chain → error
    Evidence: .sisyphus/evidence/task-8-max-depth.txt
  ```

  **Commit**: YES (grouped T6-T10)
  - Message: `feat: add topological backtrack dependency resolver`
  - Files: pkg/resolver/resolver.go, graph.go, conflict.go + tests

---

- [x] 9. Lockfile generator + reader

  **What to do**:
  - `pkg/lockfile/generate.go`: Generate `Lockfile` from `ResolvedGraph`
  - `pkg/lockfile/reader.go`: Read lockfile → map of resolved packages
  - Deterministic output: sorted keys, consistent YAML formatting (byte-identical on repeat runs)
  - Fields: version (v1), generated (ISO 8601), packages array (name, version, source, sha256, dependencies, resolved sub-deps)
  - Environment snapshot: counts per package type

  **Must NOT do**: No absolute paths. No credentials/secrets. No auto-update on add/remove.

  **Recommended Agent Profile**: `quick` — data transformation
  **Parallelization**: Wave 2 (with T6, T7, T10); Blocks T18; Blocked By T3+T8

  **Acceptance Criteria**:
  - [ ] Generate → write → read back → equal
  - [ ] Two generate calls with same input → byte-identical
  - [ ] All required fields present

  **QA Scenarios**:
  ```
  Scenario: Round-trip integrity
    Tool: Bash (go test) — generate → write → read → compare
    Evidence: .sisyphus/evidence/task-9-roundtrip.txt

  Scenario: Determinism
    Tool: Bash (go test) — generate twice → sha256 both → equal
    Evidence: .sisyphus/evidence/task-9-determinism.txt
  ```

  **Commit**: YES (grouped T6-T10)
  - Message: `feat: add lockfile generator with deterministic output`
  - Files: pkg/lockfile/generate.go, reader.go + tests

---

- [x] 10. agent.yaml reader + validator

  **What to do**:
  - `pkg/envfile/reader.go`: Read agent.yaml → `EnvironmentSpec`
  - `pkg/envfile/validator.go`: Validate required fields (name, description), version constraint syntax, source URL schemes
  - `pkg/envfile/writer.go`: Write EnvironmentSpec → agent.yaml
  - Detect unknown keys → warning with key name
  - Resolve `local:` paths relative to agent.yaml location

  **Must NOT do**: No env var resolution (activation-time). No package fetching (install-time).

  **Recommended Agent Profile**: `unspecified-high` — validation logic, good errors
  **Parallelization**: Wave 2 (with T6, T7, T9); Blocks T14, T18; Blocked By T3

  **Acceptance Criteria**:
  - [ ] Minimal agent.yaml → valid spec
  - [ ] Full agent.yaml (all 6 types) → valid spec
  - [ ] Missing name → validation error
  - [ ] Invalid source URL → error with field name
  - [ ] Unknown key → warning
  - [ ] Write → read round-trip produces equal spec

  **QA Scenarios**:
  ```
  Scenario: Validate complete agent.yaml
    Tool: Bash (go test) — 3 skills, 2 mcps, 1 agent → no errors
    Evidence: .sisyphus/evidence/task-10-validate.txt

  Scenario: Unknown key warning
    Tool: Bash (go test) — "skillz:" typo → warning, not fatal
    Evidence: .sisyphus/evidence/task-10-unknown-key.txt
  ```

  **Commit**: YES (grouped T6-T10)
  - Message: `feat: add agent.yaml reader with validation`
  - Files: pkg/envfile/reader.go, validator.go, writer.go + tests

### Wave 3: Framework Adapter + Activation

- [x] 11. Claude Code adapter (skills install/remove)

  **What to do**:
  - Implement `pkg/adapter/claude_code.go` satisfying `AgentAdapter` interface
  - `InstallSkill(pkg ResolvedPackage) error`: Symlink from store to `~/.claude/skills/<pkg-name>/`
  - `RemoveSkill(name string) error`: Remove symlink from `~/.claude/skills/<name>/`
  - `ListSkills() ([]string, error)`: List symlinked skill dirs
  - Maintain `~/.claude/.agentenv-manifest.json`: Track installed package names + versions
  - On install: check if skill already exists (npx skills installed) → warn, don't overwrite
  - On remove: only remove if in manifest (don't touch user-installed skills)

  **Must NOT do**: No MCP config manipulation here (that's T12). No modification of non-agentenv files.

  **Recommended Agent Profile**: `quick` — filesystem ops, symlink management
  **Parallelization**: Wave 3 (with T12, T13, T14); Blocks T15; Blocked By T5 (store linking)

  **References**: Claude Code skills directory: `~/.claude/skills/<skill-name>/SKILL.md`
  **Adapter interface**: `agentenv-spec.md` Appendix D

  **Acceptance Criteria**:
  - [ ] `InstallSkill` creates correct symlink path
  - [ ] `RemoveSkill` removes symlink, NOT store data
  - [ ] Manifest updated on install/remove
  - [ ] Manifest checked before remove (won't remove non-agentenv skills)
  - [ ] Pre-existing skill → warning on install

  **QA Scenarios**:
  ```
  Scenario: Install and remove skill
    Tool: Bash (go test with temp HOME)
    Steps: Install → check symlink → remove → symlink gone → manifest updated
    Evidence: .sisyphus/evidence/task-11-install-remove.txt

  Scenario: Don't remove user skill
    Tool: Bash (go test) — create skill manually (not in manifest) → remove → skill preserved
    Evidence: .sisyphus/evidence/task-11-preserve-user.txt
  ```

  **Commit**: YES (grouped T11-T14)
  - Message: `feat: add Claude Code adapter for skill installation`
  - Files: pkg/adapter/claude_code.go + tests

---

- [x] 12. Claude Code adapter (MCP config write/read/backup)

  **What to do**:
  - Implement MCP config methods on ClaudeCodeAdapter
  - `InstallMCP(pkg ResolvedPackage) error`: Add server to `~/.claude/.mcp.json`
  - `RemoveMCP(name string) error`: Remove server from `.mcp.json`
  - `ReadMCPConfig() (UniversalMCPConfig, error)`: Parse `.mcp.json` → universal format
  - `WriteMCPConfig(config UniversalMCPConfig) error`: Convert universal → Claude format, write `.mcp.json`
  - `Backup() (BackupID, error)`: Copy `.mcp.json` to `~/.agentenv/backups/claude-code/<timestamp>.json`
  - `Restore(BackupID) error`: Restore from backup
  - Convert between Universal format and Claude Code format (`mcpServers` key, no `transport` field)
  - Handle empty `.mcp.json` (file doesn't exist yet)

  **Must NOT do**: No partial merge (full overwrite with backup). No modification of user's manually added servers (tracked by manifest).

  **Recommended Agent Profile**: `unspecified-high` — JSON manipulation, format conversion, backup safety
  **Parallelization**: Wave 3 (with T11, T13, T14); Blocks T15; Blocked By T2 (UniversalMCPConfig type)

  **References**: Claude Code `.mcp.json` format, Universal MCP Config spec in `agentenv-spec.md` Appendix D

  **Acceptance Criteria**:
  - [ ] Install MCP: server appears in `.mcp.json`, backup created
  - [ ] Remove MCP: server removed from `.mcp.json`, other servers preserved
  - [ ] Backup → modify → restore → original state
  - [ ] Empty `.mcp.json` → install works (creates file)
  - [ ] Read → convert to Universal → convert back → Write → file unchanged (round-trip)

  **QA Scenarios**:
  ```
  Scenario: Install MCP with backup
    Tool: Bash (go test with temp HOME)
    Steps: Create .mcp.json with existing server → install new server → verify both present → restore backup → only original server
    Evidence: .sisyphus/evidence/task-12-backup-restore.txt

  Scenario: Format round-trip integrity
    Tool: Bash (go test) — Claude format → Universal → Claude format → compare
    Evidence: .sisyphus/evidence/task-12-roundtrip.txt
  ```

  **Commit**: YES (grouped T11-T14)
  - Message: `feat: add Claude Code MCP config management with backup/restore`
  - Files: pkg/adapter/claude_code_mcp.go + tests

---

- [x] 13. Claude Code adapter (agent subagent support)

  **What to do**:
  - `InstallAgent(pkg ResolvedPackage) error`: Copy agent definition to `.claude/agents/<pkg-name>.md`
  - `RemoveAgent(name string) error`: Remove `.claude/agents/<name>.md`
  - `ListAgents() ([]string, error)`: List installed agent files
  - Agent definition format: Markdown with optional YAML frontmatter (Claude Code convention)
  - Track in manifest

  **Must NOT do**: No agent template generation. No agent behavior modification.

  **Recommended Agent Profile**: `quick` — simple file copy + manifest tracking
  **Parallelization**: Wave 3 (with T11, T12, T14); Blocks T15; Blocked By T5

  **Acceptance Criteria**:
  - [ ] Install copies agent.md to correct path
  - [ ] Remove deletes agent.md, preserves other agents
  - [ ] Manifest tracks agent installs/removes

  **QA Scenarios**:
  ```
  Scenario: Install and remove agent
    Tool: Bash (go test) — install → check file exists → remove → file gone
    Evidence: .sisyphus/evidence/task-13-agent.txt
  ```

  **Commit**: YES (grouped T11-T14)
  - Message: `feat: add Claude Code agent subagent support`
  - Files: pkg/adapter/claude_code_agent.go + tests

---

- [x] 14. Environment create command

  **What to do**:
  - Implement `cmd/agentenv/create.go` cobra command
  - Create `~/.agentenv/envs/<name>/` directory with:
    - `agent.yaml` (minimal: name + description)
    - `state.json` (runtime state: active=false, agent_framework=claude-code)
  - Flags: `--from <template>` (clone existing env or remote template), `--agent <framework>` (default: claude-code), `--empty` (no agent.yaml)
  - `--from github:...` fetches template's agent.yaml as starting point
  - Detect existing environment → error with helpful message
  - Validate name: kebab-case, max 64 chars, no special chars except hyphen

  **Must NOT do**: No activation during create. No package installation during create.

  **Recommended Agent Profile**: `quick` — directory creation, template copying
  **Parallelization**: Wave 3 (with T11, T12, T13); Blocks T15; Blocked By T3+T10

  **Acceptance Criteria**:
  - [ ] `agentenv create my-env` creates correct directory structure
  - [ ] `agentenv create my-env --agent claude-code` sets default framework
  - [ ] `agentenv create my-env --from existing-env` copies agent.yaml
  - [ ] Duplicate name → clear error
  - [ ] Invalid name → validation error

  **QA Scenarios**:
  ```
  Scenario: Basic create
    Tool: Bash
    Steps: agentenv create test-env → ls ~/.agentenv/envs/test-env/ → agent.yaml + state.json exist
    Evidence: .sisyphus/evidence/task-14-create.txt

  Scenario: Duplicate detection
    Tool: Bash
    Steps: Create twice → second fails with "already exists"
    Evidence: .sisyphus/evidence/task-14-duplicate.txt
  ```

  **Commit**: YES (grouped T11-T14)
  - Message: `feat: add environment create command`
  - Files: cmd/agentenv/create.go + tests

---

- [x] 15. Environment activate/deactivate flow (transactional)

  **What to do**:
  - Implement `cmd/agentenv/internal_activate.go` — internal binary command, outputs JSON
  - Transactional activation protocol:
    1. Validate environment exists, has agent.lock or agent.yaml
    2. Create backup of `~/.claude/.mcp.json` and `~/.claude/.agentenv-manifest.json`
    3. Read lockfile → list of resolved packages
    4. For each package: ensure in store (fetch if missing)
    5. For each package: call adapter.Install* (skill/mcp/agent)
    6. Update manifest with new state
    7. Output JSON: `{"active": true, "env": {"AGENTENV_ACTIVE": "test-env"}, "prompt": "(agentenv:test-env)", "warnings": [...]}`
  - If any step fails: rollback (restore all backups, remove partial installs tracked in manifest)
  - Implement `cmd/agentenv/internal_deactivate.go`:
    1. Read manifest → list of agentenv-installed packages
    2. Call adapter.Remove* for each
    3. Restore MCP config from backup
    4. Output JSON: `{"active": false, "env": {"AGENTENV_ACTIVE": null}, "prompt": null}`
  - Concurrent activation safety: check `~/.agentenv/ACTIVE` lock → if already active in another shell, warn
  - Handle: store package missing (re-fetch), network offline (use cached only), partial previous state

  **Must NOT do**: No env var injection from Go binary. No prompt modification from Go binary. JSON only → shell function applies changes.

  **Recommended Agent Profile**: `deep` — complex state machine, transactional safety, error recovery, rollback
  **Parallelization**: Sequential (depends on ALL Wave 3 tasks + Wave 2)
  **Blocks**: Task 18 (install wraps activate); Blocked By: T4+T5+T8+T11+T12+T13+T14

  **Acceptance Criteria**:
  - [ ] Activate: backup created, packages installed, manifest updated, JSON output correct
  - [ ] Deactivate: packages removed, config restored, manifest cleared
  - [ ] Mid-activation failure → rollback (no partial state)
  - [ ] Activate env B while A active: A cleanly deactivated, B activated
  - [ ] Offline activate (packages in store): works
  - [ ] Offline activate (packages missing): clear error
  - [ ] Concurrent activate: lock prevents corruption

  **QA Scenarios**:
  ```
  Scenario: Full activate-deactivate cycle
    Tool: interactive_bash (tmux, zsh with init)
    Steps: eval init → create env → add packages → activate → check .mcp.json → deactivate → check .mcp.json restored
    Evidence: .sisyphus/evidence/task-15-cycle.txt

  Scenario: Rollback on failure
    Tool: Bash (go test, inject failure at step 4)
    Steps: Start activation → force fail midway → verify .mcp.json unchanged (backup restored)
    Evidence: .sisyphus/evidence/task-15-rollback.txt

  Scenario: Concurrent safety
    Tool: Bash (parallel processes)
    Steps: Activate env A in bg → immediately activate env B → verify only one active, no corruption
    Evidence: .sisyphus/evidence/task-15-concurrent.txt
  ```

  **Commit**: YES
  - Message: `feat: add transactional activate/deactivate with rollback`
  - Files: cmd/agentenv/internal_activate.go, internal_deactivate.go + tests

### Wave 4: Remaining CLI + Polish

- [x] 16. Package add/remove commands

  **What to do**:
  - `cmd/agentenv/add.go`: `agentenv add <type> <name> --source <url> --version <constraint> [--dev]`
  - Resolve package using dependency resolver → update agent.yaml (add entry if new, update if exists)
  - `cmd/agentenv/remove.go`: `agentenv remove <name>` → remove from agent.yaml, update manifest if active
  - `--dev` flag: for local sources, don't copy to store (symlink to local path for live editing)
  - `--dry-run`: show what would be added without modifying files
  - Validate package type: must be one of skill|mcp|agent|tool|hook|prompt
  - Inform user to run `agentenv lock` after adding (not auto)
  - If env is active, warn that changes won't take effect until reactivation

  **Must NOT do**: No auto-lock after add. No auto-reactivate after add.

  **Recommended Agent Profile**: `quick` — CLI wiring, YAML manipulation
  **Parallelization**: Wave 4 (with T17, T19, T20); Blocked By T8+T10

  **Acceptance Criteria**:
  - [ ] `add skill code-review --source github:...` adds to agent.yaml
  - [ ] `add mcp filesystem --source npm:...` adds MCP entry
  - [ ] `add --dev local:./my-skill` adds local dev entry
  - [ ] `add --dry-run` prints change, doesn't modify file
  - [ ] `remove code-review` removes from agent.yaml
  - [ ] Invalid type → error
  - [ ] Active env → warning about reactivation needed

  **QA Scenarios**:
  ```
  Scenario: Add skill then verify agent.yaml
    Tool: Bash
    Steps: add skill X → cat agent.yaml → X entry present with version constraint
    Evidence: .sisyphus/evidence/task-16-add.txt

  Scenario: Warn on active env
    Tool: interactive_bash
    Steps: activate env → add skill → verify warning about reactivation
    Evidence: .sisyphus/evidence/task-16-active-warn.txt
  ```

  **Commit**: YES (grouped T16-T21)
  - Message: `feat: add package add/remove commands`
  - Files: cmd/agentenv/add.go, remove.go + tests

---

- [x] 17. List/info/delete commands

  **What to do**:
  - `cmd/agentenv/list.go`: List all environments at `~/.agentenv/envs/`, show name + agent + status (active/inactive)
  - `cmd/agentenv/info.go`: Show env details (packages count, agent framework, location, last activated)
  - `cmd/agentenv/list_packages.go`: `agentenv list-packages` — show all packages in current env (or specified env)
  - `agentenv list-packages --tree`: Tree view showing dependency hierarchy
  - `agentenv list-packages --type <type>`: Filter by package type
  - `cmd/agentenv/delete.go`: `agentenv delete <name> [--force]` — remove env directory + symlinks
  - `delete`: warn if env is active, require `--force` if active
  - All commands support `--json` output

  **Must NOT do**: No recursive delete of store packages (GC is separate). No info for non-existent env.

  **Recommended Agent Profile**: `quick` — directory listing, formatting
  **Parallelization**: Wave 4 (with T16, T19, T20)

  **Acceptance Criteria**:
  - [ ] `list` shows all environments with status
  - [ ] `info` shows correct details for specified env
  - [ ] `list-packages` shows all packages, count matches
  - [ ] `list-packages --tree` shows correct hierarchy
  - [ ] `list-packages --type mcp` filters correctly
  - [ ] `delete` removes env directory
  - [ ] `delete` on active env → error (unless --force)
  - [ ] All commands support `--json`

  **QA Scenarios**:
  ```
  Scenario: list then info then delete
    Tool: Bash
    Steps: create env → list (sees it) → info (correct details) → delete → list (gone)
    Evidence: .sisyphus/evidence/task-17-crud.txt

  Scenario: tree output
    Tool: Bash
    Steps: add skill with dep → lock → list-packages --tree → verify hierarchy displayed
    Evidence: .sisyphus/evidence/task-17-tree.txt
  ```

  **Commit**: YES (grouped T16-T21)
  - Message: `feat: add list, info, list-packages, and delete commands`
  - Files: cmd/agentenv/list.go, info.go, list_packages.go, delete.go + tests

---

- [x] 18. Lock + install commands (full loop)

  **What to do**:
  - `cmd/agentenv/lock.go`: `agentenv lock [--update] [--strict]`
    - Read agent.yaml → resolve dependencies → generate lockfile → write agent.lock
    - `--update`: resolve to latest compatible versions (not just from lock)
    - `--strict`: fail on any warning (tool collisions, advisory)
    - Show progress: ⠋ Resolving N packages... package by package status
  - `cmd/agentenv/install.go`: `agentenv install [--file agent.yaml]`
    - Read agent.lock (or resolve from agent.yaml if no lock)
    - Fetch missing packages to store
    - If env is active: run activation to apply
    - If env is inactive: just ensure store is populated
    - `--file`: use alternate agent.yaml
  - **THIS IS THE E2E INTEGRATION POINT** — tests must exercise the full flow

  **Must NOT do**: No auto-lock after add. No auto-install after lock.

  **Recommended Agent Profile**: `unspecified-high` — integration, orchestrating multiple subsystems
  **Parallelization**: Sequential (depends on T8+T9+T10+T15); Blocks F1-F4
  **Blocked By**: T8, T9, T10, T15

  **Acceptance Criteria**:
  - [ ] `lock` generates valid agent.lock matching spec format
  - [ ] `lock` twice → byte-identical output
  - [ ] `lock --update` updates versions to latest compatible
  - [ ] `lock --strict` fails on warnings
  - [ ] `install` reads lockfile, fetches packages, activates
  - [ ] `install` works offline if packages cached
  - [ ] `install --file other.yaml` reads alternate file
  - [ ] Progress displayed during lock
  - [ ] Post-install: `agent.yaml` + `agent.lock` + store + activated env all consistent

  **QA Scenarios**:
  ```
  Scenario: Full lock → install → verify
    Tool: Bash (E2E)
    Steps: create env → add skill → add mcp → lock → install → verify skill symlink → verify mcp in .mcp.json
    Evidence: .sisyphus/evidence/task-18-e2e.txt

  Scenario: Lock determinism
    Tool: Bash
    Steps: lock twice → sha256 both → equal
    Evidence: .sisyphus/evidence/task-18-determinism.txt

  Scenario: Offline install
    Tool: Bash (network namespace isolated)
    Steps: lock (online) → install (offline, all cached) → success
    Evidence: .sisyphus/evidence/task-18-offline.txt
  ```

  **Commit**: YES
  - Message: `feat: add lock and install commands with full integration`
  - Files: cmd/agentenv/lock.go, install.go + E2E tests

---

- [x] 19. Error handling + actionable messages

  **What to do**:
  - Audit ALL commands for error paths → ensure every error has:
    1. What went wrong (specific, not generic)
    2. Why it happened (context)
    3. What to do about it (actionable next step)
  - Follow the 4 error principles from spec:
    - "why + how to fix"
    - actionable verbs
    - clear exit paths
    - friendly but not verbose
  - Wrap common errors: network failure, permission denied, invalid YAML, missing npx skills
  - Consistent error format: `✗ <error type>: <message>\n  <suggestion>`
  - Exit codes: 0 (success), 1 (user error), 2 (system error), 3 (network error)
  - Test error paths: ensure they don't panic

  **Must NOT do**: No stack traces in user-facing output (only with `--debug`). No "unexpected error" without context.

  **Recommended Agent Profile**: `quick` — text, consistency, UX
  **Parallelization**: Wave 4 (with T16, T17, T20); Blocked By: all prior tasks (needs to see all error paths)

  **Acceptance Criteria**:
  - [ ] Network error → "✗ Cannot reach GitHub: ...\n  Check your connection or try again in a few minutes."
  - [ ] Permission denied → "✗ Cannot write to ~/.claude/.mcp.json: ...\n  Check file permissions or run with appropriate user."
  - [ ] Missing npx skills → "✗ npx skills not found. Install with: npm install -g @anthropic-ai/skills"
  - [ ] No generic "unexpected error" messages
  - [ ] Exit codes correct for each category
  - [ ] `--debug` flag shows stack traces

  **QA Scenarios**:
  ```
  Scenario: Network unavailable
    Tool: Bash (unshare -n)
    Steps: agentenv add skill X → clear network error, exit code 3
    Evidence: .sisyphus/evidence/task-19-network.txt

  Scenario: Permission denied
    Tool: Bash (chmod 000 ~/.claude/)
    Steps: agentenv activate → clear permission error, exit code 2
    Evidence: .sisyphus/evidence/task-19-permission.txt

  Scenario: --debug shows stack trace
    Tool: Bash
    Steps: trigger error with --debug → output contains file:line
    Evidence: .sisyphus/evidence/task-19-debug.txt
  ```

  **Commit**: YES (grouped T16-T21)
  - Message: `fix: add actionable error messages with consistent format`
  - Files: all cmd/*.go (audit), pkg/errors/errors.go

---

- [x] 20. --json output + completions + help text

  **What to do**:
  - Every command supports `--json`: outputs valid JSON, no progress spinners, no colors
  - JSON schema: `{"status": "ok"|"error", "data": {...}, "error": {"code": "...", "message": "..."}}`
  - Shell completions: `agentenv completion zsh|bash` → outputs completion script
  - Completions for: commands, environment names (from `~/.agentenv/envs/`), package types
  - Cobra built-in help text for all commands
  - Root command `--version` and `--help`
  - Hidden `_internal_activate` and `_internal_deactivate` commands (not shown in help)

  **Must NOT do**: No custom completion for package names in v0.1.0 (too dynamic). No man page generation yet.

  **Recommended Agent Profile**: `quick` — cobra flags, template generation
  **Parallelization**: Wave 4 (with T16, T17, T19); Blocked By: all prior (needs command structure)

  **Acceptance Criteria**:
  - [ ] `agentenv list --json` outputs valid JSON
  - [ ] `agentenv activate test-env --json` outputs valid JSON (from internal command)
  - [ ] `agentenv completion zsh` outputs valid zsh completion
  - [ ] `agentenv completion bash` outputs valid bash completion
  - [ ] `agentenv --help` shows all commands
  - [ ] `agentenv <cmd> --help` shows cmd-specific help
  - [ ] `agentenv --version` shows version
  - [ ] Internal commands hidden from help

  **QA Scenarios**:
  ```
  Scenario: --json on all commands
    Tool: Bash
    Steps: Run each command with --json → pipe to jq → valid JSON
    Evidence: .sisyphus/evidence/task-20-json.txt

  Scenario: Completion loads correctly
    Tool: interactive_bash
    Steps: source completion → type "agentenv " + TAB → shows subcommands
    Evidence: .sisyphus/evidence/task-20-completion.txt
  ```

  **Commit**: YES (grouped T16-T21)
  - Message: `feat: add --json output, completions, and help text`
  - Files: cmd/agentenv/root.go, all cmd/*.go (json flag), completion.go

---

- [x] 21. Release engineering (goreleaser, brew formula, install.sh)

  **What to do**:
  - Configure `.goreleaser.yaml`: builds for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64
  - Generate brew formula: `goreleaser` auto-generates `agentenv.rb`
  - Create `install.sh`: curl | sh installer that detects OS/arch, downloads binary, verifies sha256, installs to `/usr/local/bin`
  - `install.sh` MUST NOT pipe to sh without checksum verification
  - GitHub Actions release workflow: on tag push, run goreleaser, upload assets, create release
  - Create Homebrew tap repo: `homebrew-agentenv` with formula
  - Test: `brew install agentenv` works on macOS
  - Test: `curl ... | sh` works on fresh Linux VM

  **Must NOT do**: No Homebrew core submission (requires popularity). No Snap/Flatpak. No Windows MSI.

  **Recommended Agent Profile**: `quick` — CI/CD, goreleaser config, shell scripting
  **Parallelization**: Wave 4 (with T16, T17, T19, T20); Blocked By: T1 (goreleaser skeleton)

  **Acceptance Criteria**:
  - [ ] `goreleaser build --snapshot --clean` produces all 4 binaries
  - [ ] `install.sh` downloads correct binary for OS/arch
  - [ ] `install.sh` verifies sha256 before installing
  - [ ] `brew install agentenv` works (from local tap)
  - [ ] GitHub release has correct assets
  - [ ] GitHub Actions release workflow triggers on tag

  **QA Scenarios**:
  ```
  Scenario: Goreleaser snapshot build
    Tool: Bash
    Steps: goreleaser build --snapshot --clean → check dist/ has 4 platform dirs
    Evidence: .sisyphus/evidence/task-21-goreleaser.txt

  Scenario: install.sh on Linux
    Tool: Bash (Docker container, ubuntu:latest)
    Steps: curl install.sh | sh → agentenv --version → outputs version
    Evidence: .sisyphus/evidence/task-21-install-sh.txt
  ```

  **Commit**: YES
  - Message: `feat: add release engineering with goreleaser, brew, and install.sh`
  - Files: .goreleaser.yaml, install.sh, .github/workflows/release.yml

---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each Must Have: verify implementation exists. For each Must NOT Have: search codebase for forbidden patterns. Check evidence files exist. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./...` + `golangci-lint run`. Review all changed files for: `interface{}` (should use generics), naked returns, unchecked errors, unused imports, commented-out code. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task. Test cross-task integration. Test edge cases: concurrent activation, network failure, npx skills missing, store GC. Save evidence to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read What to do, read actual diff. Verify 1:1 — everything specified was built, nothing beyond spec was built. Check Must NOT do compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Wave 1**: `feat: project scaffolding and core types`
- **Wave 2**: `feat: dependency resolver and source handlers`
- **Wave 3**: `feat: claude code adapter and activation flow`
- **Wave 4**: `feat: CLI commands, error handling, and release`
- **Fixes**: `fix(scope): description` — squash into parent feature commit

---

## Success Criteria

### Verification Commands
```bash
go test ./...                           # All tests pass
go vet ./...                            # No vet warnings
golangci-lint run                       # No lint errors
goreleaser build --snapshot --clean     # Binary builds for all targets
agentenv create test && agentenv activate test && agentenv deactivate  # E2E works
```

### Final Checklist
- [ ] All Must Have present
- [ ] All Must NOT Have absent
- [ ] All tests pass on macOS + Linux
- [ ] `agent.yaml` format documented
- [ ] `agent.lock` format documented
- [ ] Release artifacts: brew formula, install.sh, binary tarballs
