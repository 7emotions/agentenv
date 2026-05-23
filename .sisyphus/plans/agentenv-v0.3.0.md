# agentenv v0.3.0 — PubGrub Version Resolver

## TL;DR

> **Quick Summary**: 用 PubGrub CDCL 算法替换当前拓扑排序+简单回溯 resolver。使用 `contriboss/pubgrub-go` 库，锁定 v3 lockfile 格式（加 `resolved_by` 字段），TDD 全流程。

> **Deliverables**:
> - `pkg/resolver/pubgrub_resolver.go` — PubGrubResolver 实现 DependencyResolver 接口
> - `pkg/resolver/pubgrub_adapter.go` — SourceHandler → pubgrub Source 适配层
> - `pkg/types/lockfile.go` — LockedPackage.ResolvedBy + LockedDep.Type/Source，lockfile v3
> - `pkg/lockfile/` — v1→v3 读取迁移 + v3 写入
> - 新增测试：钻石冲突、不可满足约束、深层依赖链、v3 往返

> **Estimated Effort**: Medium
> **Parallel Execution**: YES — 5 waves
> **Critical Path**: Task 1 → Task 5 → Task 8 → Task 12 → Task 18-21

---

## Context

### Original Request
"先准备 v0.3.0 的计划" — 将 agentenv resolver 从拓扑回溯升级为 PubGrub 算法

### Interview Summary
**Key Discussions**:
- **算法**: PubGrub（非 SAT），使用 `github.com/contriboss/pubgrub-go` v0.3.4 (Apache 2.0)
- **约束语法**: 保持现状 `^ ~ >= < latest *`，不要 OR 运算符
- **Lockfile**: v3 — 每个 LockedPackage 加 `resolved_by` 字段；LockedDep 加 `type` 和 `source`
- **冲突报告**: PubGrub 默认级别 — 冲突链 + 推导树
- **测试**: TDD — 先写冲突场景测试
- **旧 resolver**: v0.3.0 保留但标记已弃用，v0.4.0 移除

**Decisions Made**:
- Lockfile v2 实际上从未写入（generate.go 写 Version:1），直接跳 v3
- pubgrub-go 要求 Go 1.25，agentenv 当前 go 1.23 → 必须升级
- 命名空间编码：`{type}:{name}` 避免 6 类型扁平命名冲突
- `depSource` 通过构造函数选项传入，不放接口

### Research Findings
- **pubgrub-go**: CDCL solver, unit propagation, conflict learning, enhanced error reporting
- **pubgrub-go Source 接口**: `GetPackageVersions(Name) ([]Version, error)` + `GetPackageDependencies(Name, Version) ([]Term, error)`
- **当前 resolver**: TopologicalBacktrackResolver (401 行) — BFS 队列，简单 ConstraintIntersection
- **版本系统**: Masterminds/semver/v3 — pubgrub 自有 semver 实现，部分兼容
- **适配核心挑战**: agentenv 通过 (Name, Type, SourceURL) 识别包；pubgrub 通过平面 Name 识别

### Metis Review
**Identified Gaps** (addressed):
- Go 1.25 必须升级 go.mod → Task 1
- pubgrub Source 适配层 → Task 4-5
- DepSource 回调通过构造函数 → Task 3
- Lockfile 当前写 v1 非 v2 → Task 7
- 命名空间冲突：编码 `{type}:{name}` → Task 5
- 取消支持 → 检查 pubgrub API
- 缓存防止重复获取 → 使用 CachedSource → Task 5
- 清单提取失败行为变更 → 文档化
- LockedDep 也加 Type+Source → Task 7
- 旧版本函数标记弃用不动 → Task 10

---

## Work Objectives

### Core Objective
实现 PubGrub-based DependencyResolver，替换 TopologicalBacktrackResolver，在复杂依赖图中自动找到版本组合或给出清晰的冲突原因。

### Concrete Deliverables
- `pkg/resolver/pubgrub_resolver.go` — PubGrubResolver 结构体 + Resolve 方法
- `pkg/resolver/pubgrub_adapter.go` — agentenv SourceHandler → pubgrub Source 适配器
- `pkg/resolver/pubgrub_resolver_test.go` — TDD 冲突场景测试（钻石/不可满足/深层链）
- `pkg/types/lockfile.go` — LockedPackage.ResolvedBy + LockedDep.Type/Source
- `pkg/lockfile/generate.go` — v3 写入逻辑
- `pkg/lockfile/reader.go` — v1→v3 自动迁移
- `go.mod` / `go.sum` — Go 1.25 + pubgrub-go 依赖

### Definition of Done
- [ ] `go test -race ./pkg/resolver/...` → PASS（所有现有 + 新测试）
- [ ] `go test -race ./pkg/lockfile/...` → PASS（v3 往返测试）
- [ ] `go build ./...` → 成功
- [ ] `go vet ./...` → 干净
- [ ] `agentenv lock` 产出 v3 锁文件，包含 `resolved_by` 字段
- [ ] 钻石冲突测试：A@^1→B@>=2, C@^1→B@<2 → 错误包含 "A" "C" "B" 冲突链
- [ ] v1 锁文件可读，自动迁移到 v3

