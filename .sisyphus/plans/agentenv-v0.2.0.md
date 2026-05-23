# agentenv v0.2.0 — 实施计划

## TL;DR

> **快速摘要**: 将 agentenv 从单框架 MVP（仅 Claude Code）扩展为支持 OpenCode、Cursor、Codex 的多框架工具。修复关键竞争条件、错误吞没和解析器源处理问题。重新设计 agentpkg.yaml v2 schema，为依赖项添加 source 字段。
>
> **交付物**:
> - OpenCode / Cursor / Codex 适配器（各 16 个方法）
> - agentpkg.yaml v2 schema（依赖项 source 字段 + 统一 6 种包类型）
> - lockfile v2 格式（含 v1→v2 迁移）
> - 竞争条件修复（sync.RWMutex on adapters map）
> - 上下文传播（移除 nolint:noctx）
> - 错误吞没审计修复（4 处位置）
> - 框架感知的停用（ACTIVE lock 中存储框架名称）
> - 格式文档（agent.yaml + agent.lock）
> - v0.1.1 bugfix 发布（2 个已修复的 bug）
>
> **预估工作量**: 大型
> **并行执行**: YES — 5 waves
> **关键路径**: Task 1 → Task 5 → Task 12 → Task 19 → Task 22 → FINAL

---

## 上下文

### 原始需求
将 agentenv 从 v0.1.0 MVP 扩展为支持多个 AI 代理框架的生产就绪工具。修复审计中发现的竞争条件、错误吞没和架构简化问题。

### 访谈摘要
**关键讨论**:
- **范围**: 审计中所有 4 层（紧急 + 高 + 中 + 低），约 24 个项目
- **适配器策略**: OpenCode 优先，验证后批量 Cursor + Codex
- **agentpkg.yaml v2**: 依赖项完整重新设计，添加 source 字段，统一 6 种包类型
- **lockfile**: 可以破坏向后兼容（MVP 阶段），但需要明确的 v1→v2 迁移路径
- **测试**: 混合 TDD（核心模块：解析器 + 适配器）+ 其他模块 tests-after
- **版本**: v0.1.1 仅 bugfix → v0.2.0 新功能
- **任务粒度**: ~20-25 个任务，混合优先级以实现最大并行度
- **模块路径**: 修正为 github.com/7emotions/agentenv

**研究成果**:
- **OpenCode 配置**: ~/.config/opencode/opencode.jsonc（JSONC 格式，支持注释），MCP 嵌入在 "mcp" 键下（非独立文件），技能为 SKILL.md 文件
- **OpenCode MCP 格式**: "type": "local" + "command" 数组（不同于 Claude Code 的扁平格式）
- **JSONC 风险**: 纯 JSON 解析器会静默剥离注释 — 这是用户信任问题
- **Claude Code 适配器**: 已实现所有 16 个方法作为参考模板
- **覆盖率**: 整体 81%，errors 包 0%，部分函数 0%

### Metis 审查
**已识别的关键差距**（已纳入计划）:
1. **JSONC 注释保留** — #1 技术风险。仅读/写二进制 JSON 会破坏 opencode.jsonc 中的用户注释。需要 JSONC 感知的解析器。
2. **`depSource` 生产存根** — `resolver.go:115` 中的 `"github:test/<name>"` 是泄漏到生产代码中的 MVP 测试存根。必须替换。
3. **框架感知的停用** — `deactivateEnv` 回退到 "claude-code"，将导致先前激活为 OpenCode 时的数据丢失。必须在 ACTIVE lock 中存储框架名称。
4. **Cursor/Codex 研究债务** — 在实现之前不要假设它们与 Claude Code 工作方式相同。
5. **Lockfile v1→v2 迁移路径** — 必须明确：静默覆盖？自动迁移？拒绝？
6. **错误吞没路径** — WalkDir、rollback、GC、restore 都静默丢弃错误。
7. **上下文传播** — `cache.go:41` 使用没有上下文的 `http.Get`，无超时传播。
8. **部分 OpenCode 配置** — 适配器必须优雅处理缺失的 "mcp" 键、缺失的目录和无效的 JSONC 语法。

---

## 工作目标

### 核心目标
交付 agentenv v0.2.0，支持 Claude Code、OpenCode、Cursor 和 Codex，配备 agentpkg.yaml v2 schema、修复的关键错误，以及为所有用户提供完整的格式文档。

### 具体交付物
- `pkg/adapter/opencode.go` — 16 方法的 OpenCode 适配器
- `pkg/adapter/cursor.go` — 16 方法的 Cursor 适配器
- `pkg/adapter/codex.go` — 16 方法的 Codex 适配器
- `pkg/adapter/jsonc.go` — JSONC 注释保留工具
- `pkg/adapter/registry.go` — sync.RWMutex 保护
- `pkg/resolver/resolver.go` — 依赖项的 source 字段传播
- `pkg/types/package.go` — PackageDependency 的 source 字段
- `pkg/parser/agentpkg.go` — agentpkg.yaml v2 解析器
- `pkg/lockfile/generate.go` — lockfile v2 格式 + v1 迁移
- `docs/agent-yaml.md` — agent.yaml 格式文档
- `docs/agent-lock.md` — agent.lock 格式文档
- 错误修复: 4 处 WalkDir/rollback/GC/restore 吞没，2 处上下文传播
- `go test -race` 在 CI 中通过

### 完成定义
- [ ] `agentenv create --agent opencode test && agentenv activate test && agentenv deactivate` 端到端正常
- [ ] `agentenv create --agent cursor test` + 激活正常
- [ ] `agentenv create --agent codex test` + 激活正常
- [ ] OpenCode adapter 在 JSONC 往返中保留注释
- [ ] `go test -race ./...` 通过，0 数据竞争
- [ ] `go vet ./...` 通过（包括 noctx）
- [ ] agentpkg.yaml v2 依赖项带有 source 字段 → resolver 使用它
- [ ] lockfile v1 → v2 迁移对于现有 v1 lockfile 正确工作
- [ ] 框架切换清理旧的适配器状态（~/.claude/ 或 ~/.config/opencode/）
- [ ] 格式文档存在于 docs/ 中
- [ ] v0.1.1 发布标签已创建（仅 bugfix）
- [ ] v0.2.0 发布标签已创建（完整功能）

### 必须包含
- JSONC 注释保留（OpenCode 适配器）
- sync.RWMutex on adapters map
- context.Context 通过 HTTP 调用传播
- 框架名称存储在 ACTIVE lock 文件中
- agentpkg.yaml v2 依赖项中的 source 字段
- Lockfile v1→v2 迁移路径
- docs/ 格式文档

### 禁止包含（护栏）
- 无动态适配器加载（所有适配器通过 init() 注册）
- 无插件系统
- 无 SAT 求解器或 PubGrub（v0.1.0 护栏重新确认）
- 无 Windows 支持
- 无遥测、无电话回拨、无分析
- 无解析器算法变更（仅 source 字段支持）
- 无备份/恢复系统重新设计（仅框架分区）
- 无新的 CLI 命令（无 search/doctor/upgrade/export 除非明确列出）
- 最大依赖深度: 3，最大包数: 50（v0.1.0 重新确认）

---

## 验证策略

### 测试决策
- **基础设施存在**: YES — Go testing + testify + 现有适配器测试模式
- **自动化测试**: 混合 — 核心模块 TDD（解析器、适配器），其他 tests-after
- **框架**: Go 标准 testing + testify + go test -race
- **CI**: GitHub Actions，macOS + Linux 矩阵，包含 -race

### QA 策略
每个任务必须包含使用指定工具类型的代理执行 QA 场景。证据保存到 `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`。

- **前端/UI**: 使用 Playwright — 导航、交互、断言 DOM、截图
- **TUI/CLI**: 使用 interactive_bash (tmux) — 运行命令、发送按键、验证输出
- **API/后端**: 使用 Bash (curl) — 发送请求、断言状态 + 响应字段
- **库/模块**: 使用 Bash (go test) — 运行测试、比较输出

