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
  <a href="#english">English</a> · <a href="#中文">中文</a>
</p>

---

<a id="english"></a>
## English

### Quick Start

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/7emotions/agentenv/main/install.sh | sh

# Create a workspace with a specific framework
agentenv create my-env --agent opencode

# Add packages from anywhere
agentenv add skill code-reviewer --source github:myorg/reviewer --version "^0.5"
agentenv add mcp filesystem --source npm:@modelcontextprotocol/server-filesystem \
  --command npx --arg -y --arg @modelcontextprotocol/server-filesystem --arg /tmp

# Resolve: PubGrub finds the best version for every package
agentenv lock

# Fetch packages into the store
agentenv install

# Activate: inject into your framework
agentenv activate my-env

# Done — your agent is ready to use all packages
agentenv deactivate  # switch back when done
```

---

### What is agentenv?

agentenv is an **environment manager for AI agents** — think conda, but for skills, MCP servers, agents, tools, hooks, and prompts.

| Problem | agentenv |
|---------|----------|
| Working on multiple projects with conflicting agent setups | Isolated environments under `~/.agentenv/` |
| Manually installing skills and MCP servers per project | Declarative `agent.yaml` + lockfile |
| "Works on my machine" across Claude Code, OpenCode, Cursor | Reproducible, version-pinned, cross-framework |
| Losing track of what's installed | Content-addressed store with dedup and garbage collection |
| Switching between Claude Code and OpenCode | One tool, 4 frameworks, same workflow |

---

### Features

- **6 package types** — skill, mcp, agent, tool, hook, prompt. Mix freely in one environment.
- **4 framework adapters** — Claude Code, OpenCode, Cursor, Codex. All 16 methods per adapter.
- **6 source schemes** — `github:`, `npm:`, `local:`, `git:`, `file:`, `url:`. Pull packages from anywhere.
- **PubGrub dependency resolution** — CDCL version solving with transitive dependency discovery, constraint checking, and clear conflict reporting
- **Deterministic lockfiles** — agent.yaml → agent.lock, semver constraints, transitive deps, SHA-256 integrity.
- **Content-addressed store** — symlink deduplication, cross-filesystem copy fallback, garbage collection.
- **Transactional activation** — backup → validate → apply → rollback on failure. No partial state.
- **Structured errors** — every error tells you what went wrong, why, and how to fix it.
- **Zero telemetry** — no analytics, no phone-home, no tracking. Period.

---

### Commands

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

### Usage

#### Creating an environment

Create a new, empty environment with a name of your choice:

```bash
# Basic: create with default framework (claude-code)
agentenv create my-env

# Specify a framework
agentenv create --agent opencode my-opencode-env

# Create in the current directory (uses folder name)
agentenv create .
```

Environments live under `~/.agentenv/environments/` with a generated `agent.yaml` manifest. You can inspect and edit this file directly.

---

#### Adding packages

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

Each `add` command writes to `agent.yaml` immediately. You can verify with `agentenv list-packages`.

---

#### Resolving dependencies

After adding packages, resolve their dependency graph:

```bash
agentenv lock
```

`agentenv lock` runs the **PubGrub CDCL resolver** to find a compatible set of versions across all packages and their transitive dependencies. It produces `agent.lock` — a deterministic, SHA-256-pinned snapshot.

If conflicts are found, the resolver reports exactly which constraints conflict and why:

```
error: conflict detected
  code-reviewer@>=1.0 requires lint-helper@^2.0
  my-other-skill@>=0.5 requires lint-helper@^1.0
  lint-helper@^2.0 and lint-helper@^1.0 are incompatible
```

---

#### Installing and activating

Once locked, download all packages and activate the environment:

```bash
# Fetch all packages into the local content-addressed store
agentenv install

# Activate — installs everything into your framework's config
agentenv activate my-env

# Deactivate — removes from framework config
agentenv deactivate
```

Activation is **transactional**: it backs up your framework config, validates the new config, applies changes, and rolls back on failure. You never get partial state.

---

#### Full workflow example

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

---

#### JSON output

All commands support `--json` for machine-readable output, perfect for scripting and tool integration:

```bash
# Get environment info as JSON
agentenv info my-env --json | jq .

# List all environments
agentenv list --json | jq '.[].name'