### Must Have
- `DependencyResolver` 接口不变
- `source.SourceHandler` 接口不变
- 所有现有 resolver 测试通过（错误字符串可更新）
- 锁文件向后兼容（v1 可读）
- pubgrub 适配层正确缓存（不回源重复获取）
- `resolved_by` 字段准确指向父包名

### Must NOT Have (Guardrails)
- 不改变 `DependencyResolver` 接口签名
- 不改变 `source.SourceHandler` 接口
- 不添加 OR 运算符支持
- 不删除 `version.go` 中的任何函数（保留+Deprecated 注释）
- 不重写 `extractManifestFromTarGz`（移动可接受，重写不）
- 不改变 `cmd/agentenv/` 中除构造函数调用外的任何代码
- 不添加 pubgrub-go 以外的依赖
- 不改变适配器层（pkg/adapter/）
- 不改变 store（pkg/store/）
- 不改变 CLI 接口
- 无新遥测、无电话回家、无分析

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES（go test -race，541 现有测试）
- **Automated tests**: TDD
- **Framework**: go test + race detection
- **TDD**: RED（冲突场景测试）→ GREEN（PubGrubResolver 实现）→ REFACTOR（接口清理）

### QA Policy
Every task MUST include agent-executed QA scenarios.

- **Backend/API**: Use Bash (go test -run TestXxx -v) — 运行测试，验证 PASS，检查输出
- **Build**: Use Bash (go build ./... + go vet ./...) — 验证编译和 lint
- **Lockfile**: Use Bash (go test + 手动检查 JSON 输出) — 验证 v3 格式

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation + dependency):
├── Task 1:  Go 1.25 升级 + pubgrub-go 依赖 [quick]
├── Task 2:  命名空间编码方案设计 + 文档 [quick]
├── Task 3:  PubGrubResolver 构造函数 + opts [quick]
├── Task 4:  SourceHandler → pubgrub Source 适配器骨架 [deep]
└── Task 5:  适配器完整实现 + CachedSource [deep]

Wave 2 (After Wave 1 — 类型系统 + 锁文件):
├── Task 6:  LockedPackage.ResolvedBy + LockedDep.Type/Source [quick]
├── Task 7:  Lockfile v3 格式定义 + 写入 [quick]
├── Task 8:  Lockfile v1→v3 读取迁移 [quick]
└── Task 9:  Lockfile v3 往返测试 [quick]

