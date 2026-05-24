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

<p align="center">
  <a href="#quick-start--快速开始">English</a> · <a href="#quick-start--快速开始">中文</a>
</p>

---

## Quick Start / 快速开始

```bash
# Install | 安装
curl -fsSL https://raw.githubusercontent.com/7emotions/agentenv/main/install.sh | sh

# Create a workspace with a specific framework | 创建指定框架的环境
agentenv create my-env --agent opencode

# Add packages from anywhere | 从各种源添加包
agentenv add skill code-reviewer --source github:myorg/reviewer --version "^0.5"
agentenv add mcp filesystem --source npm:@modelcontextprotocol/server-filesystem \
  --command npx --arg -y --arg @modelcontextprotocol/server-filesystem --arg /tmp

# Resolve: PubGrub finds the best version for every package | 依赖解析
agentenv lock

# Fetch packages into the store | 安装到本地存储
agentenv install

# Activate: inject into your framework | 激活：注入到框架配置
agentenv activate my-env

# Done — your agent is ready to use all packages | 完成
agentenv deactivate  # switch back when done | 用完后停用
```

> 中文：一条命令安装，几步创建。支持 6 种源（GitHub、npm、本地等）、6 种包类型（技能、MCP、智能体等）、4 种框架适配器。依赖解析由 PubGrub 引擎自动完成。

---

## What is agentenv? / 什么是 AgentEnv？

agentenv is an **environment manager for AI agents** — think conda, but for skills, MCP servers, agents, tools, hooks, and prompts.

> 中文：agentenv 是 **AI 智能体的环境管理器** — 就像 conda，但管理的是技能、MCP 服务器、智能体、工具、钩子和提示词。

| Problem | agentenv |
|---------|----------|
| Working on multiple projects with conflicting agent setups | Isolated environments under `~/.agentenv/` |
| Manually installing skills and MCP servers per project | Declarative `agent.yaml` + lockfile |
| "Works on my machine" across Claude Code, OpenCode, Cursor | Reproducible, version-pinned, cross-framework |
| Losing track of what's installed | Content-addressed store with dedup and garbage collection |
| Switching between Claude Code and OpenCode | One tool, 4 frameworks, same workflow |

---

## Features / 特性

- **6 package types** — skill, mcp, agent, tool, hook, prompt. Mix freely in one environment.
> 中文：支持 6 种包类型 — 技能、MCP、智能体、工具、钩子、提示词，可在同一环境中混合使用。
- **4 framework adapters** — Claude Code, OpenCode, Cursor, Codex. All 16 methods per adapter.
> 中文：4 个框架适配器 — Claude Code、OpenCode、Cursor、Codex，每个适配器实现全部 16 个方法。
- **6 source schemes** — `github:`, `npm:`, `local:`, `git:`, `file:`, `url:`. Pull packages from anywhere.
> 中文：6 种源地址协议 — 可从 GitHub、npm、本地、Git 仓库、文件路径、URL 任意来源安装包。
- **PubGrub dependency resolution** — CDCL version solving with transitive dependency discovery, constraint checking, and clear conflict reporting
> 中文：PubGrub 依赖解析 — CDCL 版本求解，支持传递依赖发现、约束检查和清晰的冲突报告。
- **Deterministic lockfiles** — agent.yaml → agent.lock, semver constraints, transitive deps, SHA-256 integrity.
> 中文：确定性锁文件 — 从 agent.yaml 生成 agent.lock，语义化版本约束、传递依赖、SHA-256 完整性校验。
- **Content-addressed store** — symlink deduplication, cross-filesystem copy fallback, garbage collection.
> 中文：内容寻址存储 — 符号链接去重、跨文件系统复制回退、垃圾回收。
- **Transactional activation** — backup → validate → apply → rollback on failure. No partial state.
> 中文：事务性激活 — 备份 → 校验 → 应用 → 失败回滚，不会留下半残状态。
- **Structured errors** — every error tells you what went wrong, why, and how to fix it.
> 中文：结构化错误 — 每个错误都说明原因、影响和修复方法。
- **Zero telemetry** — no analytics, no phone-home, no tracking. Period.
> 中文：零遥测 — 无分析、无回传、无追踪。这就是底线。

---

## Commands / 命令