---

## 执行策略

### 并行执行 Waves

```
Wave 1（立即开始 — 关键修复 + 基础）:
├── Task 1: 竞争条件修复 — sync.RWMutex on adapters map [quick]
├── Task 2: 错误吞没审计 — WalkDir, rollback, GC, restore [quick]
├── Task 3: 上下文传播 — 移除 nolint:noctx [quick]
├── Task 4: 模块路径对齐 [quick]
├── Task 5: agentpkg.yaml v2 schema + 解析器 [unspecified-high]
├── Task 6: Lockfile v2 格式 + v1 迁移 [quick]
└── Task 7: 格式文档 [writing]

Wave 2（Wave 1 后 — 适配器基础）:
├── Task 8:  JSONC 工具库 [unspecified-high]
├── Task 9:  OpenCode 适配器：路径 + 身份 + manifest [unspecified-high]
├── Task 10: OpenCode 适配器：MCP 配置 + 格式转换 [unspecified-high]
├── Task 11: OpenCode 适配器：技能 + 代理安装/移除 [unspecified-high]
├── Task 12: OpenCode 适配器：备份/恢复 [unspecified-high]
└── Task 13: OpenCode 适配器测试 [quick]

Wave 3（Wave 2 后 — 批量适配器）:
├── Task 14: Cursor 适配器研究 + 实现 [unspecified-high]
├── Task 15: Codex 适配器研究 + 实现 [unspecified-high]
├── Task 16: Cursor + Codex 适配器测试 [quick]
└── Task 17: 适配器列表命令 [quick]

Wave 4（Wave 3 后 — 解析器 + 集成）:
├── Task 18: 解析器 source 字段集成 [deep]
├── Task 19: 框架感知的停用 [deep]
├── Task 20: 多适配器 E2E 测试 [unspecified-high]
└── Task 21: v0.1.1 bugfix 发布 [quick]

Wave 5（Wave 4 后 — 打磨 + 发布）:
├── Task 22: 发布 v0.2.0 [quick]
└── Task 23: CI 更新（-race, noctx lint, macOS） [quick]

Wave FINAL（所有任务后 — 4 个并行审查）:
├── Task F1: 计划合规审计 (oracle)
├── Task F2: 代码质量审查 (unspecified-high)
├── Task F3: 真实人工 QA (unspecified-high)
└── Task F4: 范围保真度检查 (deep)
→ 呈现合并结果 → 等待显式用户确认
```

关键路径: Task 1 → Task 5 → Task 12 → Task 19 → Task 22 → FINAL
并行加速: 比顺序快约 60%
最大并发: 7（Wave 1）

---

## TODOs

> 实现 + 测试 = 一个任务。永远不要分开。
> 每个任务必须包含：推荐的代理配置文件 + 并行化信息 + QA 场景。
> **没有 QA 场景的任务是不完整的。无一例外。**

- [x] 1. 竞争条件修复 — sync.RWMutex on adapters map

  **做什么**:
  - 向 `pkg/adapter/registry.go` 添加 `sync.RWMutex`
  - 在 `Register`、`Get`、`List` 中加锁
  - 验证适配器注册仅通过 `init()` 进行（运行时不行）
  - 向 `golangci.yml` 添加 `go test -race`
  - 编写测试：10 个 goroutine 并行调用 Get/List，使用 `go test -race`
  - 确保 register_test.go 测试不存在数据竞争

  **禁止做**:
  - 不要在 init() 外添加动态/运行时注册
  - 不要使用 `sync.Map`（对于此模式过于复杂）
  - 不要改变 Register 的 panic-on-duplicate 行为

  **推荐的代理配置文件**: `quick` — Go 并发原语，sync 包
  **并行化**: Wave 1（与 T2、T3、T4 并行）；阻止：无（可立即开始）

  **验收标准**:
  - [ ] `go test -race ./pkg/adapter/` 通过，0 数据竞争
  - [ ] 10 个 goroutine 并行 Get/List 不会 panic
  - [ ] Register 在重复名称上仍 panic

  **QA 场景**:
  ```
  场景: 并发适配器访问无数据竞争
    工具: Bash (go test -race)
    步骤:
      1. 运行 go test -race -run TestRegistryRace -count=100 ./pkg/adapter/
      2. 验证输出包含 "PASS" 且无 "WARNING: DATA RACE"
    预期结果: 所有 100 次运行通过，0 数据竞争
    证据: .sisyphus/evidence/task-1-race.txt

  场景: 重复注册仍 panic
    工具: Bash (go test)
    步骤: 运行 go test -run TestRegistry_DoubleRegisterPanics ./pkg/adapter/
    预期结果: 测试通过（panic 被 recover 捕获）
    证据: .sisyphus/evidence/task-1-double-register.txt
  ```

  **提交**: YES（与 T2、T3 分组为 Wave 1）
  - 消息: `fix: add sync.RWMutex to adapters map`
  - 文件: pkg/adapter/registry.go, pkg/adapter/registry_test.go

- [x] 2. 错误吞没审计 — WalkDir、rollback、GC、restore

  **做什么**:
  - 修复 `internal_activate.go:514` — WalkDir 错误回调返回 err 而非 nil
  - 修复 `store.go:212` — WalkDir 错误回调返回 err 而非 nil
  - 修复 `internal_activate.go:250-258` — 回滚中的 Remove/Restore 错误：记录到警告切片
  - 修复 `internal_activate.go:617` — 检查 ACTIVE lock Remove 错误，如果非 IsNotExist 则添加到 removalErrors
  - 添加测试：损坏的权限目录触发 WalkDir 错误路径
  - 审查所有 `_ = err` 模式（source_test.go:74 除外 — 该处为有意且已注释）

  **禁止做**:
  - 不要改变 rollback 的成功路径行为
  - 不要使非关键错误变得致命（回滚应继续处理剩余步骤）
  - 不要从 GC 路径中静默删除 //nolint:errcheck

  **推荐的代理配置文件**: `quick` — 审计，错误处理，测试
  **并行化**: Wave 1（与 T1、T3、T4 并行）；阻止：无

  **验收标准**:
  - [ ] `go vet ./...` 报告 0 个关于已检查错误的警告
  - [ ] TestWalkDirError: 权限被拒绝的目录在 installAgentFromStore 中产生警告（非静默）
  - [ ] TestRollbackPartialFailure: 回滚移除失败不阻止剩余步骤，错误被记录
  - [ ] TestGCRemoveAllFailure: 移除失败不会使未移除的条目被计入"已移除"

  **QA 场景**:
  ```
  场景: WalkDir 遇到权限被拒绝
    工具: Bash
    前置条件: 包含权限被拒绝子目录的 tar.gz
    步骤:
      1. 创建带有不可读子目录的包 tar.gz
      2. 填充到 store
      3. agentenv activate test → 注意输出中的警告
    预期结果: 激活成功，有类似 "Cannot read directory: permission denied" 的警告
    失败指示器: 激活失败并显示 "no .md file found"（这意味着 WalkDir 静默跳过了错误）
    证据: .sisyphus/evidence/task-2-walkdir.txt

  场景: 回滚中的部分移除失败
    工具: Bash
    步骤:
      1. 安装 3 个技能，使第 2 个移除失败（权限）
      2. 触发回滚 → 第 1 个和第 3 个仍被移除
    预期结果: 回滚继续；记录错误
    证据: .sisyphus/evidence/task-2-rollback.txt
  ```

  **提交**: YES（与 T1、T3 分组为 Wave 1）
  - 消息: `fix: surface errors in WalkDir, rollback, GC, and active lock paths`
  - 文件: cmd/agentenv/internal_activate.go, pkg/store/store.go

