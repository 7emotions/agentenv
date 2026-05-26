# Framework Version Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add framework version management to AgentEnv — `agent.yaml` declares a framework (e.g., `opencode`) with optional version constraint (e.g., `>=0.7.0`), and `agentenv activate` ensures the correct framework version is used. This completes the conda analogy: AgentEnv manages framework versions like conda manages Python versions.

**Architecture:** A singular `framework` section joins `agent.yaml`. The existing `--agent` flag on `create` already sets the framework in `state.json`; this moves it into `agent.yaml` as a declarative field with optional version constraint. No new package types, no deploy command, no special runtime handling. The existing adapter model (Claude Code, OpenCode, Cursor, Codex) remains the activation target.

**Tech Stack:** Go 1.23+, existing AgentEnv codebase (cobra + types + envfile + resolver + store + adapter).

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `pkg/types/environment.go` | Modify | Add `Framework *FrameworkSpec` field to `Environment` |
| `pkg/types/agent.go` | Modify | Add `FrameworkSpec` type |
| `pkg/envfile/reader.go` | Modify | Add `Framework` field to `EnvironmentSpec`, wire through `convertToSpec` |
| `pkg/envfile/writer.go` | Modify | Add `framework` section writing |
| `pkg/envfile/validator.go` | Modify | Add `validateFramework()` |
| `cmd/agentenv/create.go` | Modify | Write `framework` section to `agent.yaml` (not just `state.json`) |
| `cmd/agentenv/info.go` | Modify | Display framework info |
| `pkg/adapter/interface.go` | Modify | Add `GetInstalledVersion()` to check installed framework version |
| `cmd/agentenv/internal_activate.go` | Modify | Warn/block if framework version mismatch on activate |
| `pkg/envfile/envfile_test.go` | Modify | Test framework field round-trip |
| `cmd/agentenv/create_test.go` | Modify | Test framework written to agent.yaml |

---

### Task 1: Add `FrameworkSpec` Type and `Framework` Field

**Files:**
- Modify: `pkg/types/agent.go`
- Modify: `pkg/types/environment.go`

- [ ] **Step 1: Add `FrameworkSpec` type**

In `pkg/types/agent.go`, append:

```go
// FrameworkSpec describes the agent framework used by this environment.
type FrameworkSpec struct {
	Type    string `json:"type" yaml:"type"`       // "claude-code", "opencode", "cursor", "codex"
	Version string `json:"version,omitempty" yaml:"version,omitempty"` // semver constraint, e.g. ">=0.7.0"
}
```

- [ ] **Step 2: Add `Framework` field to `Environment`**

In `pkg/types/environment.go`, add after `Description`:

```go
type Environment struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Framework   *FrameworkSpec         `json:"framework,omitempty" yaml:"framework,omitempty"` // NEW
	Skills      map[string]PackageSpec `json:"skills,omitempty" yaml:"skills,omitempty"`
	MCPs        map[string]PackageSpec `json:"mcps,omitempty" yaml:"mcps,omitempty"`
	Agents      map[string]PackageSpec `json:"agents,omitempty" yaml:"agents,omitempty"`
	Tools       map[string]PackageSpec `json:"tools,omitempty" yaml:"tools,omitempty"`
	Hooks       map[string]PackageSpec `json:"hooks,omitempty" yaml:"hooks,omitempty"`
	Prompts     map[string]PackageSpec `json:"prompts,omitempty" yaml:"prompts,omitempty"`
}
```

Note: `FrameworkSpec` is in `pkg/types/agent.go`. Add import or move the type — it's fine since both are in `package types`.

- [ ] **Step 3: Verify compilation**

```bash
go build ./pkg/types/...
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add pkg/types/agent.go pkg/types/environment.go
git commit -m "feat(types): add FrameworkSpec type and Framework field to Environment"
```

---

### Task 2: Wire `framework` Through Envfile Layer

**Files:**
- Modify: `pkg/envfile/reader.go`
- Modify: `pkg/envfile/writer.go`
- Modify: `pkg/envfile/validator.go`