| Command / 命令 | Description / 说明 |
|---------|-------------|
| `create` | Create a new environment / 创建新环境 |
| `add` | Add a package to an environment / 添加包到环境 |
| `remove` | Remove a package / 移除包 |
| `lock` | Resolve dependencies and generate agent.lock / 解析依赖并生成锁文件 |
| `install` | Fetch packages into the local store / 下载包到本地存储 |
| `activate` | Activate an environment (install to framework) / 激活环境（安装到框架） |
| `deactivate` | Deactivate and clean up / 停用环境并清理 |
| `list` | List all environments / 列出所有环境 |
| `list-packages` | List packages in an environment / 列出环境中的包 |
| `list-adapters` | Show available framework adapters / 显示可用框架适配器 |
| `info` | Show environment details / 查看环境详情 |
| `delete` | Delete an environment / 删除环境 |
| `completion` | Generate shell completion scripts / 生成 shell 补全脚本 |
| `init` | Generate shell init script / 生成 shell 初始化脚本 |

All commands support `--json` for machine-readable output and `--debug` for stack traces.

> 中文：所有命令都支持 `--json` 输出机器可读的结果，以及 `--debug` 查看堆栈信息。

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

## Usage / 使用指南

### Creating an environment / 创建环境

Create a new, empty environment with a name of your choice:

```bash
# Basic: create with default framework (claude-code)
agentenv create my-env

# Specify a framework
agentenv create --agent opencode my-opencode-env

# Create in the current directory (uses folder name)
agentenv create .
```

> 中文：创建一个新的空环境。可以指定框架，默认使用 claude-code。使用 `.` 可用当前文件夹名作为环境名。

Environments live under `~/.agentenv/environments/` with a generated `agent.yaml` manifest. You can inspect and edit this file directly.

> 中文：环境存储在 `~/.agentenv/environments/` 目录下，每个环境包含一个 `agent.yaml` 清单文件，可直接编辑。

---

### Adding packages / 添加包

Add skills, MCP servers, agents, tools, hooks, or prompts from any supported source:

```bash
# Add a skill from GitHub
agentenv add skill code-reviewer --source github:myorg/code-review-skill

# Add a skill with version constraint
agentenv add skill code-reviewer --source github:myorg/code-review-skill --version ">=0.5"

# Add an MCP server from npm
agentenv add mcp filesystem \
  --source npm:@modelcontextprotocol/server-filesystem \
  --command npx \
  --arg -y \
  --arg @modelcontextprotocol/server-filesystem \
  --arg /tmp

# Add a local skill during development
agentenv add skill my-custom-tool --source local:./vendor/my-tool
```

> 中文：从 GitHub、npm、本地路径等来源添加各种类型的包。可以指定版本约束，MCP 服务器支持传递命令行参数。

Each `add` command writes to `agent.yaml` immediately. You can verify with `agentenv list-packages`.

> 中文：每次 `add` 命令会立即写入 `agent.yaml`，可用 `agentenv list-packages` 验证。

---

### Resolving dependencies / 依赖解析

After adding packages, resolve their dependency graph:

```bash
agentenv lock
```

`agentenv lock` runs the **PubGrub CDCL resolver** to find a compatible set of versions across all packages and their transitive dependencies. It produces `agent.lock` — a deterministic, SHA-256-pinned snapshot.

> 中文：`agentenv lock` 使用 PubGrub CDCL 解析器，为所有包及其传递依赖找到一组兼容的版本。输出 `agent.lock` 锁文件，包含 SHA-256 完整性校验。

If conflicts are found, the resolver reports exactly which constraints conflict and why:

```
error: conflict detected
  code-reviewer@>=1.0 requires lint-helper@^2.0
  my-other-skill@>=0.5 requires lint-helper@^1.0
  lint-helper@^2.0 and lint-helper@^1.0 are incompatible
```

> 中文：如果发现版本冲突，解析器会精确报告冲突的依赖关系和版本约束。

---

### Installing and activating / 安装和激活

Once locked, download all packages and activate the environment:

```bash
# Fetch all packages into the local content-addressed store
agentenv install

# Activate — installs everything into your framework's config
agentenv activate my-env

# Deactivate — removes from framework config
agentenv deactivate
```