- [x] 3. 上下文传播 — 移除 nolint:noctx

  **做什么**:
  - 向 `pkg/store/cache.go` 的 `Download` 方法添加 `context.Context` 参数
  - 将 `http.Get(url)` 替换为 `http.NewRequestWithContext(ctx, "GET", url, nil)`
  - 移除 `//nolint:noctx`
  - 将 `noctx` 添加到 `.golangci.yml` 启用的 linter 列表
  - 从 `cmd/agentenv/install.go:108` 的调用点传播 context（`source.GetHandler` → `handler.Fetch` → `cache.Download`）
  - 测试：使用已取消的 context 并验证错误正确传播

  **禁止做**:
  - 不要改变 HTTP 客户端的超时行为（保留默认客户端）
  - 不要对仅进行文件系统操作的非 HTTP 代码路径（如 local.go）添加 context

  **推荐的代理配置文件**: `quick` — context 包，net/http，测试
  **并行化**: Wave 1（与 T1、T2、T4 并行）；阻止：无

  **验收标准**:
  - [ ] `go vet ./...` 报告 0 个 noctx 违规
  - [ ] `//nolint:noctx` 出现在 0 个文件中
  - [ ] TestContextCancellation: 取消的 context 返回 context.Canceled
  - [ ] TestContextTimeout: 超时 context 返回 context.DeadlineExceeded

  **QA 场景**:
  ```
  场景: 上下文取消停止正在进行的下载
    工具: Bash (go test)
    步骤:
      1. 运行 go test -run TestCache_Download_CanceledContext -v ./pkg/store/
      2. 验证已取消的 context 在合理时间内返回错误
    预期结果: 返回 context.Canceled；下载尝试已停止
    证据: .sisyphus/evidence/task-3-context.txt

  场景: go vet 报告 0 个 noctx 违规
    工具: Bash
    步骤: go vet ./...
    预期结果: 退出代码 0，无输出（= 无违规）
    证据: .sisyphus/evidence/task-3-vet.txt
  ```

  **提交**: YES（与 T1、T2 分组为 Wave 1）
  - 消息: `fix: propagate context through HTTP calls, remove nolint:noctx`
  - 文件: pkg/store/cache.go, cmd/agentenv/install.go, .golangci.yml

- [x] 4. 模块路径对齐

  **做什么**:
  - 验证 `.goreleaser.yaml` 使用 `github.com/7emotions/agentenv`（与 go.mod 匹配）
  - 如果存在不匹配，更新发布配置
  - 验证 `go mod tidy` 干净
  - 检查源代码中的任何 `github.com/agentenv/agentenv` 引用

  **禁止做**:
  - 不要改变 go.mod 路径（它已是正确的：7emotions/agentenv）

  **推荐的代理配置文件**: `quick` — 配置审计，sed，go mod
  **并行化**: Wave 1（与 T1、T2、T3 并行）；阻止：无

  **验收标准**:
  - [ ] `.goreleaser.yaml` 引用 `github.com/7emotions/agentenv`（不是 agentenv/agentenv）
  - [ ] `go mod tidy` 返回零更改
  - [ ] 代码库中没有 `github.com/agentenv/agentenv` 的残留引用（go.mod 除外）

  **QA 场景**:
  ```
  场景: 发布配置与 go 模块匹配
    工具: Bash
    步骤:
      1. grep "github.com/agentenv/agentenv" .goreleaser.yaml (应无匹配或匹配 7emotions)
      2. go mod tidy && git diff --exit-code (应无更改)
    预期结果: 无 grep 匹配旧的 agentenv/agentenv；go mod tidy 干净
    证据: .sisyphus/evidence/task-4-modpath.txt
  ```

  **提交**: YES（与 T1、T2、T3 分组为 Wave 1）
  - 消息: `fix: align goreleaser module path with go.mod`
  - 文件: .goreleaser.yaml（如果需要）

- [x] 5. agentpkg.yaml v2 schema + 解析器

  **做什么**:
  - 向 `pkg/types/package.go` 中的 `PackageDependency` 添加 `Source string` 字段
  - 使用 `json:"source,omitempty" yaml:"source,omitempty"` 使其可选（向后兼容）
  - 更新 `pkg/parser/agentpkg.go` — `ParseAgentPkg` 和 `decodePkgDeps` 来解析新的 source 字段
  - 添加验证：如果提供了 source，则必须是有效的 SourceURL
  - 当 source 缺失时的回退行为：空字符串（继承将在 resolver 中处理，见 Task 18）
  - 更新 `makePkgYAML` 测试辅助函数以可选接受 source

  **禁止做**:
  - 不要更改现有 v1 字段（name、type、constraint 必须保持原样）
  - 不要在解析器中强制要求 source（v2 向后兼容 v1）

  **推荐的代理配置文件**: `unspecified-high` — schema 设计，YAML 解析，类型
  **并行化**: Wave 1（与 T6、T7 并行）；阻止：无

  **验收标准**:
  - [ ] `PackageDependency` 具有可选的 `Source` 字段
  - [ ] `ParseAgentPkg` 解析带有 `source: github:owner/repo` 的依赖项条目
  - [ ] 当 source 缺失时不存在解析错误（适用于 v1 向后兼容）
  - [ ] 无效的 source URL 产生验证错误
  - [ ] 往返：agentpkg.yaml v2 → 解析 → 序列化 → 相同 YAML

  **QA 场景**:
  ```
  场景: 解析带有 source 字段的 v2 依赖项
    工具: Bash (go test)
    步骤: 使用 source: github:owner/repo 创建 agentpkg.yaml → ParseAgentPkg → Dep.Source == "github:owner/repo"
    预期结果: Source 字段已正确解析
    证据: .sisyphus/evidence/task-5-v2-schema.txt

  场景: v1 依赖项（无 source）向后兼容
    工具: Bash (go test)
    步骤: 创建不带 source 字段的 agentpkg.yaml（v1 格式）→ ParseAgentPkg → 无错误，Dep.Source == ""
    预期结果: v1 文件解析成功
    证据: .sisyphus/evidence/task-5-v1-compat.txt

  场景: 无效 source URL 验证失败
    工具: Bash (go test)
    步骤: 使用 source: "!!!invalid!!!" 解析 → 验证错误包含 "invalid source"
    预期结果: 验证捕获无效 source
    证据: .sisyphus/evidence/task-5-invalid-source.txt
  ```

  **提交**: YES（与 T6 分组）
  - 消息: `feat: agentpkg.yaml v2 schema with source field on dependencies`
  - 文件: pkg/types/package.go, pkg/parser/agentpkg.go, pkg/parser/agentpkg_test.go

- [x] 6. Lockfile v2 格式 + v1 迁移

  **做什么**:
  - 向 `pkg/types/lockfile.go` 中的 `LockedDep` 添加 `Source string` 字段
  - 更新 `pkg/lockfile/generate.go` 以包含依赖项的 source
  - 更新 `pkg/parser/lockfile.go` — `ParseLockfile` 和 `WriteLockfile` 以处理 v2
  - 在 `pkg/lockfile/reader.go` 中的 `Read()` 实现 v1→v2 自动迁移：检测 v1（LockedDep 中无 source 字段），升级到 v2（将 source 设置为空字符串）
  - `agentenv lock` 总是写入 v2 格式
  - 编写迁移测试：创建 v1 lockfile，通过 Read() 读取，验证 v2 输出

  **禁止做**:
  - 不要静默丢弃 v1 lockfile 数据
  - 不要拒绝 v1 lockfile — 自动迁移

  **推荐的代理配置文件**: `quick` — 格式迁移，测试
  **并行化**: Wave 1（与 T5、T7 并行）；阻止：T5（需要 v2 schema）

  **验收标准**:
  - [ ] `LockedDep` 具有 `Source` 字段
  - [ ] `WriteLockfile` 在 v2 格式中包含依赖项的 source
  - [ ] `Read` 自动迁移 v1 lockfile（无 source 字段 → source=""）
  - [ ] TestV1Migration: v1 lockfile 输入 → 无错误的 v2 输出
  - [ ] `agentenv lock` 输出具有 v2 格式（依赖项中的 source 字段）

  **QA 场景**:
  ```
  场景: v1 lockfile 自动迁移到 v2
    工具: Bash (go test)
    前置条件: 具有无 source 字段的 LockedDep 的 v1 lockfile 的 bytes
    步骤: 调用 lockfile.Read(v1_data) → 验证返回的 LockedDep 具有 Source=""
    预期结果: 无错误；源字段已填充为默认值
    证据: .sisyphus/evidence/task-6-migration.txt

  场景: v2 lockfile 具有依赖项的 source 字段
    工具: Bash
    步骤: 创建带有 source 依赖项的 env → agentenv lock → 验证 agent.lock 包含 "source" 字段
    预期结果: agent.lock 具有带有 source 字段的依赖项条目
    证据: .sisyphus/evidence/task-6-v2-output.txt
  ```

  **提交**: YES（与 T5 分组）
  - 消息: `feat: lockfile v2 with source field and v1→v2 migration`
  - 文件: pkg/types/lockfile.go, pkg/lockfile/generate.go, pkg/parser/lockfile.go, pkg/lockfile/reader.go