# List packages with versions
agentenv list-packages --json | jq '[.[] | {name, type, resolved}]'
```

---

#### Framework selection

agentenv supports 4 AI coding frameworks. Choose the right one for your workflow:

```bash
# List available adapters
agentenv list-adapters

# Create an environment for a specific framework
agentenv create --agent opencode my-opencode-env
agentenv create --agent cursor my-cursor-env
agentenv create --agent codex my-codex-env
```

Each adapter knows where its framework stores config, what format it uses, and how to register skills, MCP servers, and agents. Switching frameworks doesn't change your workflow — same commands, same `agent.yaml`.

| Use Case | Recommended Framework |
|----------|---------------------|
| General-purpose AI coding | `claude-code` |
| Open-source, configurable | `opencode` |
| IDE-integrated | `cursor` |
| Lightweight terminal | `codex` |

---

### Supported Frameworks

| Framework | Adapter | Config Path | MCP Format |
|-----------|---------|-------------|------------|
| **Claude Code** | `claude-code` | `~/.claude/` | `.mcp.json` |
| **OpenCode** | `opencode` | `~/.config/opencode/` | Inline `opencode.jsonc` (JSONC) |
| **Cursor** | `cursor` | `~/.cursor/` | Framework-specific |
| **Codex** | `codex` | N/A | TOML-based (partial support) |

Each adapter manages skills, MCP servers, agents, and manifests — with automatic backup before modification and rollback on failure.

---

### agent.yaml — Environment Definition

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

### agent.lock — Deterministic Lockfile

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

- Packages are sorted by `(type, name)` for deterministic output
- v1 lockfiles auto-migrate to v3 on read — no data loss
- `agentenv lock` always writes v3 format
- Each package tracks its `resolved_by` parent — root packages show `(root)`, transitive deps show their parent's name

[Full format reference →](docs/agent-lock.md)

---

### Source URL Schemes

| Scheme | Format | Example |
|--------|--------|---------|
| `github` | `github:owner/repo[/path]` | `github:myorg/code-review-skill` |
| `npm` | `npm:@scope/package` | `npm:@anthropic/claude-code` |
| `local` | `local:./relative/path` | `local:./vendor/my-skill` |
| `git` | `git:https://repo-url` | `git:https://github.com/org/repo.git` |
| `file` | `file:/absolute/path` | `file:/home/user/packages/skill.tar.gz` |
| `url` | `url:https://download-url` | `url:https://example.com/pkg.tar.gz` |

---

### Architecture

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

### Install

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

### Design Guardrails

- **No telemetry** — zero analytics, zero tracking, zero phone-home
- **No plugins** — all adapters registered at compile time via `init()`
- **PubGrub CDCL resolver** — handles complex dependency graphs with conflict learning
- **No Windows** — deliberate scope boundary
- **No breaking JSONC** — OpenCode config comments are preserved, not stripped

---

### AgentEnv Pro

AgentEnv is free and open source. **[AgentEnv Pro](docs/pro.md)** adds three features for teams and power users:

| Feature | Description |
|---------|-------------|
| `agentenv sync` | Export/import environments as portable bundles — swap machines without rebuilding |
| `agentenv bundle` | Package environments for team distribution — share your exact setup in one command |
| `agentenv update --check` | Auto-check package updates with changelogs |