> 中文：`install` 将所有解析后的包下载到本地内容寻址存储。`activate` 将环境注册到当前框架的配置中。`deactivate` 则从框架配置中移除。

Activation is **transactional**: it backs up your framework config, validates the new config, applies changes, and rolls back on failure. You never get partial state.

> 中文：激活是**事务性**的 — 备份框架配置、校验新配置、应用变更、失败自动回滚，永远不会留下不完整的配置状态。

---

### Full workflow example / 完整工作流程

End-to-end: from zero to a running environment with multiple packages.

```bash
# 1. Create a project environment
cd ~/projects/my-ai-project
agentenv create .

# 2. Add packages of different types
agentenv add skill code-reviewer --source github:myorg/code-review-skill
agentenv add mcp filesystem \
  --source npm:@modelcontextprotocol/server-filesystem \
  --command npx --arg -y \
  --arg @modelcontextprotocol/server-filesystem \
  --arg /tmp/projects
agentenv add agent my-helper --source github:myorg/helper-agent

# 3. Check what's declared
cat agent.yaml

# 4. Resolve dependencies
agentenv lock

# 5. Inspect the lockfile
cat agent.lock

# 6. Download packages
agentenv install

# 7. Activate (installs into your framework)
agentenv activate my-ai-project

# 8. Verify
agentenv list-packages
agentenv info my-ai-project

# 9. When done
agentenv deactivate
```

> 中文：完整流程演示 — 从创建环境、添加多种类型的包、解析依赖、下载安装到激活使用。整个过程声明式、可重现。

---

### JSON output / JSON 输出

All commands support `--json` for machine-readable output, perfect for scripting and tool integration:

```bash
# Get environment info as JSON
agentenv info my-env --json | jq .

# List all environments
agentenv list --json | jq '.[].name'

# List packages with versions
agentenv list-packages --json | jq '[.[] | {name, type, resolved}]'
```

> 中文：所有命令均支持 `--json` 输出，便于脚本集成。结合 `jq` 可以灵活提取所需信息。

---

### Framework selection / 框架选择

agentenv supports 4 AI coding frameworks. Choose the right one for your workflow:

```bash
# List available adapters
agentenv list-adapters

# Create an environment for a specific framework
agentenv create --agent opencode my-opencode-env
agentenv create --agent cursor my-cursor-env
agentenv create --agent codex my-codex-env
```

> 中文：agentenv 支持 4 种 AI 编码框架。`list-adapters` 查看可用框架，`create --agent` 指定框架创建环境。

Each adapter knows where its framework stores config, what format it uses, and how to register skills, MCP servers, and agents. Switching frameworks doesn't change your workflow — same commands, same `agent.yaml`.

| Use Case | Recommended Framework |
|----------|---------------------|
| General-purpose AI coding | `claude-code` |
| Open-source, configurable | `opencode` |
| IDE-integrated | `cursor` |
| Lightweight terminal | `codex` |

> 中文：每个适配器知道对应框架的配置路径和格式。切换框架不影响你的工作流 — 命令不变，agent.yaml 不变。

---

## Supported Frameworks / 支持的框架

| Framework / 框架 | Adapter / 适配器 | Config Path / 配置路径 | MCP Format / MCP 格式 |
|-----------|---------|-------------|------------|
| **Claude Code** | `claude-code` | `~/.claude/` | `.mcp.json` |
| **OpenCode** | `opencode` | `~/.config/opencode/` | Inline `opencode.jsonc` (JSONC) |
| **Cursor** | `cursor` | `~/.cursor/` | Framework-specific |
| **Codex** | `codex` | N/A | TOML-based (partial support) |

Each adapter manages skills, MCP servers, agents, and manifests — with automatic backup before modification and rollback on failure.

> 中文：每个适配器管理技能、MCP 服务器、智能体和清单文件 — 修改前自动备份，失败时自动回滚。

---

## agent.yaml — Environment Definition / 环境定义

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
- `source` (required) — Source URL with scheme prefix / 源地址（含协议前缀）
- `version` (optional) — Semver constraint, defaults to `*` / 语义化版本约束，默认 `*`
- `config` (optional) — Arbitrary package-specific configuration / 包特定配置

> 中文：agent.yaml 是环境定义文件，YAML 格式。支持 6 种包类型，每种包用 `source` + `version` + `config` 描述。