- [x] 7. 格式文档

  **做什么**:
  - 创建 `docs/agent-yaml.md` — agent.yaml v1 格式的完整参考
  - 创建 `docs/agent-lock.md` — agent.lock v1/v2 格式的完整参考
  - 每个文档包含：schema（所有字段、类型、必填/可选）、示例、字段描述
  - agent.yaml 文档必须覆盖所有 6 个包部分（skills、mcps、agents、tools、hooks、prompts）
  - agent.lock 文档必须记录 v1→v2 变更以及迁移行为

  **禁止做**:
  - 不要编写面向未来的文档（不存在的功能没有待定部分）
  - 不要重复代码中的 JSON schema 注释 — 文档是面向用户的

  **推荐的代理配置文件**: `writing` — 技术文档，markdown
  **并行化**: Wave 1（与 T5、T6 并行）；阻止：T5、T6（需要 schema 最终确定）

  **验收标准**:
  - [ ] `docs/agent-yaml.md` 存在于仓库中
  - [ ] `docs/agent-lock.md` 存在于仓库中
  - [ ] 每个文档涵盖所有字段及类型、必填/可选、示例
  - [ ] lockfile 文档涵盖 v1→v2 迁移
  - [ ] 文档中的示例是有效的 YAML/JSON

  **QA 场景**:
  ```
  场景: docs/ 文件存在且非空
    工具: Bash
    步骤: test -s docs/agent-yaml.md && test -s docs/agent-lock.md
    预期结果: 两个文件存在
    证据: .sisyphus/evidence/task-7-docs.txt
  ```

  **提交**: YES
  - 消息: `docs: agent.yaml and agent.lock format reference`
  - 文件: docs/agent-yaml.md, docs/agent-lock.md

- [x] 8. JSONC 工具库

  **做什么**:
  - 研究 Go JSONC 库：评估 `github.com/tailscale/hujson`（Tailscale，维护良好）vs 自定义注释剥离器
  - 选择库并添加到 go.mod
  - 创建 `pkg/adapter/jsonc.go` 包含：`ReadJSONC(path) ([]byte, error)` — 读取 JSONC，剥离注释，返回有效 JSON
  - 创建 `WriteJSONC(path string, data interface{}) error` — 将 Go 结构体序列化为 JSON，嵌入原始注释（如果适用）
  - 编写往返测试：JSONC 文件 → 读取 → 修改 → 写入 → 文件仍为有效 JSONC，注释保留
  - 处理边缘情况：字符串内的 `//`（不是注释），字符串内的 `/* */`，尾随逗号

  **禁止做**:
  - 不要静默剥离注释 — 文档化限制
  - 不要使用正则表达式进行 JSONC 解析（在字符串字面量上会失败）

  **推荐的代理配置文件**: `unspecified-high` — 库评估，JSON 解析，测试
  **并行化**: Wave 2（与 T9 并行）；阻止：无

  **验收标准**:
  - [ ] `go mod tidy` 显示新的 JSONC 依赖
  - [ ] 往返测试：JSONC 输入 → 解析 → 修改 → 序列化 → 输出保留原始注释
  - [ ] 包含 URL 的注释（如 `"url": "https://example.com/api"`）在字符串内不触发错误的剥离

  **QA 场景**:
  ```
  场景: JSONC 往返保留注释
    工具: Bash (go test)
    步骤: 创建包含 // inline 和 /* block */ 注释的 .jsonc 文件 → ReadJSONC → 修改 → WriteJSONC → 验证注释存活
    预期结果: 注释在往返后仍然存在
    证据: .sisyphus/evidence/task-8-jsonc-roundtrip.txt

  场景: URL 字符串不被错误剥离
    工具: Bash (go test)
    步骤: JSONC 包含 "url": "https://api.example.com/v1//path" → 解析 → url 字段为 "https://api.example.com/v1//path"（不是截断的）
    预期结果: URL 内的 // 被保留
    证据: .sisyphus/evidence/task-8-url-in-string.txt
  ```

  **提交**: YES
  - 消息: `feat: JSONC utility for OpenCode config read/write`
  - 文件: pkg/adapter/jsonc.go, pkg/adapter/jsonc_test.go, go.mod, go.sum

- [x] 9. OpenCode 适配器：路径 + 身份 + manifest

  **做什么**:
  - 创建 `pkg/adapter/opencode.go`
  - 实现身份方法：`Name()` → `"opencode"`，`DisplayName()` → `"OpenCode"`
  - 路径解析：`~/.config/opencode/`（基础），`~/.config/opencode/skills/`，`~/.config/opencode/agents/`
  - Manifest：`~/.config/opencode/.agentenv-manifest.json`（与 Claude Code 相同结构）
  - 实现 `ReadManifest`、`WriteManifest`（复用 Claude Code 逻辑，不同的文件路径）
  - 实现 `GetSkillBasePath`、`GetAgentBasePath`
  - 构造函数：`NewOpenCodeAdapter()` + `NewOpenCodeAdapterWithBase(basePath)`（用于测试）
  - 注册：向 `pkg/adapter/registry.go` 的 `init()` 添加 `Register("opencode", NewOpenCodeAdapter())`

  **禁止做**:
  - 不要复制粘贴 Claude Code 的 manifest 逻辑 — 提取共享代码到内部辅助函数
  - 不要假设 OpenCode 使用 `~/.claude/` 路径

  **推荐的代理配置文件**: `unspecified-high` — 适配器模式，文件系统路径
  **并行化**: Wave 2（与 T8、T10 并行）；阻止：T8（需要 JSONC）

  **验收标准**:
  - [ ] `OpenCodeAdapter` 实现所有身份和 manifest 方法
  - [ ] `Name()` 返回 `"opencode"`，`DisplayName()` 返回 `"OpenCode"`
  - [ ] `ReadManifest` 从 `~/.config/opencode/.agentenv-manifest.json` 读取
  - [ ] `GetSkillBasePath()` 返回 `~/.config/opencode/skills/`

  **QA 场景**:
  ```
  场景: OpenCode 适配器注册并可用
    工具: Bash (go test)
    步骤: adapter.Get("opencode") → 非 nil 适配器，无错误
    预期结果: 适配器已找到并可获取
    证据: .sisyphus/evidence/task-9-registry.txt

  场景: Manifest 读写往返
    工具: Bash (go test)
    步骤: 写入 manifest → 读取 → 相同数据
    预期结果: 往返成功
    证据: .sisyphus/evidence/task-9-manifest.txt
  ```

  **提交**: YES（与 T10、T11、T12 分组为 Wave 2）
  - 消息: `feat: OpenCode adapter — identity, paths, and manifest`
  - 文件: pkg/adapter/opencode.go, pkg/adapter/registry.go