**¥99 / $19 lifetime** (one-time, not subscription). [Get Pro →](https://gumroad.com/l/agentenv-pro)

---

### License

MIT — see [LICENSE](LICENSE).

---

<a id="中文"></a>
## 中文

### 快速开始

```bash
# 安装
curl -fsSL https://raw.githubusercontent.com/7emotions/agentenv/main/install.sh | sh

# 创建指定框架的环境
agentenv create my-env --agent opencode

# 从各种源添加包
agentenv add skill code-reviewer --source github:myorg/reviewer --version "^0.5"
agentenv add mcp filesystem --source npm:@modelcontextprotocol/server-filesystem \
  --command npx --arg -y --arg @modelcontextprotocol/server-filesystem --arg /tmp

# 依赖解析：PubGrub 为每个包找到最佳版本
agentenv lock

# 安装到本地存储
agentenv install

# 激活：注入到框架配置
agentenv activate my-env

# 完成 — 智能体已准备好使用所有包
agentenv deactivate  # 用完后停用
```

一条命令安装，几步创建。支持 6 种源（GitHub、npm、本地等）、6 种包类型（技能、MCP、智能体等）、4 种框架适配器。依赖解析由 PubGrub 引擎自动完成。

---

### 什么是 AgentEnv？

agentenv 是 **AI 智能体的环境管理器** — 就像 conda，但管理的是技能、MCP 服务器、智能体、工具、钩子和提示词。

| 问题 | agentenv |
|---------|----------|
| 多个项目使用不同的智能体配置，互相冲突 | `~/.agentenv/` 下的隔离环境 |
| 每个项目手动安装技能和 MCP 服务器 | 声明式 `agent.yaml` + 锁文件 |
| 在 Claude Code、OpenCode、Cursor 之间"我机器上能跑"的问题 | 可重现、版本锁定、跨框架 |
| 记不清装了什么 | 内容寻址存储，去重和垃圾回收 |
| 在 Claude Code 和 OpenCode 之间切换 | 一个工具，4 个框架，同一套工作流 |

---

### 特性

- **6 种包类型** — skill、mcp、agent、tool、hook、prompt，可在同一环境中混合使用
- **4 个框架适配器** — Claude Code、OpenCode、Cursor、Codex，每个适配器实现全部 16 个方法
- **6 种源地址协议** — 可从 GitHub、npm、本地、Git 仓库、文件路径、URL 任意来源安装包
- **PubGrub 依赖解析** — CDCL 版本求解，支持传递依赖发现、约束检查和清晰的冲突报告
- **确定性锁文件** — 从 agent.yaml 生成 agent.lock，语义化版本约束、传递依赖、SHA-256 完整性校验
- **内容寻址存储** — 符号链接去重、跨文件系统复制回退、垃圾回收
- **事务性激活** — 备份 → 校验 → 应用 → 失败回滚，不会留下半残状态
- **结构化错误** — 每个错误都说明原因、影响和修复方法
- **零遥测** — 无分析、无回传、无追踪。这就是底线

---

### 命令

| 命令 | 说明 |
|---------|-------------|
| `create` | 创建新环境 |
| `add` | 添加包到环境 |
| `remove` | 移除包 |
| `lock` | 解析依赖并生成锁文件 |
| `install` | 下载包到本地存储 |
| `activate` | 激活环境（安装到框架） |
| `deactivate` | 停用环境并清理 |
| `list` | 列出所有环境 |
| `list-packages` | 列出环境中的包 |
| `list-adapters` | 显示可用框架适配器 |
| `info` | 查看环境详情 |
| `delete` | 删除环境 |
| `completion` | 生成 shell 补全脚本 |
| `init` | 生成 shell 初始化脚本 |

所有命令都支持 `--json` 输出机器可读的结果，以及 `--debug` 查看堆栈信息。

```bash
# 创建指定框架的环境
agentenv create --agent opencode my-opencode-env

# 查看所有可用框架
agentenv list-adapters
# claude-code  │  Claude Code
# opencode     │  OpenCode
# cursor       │  Cursor
# codex        │  Codex

# JSON 输出
agentenv info my-env --json | jq .
```

---

### 使用指南

#### 创建环境

创建一个新的空环境，可以指定框架：

```bash
# 基本：创建默认框架（claude-code）的环境
agentenv create my-env

# 指定框架
agentenv create --agent opencode my-opencode-env

# 在当前目录创建（使用文件夹名作为环境名）
agentenv create .
```

环境存储在 `~/.agentenv/environments/` 目录下，每个环境包含一个 `agent.yaml` 清单文件，可直接编辑。

---

#### 添加包

从 GitHub、npm、本地路径等来源添加技能、MCP 服务器、智能体、工具、钩子或提示词：

```bash
# 从 GitHub 添加技能
agentenv add skill code-reviewer --source github:myorg/code-review-skill

# 带版本约束添加技能
agentenv add skill code-reviewer --source github:myorg/code-review-skill --version ">=0.5"

# 从 npm 添加 MCP 服务器
agentenv add mcp filesystem \
  --source npm:@modelcontextprotocol/server-filesystem \
  --command npx \
  --arg -y \
  --arg @modelcontextprotocol/server-filesystem \
  --arg /tmp

# 开发中添加本地技能
agentenv add skill my-custom-tool --source local:./vendor/my-tool
```

每次 `add` 命令会立即写入 `agent.yaml`，可用 `agentenv list-packages` 验证。

---

#### 依赖解析

添加包之后，解析它们的依赖关系图：

```bash
agentenv lock
```

`agentenv lock` 使用 **PubGrub CDCL 解析器**，为所有包及其传递依赖找到一组兼容的版本。输出 `agent.lock` 锁文件，包含 SHA-256 完整性校验。

如果发现版本冲突，解析器会精确报告冲突的依赖关系和版本约束：

```
error: conflict detected
  code-reviewer@>=1.0 requires lint-helper@^2.0
  my-other-skill@>=0.5 requires lint-helper@^1.0
  lint-helper@^2.0 and lint-helper@^1.0 are incompatible
```

---

#### 安装和激活

锁定依赖后，下载所有包并激活环境：

```bash
# 将所有解析后的包下载到本地内容寻址存储
agentenv install

# 激活 — 将环境注册到当前框架的配置中
agentenv activate my-env

# 停用 — 从框架配置中移除
agentenv deactivate
```

激活是**事务性**的 — 备份框架配置、校验新配置、应用变更、失败自动回滚，永远不会留下不完整的配置状态。

---

#### 完整工作流程

端到端演示：从零开始到包含多个包的运行环境。

```bash
# 1. 创建项目环境
cd ~/projects/my-ai-project
agentenv create .

# 2. 添加不同类型的包
agentenv add skill code-reviewer --source github:myorg/code-review-skill
agentenv add mcp filesystem \
  --source npm:@modelcontextprotocol/server-filesystem \
  --command npx --arg -y \
  --arg @modelcontextprotocol/server-filesystem \
  --arg /tmp/projects
agentenv add agent my-helper --source github:myorg/helper-agent

# 3. 查看声明文件
cat agent.yaml

# 4. 解析依赖
agentenv lock

# 5. 查看锁文件
cat agent.lock

# 6. 下载包
agentenv install

# 7. 激活（安装到框架）
agentenv activate my-ai-project

# 8. 验证
agentenv list-packages
agentenv info my-ai-project

# 9. 完成后停用
agentenv deactivate
```

整个过程声明式、可重现。

---

#### JSON 输出

所有命令均支持 `--json` 输出，便于脚本集成：

```bash
# 以 JSON 格式获取环境信息
agentenv info my-env --json | jq .

# 列出所有环境
agentenv list --json | jq '.[].name'

# 列出包及其版本
agentenv list-packages --json | jq '[.[] | {name, type, resolved}]'
```

结合 `jq` 可以灵活提取所需信息。

---

#### 框架选择

agentenv 支持 4 种 AI 编码框架，选择适合你工作流的一款：

```bash
# 列出可用适配器
agentenv list-adapters

# 为特定框架创建环境
agentenv create --agent opencode my-opencode-env
agentenv create --agent cursor my-cursor-env
agentenv create --agent codex my-codex-env
```

每个适配器知道对应框架的配置路径和格式。切换框架不影响你的工作流 — 命令不变，agent.yaml 不变。

| 使用场景 | 推荐框架 |
|----------|---------------------|
| 通用 AI 编码 | `claude-code` |
| 开源、可配置 | `opencode` |
| IDE 集成 | `cursor` |
| 轻量终端 | `codex` |

---

### 支持的框架

| 框架 | 适配器 | 配置路径 | MCP 格式 |
|-----------|---------|-------------|------------|
| **Claude Code** | `claude-code` | `~/.claude/` | `.mcp.json` |
| **OpenCode** | `opencode` | `~/.config/opencode/` | 内联 `opencode.jsonc` (JSONC) |
| **Cursor** | `cursor` | `~/.cursor/` | 框架特有格式 |
| **Codex** | `codex` | 无 | TOML 格式（部分支持） |

每个适配器管理技能、MCP 服务器、智能体和清单文件 — 修改前自动备份，失败时自动回滚。

---

### agent.yaml — 环境定义

```yaml
name: my-ai-env
description: "我的个人 AI 开发环境"

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

**包规范字段**（所有 6 种包类型通用）：
- `source`（必填）— 源地址（含协议前缀）
- `version`（可选）— 语义化版本约束，默认为 `*`
- `config`（可选）— 包特定配置

[完整格式参考 →](docs/agent-yaml.md)

---

### agent.lock — 确定性锁文件

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

- 包按 `(类型, 名称)` 排序确保确定性输出
- v1 锁文件读取时自动迁移到 v3，数据不丢失
- `agentenv lock` 始终输出 v3 格式
- 每个包记录其 `resolved_by` 父包 — 根级包显示 `(root)`，传递依赖显示父包名称

[完整格式参考 →](docs/agent-lock.md)

---

### 源地址协议

| 协议 | 格式 | 示例 |
|--------|--------|---------|
| `github` | `github:owner/repo[/path]` | `github:myorg/code-review-skill` |
| `npm` | `npm:@scope/package` | `npm:@anthropic/claude-code` |
| `local` | `local:./relative/path` | `local:./vendor/my-skill` |
| `git` | `git:https://repo-url` | `git:https://github.com/org/repo.git` |
| `file` | `file:/absolute/path` | `file:/home/user/packages/skill.tar.gz` |
| `url` | `url:https://download-url` | `url:https://example.com/pkg.tar.gz` |

---

### 架构

```
agentenv CLI (cobra)
├── cmd/agentenv/          → CLI 命令、激活/停用
├── pkg/adapter/           → 框架适配器（4 个 × 16 个方法）
│   ├── interface.go       → AgentAdapter 接口约定
│   ├── registry.go        → 线程安全注册表（sync.RWMutex）
│   ├── jsonc.go           → JSONC 注释保留工具
│   └── mcp.go             → 通用 MCP 配置
├── pkg/types/             → 核心数据类型
│   ├── source.go          → SourceURL（6 种协议解析器）
│   ├── package.go         → PackageType、PackageDependency（v2 source 字段）
│   ├── environment.go     → Environment（6 个分区映射）
│   └── lockfile.go        → 锁文件 v1→v2 迁移
├── pkg/parser/            → YAML/JSON 解析器
├── pkg/envfile/           → agent.yaml 读写/校验/合并
├── pkg/source/            → 源获取器（github、npm、local、git）
├── pkg/resolver/          → PubGrub 依赖求解器（CDCL 算法）
│   ├── types.go           → 共享类型 + 清单提取
│   ├── pubgrub_resolver.go → PubGrubResolver（DependencyResolver 实现）
│   ├── pubgrub_adapter.go → SourceHandler → pubgrub Source 适配器
│   ├── pubgrub_err.go     → 冲突错误报告
│   └── namespace.go       → 类型安全名称编码
├── pkg/store/             → 内容寻址存储（符号链接去重、GC、缓存）
└── pkg/errors/            → 结构化错误（用户/系统/网络）
```

---

### 安装

```bash
# Shell 脚本（推荐）
curl -fsSL https://raw.githubusercontent.com/7emotions/agentenv/main/install.sh | sh

# Go 安装
go install github.com/7emotions/agentenv/cmd/agentenv@latest

# Homebrew
brew tap 7emotions/homebrew-agentenv
brew install agentenv

# 源码编译
git clone https://github.com/7emotions/agentenv.git
cd agentenv && make build
```

**支持平台**：macOS（darwin）+ Linux，amd64 + arm64。  
**依赖要求**：Go 1.23+（仅源码编译需要 — 二进制发布版无需依赖）。

---

### 设计原则

- **无遥测** — 无分析、无追踪、无回传
- **无插件机制** — 所有适配器在编译时通过 `init()` 注册
- **PubGrub CDCL 解析器** — 处理复杂依赖图与冲突学习
- **不支持 Windows** — 有意的范围边界
- **不破坏 JSONC** — OpenCode 配置注释完整保留

---

### AgentEnv Pro 专业版

AgentEnv 开源免费。**[AgentEnv Pro](docs/pro.md)** 为团队和重度用户增加三个功能：

| 功能 | 说明 |
|---------|-------------|
| `agentenv sync` | 导出/导入环境为便携包 — 换电脑不用重新配置 |
| `agentenv bundle` | 打包环境供团队分发 — 一条命令共享你的完整配置 |
| `agentenv update --check` | 自动检测包更新并显示 changelog |

**¥99 / $19 永久买断**（一次付费，终身使用）。[获取 Pro →](https://gumroad.com/l/agentenv-pro)

---

### 许可证

MIT 许可证 — 详见 [LICENSE](LICENSE) 文件。
