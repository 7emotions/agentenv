<p align="center">
  <h1 align="center">AgentEnv</h1>
  <p align="center"><strong>conda for AI agents</strong> — isolated, reproducible environments for agent development.</p>
</p>

<p align="center">
  <img src="https://img.shields.io/github/go-mod/go-version/7emotions/agentenv?label=go" alt="Go Version">
  <img src="https://img.shields.io/github/license/7emotions/agentenv" alt="License">
  <img src="https://img.shields.io/github/v/release/7emotions/agentenv" alt="Release">
  <img src="https://img.shields.io/github/actions/workflow/status/7emotions/agentenv/ci.yml?branch=main" alt="CI">
</p>

---

## Quick Start

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/7emotions/agentenv/main/install.sh | sh

# Create your first environment
agentenv create my-env

# Add a skill from GitHub
agentenv add skill code-reviewer --source github:myorg/code-review-skill

# Lock and install dependencies
agentenv lock
agentenv install

# Activate
agentenv activate my-env
```

---

## What is agentenv?

agentenv is an **environment manager for AI agents** — think conda, but for skills, MCP servers, agents, tools, hooks, and prompts.

| Problem | agentenv |
|---------|----------|
| Working on multiple projects with conflicting agent setups | Isolated environments under `~/.agentenv/` |
| Manually installing skills and MCP servers per project | Declarative `agent.yaml` + lockfile |
| "Works on my machine" across Claude Code, OpenCode, Cursor | Reproducible, version-pinned, cross-framework |
| Losing track of what's installed | Content-addressed store with dedup and garbage collection |
| Switching between Claude Code and OpenCode | One tool, 4 frameworks, same workflow |

---

## Features

- **6 package types** — skill, mcp, agent, tool, hook, prompt. Mix freely in one environment.
- **4 framework adapters** — Claude Code, OpenCode, Cursor, Codex. All 16 methods per adapter.
- **6 source schemes** — `github:`, `npm:`, `local:`, `git:`, `file:`, `url:`. Pull packages from anywhere.
- **Deterministic lockfiles** — agent.yaml → agent.lock, semver constraints, transitive deps, SHA-256 integrity.
- **Content-addressed store** — symlink deduplication, cross-filesystem copy fallback, garbage collection.
- **Transactional activation** — backup → validate → apply → rollback on failure. No partial state.
- **Structured errors** — every error tells you what went wrong, why, and how to fix it.
- **Zero telemetry** — no analytics, no phone-home, no tracking. Period.

---

## Commands

| Command | Description |
|---------|-------------|
| `create` | Create a new environment |
| `add` | Add a package to an environment |
| `remove` | Remove a package |
| `lock` | Resolve dependencies and generate agent.lock |
| `install` | Fetch packages into the local store |
| `activate` | Activate an environment (install to framework) |
| `deactivate` | Deactivate and clean up |
| `list` | List all environments |
| `list-packages` | List packages in an environment |
| `list-adapters` | Show available framework adapters |
| `info` | Show environment details |
| `delete` | Delete an environment |
| `completion` | Generate shell completion scripts |
| `init` | Generate shell init script |

All commands support `--json` for machine-readable output and `--debug` for stack traces.

```bash
# Create with a specific framework
agentenv create --agent opencode my-opencode-env

# See all available frameworks
agentenv list-adapters
# claude-code  │  Claude Code
# opencode     │  OpenCode
# cursor       │  Cursor
# codex        │  Codex

# JSON output
agentenv info my-env --json | jq .
```

---

## Supported Frameworks

| Framework | Adapter | Config Path | MCP Format |
|-----------|---------|-------------|------------|
| **Claude Code** | `claude-code` | `~/.claude/` | `.mcp.json` |
| **OpenCode** | `opencode` | `~/.config/opencode/` | Inline `opencode.jsonc` (JSONC) |
| **Cursor** | `cursor` | `~/.cursor/` | Framework-specific |
| **Codex** | `codex` | N/A | TOML-based (partial support) |

Each adapter manages skills, MCP servers, agents, and manifests — with automatic backup before modification and rollback on failure.

---

## agent.yaml — Environment Definition

```yaml
name: my-ai-env
description: "My personal AI development environment"