- [ ] **Step 1: Add `framework` to `knownTopLevelKeys`**

In `pkg/envfile/reader.go`:

```go
var knownTopLevelKeys = map[string]bool{
	"name":        true,
	"description": true,
	"framework":   true, // NEW
	"skills":      true,
	"mcps":        true,
	"agents":      true,
	"tools":       true,
	"hooks":       true,
	"prompts":     true,
}
```

- [ ] **Step 2: Add `Framework` to `EnvironmentSpec`**

In `pkg/envfile/reader.go`, add to `EnvironmentSpec`:

```go
type EnvironmentSpec struct {
	Name        string                `json:"name" yaml:"name"`
	Description string                `json:"description" yaml:"description"`
	Framework   *types.FrameworkSpec  `json:"framework,omitempty" yaml:"framework,omitempty"` // NEW
	Skills      map[string]PackageRef `json:"skills,omitempty" yaml:"skills,omitempty"`
	MCPs        map[string]PackageRef `json:"mcps,omitempty" yaml:"mcps,omitempty"`
	Agents      map[string]PackageRef `json:"agents,omitempty" yaml:"agents,omitempty"`
	Tools       map[string]PackageRef `json:"tools,omitempty" yaml:"tools,omitempty"`
	Hooks       map[string]PackageRef `json:"hooks,omitempty" yaml:"hooks,omitempty"`
	Prompts     map[string]PackageRef `json:"prompts,omitempty" yaml:"prompts,omitempty"`
}
```

Note: `FrameworkSpec` is not a `PackageRef` — it has `Type` and `Version`, not `Source`.

- [ ] **Step 3: Wire `Framework` in `convertToSpec`**

In `pkg/envfile/reader.go`:

```go
func convertToSpec(env *types.Environment) *EnvironmentSpec {
	spec := &EnvironmentSpec{
		Name:        env.Name,
		Description: env.Description,
	}
	if env.Framework != nil {
		spec.Framework = &types.FrameworkSpec{
			Type:    env.Framework.Type,
			Version: env.Framework.Version,
		}
	}
	// ... rest unchanged
```

- [ ] **Step 4: Write `framework` in `Write()`**

In `pkg/envfile/writer.go`, add before the package sections loop, after `description`:

```go
	// framework (singular, before package sections)
	if spec.Framework != nil {
		fwNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		fwNode.Content = append(fwNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "type", Tag: "!!str"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: spec.Framework.Type, Tag: "!!str"},
		)
		if spec.Framework.Version != "" && spec.Framework.Version != "*" {
			fwNode.Content = append(fwNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "version", Tag: "!!str"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: spec.Framework.Version, Tag: "!!str"},
			)
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "framework", Tag: "!!str"},
			fwNode,
		)
	}
```

- [ ] **Step 5: Handle `framework` in `Merge`**

In `pkg/envfile/writer.go` `Merge()`, add after description merge logic:

```go
	if overlay.Framework != nil {
		result.Framework = overlay.Framework
	} else if base.Framework != nil {
		result.Framework = base.Framework
	}
```

- [ ] **Step 6: Add `validateFramework` to validator**

In `pkg/envfile/validator.go`, add function:

```go
var supportedFrameworks = []string{"claude-code", "opencode", "cursor", "codex"}

func validateFramework(spec *EnvironmentSpec, result *ValidationResult) {
	if spec.Framework == nil {
		return // framework is optional — defaults to claude-code
	}

	fw := spec.Framework
	field := "framework"

	if fw.Type == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   field + ".type",
			Message: "framework type is required when framework is declared",
		})
		return
	}

	found := false
	for _, s := range supportedFrameworks {
		if fw.Type == s {
			found = true
			break
		}
	}
	if !found {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   field + ".type",
			Message: fmt.Sprintf("unsupported framework %q (supported: %s)", fw.Type, strings.Join(supportedFrameworks, ", ")),
		})
	}

	if fw.Version != "" && fw.Version != "*" && fw.Version != "latest" {
		if _, err := semver.NewConstraint(fw.Version); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   field + ".version",
				Message: fmt.Sprintf("invalid version constraint %q: %v", fw.Version, err),
			})
		}
	}
}
```