- [x] 10. OpenCode 适配器：MCP 配置 + 格式转换

  **做什么**:
  - 在 `opencode.go` 中实现 MCP 方法：`ReadMCPConfig`、`WriteMCPConfig`、`InstallMCP`、`RemoveMCP`
  - MCP 配置位置：`opencode.jsonc` 文件中的 `"mcp"` 键（不是独立文件！）
  - 格式转换：OpenCode 格式 → `UniversalMCPConfig` 并返回
  - OpenCode 格式："type": "local" 带有 "command" 数组 vs Claude 的扁平命令字符串
  - `InstallMCP`：备份 → 解析 JSONC → 插入/更新 "mcp" 键 → 写入 JSONC → 更新 manifest
  - `RemoveMCP`：备份 → 解析 JSONC → 从 "mcp" 键中移除 → 写入 JSONC → 更新 manifest
  - 使用 JSONC 工具（Task 8）进行保留注释的读取/写入

  **禁止做**:
  - 不要写入独立文件 `.mcp.json`（OpenCode 在 opencode.jsonc 中内联 MCP）
  - 不要将 OpenCode 配置键命名为 `mcpServers`（Claude Code 特定）

  **推荐的代理配置文件**: `unspecified-high` — 格式转换，JSONC，MCP 协议
  **并行化**: Wave 2（与 T9、T11 并行）；阻止：T8（需要 JSONC 工具）

  **验收标准**:
  - [ ] `ReadMCPConfig` 解析 `opencode.jsonc` 中的 "mcp" 键
  - [ ] `InstallMCP` 向 "mcp" 键添加服务器条目
  - [ ] `RemoveMCP` 从 "mcp" 键中移除服务器条目
  - [ ] JSONC 注释在 MCP 安装/移除后仍然保留
  - [ ] 往返：Universal → OpenCode → Universal 产生相同数据

  **QA 场景**:
  ```
  场景: MCP 安装到 opencode.jsonc
    工具: Bash (go test)
    前置条件: 带有注释的最小 opencode.jsonc：{ "mcp": {}, "provider": {} }
    步骤: InstallMCP("test-server", server) → 验证 "mcp" 键包含 "test-server"
    预期结果: 服务器已添加，注释保留，provider 键不变
    证据: .sisyphus/evidence/task-10-mcp-install.txt

  场景: JSONC 往返保留注释
    工具: Bash (go test)
    前置条件: 带有 /* block comment */ 的 opencode.jsonc
    步骤: ReadMCPConfig → 修改 → WriteMCPConfig → 读取文件 → 注释仍存在
    预期结果: 块注释存活
    证据: .sisyphus/evidence/task-10-comment-preserve.txt
  ```

  **提交**: YES（与 T9、T11、T12 分组）
  - 消息: `feat: OpenCode adapter — MCP config with JSONC support`
  - 文件: pkg/adapter/opencode.go

- [x] 11. OpenCode 适配器：技能 + 代理安装/移除

  **做什么**:
  - 实现技能方法：`InstallSkill`、`RemoveSkill`、`ListSkills`
  - 技能目录：`~/.config/opencode/skills/<name>/SKILL.md`
  - 安装：创建目录，从源路径复制 SKILL.md，更新 manifest
  - 移除：删除目录（如果在 manifest 中管理），更新 manifest
  - 实现代理方法：`InstallAgent`、`RemoveAgent`、`ListAgents`
  - 代理目录：`~/.config/opencode/agents/<name>.md`
  - 测试：来自 Claude Code 适配器的平行测试（相同模式，不同路径）

  **禁止做**:
  - 不要对技能安装使用 symlink（OpenCode 期望 SKILL.md 文件）
  - 不要覆盖未管理的技能（manifest 检查，与 Claude Code 相同）

  **推荐的代理配置文件**: `unspecified-high` — 文件系统操作，manifest 管理
  **并行化**: Wave 2（与 T9、T10 并行）；阻止：T9（需要路径 + manifest）

  **验收标准**:
  - [ ] `InstallSkill` 在 `~/.config/opencode/skills/<name>/SKILL.md` 创建文件
  - [ ] `RemoveSkill` 移除目录（如果在 manifest 中）
  - [ ] `InstallAgent` 在 `~/.config/opencode/agents/<name>.md` 创建文件
  - [ ] `RemoveAgent` 移除文件（如果在 manifest 中）
  - [ ] 未管理的技能/代理保留（不覆盖）

  **QA 场景**:
  ```
  场景: 安装并列出技能
    工具: Bash (go test)
    步骤: InstallSkill → ListSkills → 名称出现在列表中
    预期结果: 技能已安装且可发现
    证据: .sisyphus/evidence/task-11-skill.txt

  场景: 移除未管理的技能是空操作
    工具: Bash (go test)
    步骤: 手动创建 skill dir（不在 manifest 中）→ RemoveSkill → 目录仍然存在
    预期结果: 未管理的技能未受影响
    证据: .sisyphus/evidence/task-11-unmanaged.txt
  ```

  **提交**: YES（与 T9、T10、T12 分组）
  - 消息: `feat: OpenCode adapter — skill and agent management`
  - 文件: pkg/adapter/opencode.go

- [x] 12. OpenCode 适配器：备份/恢复

  **做什么**:
  - 实现 `Backup`：复制 `opencode.jsonc` → `~/.agentenv/backups/opencode/<timestamp>.json`
  - 实现 `Restore`：从备份 ID 恢复 `opencode.jsonc` 和 manifest
  - 备份目录：`~/.agentenv/backups/opencode/`（每个适配器的子目录）
  - 当文件缺失时优雅处理（首次运行，无现有配置）
  - 备份失败不阻止激活（警告并继续）

  **禁止做**:
  - 不要备份整个 `~/.config/opencode/` 目录（仅 agentenv 管理的文件）
  - 不要锁定备份目录 — 多个备份并发是安全的

  **推荐的代理配置文件**: `unspecified-high` — 文件 I/O，时间戳，恢复
  **并行化**: Wave 2（与 T9、T10、T11 并行）；阻止：T9、T10

  **验收标准**:
  - [ ] `Backup` 创建 `~/.agentenv/backups/opencode/<timestamp>.json`
  - [ ] `Restore` 从备份返回 config 到之前的状态
  - [ ] 当备份目录不存在时，`Backup` 仍然成功（首次运行）
  - [ ] 恢复后 manifest 匹配备份状态

  **QA 场景**:
  ```
  场景: 备份并恢复 OpenCode 配置
    工具: Bash (go test)
    步骤: InstallMCP → Backup → RemoveMCP → Restore → ReadMCPConfig → MCP 服务器返回
    预期结果: 恢复将配置恢复到安装后状态
    证据: .sisyphus/evidence/task-12-backup-restore.txt

  场景: 首次运行备份
    工具: Bash (go test)
    前置条件: 无 ~/.agentenv/backups/opencode/ 目录
    步骤: Backup → 创建目录和备份文件
    预期结果: 无错误，备份文件已创建
    证据: .sisyphus/evidence/task-12-firstrun.txt
  ```

  **提交**: YES（与 T9、T10、T11 分组）
  - 消息: `feat: OpenCode adapter — backup and restore`
  - 文件: pkg/adapter/opencode.go