skills:
  code-reviewer:
    source: github:myorg/code-review-skill
    version: ">=0.5"

mcps:
  filesystem:
    source: npm:@modelcontextprotocol/server-filesystem
    version: "0.6.0"
    config:
      root: /home/user/projects
```

**Package spec schema** (same for all 6 types):
- `source` (required) — Source URL with scheme prefix
- `version` (optional) — Semver constraint, defaults to `*`
- `config` (optional) — Arbitrary package-specific configuration

[Full format reference →](docs/agent-yaml.md)

---

## agent.lock — Deterministic Lockfile

```json
{
  "version": 1,
  "generated": "2026-05-24T12:00:00Z",
  "packages": [
    {
      "name": "code-reviewer",
      "type": "skill",
      "source": "github:myorg/code-review-skill",
      "version": ">=0.5",
      "resolved": "0.7.2",
      "sha256": "abc123...",
      "dependencies": [
        {
          "name": "lint-helper",
          "version": "^1.0",
          "resolved": "1.2.0",
          "source": ""
        }
      ]
    }
  ],
  "environment_snapshot": {
    "skills_count": 1,
    "mcps_count": 0,
    "agents_count": 0,
    "tools_count": 0,
    "hooks_count": 0,
    "prompts_count": 0
  }
}
```

- Packages are sorted by `(type, name)` for deterministic output
- v1 lockfiles auto-migrate to v2 on read — no data loss
- `agentenv lock` always writes v2 format

[Full format reference →](docs/agent-lock.md)

---

## Source URL Schemes

| Scheme | Format | Example |
|--------|--------|---------|
| `github` | `github:owner/repo[/path]` | `github:myorg/code-review-skill` |
| `npm` | `npm:@scope/package` | `npm:@anthropic/claude-code` |
| `local` | `local:./relative/path` | `local:./vendor/my-skill` |
| `git` | `git:https://repo-url` | `git:https://github.com/org/repo.git` |
| `file` | `file:/absolute/path` | `file:/home/user/packages/skill.tar.gz` |
| `url` | `url:https://download-url` | `url:https://example.com/pkg.tar.gz` |

---

## Architecture

```
agentenv CLI (cobra)
├── cmd/agentenv/          → CLI commands, activation/deactivation
├── pkg/adapter/           → Framework adapters (4× 16 methods)
│   ├── interface.go       → AgentAdapter contract
│   ├── registry.go        → Thread-safe registry (sync.RWMutex)
│   ├── jsonc.go           → JSONC comment-preserving utilities
│   └── mcp.go             → Universal MCP config
├── pkg/types/             → Core data types
│   ├── source.go          → SourceURL (6 scheme parser)
│   ├── package.go         → PackageType, PackageDependency (v2 source field)
│   ├── environment.go     → Environment (6 section maps)
│   └── lockfile.go        → Lockfile v1→v2 migration
├── pkg/parser/            → YAML/JSON parsers
├── pkg/envfile/           → agent.yaml read/write/validate/merge
├── pkg/source/            → Source fetchers (github, npm, local, git)
├── pkg/resolver/          → Dependency resolver (topological, semver, backtracking)
├── pkg/store/             → Content-addressed store (symlink dedup, GC, cache)
└── pkg/errors/            → Structured errors (User/System/Network)
```

---

## Install

```bash
# Shell script (recommended)
curl -fsSL https://raw.githubusercontent.com/7emotions/agentenv/main/install.sh | sh

# Go install
go install github.com/7emotions/agentenv/cmd/agentenv@latest

# Homebrew
brew tap 7emotions/homebrew-agentenv
brew install agentenv

# Build from source
git clone https://github.com/7emotions/agentenv.git
cd agentenv && make build
```

**Platforms**: macOS (darwin) + Linux, amd64 + arm64.  
**Requirements**: Go 1.23+ (for source builds only — binary releases are self-contained).

---

## Design Guardrails

- **No telemetry** — zero analytics, zero tracking, zero phone-home
- **No plugins** — all adapters registered at compile time via `init()`
- **No SAT solver** — topological sort with backtracking; interface designed for future algorithm swap
- **No Windows** — deliberate scope boundary
- **No breaking JSONC** — OpenCode config comments are preserved, not stripped

---

## License

MIT — see [LICENSE](LICENSE).