Call it in `Validate()` after `validateDescription`:

```go
validateFramework(spec, result)
```

- [ ] **Step 7: Verify compilation and run tests**

```bash
go build ./pkg/envfile/...
go test ./pkg/envfile/... -v
```
Expected: existing tests PASS.

- [ ] **Step 8: Commit**

```bash
git add pkg/envfile/reader.go pkg/envfile/writer.go pkg/envfile/validator.go
git commit -m "feat(envfile): add framework field to EnvironmentSpec and agent.yaml handling"
```

---

### Task 3: Write `framework` to `agent.yaml` on `create`

**Files:**
- Modify: `cmd/agentenv/create.go`

Currently `create --agent opencode` writes framework choice only to `state.json`. We need it in `agent.yaml` too.

- [ ] **Step 1: Modify `create` to write framework in agent.yaml**

In `cmd/agentenv/create.go`, modify the YAML generation in the non-`--from`, non-`--empty` branch:

```go
} else if !createEmpty {
	yamlLines := []string{
		fmt.Sprintf("name: %s", name),
		fmt.Sprintf(`description: "Environment created on %s"`, time.Now().UTC().Format(time.RFC3339)),
	}
	if createAgent != "" {
		yamlLines = append(yamlLines, "framework:", fmt.Sprintf("  type: %s", createAgent))
	}
	yamlContent := strings.Join(yamlLines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(envDir, "agent.yaml"), []byte(yamlContent), 0o644); err != nil {
		failed = true
		return agentenvError.SystemError(
			fmt.Sprintf("Cannot write agent.yaml to %s", envDir),
			"Check file permissions and disk space.").WithCause(err)
	}
}
```

Add `"strings"` to the imports if not already present.

- [ ] **Step 2: Verify compilation and run create tests**

```bash
go build ./cmd/agentenv/...
go test ./cmd/agentenv/... -v -run TestCreate
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/agentenv/create.go
git commit -m "feat(create): write framework declaration to agent.yaml"
```

---

### Task 4: Add `GetInstalledVersion` to Adapter Interface

**Files:**
- Modify: `pkg/adapter/interface.go`
- Modify: `pkg/adapter/opencode.go` (and other adapters as needed)