Wave 3 (After Wave 1+2 — TDD RED 阶段):
├── Task 10: 标记旧 resolver 弃用 + 版本函数保留 [quick]
├── Task 11: TDD: 简单图测试 (单根无冲突) [quick]
├── Task 12: TDD: 钻石冲突测试 (A→B@>=2, C→B@<2) [unspecified-high]
├── Task 13: TDD: 不可满足约束测试 [unspecified-high]
├── Task 14: TDD: 深层依赖链测试 (A→B→C→D→E 冲突) [unspecified-high]
└── Task 15: TDD: latest/* 约束测试 [quick]

Wave 4 (After Wave 3 — GREEN 实现 + REFACTOR):
├── Task 16: PubGrubResolver.Resolve 核心实现 [deep]
├── Task 17: 冲突报告包装 + agentenv 错误格式 [unspecified-high]
├── Task 18: 适配器 web 获取 + manifest 提取集成 [deep]
├── Task 19: 回填 resolved_by 到 Lockfile v3 [unspecified-high]
└── Task 20: 构造函数切换 (lock.go/install.go → PubGrubResolver) [quick]

Wave 5 (After Wave 4 — 验证 + 清理):
├── Task 21: 所有现有测试回归验证 [unspecified-high]
├── Task 22: 边界情况：取消、空包、无版本、无标签 [unspecified-high]
└── Task 23: go.mod tidy + build + vet 最终检查 [quick]

Wave FINAL (After ALL tasks — 4 并行审查, then user okay):
├── Task F1: Plan Compliance Audit (oracle)
├── Task F2: Code Quality Review (unspecified-high)
├── Task F3: Real Manual QA (unspecified-high)
└── Task F4: Scope Fidelity Check (deep)
→ 展示结果 → 获取用户明确 okay

Critical Path: Task 1 → Task 5 → Task 8 → Task 12 → Task 16 → Task 18 → F1-F4
Parallel Speedup: ~65% faster than sequential
Max Concurrent: 5 (Waves 1, 3)
```

---

## TODOs

- [x] 1. Go 1.25 升级 + pubgrub-go 依赖

  **What to do**:
  - 修改 `go.mod`: `go 1.23` → `go 1.25`
  - 运行 `go get github.com/contriboss/pubgrub-go@v0.3.4`
  - 运行 `go mod tidy`
  - 确认 `/usr/local/go/bin/go version` ≥ 1.25
  - 运行 `go build ./...` 确认编译

  **Must NOT do**:
  - 不要升级其他依赖
  - 不要改 go.sum 以外的文件（pubgrub 尚不被引用但需下载）

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 纯依赖管理，单文件修改
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 2 并行）
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 3, 4, 5
  - **Blocked By**: None

  **References**:
  - `go.mod:1-5` — 当前 Go 版本声明
  - `.goreleaser.yaml:1-3` — CI Go 版本，可能也需更新
  - pubgrub-go: `https://pkg.go.dev/github.com/contriboss/pubgrub-go`

  **Acceptance Criteria**:
  - [ ] `go build ./...` → 成功
  - [ ] `grep "go 1.25" go.mod` → 匹配
  - [ ] `grep "contriboss/pubgrub-go" go.mod` → 匹配

  **QA Scenarios**:
  ```
  Scenario: go.mod 升级成功
    Tool: Bash
    Steps:
      1. go version → 输出含 "go1.25"+
      2. go build ./... → exit 0
      3. grep "go 1.25" go.mod → 匹配
    Expected Result: exit 0，go.mod 含 go 1.25 和 pubgrub-go
    Evidence: .sisyphus/evidence/task-1-go-mod.txt
  ```

  **Commit**: YES
  - Message: `chore: upgrade Go 1.25 + add pubgrub-go dependency`
  - Files: `go.mod`, `go.sum`

- [x] 2. 命名空间编码方案

  **What to do**:
  - 创建 `pkg/resolver/namespace.go`
  - `EncodeName(pkgType, name string) string` → `"{type}:{name}"`
  - `DecodeName(encoded string) (pkgType, name string, err error)`
  - 单元测试：往返、特殊字符、空输入、缺少冒号

  **Must NOT do**:
  - 不要在 Name 中编码 source URL
  - 不要改变 PackageType 定义

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 简单工具函数 + 测试
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 1 并行）
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 5
  - **Blocked By**: None

  **References**:
  - `pkg/types/package.go` — PackageType 常量

  **Acceptance Criteria**:
  - [ ] `EncodeName("skill", "code-reviewer")` → `"skill:code-reviewer"`
  - [ ] `DecodeName("skill:code-reviewer")` → `("skill", "code-reviewer", nil)`
  - [ ] `DecodeName("bad")` → error
  - [ ] `go test -run TestNamespace -v` → PASS

  **QA Scenarios**:
  ```
  Scenario: 命名空间编码往返
    Tool: Bash
    Steps:
      1. go test -run TestNamespace -v ./pkg/resolver/
    Expected Result: 所有测试 PASS
    Evidence: .sisyphus/evidence/task-2-namespace.txt
  ```

  **Commit**: YES
  - Message: `feat(resolver): namespace encoding for PubGrub flat name space`
  - Files: `pkg/resolver/namespace.go`, `pkg/resolver/namespace_test.go`

- [x] 3. PubGrubResolver 构造函数 + 配置选项

  **What to do**:
  - 创建 `pkg/resolver/pubgrub_resolver.go`
  - `PubGrubResolver` 结构体 + `PubGrubResolverOption` 选项模式
  - `NewPubGrubResolver(opts ...PubGrubResolverOption) *PubGrubResolver`
  - 选项: `WithDepSource(fn)`, `WithMaxSteps(n)`, `WithLogger(logger)`
  - 骨架 `Resolve()` → 返回 `ErrNotImplemented`
  - 编译时接口检查: `var _ DependencyResolver = (*PubGrubResolver)(nil)`

  **Must NOT do**:
  - 不要实现 Resolve 逻辑（Task 16）
  - 不要移除 TopologicalBacktrackResolver
  - 不要改变 DependencyResolver 接口

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 结构体定义 + 选项模式，标准 Go 惯用法
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 5, 16
  - **Blocked By**: Task 1

  **References**:
  - `pkg/resolver/resolver.go:49-66` — DependencyResolver 接口
  - `pkg/resolver/resolver.go:83-89` — TopologicalBacktrackResolver 结构
  - `cmd/agentenv/lock.go:90-91` — 调用者构造方式

  **Acceptance Criteria**:
  - [ ] `NewPubGrubResolver()` 编译通过
  - [ ] `WithDepSource(fn)` 正确设置字段
  - [ ] 接口编译时检查通过
  - [ ] `Resolve()` 返回未实现错误

  **QA Scenarios**:
  ```
  Scenario: 构造函数编译 + 接口合规
    Tool: Bash
    Steps:
      1. go build ./pkg/resolver/
      2. go vet ./pkg/resolver/
    Expected Result: exit 0
    Evidence: .sisyphus/evidence/task-3-constructor.txt
  ```

  **Commit**: YES
  - Message: `feat(resolver): PubGrubResolver constructor with options pattern`
  - Files: `pkg/resolver/pubgrub_resolver.go`

- [x] 4. SourceHandler → pubgrub Source 适配器骨架

  **What to do**:
  - 创建 `pkg/resolver/pubgrub_adapter.go`
  - `agentenvSource` 结构体: 持有 `SourceHandler` + `*SourceURL` + `depSource` 回调
  - 实现 pubgrub `Source` 接口:
    - `GetPackageVersions(Name) ([]Version, error)` → 调用 handler.ListVersions，转换 `[]string` → `[]pubgrub.Version`
    - `GetPackageDependencies(Name, Version) ([]Term, error)` → 骨架返回空
  - 单元测试：版本列表转换、空版本

  **Must NOT do**:
  - 不要实现 GetPackageDependencies 完整逻辑（Task 5）
  - 不要改变 SourceHandler 接口

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: 跨系统接口适配，类型映射容易出错
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 3 并行）
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 5, 16
  - **Blocked By**: Task 1

  **References**:
  - `pkg/source/source.go` — SourceHandler 接口
  - `pkg/types/source.go` — SourceURL 类型
  - pubgrub-go Source: `https://pkg.go.dev/github.com/contriboss/pubgrub-go#Source`

  **Acceptance Criteria**:
  - [ ] `agentenvSource` 实现 `pubgrub.Source`
  - [ ] `GetPackageVersions` 正确调用 handler.ListVersions
  - [ ] `go test -run TestAdapter -v` → PASS

  **QA Scenarios**:
  ```
  Scenario: 版本列表转换
    Tool: Bash
    Steps:
      1. go test -run TestAdapter_GetPackageVersions -v ./pkg/resolver/
    Expected Result: PASS
    Evidence: .sisyphus/evidence/task-4-adapter.txt
  ```

  **Commit**: YES
  - Message: `feat(resolver): SourceHandler to pubgrub Source adapter skeleton`
  - Files: `pkg/resolver/pubgrub_adapter.go`, `pkg/resolver/pubgrub_adapter_test.go`

- [x] 5. 适配器完整实现 + CachedSource + 依赖发现

  **What to do**:
  - 完成 `GetPackageDependencies`:
    - `DecodeName(Name)` → `(pkgType, name)`
    - `handler.Fetch(srcURL, version)` → tar.gz
    - `extractManifestFromTarGz(data)` → agentpkg.yaml
    - `parser.ParseAgentPkg(manifestData)` → spec.Dependencies
    - 每个 dep → `pubgrub.Term`（EncodeName + pubgrub condition）
    - 无 agentpkg.yaml → 空列表（非错误）
  - 用 `pubgrub.CachedSource` 包装
  - `depSource` 回调用于隐式源
  - 测试：有/无依赖、manifest 解析失败、缓存命中

  **Must NOT do**:
  - 不修改 `extractManifestFromTarGz` 函数
  - 获取失败不静默吞掉

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: 核心适配逻辑，多组件集成
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 末尾
  - **Blocks**: Task 12-15, 16
  - **Blocked By**: Task 2, 3, 4

  **References**:
  - `pkg/resolver/resolver.go:118-140` — extractManifestFromTarGz
  - `pkg/parser/agentpkg.go` — ParseAgentPkg
  - `pkg/types/package.go` — AgentSpec.Dependencies
  - pubgrub-go CachedSource: `https://pkg.go.dev/github.com/contriboss/pubgrub-go#CachedSource`

  **Acceptance Criteria**:
  - [ ] 含 agentpkg.yaml 的包 → 返回正确 Term 列表
  - [ ] 无 agentpkg.yaml → 空列表无错误
  - [ ] 缓存：同包请求两次只 Fetch 一次
  - [ ] `go test -run TestAdapter_Dependencies -v` → PASS

  **QA Scenarios**:
  ```
  Scenario: 依赖发现 + 缓存验证
    Tool: Bash
    Steps:
      1. go test -run TestAdapter_GetPackageDependencies -v ./pkg/resolver/
      2. go test -run TestAdapter_CachedFetch -v ./pkg/resolver/
    Expected Result: 全部 PASS
    Evidence: .sisyphus/evidence/task-5-deps.txt
  ```

  **Commit**: YES
  - Message: `feat(resolver): complete pubgrub adapter with dependency discovery and caching`
  - Files: `pkg/resolver/pubgrub_adapter.go`, `pkg/resolver/pubgrub_adapter_test.go`

- [x] 6. LockedPackage.ResolvedBy + LockedDep.Type/Source

  **What to do**: LockedPackage 加 `ResolvedBy string`；LockedDep 加 `Type string` + `Source string`；更新引用代码；JSON 序列化测试

  **Must NOT do**: 不改已有字段

  **Recommended Agent Profile**: `quick` — 字段追加
  **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 7, 8 并行）
  - **Parallel Group**: Wave 2 | **Blocks**: Task 9, 19 | **Blocked By**: Wave 1

  **References**: `pkg/types/lockfile.go` — LockedDep, LockedPackage 定义

  **Acceptance Criteria**:
  - [ ] `LockedPackage.ResolvedBy` + `LockedDep.Type` + `LockedDep.Source` 编译通过
  - [ ] `go test ./pkg/types/ -v` → PASS

  **QA Scenarios**: `go test -run TestLockedDep_TypeSource -v ./pkg/types/` → PASS
  **Evidence**: `.sisyphus/evidence/task-6-types.txt`

  **Commit**: YES — `feat(types): add ResolvedBy to LockedPackage, Type+Source to LockedDep` — `pkg/types/lockfile.go`

- [x] 7. Lockfile v3 格式定义 + 写入

  **What to do**: `pkg/lockfile/generate.go`: `Version: 3`，填充 `ResolvedBy` + `Dep.Type` + `Dep.Source`

  **Must NOT do**: 不改排序逻辑

  **Recommended Agent Profile**: `quick`
  **Skills**: `[]`

  **Parallelization**: YES（与 T6, T8 并行）| **Wave 2** | **Blocks**: T9, T19 | **Blocked By**: Wave 1

  **References**: `pkg/lockfile/generate.go`

  **Acceptance Criteria**: 输出 `version: 3`，包有 `resolved_by`

  **QA Scenarios**: `go test -run TestGenerate -v ./pkg/lockfile/` → PASS
  **Evidence**: `.sisyphus/evidence/task-7-lockfile.txt`

  **Commit**: YES — `feat(lockfile): v3 format with resolved_by` — `pkg/lockfile/generate.go`, `pkg/lockfile/generate_test.go`

- [x] 8. Lockfile v1→v3 读取迁移

  **What to do**: 读 version < 3 → 内存迁移（ResolvedBy=""，Dep.Type/Source=""）

  **Must NOT do**: 不自动写回文件

  **Recommended Agent Profile**: `quick`
  **Skills**: `[]`

  **Parallelization**: YES（与 T6, T7 并行）| **Wave 2** | **Blocks**: T9 | **Blocked By**: Wave 1

  **References**: `pkg/lockfile/reader.go`

  **Acceptance Criteria**: v1 输入 → 解析成功，v3 输入 → ResolvedBy 保留

  **QA Scenarios**: `go test -run TestRead -v ./pkg/lockfile/` → PASS
  **Evidence**: `.sisyphus/evidence/task-8-migration.txt`

  **Commit**: YES — `feat(lockfile): v1→v3 auto-migration on read` — `pkg/lockfile/reader.go`, `pkg/lockfile/reader_test.go`

- [x] 9. Lockfile v3 往返测试

  **What to do**: 写→读→字段不丢失；v1→v3→v3 迁移往返

  **Recommended Agent Profile**: `quick`
  **Skills**: `[]`

  **Parallelization**: NO | **Wave 2 末尾** | **Blocks**: T19 | **Blocked By**: T6, T7, T8

  **Acceptance Criteria**: v3 往返字段完整，v1→v3→v3 正确

  **QA Scenarios**: `go test -run TestLockfileV3Roundtrip -v ./pkg/lockfile/` → PASS
  **Evidence**: `.sisyphus/evidence/task-9-roundtrip.txt`

  **Commit**: YES — `test(lockfile): v3 roundtrip and v1→v3 migration` — `pkg/lockfile/*_test.go`

- [x] 10. 标记旧 resolver 弃用

  **What to do**: `TopologicalBacktrackResolver` + `NewResolver()` + version.go 导出函数 → `// Deprecated:` 注释

  **Must NOT do**: 不删除任何函数

  **Recommended Agent Profile**: `quick`
  **Skills**: `[]`

  **Parallelization**: YES（与 T11-15 并行）| **Wave 3** | **Blocks**: None | **Blocked By**: Wave 1

  **Acceptance Criteria**: 每个弃用函数有注释，`go build ./...` 成功

  **QA Scenarios**: `go build ./... && go vet ./...` → exit 0
  **Evidence**: `.sisyphus/evidence/task-10-deprecated.txt`

  **Commit**: YES — `chore(resolver): deprecate old resolver functions` — `pkg/resolver/resolver.go`, `version.go`

- [x] 11. TDD RED: 简单图测试

  **What to do**: 创建 `pkg/resolver/pubgrub_resolver_test.go`，测试 PubGrubResolver 在无冲突简单图上的行为。单根 A@^1.0 依赖 B@>=2.0，B@2.1.0 存在，B 无依赖。验证：Resolver 返回 ResolvedPackage{A@1.x, B@2.1.0}

  **Must NOT do**: 不要实现 Resolve（只写测试）

  **Recommended Agent Profile**: `quick`
  **Skills**: `[]`

  **Parallelization**: YES（与 T10, T12-15 并行）| **Wave 3** | **Blocks**: T16 | **Blocked By**: Wave 1

  **References**: `pkg/resolver/resolver_test.go` — 现有 resolver 测试模式

  **Acceptance Criteria**: 测试编译通过但 FAIL（RED 阶段）

  **QA Scenarios**: `go test -run TestPubGrubSimpleGraph -v ./pkg/resolver/` → FAIL（预期）
  **Evidence**: `.sisyphus/evidence/task-11-simple-red.txt`

  **Commit**: YES — `test(resolver): TDD simple graph test for PubGrubResolver` — `pkg/resolver/pubgrub_resolver_test.go`

- [x] 12. TDD RED: 钻石冲突测试

  **What to do**: A@^1 → B@>=2.0，C@^1 → B@<2.0。B 不存在同时满足两者的版本。测试 PubGrubResolver 应返回包含 "A" "C" "B" 名字的冲突错误

  **Must NOT do**: 不实现

  **Recommended Agent Profile**: `unspecified-high`
  **Skills**: `[]`

  **Parallelization**: YES（与 T10, T11, T13-15 并行）| **Wave 3** | **Blocks**: T16 | **Blocked By**: Wave 1

  **Acceptance Criteria**: 测试 FAIL — Resolve 返回未实现错误（RED）

  **QA Scenarios**: `go test -run TestPubGrubDiamondConflict -v ./pkg/resolver/` → FAIL
  **Evidence**: `.sisyphus/evidence/task-12-diamond-red.txt`

  **Commit**: YES — `test(resolver): TDD diamond conflict test` — `pkg/resolver/pubgrub_resolver_test.go`

- [x] 13. TDD RED: 不可满足约束测试

  **What to do**: A@^1 → B@>=2.0，B 只有 1.0.0 版本。测试应失败——无版本满足约束

  **Recommended Agent Profile**: `unspecified-high`
  **Skills**: `[]`

  **Parallelization**: YES | **Wave 3** | **Blocks**: T16 | **Blocked By**: Wave 1

  **Acceptance Criteria**: FAIL（RED）
  **QA Scenarios**: `go test -run TestPubGrubUnsatisfiable -v ./pkg/resolver/` → FAIL
  **Evidence**: `.sisyphus/evidence/task-13-unsat-red.txt`
  **Commit**: YES — `test(resolver): TDD unsatisfiable constraint test`

- [x] 14. TDD RED: 深层依赖链冲突测试

  **What to do**: A→B→C→D→E，其中 D 和 E 各自要求不同版本。验证多层回溯正确

  **Recommended Agent Profile**: `unspecified-high`
  **Skills**: `[]`

  **Parallelization**: YES | **Wave 3** | **Blocks**: T16 | **Blocked By**: Wave 1

  **Acceptance Criteria**: FAIL（RED）
  **QA Scenarios**: `go test -run TestPubGrubDeepConflict -v ./pkg/resolver/` → FAIL
  **Evidence**: `.sisyphus/evidence/task-14-deep-red.txt`
  **Commit**: YES — `test(resolver): TDD deep chain conflict test`

- [x] 15. TDD RED: latest/* 约束测试

  **What to do**: 包无版本标签 → 约束 `latest` 或 `*` 应匹配 "latest" 版本

  **Recommended Agent Profile**: `quick`
  **Skills**: `[]`

  **Parallelization**: YES | **Wave 3** | **Blocks**: T16 | **Blocked By**: Wave 1

  **Acceptance Criteria**: FAIL（RED）
  **QA Scenarios**: `go test -run TestPubGrubLatest -v ./pkg/resolver/` → FAIL
  **Evidence**: `.sisyphus/evidence/task-15-latest-red.txt`
  **Commit**: YES — `test(resolver): TDD latest/* constraint test`

- [x] 16. PubGrubResolver.Resolve 核心实现 (GREEN)

  **What to do**:
  - 实现 `PubGrubResolver.Resolve()`:
    - 遍历 `[]PackageRequest` → 构建 `pubgrub.RootSource`（用 Task 2 EncodeName）
    - 从 `r.handlers` 构建每个源的 `agentenvSource` 列表 → `pubgrub.Source`
    - 调用 `pubgrub.NewSolver(root, sources...).EnableIncompatibilityTracking().Solve(root.Term())`
    - 从 Solution 构建 `ResolvedPackage` 列表（用 Task 2 DecodeName）
    - 处理 pubgrub 错误 → 映射到 agentenv 错误格式
  - 验证所有 TDD RED 测试变 GREEN

  **Must NOT do**: 不改变 Resolve 签名

  **Recommended Agent Profile**: `deep`
  **Skills**: `[]`

  **Parallelization**: NO | **Wave 4** | **Blocks**: T17-20 | **Blocked By**: Wave 3 (T11-15 RED)

  **References**: `pkg/resolver/resolver.go:150-401` — 原 Resolve 实现参考
  - pubgrub-go API: `https://pkg.go.dev/github.com/contriboss/pubgrub-go#Solver`

  **Acceptance Criteria**:
  - [ ] T11 简单图 → PASS（GREEN）
  - [ ] T12 钻石冲突 → PASS（GREEN）
  - [ ] T13 不可满足 → PASS（GREEN）
  - [ ] T14 深层链 → PASS（GREEN）
  - [ ] T15 latest/* → PASS（GREEN）

  **QA Scenarios**:
  ```
  Scenario: 所有 TDD 测试变绿
    Tool: Bash
    Steps:
      1. go test -run "TestPubGrub" -v ./pkg/resolver/
    Expected Result: 5 个测试全部 PASS
    Evidence: .sisyphus/evidence/task-16-green.txt
  ```

  **Commit**: YES
  - Message: `feat(resolver): PubGrubResolver.Resolve implementation (GREEN)`
  - Files: `pkg/resolver/pubgrub_resolver.go`

- [ ] 17. 冲突报告包装

  **What to do**:
  - pubgrub 的 `NoSolutionError` → agentenv 错误格式
  - 提取冲突链中的包名和版本 → 构造 `"version conflict: X requires Y@v1, but Z requires Y@v2"`
  - 实现 `pubgrub_err.go` — 错���包装函数

  **Must NOT do**: 不改变 pkg/errors 包

  **Recommended Agent Profile**: `unspecified-high`
  **Skills**: `[]`

  **Parallelization**: YES（与 T18, T19 并行）| **Wave 4** | **Blocks**: None | **Blocked By**: T16

  **References**: `pkg/errors/` — agentenv 错误类型
  - pubgrub NoSolutionError: `https://pkg.go.dev/github.com/contriboss/pubgrub-go#NoSolutionError`

  **Acceptance Criteria**:
  - [ ] 钻石冲突错误消息包含 "A", "C", "B"
  - [ ] 错误是 agentenv 的 SystemError 类型

  **QA Scenarios**: `go test -run TestPubGrubDiamondConflict -v ./pkg/resolver/` → PASS 且错误消息包含期望的包名
  **Evidence**: `.sisyphus/evidence/task-17-error.txt`

  **Commit**: YES — `feat(resolver): wrap pubgrub errors in agentenv format` — `pkg/resolver/pubgrub_err.go`

- [ ] 18. 适配器获取 + manifest 提取集成

  **What to do**:
  - 验证 Task 5 的适配器在真实的 `SourceHandler`（github/npm/local/git/file/url）下工作
  - 测试 `GetPackageDependencies` 在真实 tar.gz 上正确提取 agentpkg.yaml
  - 处理边界：损坏的 tar.gz、缺失 manifest、非 semver 版本

  **Must NOT do**: 不修改 source handler 实现

  **Recommended Agent Profile**: `deep`
  **Skills**: `[]`

  **Parallelization**: YES（与 T17, T19 并行）| **Wave 4** | **Blocks**: None | **Blocked By**: T16

  **References**: `pkg/source/github.go` — GitHub Fetch 实现参考
  - `pkg/source/local.go` — local 文件 source handler

  **Acceptance Criteria**:
  - [ ] GitHub source → 获取 tar.gz → 提取 manifest → 返回 Term 列表
  - [ ] 损坏 tar.gz → 返回错误（不 panic）
  - [ ] `go test -run TestAdapter_Integration -v ./pkg/resolver/` → PASS

  **QA Scenarios**: `go test -run TestAdapter_Integration -v ./pkg/resolver/` → PASS
  **Evidence**: `.sisyphus/evidence/task-18-integration.txt`

  **Commit**: YES — `test(resolver): adapter integration with real source handlers` — `pkg/resolver/pubgrub_adapter_test.go`

- [ ] 19. 回填 resolved_by 到 Lockfile v3

  **What to do**:
  - PubGrubResolver 的 Solution → 提取解析路径 → 填充每个 LockedPackage.ResolvedBy
  - 根包 → `resolved_by: "(root)"`
  - 传递依赖 → `resolved_by: "<父包名>"`
  - 集成到 `lock.go` 的生成流程

  **Must NOT do**: 不改变 Resolve 返回值签名

  **Recommended Agent Profile**: `unspecified-high`
  **Skills**: `[]`

  **Parallelization**: YES（与 T17, T18 并行）| **Wave 4** | **Blocks**: None | **Blocked By**: T16

  **References**: `cmd/agentenv/lock.go` — lock 命令的 resolver 调用和 lockfile 生成

  **Acceptance Criteria**:
  - [ ] 根包 ResolvedBy = "(root)"
  - [ ] 传递依赖 ResolvedBy = 父包名
  - [ ] `go test -run TestPubGrubResolvedBy -v ./pkg/resolver/` → PASS

  **QA Scenarios**: `go test -run TestPubGrubResolvedBy -v ./pkg/resolver/` → PASS
  **Evidence**: `.sisyphus/evidence/task-19-resolvedby.txt`

  **Commit**: YES — `feat(resolver): backfill resolved_by from PubGrub solution path` — `pkg/resolver/pubgrub_resolver.go`

- [ ] 20. 构造函数切换

  **What to do**:
  - `cmd/agentenv/lock.go:90-91`: `resolver.NewResolver()` → `resolver.NewPubGrubResolver(resolver.WithDepSource(depSource))`
  - `cmd/agentenv/install.go`: 同理
  - 确保配置选项正确传递

  **Must NOT do**: 不改变其他 cmd/ 代码

  **Recommended Agent Profile**: `quick`
  **Skills**: `[]`

  **Parallelization**: NO | **Wave 4 末尾** | **Blocks**: T21 | **Blocked By**: T16

  **References**: `cmd/agentenv/lock.go:90-91` — 当前构造函数调用
  - `cmd/agentenv/install.go` — 可能有额外的 resolver 使用

  **Acceptance Criteria**:
  - [ ] `go build ./cmd/agentenv/` → 成功
  - [ ] `agentenv lock` 使用 PubGrubResolver

  **QA Scenarios**: `go build ./... && go vet ./...` → exit 0
  **Evidence**: `.sisyphus/evidence/task-20-switch.txt`

  **Commit**: YES — `feat: switch to PubGrubResolver in lock/install` — `cmd/agentenv/lock.go`, `cmd/agentenv/install.go`

- [ ] 21. 所有现有测试回归验证

  **What to do**:
  - 运行完整测试套件：`go test -race ./...`
  - 更新因 resolver 切换而改变的错误字符串
  - 修复任何编译错误
  - 确认无新增 race

  **Must NOT do**: 不降低测试覆盖率

  **Recommended Agent Profile**: `unspecified-high`
  **Skills**: `[]`

  **Parallelization**: NO | **Wave 5** | **Blocks**: None | **Blocked By**: T20

  **Acceptance Criteria**:
  - [ ] `go test -race ./...` → 所有包 PASS（或已更新预期）
  - [ ] 失败数 ≤ 当前旧 resolver 特定测试（因行为变更）

  **QA Scenarios**: `go test -race ./... 2>&1 | tail -20` → 0 失败
  **Evidence**: `.sisyphus/evidence/task-21-regression.txt`

  **Commit**: YES — `test: update tests for PubGrub behavior changes` — 受影响的测试文件

- [ ] 22. 边界情况测试

  **What to do**:
  - 取消：context.Cancel → Resolve 应停止（检查 pubgrub Solve 是否接受 context）
  - 空包：0 个 request → 返回空结果
  - 无版本：包无发布版本 → 错误
  - 无标签：GitHub repo 无 tags → `latest` 约束应工作
  - 命名冲突：相同名称不同 type → 正确处理

  **Must NOT do**: 不重写已有测试

  **Recommended Agent Profile**: `unspecified-high`
  **Skills**: `[]`

  **Parallelization**: YES | **Wave 5** | **Blocks**: None | **Blocked By**: T20

  **Acceptance Criteria**:
  - [ ] 每个边界情况有对应测试
  - [ ] `go test -run TestPubGrubEdgeCases -v ./pkg/resolver/` → PASS

  **QA Scenarios**: `go test -run TestPubGrubEdgeCases -v ./pkg/resolver/` → 6+ 测试 PASS
  **Evidence**: `.sisyphus/evidence/task-22-edge.txt`

  **Commit**: YES — `test(resolver): edge cases for PubGrub (cancel/empty/notags/conflict)` — `pkg/resolver/pubgrub_resolver_test.go`

- [ ] 23. go.mod tidy + build + vet 最终检查

  **What to do**: `go mod tidy && go build ./... && go vet ./... && go test -race ./...`

  **Must NOT do**: 不跳过任何步骤

  **Recommended Agent Profile**: `quick`
  **Skills**: `[]`

  **Parallelization**: NO | **Wave 5 末尾** | **Blocks**: None | **Blocked By**: T21, T22

  **Acceptance Criteria**:
  - [ ] go mod tidy → 无变化（清洁）
  - [ ] go build ./... → 成功
  - [ ] go vet ./... → 干净
  - [ ] go test -race ./... → PASS

  **QA Scenarios**: 所有四个命令均 exit 0
  **Evidence**: `.sisyphus/evidence/task-23-final.txt`

  **Commit**: YES — `chore: final go mod tidy and verification` — `go.mod`, `go.sum`

---

## Final Verification Wave

> 4 review agents run in PARALLEL. ALL must APPROVE. Present results to user → get explicit "okay".

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read plan end-to-end. Verify: Must Have (7 items) all present; Must NOT Have (10 items) all absent. Check evidence files in `.sisyphus/evidence/`. Compare deliverables against plan.

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./...` + `go vet ./...` + `go test -race ./...`. Review all changed files for: `as any`/`@ts-ignore` equivalence, empty catches, dead code, AI slop (excessive comments, over-abstraction, generic names).

- [ ] F3. **Real Manual QA** — `unspecified-high`
  Execute EVERY QA scenario from EVERY task. Test cross-task integration. Verify: TDD tests all GREEN, lockfile v3 output correct, diamond conflict error clear, v1→v3 migration works. Save to `.sisyphus/evidence/final-qa/`.

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — nothing missing, nothing extra. Check Must NOT compliance. Detect cross-task contamination.

---

## Commit Strategy

- **Wave 1**: `chore: upgrade Go 1.25 + pubgrub-go` + `feat: namespace + constructor + adapter`
- **Wave 2**: `feat: lockfile v3 types + generation + migration + tests`
- **Wave 3**: `test: TDD conflict scenarios (RED)` + `chore: deprecate old resolver`
- **Wave 4**: `feat: PubGrubResolver.Resolve (GREEN)` + `feat: error + resolved_by + switch`
- **Wave 5**: `test: regression + edge cases + final verification`

## Success Criteria

### Verification Commands
```bash
go build ./...                              # 成功
go vet ./...                                # 干净
go test -race ./pkg/resolver/...            # 所有测试 PASS
go test -race ./pkg/lockfile/...            # v3 往返 PASS
go test -race ./...                         # 全部 PASS
./dist/agentenv lock                        # 产出 v3 锁文件含 resolved_by
./dist/agentenv --version                   # 正确版本
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] TDD: 5 RED → 5 GREEN
- [ ] v3 lockfile 往返正确
- [ ] v1→v3 迁移无损
- [ ] 钻石冲突错误清晰
- [ ] 旧 resolver 已弃用但仍在位