[Full format reference →](docs/agent-yaml.md)

---

## agent.lock — Deterministic Lockfile / 确定性锁文件

```json
{
  "version": 3,
  "generated": "2026-05-24T12:00:00Z",
  "packages": [
    {
      "name": "code-reviewer",
      "type": "skill",
      "source": "github:myorg/code-review-skill",
      "version": ">=0.5",
      "resolved": "0.7.2",
      "sha256": "abc123...",
      "resolved_by": "(root)",
      "dependencies": [
        {
          "name": "lint-helper",
          "version": "^1.0",
          "resolved": "1.2.0",
          "source": "",
          "type": "skill"
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

- Packages are sorted by `(type, name)` for deterministic output / 包按 `(类型, 名称)` 排序确保确定性输出
- v1 lockfiles auto-migrate to v3 on read — no data loss / v1 锁文件读取时自动迁移到 v3，数据不丢失
- `agentenv lock` always writes v3 format / `agentenv lock` 始终输出 v3 格式
- Each package tracks its `resolved_by` parent — root packages show `(root)`, transitive deps show their parent's name / 每个包记录其 `resolved_by` 父包 — 根级包显示 `(root)`，传递依赖显示父包名称

[Full format reference →](docs/agent-lock.md)

---

## Source URL Schemes / 源地址协议

| Scheme / 协议 | Format / 格式 | Example / 示例 |
|--------|--------|---------|
| `github` | `github:owner/repo[/path]` | `github:myorg/code-review-skill` |
| `npm` | `npm:@scope/package` | `npm:@anthropic/claude-code` |
| `local` | `local:./relative/path` | `local:./vendor/my-skill` |
| `git` | `git:https://repo-url` | `git:https://github.com/org/repo.git` |
| `file` | `file:/absolute/path` | `file:/home/user/packages/skill.tar.gz` |
| `url` | `url:https://download-url` | `url:https://example.com/pkg.tar.gz` |

---

## Architecture / 架构

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
├── pkg/resolver/          → PubGrub dependency solver (CDCL algorithm)
│   ├── types.go           → Shared types + manifest extraction
│   ├── pubgrub_resolver.go → PubGrubResolver (DependencyResolver impl)
│   ├── pubgrub_adapter.go → SourceHandler → pubgrub Source adapter
│   ├── pubgrub_err.go     → Conflict error reporting
│   └── namespace.go       → Type-safe name encoding
├── pkg/store/             → Content-addressed store (symlink dedup, GC, cache)
└── pkg/errors/            → Structured errors (User/System/Network)
```

---

## Install / 安装

```bash
# Shell script (recommended) / Shell 脚本（推荐）
curl -fsSL https://raw.githubusercontent.com/7emotions/agentenv/main/install.sh | sh

# Go install / Go 安装
go install github.com/7emotions/agentenv/cmd/agentenv@latest

# Homebrew
brew tap 7emotions/homebrew-agentenv
brew install agentenv

# Build from source / 源码编译
git clone https://github.com/7emotions/agentenv.git
cd agentenv && make build
```

**Platforms**: macOS (darwin) + Linux, amd64 + arm64.  
**Requirements**: Go 1.23+ (for source builds only — binary releases are self-contained).

> 中文：支持 macOS 和 Linux 的 amd64/arm64 平台。二进制发布版无需依赖，源码编译需要 Go 1.23+。

---

## Design Guardrails / 设计原则

- **No telemetry** — zero analytics, zero tracking, zero phone-home / 无遥测 — 无分析、无追踪、无回传
- **No plugins** — all adapters registered at compile time via `init()` / 无插件机制 — 所有适配器在编译时通过 `init()` 注册
- **PubGrub CDCL resolver** — handles complex dependency graphs with conflict learning / PubGrub CDCL 解析器 — 处理复杂依赖图与冲突学习
- **No Windows** — deliberate scope boundary / 不支持 Windows — 有意的范围边界
- **No breaking JSONC** — OpenCode config comments are preserved, not stripped / 不破坏 JSONC — OpenCode 配置注释完整保留

---

## License / 许可证

MIT — see [LICENSE](LICENSE). / 详见 [LICENSE](LICENSE) 文件。