Each adapter already knows how to detect its framework (currently via `Detect()` in `agentctl` style, but AgentEnv's adapters have no detection method). Add a method to check the installed version.

- [ ] **Step 1: Add to interface**

In `pkg/adapter/interface.go`:

```go
	// GetInstalledVersion returns the currently installed version of this agent
	// framework (e.g., "0.7.3"). Returns ("", nil) if the framework is not installed
	// or the version cannot be determined.
	GetInstalledVersion(ctx context.Context) (string, error)
```

- [ ] **Step 2: Implement for OpenCode**

In `pkg/adapter/opencode.go`, add method. Check version via `opencode --version` or parse from installed binary path:

```go
func (a *OpenCodeAdapter) GetInstalledVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "opencode", "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", nil // not installed or unknown version — non-fatal
	}
	return strings.TrimSpace(string(out)), nil
}
```

- [ ] **Step 3: Stub for other adapters**

For `claude_code.go`, `cursor.go`, `codex.go` — return `("", nil)` as a stub. Each framework needs its own version detection logic, but that can be added incrementally.

```go
func (a *ClaudeCodeAdapter) GetInstalledVersion(ctx context.Context) (string, error) {
	return "", nil // TODO: implement version detection
}
```

- [ ] **Step 4: Verify compilation**

```bash
go build ./pkg/adapter/...
```
Expected: PASS (all adapters implement the new method).

- [ ] **Step 5: Commit**

```bash
git add pkg/adapter/interface.go pkg/adapter/opencode.go pkg/adapter/claude_code.go pkg/adapter/cursor.go pkg/adapter/codex.go
git commit -m "feat(adapter): add GetInstalledVersion to AgentAdapter interface"
```

---

### Task 5: Version Check on `activate`

**Files:**
- Modify: `cmd/agentenv/internal_activate.go`

When the user runs `agentenv activate <name>`, check if the installed framework version satisfies the constraint declared in `agent.yaml`. Warn on mismatch, block on incompatible version.

- [ ] **Step 1: Read `cmd/agentenv/internal_activate.go`**

Read the file to understand the existing activation flow.

- [ ] **Step 2: Add version check logic**

After reading `agent.yaml` in the activate flow, add:

```go
	// Check framework version constraint
	if spec.Framework != nil && spec.Framework.Version != "" && spec.Framework.Version != "*" {
		adapter, err := adapter.Get(spec.Framework.Type)
		if err != nil {
			return agentenvError.UserError(
				fmt.Sprintf("Framework %q not supported", spec.Framework.Type),
				"Run 'agentenv list-adapters' to see available frameworks.")
		}

		installedVersion, err := adapter.GetInstalledVersion(context.Background())
		if err != nil {
			// Version check failed — warn but continue
			fmt.Fprintf(cmd.ErrOrStderr(), "⚠ Cannot determine installed version of %s\n", spec.Framework.Type)
		} else if installedVersion != "" {
			constraint, err := semver.NewConstraint(spec.Framework.Version)
			if err != nil {
				return agentenvError.UserError(
					fmt.Sprintf("Invalid framework version constraint %q", spec.Framework.Version),
					"Fix the version constraint in agent.yaml.")
			}

			installed, err := semver.NewVersion(installedVersion)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "⚠ Cannot parse installed version %q of %s\n", installedVersion, spec.Framework.Type)
			} else if !constraint.Check(installed) {
				return agentenvError.UserError(
					fmt.Sprintf("Framework %s version %s does not satisfy constraint %s (installed: %s)",
						spec.Framework.Type, spec.Framework.Version, spec.Framework.Version, installedVersion),
					fmt.Sprintf("Install %s %s or update the version constraint in agent.yaml.", spec.Framework.Type, spec.Framework.Version))
			}
		}
	}
```

Note: this requires importing `"github.com/Masterminds/semver/v3"` (already a dependency) and `"github.com/7emotions/agentenv/pkg/adapter"`.

- [ ] **Step 3: Verify compilation and run activate tests**

```bash
go build ./cmd/agentenv/...
go test ./cmd/agentenv/... -v -run TestActivate
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/agentenv/internal_activate.go
git commit -m "feat(activate): check framework version constraint on activation"
```

---

### Task 6: Display Framework in `agentenv info`

**Files:**
- Modify: `cmd/agentenv/info.go`

The `info` command should show the framework declaration from `agent.yaml`.

- [ ] **Step 1: Read `cmd/agentenv/info.go`**

Read to understand the current output format.

- [ ] **Step 2: Add framework to info output**

In the `info` command's output, add after the environment name/description:

```go
	if spec.Framework != nil {
		fwInfo := spec.Framework.Type
		if spec.Framework.Version != "" && spec.Framework.Version != "*" {
			fwInfo += fmt.Sprintf(" (%s)", spec.Framework.Version)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Framework: %s\n", fwInfo)
	}
```

For JSON output, add a `"framework"` key to the info map.

- [ ] **Step 3: Verify compilation and run info tests**

```bash
go build ./cmd/agentenv/...
go test ./cmd/agentenv/... -v -run TestInfo
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/agentenv/info.go
git commit -m "feat(info): display framework declaration in agentenv info"
```

---

### Task 7: Add Tests

**Files:**
- Modify: `pkg/envfile/envfile_test.go`
- Modify: `cmd/agentenv/create_test.go`

- [ ] **Step 1: Test framework field round-trip in envfile**

In `pkg/envfile/envfile_test.go`:

```go
func TestFrameworkFieldRoundTrip(t *testing.T) {
	spec := &EnvironmentSpec{
		Name: "test-env",
		Framework: &types.FrameworkSpec{
			Type:    "opencode",
			Version: ">=0.7.0",
		},
	}

	data, err := Write(spec)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := Read(data)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got.Framework == nil {
		t.Fatal("Framework field is nil after round-trip")
	}
	if got.Framework.Type != "opencode" {
		t.Errorf("Framework.Type = %q, want %q", got.Framework.Type, "opencode")
	}
	if got.Framework.Version != ">=0.7.0" {
		t.Errorf("Framework.Version = %q, want %q", got.Framework.Version, ">=0.7.0")
	}
}

func TestFrameworkFieldOptional(t *testing.T) {
	spec := &EnvironmentSpec{Name: "test-env"}
	data, err := Write(spec)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := Read(data)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got.Framework != nil {
		t.Error("Framework should be nil when not set")
	}
}

func TestFrameworkInvalidType(t *testing.T) {
	spec := &EnvironmentSpec{
		Name: "test-env",
		Framework: &types.FrameworkSpec{
			Type: "invalid-fw",
		},
	}
	result := Validate(spec)
	if result.Valid {
		t.Error("Expected validation failure for unsupported framework type")
	}
}
```

- [ ] **Step 2: Test create writes framework to agent.yaml**

In `cmd/agentenv/create_test.go`:

```go
func TestCreateWritesFrameworkToYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	createCmd.SetArgs([]string{"test-fw", "--agent", "opencode"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	yamlPath := filepath.Join(tmp, ".agentenv", "envs", "test-fw", "agent.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read agent.yaml: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "framework:") {
		t.Error("agent.yaml should contain framework field")
	}
	if !strings.Contains(content, "type: opencode") {
		t.Error("agent.yaml should contain framework type")
	}
}
```

- [ ] **Step 3: Run all tests**

```bash
go test ./... -v
```
Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add pkg/envfile/envfile_test.go cmd/agentenv/create_test.go
git commit -m "test: add tests for framework field and create command"
```

---

### Task 8: Update Documentation

**Files:**
- Modify: `README.md`
- Modify: `docs/agent-yaml.md`

- [ ] **Step 1: Update `docs/agent-yaml.md`**

Add to Top-Level Fields table:

```markdown
| `framework` | object | no | Framework declaration. `type` (required) + `version` (optional semver constraint). |
```

Add example:

```yaml
framework:
  type: opencode
  version: ">=0.7.0"
```

- [ ] **Step 2: Update `README.md`**

Update the `agent.yaml` example to include `framework`. Update the create command example. Add a note about version pinning.

- [ ] **Step 3: Commit**

```bash
git add README.md docs/agent-yaml.md
git commit -m "docs: document framework field in agent.yaml"
```

---

## What This Plan Does NOT Do

- **No deploy command.** The activation target remains the developer's local framework (OpenCode/Cursor/Claude Code/Codex).
- **No AgentMini/gocode integration.** gocode can be supported later as a framework adapter, on par with OpenCode/Cursor — no special treatment.
- **No runtime binary download.** Framework installation is handled by the user (or can be added later as `agentenv install --framework`). This plan focuses on version *checking*, not downloading.
- **No changes to the PubGrub resolver.** The framework is not a package — it doesn't go through the dependency graph.
- **No GitHub release fetcher.** Not needed in this scope.

---

## Self-Review Checklist

1. **Spec coverage**: Framework type → DONE (Task 1). Envfile handling → DONE (Task 2). Create command → DONE (Task 3). Adapter version check → DONE (Task 4). Activate version check → DONE (Task 5). Info display → DONE (Task 6). Tests → DONE (Task 7). Docs → DONE (Task 8).

2. **Placeholder scan**: No TBDs, no TODOs. All methods have concrete implementations. Adapter stubs for version detection are explicitly noted as incremental.

3. **Type consistency**: `FrameworkSpec` used in types (Task 1), envfile (Task 2), activate (Task 5), info (Task 6). `GetInstalledVersion` defined in interface (Task 4), called in activate (Task 5).

4. **Minimal scope**: This plan adds ~300 lines across 8 tasks. No new commands, no new source schemes, no structural changes to the resolver or store.