- [x] 13. OpenCode 适配器测试

  **做什么**:
  - 创建 `pkg/adapter/opencode_test.go`
  - 复制 Claude Code 适配器的测试模式（adapter_test.go 作为模板）
  - 所有 16 个方法的并行测试套件
  - 特定测试：JSONC 注释保留、OpenCode MCP 格式（"type": "local"/"remote"）、空 config 处理
  - 特定测试：适配器清单中的多个适配器

  **禁止做**:
  - 不要将 OpenCode 测试与 Claude Code 测试混在一起（单独的文件）
  - 不要接触真实的 `~/.config/opencode/` — 使用 `NewOpenCodeAdapterWithBase(t.TempDir())`

  **推荐的代理配置文件**: `quick` — 测试编写（遵循已建立的模式）
  **并行化**: Wave 2（在 T9-T12 之后）；阻止：T9、T10、T11、T12

  **验收标准**:
  - [ ] `go test ./pkg/adapter/ -run OpenCode -v` 通过所有测试
  - [ ] `go test -race ./pkg/adapter/ -run OpenCode` 通过
  - [ ] 测试覆盖：每个方法 ≥ 1 个快乐路径 + 1 个边缘情况

  **QA 场景**:
  ```
  场景: 所有 OpenCode 测试通过
    工具: Bash
    步骤: go test -run OpenCode -v ./pkg/adapter/
    预期结果: 所有测试通过，0 失败
    证据: .sisyphus/evidence/task-13-tests.txt
  ```

  **提交**: YES
  - 消息: `test: OpenCode adapter test suite`
  - 文件: pkg/adapter/opencode_test.go

- [x] 14. Cursor 适配器研究 + 实现

  **做什么**:
  - 研究 Cursor 配置：配置路径（~/.cursor/？），MCP 格式，技能位置，代理定义
  - 创建 `pkg/adapter/cursor.go` — 从 OpenCode 适配器作为模板实现所有 16 个方法
  - 如果 Cursor 没有代理系统：`InstallAgent` 返回描述性错误（不是 panic）
  - MCP 格式转换：Cursor 特定格式 → UniversalMCPConfig 并返回
  - 构造函数：`NewCursorAdapter()` + `NewCursorAdapterWithBase(basePath)`
  - 注册：向 registry.go 的 `init()` 添加 `Register("cursor", NewCursorAdapter())`
  - 创建 `pkg/adapter/cursor_test.go` — 遵循已建立的测试模式

  **禁止做**:
  - 不要为不存在于 Cursor 中的功能假设路径 — 优雅降级
  - 不要为这个适配器重新实现 JSONC（如果需要 JSONC，使用 Task 8 中的工具）

  **推荐的代理配置文件**: `unspecified-high` — 研究，适配器实现
  **并行化**: Wave 3（与 T15、T16、T17 并行）；阻止：T13（需要 OpenCode 测试模式作为参考）

  **验收标准**:
  - [ ] Cursor 配置格式已研究并记录（在任务注释中）
  - [ ] `CursorAdapter` 实现所有 16 个 AgentAdapter 方法
  - [ ] `go test ./pkg/adapter/ -run Cursor -v` 通过所有测试
  - [ ] 适配器在测试之外不接触真实的 `~/.cursor/`

  **QA 场景**:
  ```
  场景: Cursor 适配器注册并返回身份
    工具: Bash (go test)
    步骤: adapter.Get("cursor") → Name() == "cursor", DisplayName() 非空
    预期结果: 适配器已找到且可识别
    证据: .sisyphus/evidence/task-14-cursor.txt
  ```

  **提交**: YES
  - 消息: `feat: Cursor adapter with research-based implementation`
  - 文件: pkg/adapter/cursor.go, pkg/adapter/cursor_test.go, pkg/adapter/registry.go

- [x] 15. Codex 适配器研究 + 实现

  **做什么**:
  - 研究 Codex（OpenAI）配置：CLI 工具存在？配置路径？MCP/Skills 支持？
  - 创建 `pkg/adapter/codex.go` — 实现所有 16 个方法
  - 如果 Codex 没有技能/代理/MCP 概念的正式支持：特定方法返回描述性错误
  - 构造函数：`NewCodexAdapter()` + `NewCodexAdapterWithBase(basePath)`
  - 注册：向 registry.go 的 `init()` 添加 `Register("codex", NewCodexAdapter())`
  - 创建 `pkg/adapter/codex_test.go`

  **禁止做**:
  - 不要为不存在于 Codex 中的功能编造路径 — 记录限制
  - 不要因为某些方法返回"不支持"就让此适配器阻塞其他适配器

  **推荐的代理配置文件**: `unspecified-high` — 研究，存根实现
  **并行化**: Wave 3（与 T14、T16、T17 并行）；阻止：T13

  **验收标准**:
  - [ ] Codex 功能已研究并记录
  - [ ] `CodexAdapter` 实现所有 16 个方法（如果功能不可用，则优雅降级）
  - [ ] `go test ./pkg/adapter/ -run Codex -v` 通过

  **QA 场景**:
  ```
  场景: Codex 适配器注册
    工具: Bash (go test)
    步骤: adapter.Get("codex") → 适配器非 nil
    预期结果: 适配器存在
    证据: .sisyphus/evidence/task-15-codex.txt
  ```

  **提交**: YES
  - 消息: `feat: Codex adapter (degraded — limited feature support)`
  - 文件: pkg/adapter/codex.go, pkg/adapter/codex_test.go, pkg/adapter/registry.go

