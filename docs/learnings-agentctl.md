# Learnings from `iheanyi/agentctl`

> 调研日期：2026-05-24 | 仓库：[iheanyi/agentctl](https://github.com/iheanyi/agentctl)

## 核心定位差异

| | agentctl | agentenv |
|---|---|---|
| **定位** | 配置同步器 | 包管理器 |
| **类比** | `kubectl apply` for agent configs | `conda` / `npm` for agent packages |
| **核心操作** | sync config entries across tools | resolve → fetch → install → activate |
| **配置格式** | 单一 JSON 文件 | YAML manifest + JSON lockfile |
| **隔离** | global + project-local scope | isolated environments |
| **依赖解析** | 无 | PubGrub CDCL |
| **架构复杂度** | ~15k 行（含 6k TUI） | 相对紧凑（专注核心逻辑） |

两者是**不同抽象层**的工具，互补而非替代。

---

## 架构对比

### 适配器接口设计

**agentctl** — 可组合接口（4 base + 6 optional）：
```go
// Base: 每个适配器必须实现的
type Adapter interface {
    Name() string
    Detect() (bool, error)           // 检测工具是否已安装
    ConfigPath() string
    SupportedResources() []ResourceType
}

// 可选子接口——通过类型断言判断能力
type ServerAdapter interface { Adapter; ReadServers(); WriteServers() }
type CommandsAdapter interface { Adapter; ReadCommands(); WriteCommands() }
type RulesAdapter interface { Adapter; ReadRules(); WriteRules() }
type SkillsAdapter interface { Adapter; ReadSkills(); WriteSkills() }
type AgentsAdapter interface { Adapter; ReadAgents(); WriteAgents() }

// 辅助函数
func AsServerAdapter(a Adapter) (ServerAdapter, bool) { ... }
```

**agentenv** — 统一接口（16 方法）：
```go
type AgentAdapter interface {
    Name() string
    DisplayName() string
    // Skills
    InstallSkill, RemoveSkill, ListSkills, GetSkillBasePath
    // Manifest
    ReadManifest, WriteManifest
    // Agents
    InstallAgent, RemoveAgent, ListAgents, GetAgentBasePath
    // MCP
    InstallMCP, RemoveMCP, ReadMCPConfig, WriteMCPConfig
    // Safety
    Backup, Restore
}
```

**结论**：agentenv 的 16 方法接口对包管理器是正确的设计——你需要的对称性（Install/Remove/List）agentctl 不需要。但当适配器扩展到 8+ 时，考虑拆分为可选子接口以防 stub 泛滥。

### 适配器数量

| agentctl（11 个） | agentenv（4 个） |
|---|---|
| Claude Code, Claude Desktop | Claude Code |
| Cursor | Cursor |
| Codex | Codex |
| OpenCode | OpenCode |
| Cline, Windsurf, Zed, Continue, Gemini, Copilot | — |

agentctl 多出来的适配器（Cline, Windsurf, Zed, Continue, Gemini, Copilot）属于编辑器/桌面应用范畴，你的 4 个都是 AI coding CLI 工具——这是合理的 scoping。

### Managed artifact tracking

**agentctl** — 两种追踪策略：

1. **内联标记**（大多数工具）：在目标 JSON 中插入 `"_managedBy": "agentctl"` 字段
2. **外部状态文件**（OpenCode）：在 `~/.config/agentctl/sync-state.json` 独立追踪

```go
// 原因：OpenCode 的 JSON schema 不接受未知字段
// 所以用外部文件替代内联标记
```

**agentenv** — 每个环境有自己的 `Manifest` 文件（JSON），记录 skills/MCP/agents：

```go
type Manifest struct {
    Version    int                     `json:"version"`
    EnvName    string                  `json:"env_name"`
    Skills     map[string]ManifestItem `json:"skills,omitempty"`
    MCPServers map[string]ManifestItem `json:"mcp_servers,omitempty"`
    Agents     map[string]ManifestItem `json:"agents,omitempty"`
}
```

**结论**：agentenv 的 Manifest 模式更好——独立、可审计、不污染框架原生配置。agentctl 的内联标记是无奈之举（需要反向追踪哪些是它管理的）。

---

## ✅ 值得借鉴的模式

### 1. Golden 测试基础设施 ⭐⭐⭐⭐⭐

agentctl 的 golden test 是最值得照搬的。你的适配器将 `UniversalMCPConfig` 翻译到各框架原生格式——这是 golden test 的完美场景。

**现有结构**：
```
pkg/sync/golden_test.go           # 764 行，9 个测试函数
testdata/golden/<adapter>/<case>.golden.json
testdata/golden.go                 # 比较实用函数
testdata/diff_viewer.go            # 彩色 diff 输出
```

**对 agentenv 的建议实现路径**：
```
pkg/adapter/golden_test.go                    # 新建
testdata/golden/
├── claude-code/
│   ├── mcp_write_basic.golden.json
│   └── mcp_write_multiple.golden.json
├── opencode/
│   ├── mcp_write_basic.golden.json
│   └── mcp_write_headers.golden.json
├── cursor/
│   └── ...
└── codex/
    └── ...
```

```go
// pkg/adapter/golden_test.go 示例
func TestGoldens(t *testing.T) {
    tests := []struct {
        adapter string
        input   *types.UniversalMCPConfig
        golden  string
    }{
        {
            adapter: "opencode",
            input:   basicMCPServers(),
            golden:  "testdata/golden/opencode/mcp_write_basic.golden.json",
        },
        // ...
    }
    for _, tt := range tests {
        t.Run(tt.adapter+"/"+filepath.Base(tt.golden), func(t *testing.T) {
            a := createAdapterForTest(tt.adapter)
            tmp := t.TempDir()
            // 写入 + 比较 golden
            a.WriteMCPConfig(ctx, tt.input)
            got := readWrittenConfig(t, tmp, tt.adapter)
            golden.Assert(t, got, tt.golden)
        })
    }
}
```

环境变量控制更新：`UPDATE_GOLDENS=1 make test`

### 2. 作用域系统（Scope） ⭐⭐⭐⭐

```go
type Scope string  // "local", "global", "all"
// 别名: "project" → "local", "user" → "global"
func (s Scope) ShortString() string {
    switch s {
    case ScopeLocal:  return "[L]"
    case ScopeGlobal: return "[G]"
    default:          return ""
    }
}
```

**对 agentenv 的建议**：
- 项目根目录放置 `.agentenv` 文件，内容：`environment: my-env`
- `agentenv activate` 无参数时自动向上查找 `.agentenv` 并绑定
- `agentenv list` 和 `agentenv info` 中显示本地绑定的环境

### 3. 输出符号系统 ⭐⭐⭐⭐

```go
type Writer struct {
    Out, Err io.Writer
    IsaTTY   bool
}
func (w *Writer) Success(f string, a ...interface{}) { fmt.Fprintf(w.Out, "✓ "+f+"\n", a...) }
func (w *Writer) Error(f string, a ...interface{})   { fmt.Fprintf(w.Err, "✗ "+f+"\n", a...) }
func (w *Writer) Info(f string, a ...interface{})    { fmt.Fprintf(w.Out, "• "+f+"\n", a...) }
func (w *Writer) Warning(f string, a ...interface{}) { fmt.Fprintf(w.Out, "⚠ "+f+"\n", a...) }
```

agentenv 已经有 `jsonOutput` 持久 flag + `writeJSON`。补充非 JSON 模式的符号系统即可。

### 4. Doctor 命令 ⭐⭐⭐⭐

`agentctl doctor` 并行检查运行时 + 工具 + 配置：

```go
// 并行检查运行时
var wg sync.WaitGroup
for _, rt := range runtimes {
    wg.Add(1)
    go func(cmd string, args []string) {
        defer wg.Done()
        version, err := getVersion(cmd, args)
        results[i] = result{version, err}
    }(rt.command, rt.args)
}
wg.Wait()
```

**对 agentenv 的建议——`agentenv doctor` 检查项**：

```
Configuration:
  ✓ ~/.agentenv/envs/ (3 environments)
  ✓ agent.yaml: valid
  ⚠ agent.lock: outdated (run 'agentenv lock')

Runtimes:
  ✓ Go: go1.23.5 (required: ≥1.23)
  ✓ git: 2.43.0
  ✓ npm: 10.9.0 (for npm: source scheme)
  - uv: not found (optional)

Adapters:
  ✓ claude-code: ~/.claude/ detected
  ✓ opencode: ~/.config/opencode/ detected
  - cursor: not installed
  - codex: not installed

Store:
  ✓ ~/.agentenv/store/ (12 packages, 45MB)
  ✓ No orphaned packages
  ⚠ Cache: 3 stale entries (run 'agentenv gc')

Source Connectivity:
  ✓ github: (reachable)
  ✓ npm: (reachable)
```

### 5. TTY 检测 + 默认信息面板 ⭐⭐⭐

```go
func runRoot(cmd *cobra.Command, args []string) error {
    if isatty.IsTerminal(os.Stdout.Fd()) {
        return showDashboard()   // 自定义轻量信息面板
    }
    return cmd.Help()
}
```

agentctl 在这里启动完整 TUI。对 agentenv，一个轻量文本面板更合适：

```
$ agentenv
  agentenv v0.2.0  │  3 environments, 0 active

  Environments:
  • my-env        │  claude-code   │  3 packages   [idle]
  • my-opencode   │  opencode      │  5 packages   [active: 2h ago]
  • ci-env        │  cursor        │  1 package    [idle]

  Run 'agentenv create <name>' to start a new environment
  Run 'agentenv activate <name>' to switch
```

**不需要 TUI**——纯文本、零依赖、高信息密度。

### 6. 下一步提示（Next-Step Hints） ⭐⭐⭐

每个操作后提示用户可以做的下一件事：

```go
// agentctl 的模式
fmt.Println("✓ Added server 'figma' to local config")
fmt.Println("  Run 'agentctl sync' to update your tools")
```

agentenv 的工作流线天然适合这个模式：

| 当前操作 | 提示 |
|---------|------|
| `create` | `Run 'agentenv add <type> <name> --source ...' to add packages` |
| `add` | `Run 'agentenv lock' to resolve dependencies` |
| `lock` | `Run 'agentenv install' to fetch packages` |
| `install` | `Run 'agentenv activate <env>' to activate` |
| `activate` | `Run 'agentenv deactivate' when done` |

### 7. 内嵌注册表 ⭐⭐

```go
//go:embed aliases.json
var embeddedAliases embed.FS
```

agentctl 把 20+ 常用 MCP server 别名编译进二进制。

**对 agentenv 的建议**：`~/.agentenv/registry/` 目录（或内嵌 JSON），内容：

```json
{
  "mcp": {
    "figma": {
      "source": "npm:@anthropic/mcp-figma",
      "command": "npx",
      "args": ["-y", "@anthropic/mcp-figma"],
      "description": "Figma MCP server"
    },
    "filesystem": {
      "source": "npm:@modelcontextprotocol/server-filesystem",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem"]
    }
  }
}
```

然后用户只需：`agentenv add mcp figma`（自动解析 source + command + args）。

### 8. Dry-run 模式 ⭐⭐

```go
syncCmd.Flags().BoolVarP(&syncDryRun, "dry-run", "n", false, "Preview changes without applying")
```

`agentenv lock` 输出 `[+]/[-]/[~]` diff 预览，`agentenv activate --dry-run` 显示将写入框架的配置而不实际修改。降低「我不知道这会改什么」的恐惧。

---

## ⚠️ 不应借鉴的（已知缺陷）

| agentctl 的问题 | 为什么 agentenv 不要学 |
|---|---|
| **单文件 6010 行 TUI** | 难以维护、测试、review。agentenv 保持命令行工具定位即可 |
| **注册表无并发保护** | `map[string]Adapter` 无 mutex。agentenv 的 `sync.RWMutex` 是正确的 |
| **配置路径硬编码** | 每个适配器 `ConfigPath()` 写死路径，测试需 wrapper。agentenv 的配置模型更灵活 |
| **无依赖解析** | 纯粹的 config sync，无传递依赖概念。agentenv 的 PubGrub 是核心优势 |
| **无结构化错误** | 只用 `fmt.Errorf`。agentenv 的 `UserError/SystemError/NetworkError` 更好 |
| **无环境隔离** | 只能管理 global + project 两层。agentenv 的隔离 environments 更强 |
| **List 命令 757 行** | JSON 和文本输出逻辑交织。agentenv 的 `jsonOutput` + `writeJSON` 模式分离更清晰 |
| **无 debug 标志** | 用户看不到堆栈。agentenv 的 `--debug` 提供完整的 `runtime/debug.Stack()` |

---

## 优先级路线图

### P0 — 立即做（低投入，高回报）

1. **Golden 测试** — `pkg/adapter/golden_test.go` + `testdata/golden/`
   - 验证各适配器的 MCP config 翻译正确性
   - 环境变量 `UPDATE_GOLDENS=1` 控制更新
   - 投入：1-2 天

2. **输出符号统一** — 在非 JSON 输出中使用 ✓✗•⚠
   - 当前已有 `jsonOutput` 双模式，只需统一文本侧的符号
   - 投入：半天

### P1 — 短期（中等投入，显著 UX 提升）

3. **`agentenv doctor`** — 并行检查 runtime + adapters + store + connectivity
   - 投入：1-2 天

4. **无参运行信息面板** — `isatty` 检测 + 轻量文本面板
   - 投入：半天

5. **下一步提示** — 每个命令成功后输出下一条建议
   - 投入：半天

6. **项目本地 `.agentenv` 绑定** — 自动从当前目录查找环境绑定
   - 投入：半天

### P2 — 长期（架构演进）

7. **`agentenv import`** — 从现存 `.claude/`/`.cursor/` 配置迁移
   - 借鉴 agentctl 的 `discovery` 包 + scanner 模式
   - 投入：2-3 天

8. **内嵌常用包注册表** — `//go:embed` 常用 MCP server 别名
   - 投入：1 天

9. **适配器接口拆分** — 当扩展到 8+ 适配器时，考虑可选子接口
   - 投入：重构 2-3 天

---

## agentctl 命令树速查

供设计 agentenv 子命令时参考：

```
agentctl     # 无参 → TUI
├── add/install     # 添加 MCP server（交互式或 flags）
├── remove/rm       # 移除
├── list/ls         # 列出资源（支持 --type, --scope, --native, --profile）
├── sync            # 同步到所有工具（--tool, --dry-run, --verbose）
├── alias           # 管理 server 别名（list, add, remove, manage）
├── profile         # 管理配置 profile（create, switch, export, import）
├── init            # 交互式初始化
├── doctor          # 健康检查（--verbose）
├── validate        # 配置校验
├── status          # 资源状态总览
├── search          # 搜索 MCP server
├── new             # 从模板创建资源（command, rule, skill, agent）
├── import          # 从工具原生配置导入
├── update          # 更新 MCP server
├── secret          # keychain 密钥管理
├── test            # 测试 MCP server 连接
├── config          # 查看/编辑配置（show, get, set, edit, path）
├── ui              # 启动 TUI
├── daemon          # 后台守护进程
├── backup          # 配置备份管理
├── copy            # 在 scope 间复制资源
├── skill           # 管理 skills（show, edit, add, remove, copy）
└── version         # 版本信息
```

---

## 测试模式速查

agentctl 的测试文件组织，作为 agentenv golden test 实施参考：

| 文件 | 行数 | 用途 |
|------|------|------|
| `pkg/sync/golden_test.go` | 764 | 9 个 golden test 函数 |
| `pkg/sync/adapter_test.go` | — | 注册表 + 适配器单元测试 |
| `internal/tui/golden_test.go` | — | TUI 截图 golden tests |
| `testdata/golden/` | — | 每个适配器/场景的 `.golden.json` |
| `testdata/golden.go` | — | 比较 + diff 工具函数 |
| `testdata/diff_viewer.go` | — | 彩色 diff 输出 |

CI 中有专门的 `golden` job：
```yaml
golden:
  runs-on: ubuntu-latest
  steps:
    - run: go test ./... -run Golden
    - run: git status --porcelain   # 检测过时的 golden 文件
      # 如果 git status 有变动 → 失败（提醒运行 UPDATE_GOLDENS=1）
```

---

*本文档基于 2026-05-24 对 `iheanyi/agentctl`（commit dd74288）的完整代码阅读。*