- [x] 16. Cursor + Codex 适配器测试

  **做什么**:
  - 运行完整的适配器测试套件，包括所有 4 个适配器：`go test -race ./pkg/adapter/ -v`
  - 验证所有 4 个适配器共存无冲突
  - 验证 `adapter.List()` 返回所有 4 个名称
  - 验证 `adapter.Register` 在有重复时正确 panic（重复注册测试）
  - 修复从全测试套件中出现的任何测试隔离问题

  **禁止做**:
  - 不要修改现有的 Claude Code 或 OpenCode 测试以适应新适配器
  - 不要为每个适配器添加独立测试 — 验证它们在同一进程中协同工作

  **推荐的代理配置文件**: `quick` — 测试执行，调试
  **并行化**: Wave 3（与 T14、T15、T17 并行）；阻止：T14、T15

  **验收标准**:
  - [ ] `go test -race ./pkg/adapter/ -v` 所有 4 个适配器的所有测试通过
  - [ ] `adapter.List()` 返回 ["claude-code", "opencode", "cursor", "codex"]（任意顺序）
  - [ ] 0 数据竞争

  **QA 场景**:
  ```
  场景: 所有 4 个适配器共存
    工具: Bash
    步骤: go test -race -v ./pkg/adapter/ → 所有通过，0 数据竞争
    预期结果: 4 个适配器，无冲突
    证据: .sisyphus/evidence/task-16-all-adapters.txt
  ```

  **提交**: YES
  - 消息: `test: verify all 4 adapters coexist without race conditions`
  - 文件: pkg/adapter/*_test.go（根据需要）

- [x] 17. 适配器列表命令

  **做什么**:
  - 向 `cmd/agentenv/` 添加 `agentenv list-adapters` 子命令
  - 输出：可用适配器名称和显示名称的表格
  - 支持 `--json` 标志：JSON 数组 `[{"name": "...", "display": "..."}]`
  - 调用 `adapter.List()` 和 `adapter.Get()` 以获取显示名称

  **禁止做**:
  - 不要在适配器命令中添加框架特定的配置修改

  **推荐的代理配置文件**: `quick` — cobra 子命令，表格格式化
  **并行化**: Wave 3（与 T14、T15、T16 并行）；阻止：T14、T15（需要注册的适配器）

  **验收标准**:
  - [ ] `agentenv list-adapters` 打印所有 4 个适配器
  - [ ] `agentenv list-adapters --json` 输出有效的 JSON 数组
  - [ ] 每个适配器显示名称（非内部名称）

  **QA 场景**:
  ```
  场景: 列出所有适配器
    工具: Bash
    步骤: agentenv list-adapters → 输出包含 "Claude Code", "OpenCode", "Cursor", "Codex"
    预期结果: 所有 4 个适配器都已列出，带有显示名称
    证据: .sisyphus/evidence/task-17-list.txt

  场景: JSON 输出
    工具: Bash
    步骤: agentenv list-adapters --json | jq '.[].name'
    预期结果: 有效的 JSON 数组
    证据: .sisyphus/evidence/task-17-json.txt
  ```

  **提交**: YES
  - 消息: `feat: add agentenv list-adapters command`
  - 文件: cmd/agentenv/list_adapters.go

- [x] 18. 解析器 source 字段集成

  **做什么**:
  - 更新 `pkg/resolver/resolver.go` 的 `Resolve()` 方法：当依赖项具有显式 `source` 时，在 `PackageRequest.Source` 中使用它
  - 当依赖项的 source 为空时，回退到 `r.depSource(name, pkgType)`（现有行为，但默认现在为生产质量）
  - 替换 `resolver.go:115` 的默认回退：将 `"github:test/" + name` 改为 `"github:agentenv/" + name`（从测试存根变为生产默认值）
  - 更新 `lock.go:228-234` 的 `depSource()`：添加 MCP 以外的包类型特定逻辑（技能：`github:agentenv/skills/<name>`，代理：`github:agentenv/agents/<name>` 等）
  - 编写测试：带有显式 source 的依赖项通过解析器传播
  - 编写测试：空 source 回退到 depSource 的结果
  - 修复 `TestLockWithTransitiveDeps` 以使用正确的 source URL（已在之前的会话中取消跳过，现在验证端到端）

  **禁止做**:
  - 不要改变解析器算法本身（拓扑 + 回溯保持不变）
  - 不要在 resolver 中硬编码包类型 — 使用 types.PackageType 常量
  - 不要破坏现有的没有 source 字段的 v1 agentpkg.yaml 测试

  **推荐的代理配置文件**: `deep` — 解析器修改，source 传播，测试
  **并行化**: Wave 4（与 T19 并行）；阻止：T5（v2 schema），T6（lockfile v2）

  **验收标准**:
  - [ ] 依赖项 source 字段通过 resolver 传播到 `ResolvedPackage.Source`
  - [ ] `depSource` 默认不再返回测试存根（`"github:test/"` 从生产代码中消失）
  - [ ] `TestLockWithTransitiveDeps` 使用正确的 source URL 端到端通过
  - [ ] 具有显式 source 的依赖项覆盖 depSource 回退

  **QA 场景**:
  ```
  场景: 依赖项的显式 source 被使用
    工具: Bash (go test)
    步骤: 具有 deps: [{name: tool-a, source: github:myorg/tool-a}] 的 agentpkg.yaml → lock → 验证锁定的 dep 具有 source: "github:myorg/tool-a"
    预期结果: 显式 source 在 lockfile 中存活的端到端
    证据: .sisyphus/evidence/task-18-explicit-source.txt

  场景: 空 source 回退到 depSource
    工具: Bash (go test)
    步骤: 具有 deps: [{name: dep-b}]（无 source）的 agentpkg.yaml → resolv → 验证 source 为 "github:agentenv/skills/dep-b"
    预期结果: 回退到 depSource
    证据: .sisyphus/evidence/task-18-fallback.txt
  ```

  **提交**: YES
  - 消息: `feat: propagate dependency source field through resolver`
  - 文件: pkg/resolver/resolver.go, cmd/agentenv/lock.go, cmd/agentenv/lock_install_test.go

- [x] 19. 框架感知的停用

  **做什么**:
  - 修改 `internal_activate.go` 的 `activateEnv()`：在 ACTIVE lock 中存储 `name:framework`
  - 格式：`<env-name>:<framework>`（例如，`test-env:opencode`）
  - 修改 `deactivateEnv()`：从 ACTIVE lock 读取框架，使用它调用 `adapter.Get(framework)`
  - 移除 `deactivateEnv()` 中对 "claude-code" 的回退（第 547 行）— 相反，如果无法确定框架，则返回错误
  - 向后兼容：检测旧格式（仅名称，无冒号）→ 默认为 "claude-code" + 警告
  - 编写测试：在 ACTIVE lock 中激活 OpenCode → deactivate → 验证 OpenCode 适配器被调用
  - 编写测试：激活 env A（Claude Code）→ 激活 env B（OpenCode）→ 验证 env A 已使用 Claude Code 适配器停用

  **禁止做**:
  - 不要改变 ACTIVE lock 文件位置（保留在 ~/.agentenv/ACTIVE）
  - 不要在停用完成之前从 ACTIVE lock 中移除框架名称

  **推荐的代理配置文件**: `deep` — 状态管理，框架分发，向后兼容
  **并行化**: Wave 4（与 T18、T20 并行）；阻止：T9-T12（需要 OpenCode 适配器工作）

  **验收标准**:
  - [ ] ACTIVE lock 包含 `<name>:<framework>` 格式
  - [ ] 激活 OpenCode env 会在停用时调用 OpenCode 适配器
  - [ ] 旧格式 ACTIVE lock（仅名称）默认为 Claude Code，并带有警告
  - [ ] 跨框架切换：env A（Claude Code）→ env B（OpenCode）→ env A 适配器状态已清理

  **QA 场景**:
  ```
  场景: OpenCode 激活写入正确的 ACTIVE lock 格式
    工具: Bash
    前置条件: 使用 --agent opencode 创建 env
    步骤: agentenv activate opencode-env → cat ~/.agentenv/ACTIVE → "opencode-env:opencode"
    预期结果: ACTIVE lock 包含框架名称
    证据: .sisyphus/evidence/task-19-active-format.txt

  场景: 跨框架切换清理旧状态
    工具: Bash
    步骤: activate claude-env → activate opencode-env → 验证 claude-env 的 MCP 服务器已被移除
    预期结果: 旧框架的状态在切换时被清理
    证据: .sisyphus/evidence/task-19-cross-framework.txt
  ```

  **提交**: YES
  - 消息: `feat: framework-aware deactivation with ACTIVE lock format`
  - 文件: cmd/agentenv/internal_activate.go, cmd/agentenv/activate_test.go

- [x] 20. 多适配器 E2E 测试

  **做什么**:
  - 为每个适配器创建端到端测试：create → add → lock → install → activate → verify → deactivate → verify cleanup
  - 测试跨框架隔离：env A（Claude Code）和 env B（OpenCode）可以独立激活
  - 测试错误恢复：安装失败 → 回滚 → 验证干净状态
  - 测试空 env：激活具有 0 个包的 env
  - 测试源不可达：锁定失败并显示清晰错误（不是 panic）
  - 所有 E2E 测试使用 `t.TempDir()` 和模拟适配器（不是真实的 `~/.claude/` 或 `~/.config/opencode/`）

  **禁止做**:
  - 不要在 E2E 测试中接触真实的文件系统路径
  - 不要跳过网络相关测试 — 使用模拟源

  **推荐的代理配置文件**: `unspecified-high` — 集成测试，模拟
  **并行化**: Wave 4（与 T19 并行）；阻止：T19（需要框架感知的停用）

  **验收标准**:
  - [ ] 每个适配器的 E2E 测试：创建 → 激活 → 停用 完全循环
  - [ ] TestCrossFrameworkIsolation: 两个不同框架的 env 不冲突
  - [ ] TestEmptyEnvActivation: 具有 0 个包的 env 激活无错误
  - [ ] TestFailedInstallRollback: 失败的安装回滚到之前的状态
  - [ ] `go test -race ./cmd/agentenv/ -run E2E -v` 通过

  **QA 场景**:
  ```
  场景: 完整的 OpenCode E2E 循环
    工具: Bash (go test)
    步骤: create --agent opencode e2e-test → add skill test-skill → lock → activate → deactivate
    预期结果: 所有步骤成功，停用后无残留状态
    证据: .sisyphus/evidence/task-20-opencode-e2e.txt

  场景: 跨框架隔离
    工具: Bash (go test)
    步骤: 激活 claude-env → 验证 Claude Code 适配器状态 → 激活 opencode-env → 验证 Claude Code 已清理，OpenCode 已激活
    预期结果: 无跨框架污染
    证据: .sisyphus/evidence/task-20-isolation.txt
  ```

  **提交**: YES
  - 消息: `test: multi-adapter E2E tests with cross-framework isolation`
  - 文件: cmd/agentenv/e2e_test.go

- [x] 21. v0.1.1 bugfix 发布

  **做什么**:
  - 创建并推送 git 标签 `v0.1.1`
  - 变更日志条目：空 lockfile 修复、传递依赖项修复
  - 触发 goreleaser 构建（如果 CI 已设置）或手动构建二进制文件
  - 更新 install.sh 以指向 v0.1.1（如果需要）
  - 验证：从干净机器上的标签构建 → `agentenv --version` 显示 v0.1.1
  - 验证：之前失败的场景现在正常（空 lockfile 激活，传递 dep 锁定）

  **禁止做**:
  - 不要在 v0.1.1 中包含新功能 — 仅 bugfix
  - 不要创建 v0.1.1 分支 — 从 main 直接标记

  **推荐的代理配置文件**: `quick` — git 标签，发布
  **并行化**: Wave 4（与 T18、T19、T20 并行）；阻止：T18（如果在同一 wave 中，则基本上独立）

  **验收标准**:
  - [ ] `git tag -l v0.1.1` 显示标签存在
  - [ ] 从 v0.1.1 标签构建产生功能二进制文件
  - [ ] `agentenv --version` 显示 v0.1.1

  **QA 场景**:
  ```
  场景: v0.1.1 标签存在且可构建
    工具: Bash
    步骤: git checkout v0.1.1 && go build ./cmd/agentenv && ./agentenv --version
    预期结果: 构建成功，版本字符串包含 v0.1.1
    证据: .sisyphus/evidence/task-21-tag.txt
  ```

  **提交**: N/A（标签，不是提交）
  - 命令: `git tag -a v0.1.1 -m "v0.1.1: bugfix release"`

- [x] 22. 发布 v0.2.0

  **做什么**:
  - 生成变更日志条目（从 v0.1.0 以来的所有提交）
  - 更新仓库中的版本字符串（`cmd/agentenv/main.go:getVersion()` 如果需要的话）
  - 创建并推送 git 标签 `v0.2.0`
  - 使用 goreleaser 构建发布二进制文件（如果 CI 已设置）
  - 更新 install.sh 以指向 v0.2.0
  - 验证：`agentenv --version` 显示 v0.2.0
  - 推送标签到 GitHub：`git push origin v0.2.0`

  **禁止做**:
  - 如果未通过所有测试（包括 -race），不要推送标签

  **推荐的代理配置文件**: `quick` — 发布工程
  **并行化**: Wave 5（与 T23 并行）；阻止：所有之前的任务

  **验收标准**:
  - [ ] `git tag -l v0.2.0` 显示标签存在
  - [ ] 所有 4 个适配器出现在 `agentenv list-adapters` 中
  - [ ] `agentenv --version` 显示 v0.2.0
  - [ ] `go test -race ./...` 通过

  **QA 场景**:
  ```
  场景: v0.2.0 完全集成检查
    工具: Bash
    步骤:
      1. agentenv list-adapters → 4 个适配器
      2. agentenv create --agent opencode v2-test → 成功
      3. agentenv create --agent cursor cursor-test → 成功
      4. agentenv --version → v0.2.0
    预期结果: 所有命令成功
    证据: .sisyphus/evidence/task-22-release.txt
  ```

  **提交**: N/A（标签）
  - 命令: `git tag -a v0.2.0 -m "v0.2.0: multi-framework support, v2 schema, fixes"`

- [x] 23. CI 更新

  **做什么**:
  - 向 CI 矩阵添加 `go test -race ./...`（GitHub Actions）
  - 启用 golangci-lint 配置中的 `noctx` linter
  - 向 CI 矩阵添加 macOS runner（如果尚未存在）
  - 验证 goreleaser 在 CI 中正确构建（`--snapshot`）
  - 确保所有 4 个适配器在 CI 中编译
  - 如果适用，添加跨平台（darwin/linux）构建矩阵

  **禁止做**:
  - 如果未通过测试，不要使 CI 过于严格（仅限信息 lint）

  **推荐的代理配置文件**: `quick` — CI/CD 配置
  **并行化**: Wave 5（与 T22 并行）；阻止：所有之前的任务

  **验收标准**:
  - [ ] CI 工作流包含 `go test -race ./...`
  - [ ] CI 工作流在矩阵中包含 macOS runner
  - [ ] golangci.yml 启用了 `noctx`
  - [ ] goreleaser 快照构建在 CI 中通过

  **QA 场景**:
  ```
  场景: CI 配置有效（手动干运行）
    工具: Bash
    步骤: golangci-lint run && go test -race ./... && goreleaser build --snapshot --clean
    预期结果: 所有步骤通过
    证据: .sisyphus/evidence/task-23-ci.txt
  ```

  **提交**: YES
  - 消息: `ci: add race detector, noctx linter, and macOS runner`
  - 文件: .github/workflows/*.yml, .golangci.yml

- [x] F1. **计划合规审计** — `oracle`
  端到端阅读计划。对于每个"必须包含"：验证实现是否存在（读取文件、curl 端点、运行命令）。对于每个"禁止包含"：在代码库中搜索禁止模式。检查 `.sisyphus/evidence/` 中是否存在证据文件。将交付物与计划进行比较。
  输出: `必须包含 [N/N] | 禁止包含 [N/N] | 任务 [N/N] | 裁决: 批准/拒绝`

- [x] F2. **代码质量审查** — `unspecified-high`
  运行 `go vet ./...` + `golangci-lint run` + `go test -race ./...`。审查所有更改的文件：无注释的 JSONC 剥离、无未检查的回滚错误、无裸的 context.Background()、无未使用的导入、无注释掉的代码。检查 AI slop：过多注释、过度抽象、通用名称。
  输出: `构建 [通过/失败] | Lint [通过/失败] | 测试 [N 通过/N 失败] | Race [通过/失败] | 裁决`

- [x] F3. **真实人工 QA** — `unspecified-high` (+ `playwright` skill 如有 UI)
  从干净状态开始。从每个任务执行每个 QA 场景。测试跨任务集成。测试边缘情况：并发激活、框架切换、JSONC 注释、空 env、零包、部分配置。保存到 `.sisyphus/evidence/final-qa/`。
  输出: `场景 [N/N 通过] | 集成 [N/N] | 边缘情况 [N 已测试] | 裁决`

- [x] F4. **范围保真度检查** — `deep`
  对于每个任务：读取"做什么"，读取实际差异。验证 1:1 — 规范中的所有内容都已构建，规范外的任何内容都未构建。检查"禁止做"合规性。检测跨任务污染。标记未计入的更改。
  输出: `任务 [N/N 合规] | 污染 [干净/N 问题] | 未计入 [干净/N 文件] | 裁决`

---

## 提交策略

- **Wave 1**: `fix: race condition, error swallowing, context, module path, v2 schema`
- **Wave 2**: `feat: OpenCode adapter with JSONC support`
- **Wave 3**: `feat: Cursor and Codex adapters`
- **Wave 4**: `feat: resolver source integration and framework-aware deactivation`
- **Wave 5**: `chore: release v0.1.1 and v0.2.0`
- **修复**: `fix(scope): description` — 压缩到父功能提交

---

## 成功标准

### 验证命令
```bash
go test -race ./...                # 所有测试通过，0 数据竞争
go vet ./...                       # 无 vet 警告
golangci-lint run                  # 无 lint 错误
goreleaser build --snapshot --clean  # 二进制文件为所有目标构建
agentenv create --agent opencode test && agentenv activate test && agentenv deactivate  # E2E
```

### 最终检查清单
- [ ] 所有"必须包含"存在
- [ ] 所有"禁止包含"缺失
- [ ] 所有测试在 macOS + Linux 上通过
- [ ] `go test -race ./...` 通过
- [ ] `agent.yaml` 格式已记录（docs/）
- [ ] `agent.lock` 格式已记录（docs/）
- [ ] 发布产物：v0.1.1 标签 + v0.2.0 标签 + 更新后的 brew 配方
- [ ] OpenCode JSONC 往返保留注释
