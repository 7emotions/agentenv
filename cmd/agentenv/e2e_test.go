package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/7emotions/agentenv/pkg/adapter"
)

func setupAdapterForFramework(t *testing.T, home, framework string) {
	t.Helper()

	var testAdapter adapter.AgentAdapter
	switch framework {
	case "claude-code":
		testAdapter = adapter.NewClaudeCodeAdapterWithBase(filepath.Join(home, ".claude"))
	case "opencode":
		testAdapter = adapter.NewOpenCodeAdapterWithBaseAndHome(
			filepath.Join(home, ".config", "opencode"), home)
	case "cursor":
		testAdapter = adapter.NewCursorAdapterWithBaseAndHome(
			filepath.Join(home, ".cursor"), home)
	case "codex":
		testAdapter = adapter.NewCodexAdapterWithBaseAndHome(
			filepath.Join(home, ".codex"), home)
	default:
		t.Fatalf("unknown framework: %s", framework)
	}

	original := getAdapter
	getAdapter = func(f string) (adapter.AgentAdapter, error) {
		return testAdapter, nil
	}
	t.Cleanup(func() {
		getAdapter = original
	})
}

func setupMultiAdapter(t *testing.T, home string) {
	t.Helper()

	cc := adapter.NewClaudeCodeAdapterWithBase(filepath.Join(home, ".claude"))
	oc := adapter.NewOpenCodeAdapterWithBaseAndHome(
		filepath.Join(home, ".config", "opencode"), home)
	cur := adapter.NewCursorAdapterWithBaseAndHome(
		filepath.Join(home, ".cursor"), home)
	co := adapter.NewCodexAdapterWithBaseAndHome(
		filepath.Join(home, ".codex"), home)

	original := getAdapter
	getAdapter = func(framework string) (adapter.AgentAdapter, error) {
		switch framework {
		case "claude-code":
			return cc, nil
		case "opencode":
			return oc, nil
		case "cursor":
			return cur, nil
		case "codex":
			return co, nil
		default:
			return nil, fmt.Errorf("unknown framework: %s", framework)
		}
	}
	t.Cleanup(func() {
		getAdapter = original
	})
}

func fullCycleSkillTarGz() []byte {
	return createTestTarGz(map[string]string{
		"skill.md": "# E2E Skill\n\nAn end-to-end test skill.",
	})
}

func fullCycleAgentTarGz() []byte {
	return createTestTarGz(map[string]string{
		"my-agent.md": "# My Agent\n\nAn end-to-end test agent.\n",
	})
}

func fullCycleYAML(name string) string {
	return fmt.Sprintf(`name: %s
description: "E2E full cycle test"
skills:
  e2e-skill:
    source: github:test/e2e-skill
    version: "1.0.0"
mcps:
  e2e-mcp:
    source: npm:@test/e2e-server
    version: "1.0.0"
    config:
      command: npx
      args: ["-y", "@test/e2e-server"]
agents:
  e2e-agent:
    source: github:test/e2e-agent
    version: "1.0.0"
`, name)
}

const fullCycleLockfile = `version: 1
generated: "2026-05-24T12:00:00Z"
packages:
  - name: e2e-skill
    type: skill
    source: github:test/e2e-skill
    version: "1.0.0"
    resolved: 1.0.0
    sha256: abc123
  - name: e2e-mcp
    type: mcp
    source: npm:@test/e2e-server
    version: "1.0.0"
    resolved: 1.0.0
    sha256: def456
  - name: e2e-agent
    type: agent
    source: github:test/e2e-agent
    version: "1.0.0"
    resolved: 1.0.0
    sha256: ghi789
environment_snapshot:
  skills_count: 1
  mcps_count: 1
  agents_count: 1
`

func populateFullCycleStore(t *testing.T, home string) {
	t.Helper()
	populateStore(t, home, "skill", "github_test_e2e-skill", "1.0.0", fullCycleSkillTarGz())
	populateStore(t, home, "mcp", "npm_at_test_e2e-server", "1.0.0", []byte("dummy-mcp-data"))
	populateStore(t, home, "agent", "github_test_e2e-agent", "1.0.0", fullCycleAgentTarGz())
}

type e2ePaths struct {
	skillDir  string
	agentDir  string
	mcpPath   string
	skillName string
	agentName string
	mcpKey    string
	isTOML    bool
}

func e2ePathsForFramework(home, framework string) e2ePaths {
	switch framework {
	case "claude-code":
		return e2ePaths{
			skillDir:  filepath.Join(home, ".claude", "skills"),
			agentDir:  filepath.Join(home, ".claude", "agents"),
			mcpPath:   filepath.Join(home, ".claude", ".mcp.json"),
			skillName: "e2e-skill",
			agentName: "e2e-agent.md",
			mcpKey:    "mcpServers",
		}
	case "opencode":
		return e2ePaths{
			skillDir:  filepath.Join(home, ".config", "opencode", "skills"),
			agentDir:  filepath.Join(home, ".config", "opencode", "agents"),
			mcpPath:   filepath.Join(home, ".config", "opencode", "opencode.jsonc"),
			skillName: "e2e-skill",
			agentName: "e2e-agent.md",
			mcpKey:    "mcp",
		}
	case "cursor":
		return e2ePaths{
			skillDir:  filepath.Join(home, ".cursor", "skills"),
			agentDir:  filepath.Join(home, ".cursor", "agents"),
			mcpPath:   filepath.Join(home, ".cursor", "mcp.json"),
			skillName: "e2e-skill",
			agentName: "e2e-agent.md",
			mcpKey:    "mcpServers",
		}
	case "codex":
		return e2ePaths{
			mcpPath: filepath.Join(home, ".codex", "config.toml"),
			mcpKey:  "mcp_servers",
			isTOML:  true,
		}
	default:
		panic("unknown framework: " + framework)
	}
}

func verifyActiveLock(t *testing.T, home, expected string) {
	t.Helper()
	activeLock := filepath.Join(home, ".agentenv", "ACTIVE")
	data, err := os.ReadFile(activeLock)
	if err != nil {
		t.Fatalf("read ACTIVE lock: %v", err)
	}
	got := strings.TrimSpace(string(data))
	if got != expected {
		t.Errorf("ACTIVE lock = %q, want %q", got, expected)
	}
}

func verifyActiveLockRemoved(t *testing.T, home string) {
	t.Helper()
	activeLock := filepath.Join(home, ".agentenv", "ACTIVE")
	if _, err := os.Stat(activeLock); !os.IsNotExist(err) {
		t.Error("ACTIVE lock should be removed after deactivation")
	}
}

func verifySkillInstalled(t *testing.T, paths e2ePaths, framework string) {
	t.Helper()
	if paths.skillDir == "" {
		return
	}
	skillPath := filepath.Join(paths.skillDir, paths.skillName)
	info, err := os.Lstat(skillPath)
	if err != nil {
		t.Fatalf("skill not found at %s: %v", skillPath, err)
	}
	if framework == "claude-code" {
		if info.Mode()&os.ModeSymlink == 0 {
			t.Error("skill should be a symlink for claude-code")
		}
	} else {
		if !info.IsDir() {
			t.Errorf("skill should be a directory for %s, got: %v", framework, info.Mode())
		}
	}
}

func verifySkillRemoved(t *testing.T, paths e2ePaths) {
	t.Helper()
	if paths.skillDir == "" {
		return
	}
	skillPath := filepath.Join(paths.skillDir, paths.skillName)
	if _, err := os.Lstat(skillPath); !os.IsNotExist(err) {
		t.Errorf("skill %s should be removed", skillPath)
	}
}

func verifyMCPInstalled(t *testing.T, paths e2ePaths) {
	t.Helper()
	data, err := os.ReadFile(paths.mcpPath)
	if err != nil {
		t.Fatalf("read MCP config %s: %v", paths.mcpPath, err)
	}

	if paths.isTOML {
		if !strings.Contains(string(data), "e2e-mcp") {
			t.Error("config.toml should contain e2e-mcp server")
		}
		return
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse MCP config: %v", err)
	}
	servers, ok := raw[paths.mcpKey].(map[string]interface{})
	if !ok {
		t.Fatalf("%q key not found in MCP config", paths.mcpKey)
	}
	if _, ok := servers["e2e-mcp"]; !ok {
		t.Errorf("e2e-mcp not found under %q in MCP config", paths.mcpKey)
	}
}

func verifyMCPRemoved(t *testing.T, paths e2ePaths) {
	t.Helper()
	data, err := os.ReadFile(paths.mcpPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("read MCP config: %v", err)
	}

	if paths.isTOML {
		if strings.Contains(string(data), "[mcp_servers.e2e-mcp]") {
			t.Error("config.toml should not reference e2e-mcp after deactivation")
		}
		return
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	servers, ok := raw[paths.mcpKey].(map[string]interface{})
	if ok {
		if _, exists := servers["e2e-mcp"]; exists {
			t.Error("e2e-mcp should be removed from MCP config")
		}
	}
}

func verifyAgentInstalled(t *testing.T, paths e2ePaths) {
	t.Helper()
	if paths.agentDir == "" {
		return
	}
	agentPath := filepath.Join(paths.agentDir, paths.agentName)
	if _, err := os.Stat(agentPath); os.IsNotExist(err) {
		t.Fatalf("agent file not found: %s", agentPath)
	}
}

func verifyAgentRemoved(t *testing.T, paths e2ePaths) {
	t.Helper()
	if paths.agentDir == "" {
		return
	}
	agentPath := filepath.Join(paths.agentDir, paths.agentName)
	if _, err := os.Stat(agentPath); !os.IsNotExist(err) {
		t.Errorf("agent file should be removed: %s", agentPath)
	}
}

func verifyManifestHasItems(t *testing.T, home, framework string, wantSkills, wantMCPs, wantAgents int) {
	t.Helper()
	manifestPath := e2eManifestPath(home, framework)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m adapter.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(m.Skills) != wantSkills {
		t.Errorf("manifest skills = %d, want %d", len(m.Skills), wantSkills)
	}
	if len(m.MCPServers) != wantMCPs {
		t.Errorf("manifest mcps = %d, want %d", len(m.MCPServers), wantMCPs)
	}
	if len(m.Agents) != wantAgents {
		t.Errorf("manifest agents = %d, want %d", len(m.Agents), wantAgents)
	}
}

func fixFrameworkInState(t *testing.T, home, envName, framework string) {
	t.Helper()
	statePath := filepath.Join(home, ".agentenv", "envs", envName, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("unmarshal state.json: %v", err)
	}
	state["agent_framework"] = framework
	newData, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state.json: %v", err)
	}
	if err := os.WriteFile(statePath, newData, 0o644); err != nil {
		t.Fatalf("write state.json: %v", err)
	}
}

func e2eManifestPath(home, framework string) string {
	switch framework {
	case "claude-code":
		return filepath.Join(home, ".claude", ".agentenv-manifest.json")
	case "opencode":
		return filepath.Join(home, ".config", "opencode", ".agentenv-manifest.json")
	case "cursor":
		return filepath.Join(home, ".cursor", ".agentenv-manifest.json")
	case "codex":
		return filepath.Join(home, ".codex", ".agentenv-manifest.json")
	default:
		return filepath.Join(home, ".claude", ".agentenv-manifest.json")
	}
}

func e2eFullCycleTest(t *testing.T, framework, envName string) {
	home := newTestHome(t)
	setupAdapterForFramework(t, home, framework)

	createTestEnv(t, home, envName, fullCycleYAML(envName), fullCycleLockfile)
	fixFrameworkInState(t, home, envName, framework)
	populateFullCycleStore(t, home)

	internalActivateCmd.SetOut(&bytes.Buffer{})
	if err := internalActivateCmd.RunE(internalActivateCmd, []string{envName}); err != nil {
		t.Fatalf("activate %s: %v", framework, err)
	}

	expectedActive := fmt.Sprintf("%s:%s", envName, framework)
	verifyActiveLock(t, home, expectedActive)
	paths := e2ePathsForFramework(home, framework)
	verifySkillInstalled(t, paths, framework)
	verifyMCPInstalled(t, paths)
	verifyAgentInstalled(t, paths)
	verifyManifestHasItems(t, home, framework, 1, 1, 1)

	internalDeactivateCmd.SetOut(&bytes.Buffer{})
	if err := internalDeactivateCmd.RunE(internalDeactivateCmd, nil); err != nil {
		t.Fatalf("deactivate %s: %v", framework, err)
	}

	verifyActiveLockRemoved(t, home)
	verifySkillRemoved(t, paths)
	verifyAgentRemoved(t, paths)
	verifyManifestHasItems(t, home, framework, 0, 0, 0)
}

func TestE2E_ClaudeCode_FullCycle(t *testing.T) {
	e2eFullCycleTest(t, "claude-code", "e2e-cc")
}

func TestE2E_OpenCode_FullCycle(t *testing.T) {
	home := newTestHome(t)
	setupAdapterForFramework(t, home, "opencode")

	yamlContent := `name: e2e-oc
description: "OpenCode E2E test"
mcps:
  e2e-mcp:
    source: npm:@test/e2e-server
    version: "1.0.0"
    config:
      command: npx
      args: ["-y", "@test/e2e-server"]
agents:
  e2e-agent:
    source: github:test/e2e-agent
    version: "1.0.0"
`
	lockfileContent := `version: 1
generated: "2026-05-24T12:00:00Z"
packages:
  - name: e2e-mcp
    type: mcp
    source: npm:@test/e2e-server
    version: "1.0.0"
    resolved: 1.0.0
    sha256: abc123
  - name: e2e-agent
    type: agent
    source: github:test/e2e-agent
    version: "1.0.0"
    resolved: 1.0.0
    sha256: ghi789
environment_snapshot:
  mcps_count: 1
  agents_count: 1
`

	createTestEnv(t, home, "e2e-oc", yamlContent, lockfileContent)
	fixFrameworkInState(t, home, "e2e-oc", "opencode")
	populateStore(t, home, "mcp", "npm_at_test_e2e-server", "1.0.0", []byte("dummy-mcp-data"))
	populateStore(t, home, "agent", "github_test_e2e-agent", "1.0.0", fullCycleAgentTarGz())

	internalActivateCmd.SetOut(&bytes.Buffer{})
	if err := internalActivateCmd.RunE(internalActivateCmd, []string{"e2e-oc"}); err != nil {
		t.Fatalf("activate opencode: %v", err)
	}

	verifyActiveLock(t, home, "e2e-oc:opencode")
	paths := e2ePathsForFramework(home, "opencode")
	verifyMCPInstalled(t, paths)
	verifyAgentInstalled(t, paths)
	verifyManifestHasItems(t, home, "opencode", 0, 1, 1)

	internalDeactivateCmd.SetOut(&bytes.Buffer{})
	if err := internalDeactivateCmd.RunE(internalDeactivateCmd, nil); err != nil {
		t.Fatalf("deactivate opencode: %v", err)
	}

	verifyActiveLockRemoved(t, home)
	verifyAgentRemoved(t, paths)
	verifyManifestHasItems(t, home, "opencode", 0, 0, 0)
}

func TestE2E_Cursor_FullCycle(t *testing.T) {
	home := newTestHome(t)
	setupAdapterForFramework(t, home, "cursor")

	yamlContent := `name: e2e-cur
description: "Cursor E2E test"
mcps:
  e2e-mcp:
    source: npm:@test/e2e-server
    version: "1.0.0"
    config:
      command: npx
      args: ["-y", "@test/e2e-server"]
agents:
  e2e-agent:
    source: github:test/e2e-agent
    version: "1.0.0"
`
	lockfileContent := `version: 1
generated: "2026-05-24T12:00:00Z"
packages:
  - name: e2e-mcp
    type: mcp
    source: npm:@test/e2e-server
    version: "1.0.0"
    resolved: 1.0.0
    sha256: abc123
  - name: e2e-agent
    type: agent
    source: github:test/e2e-agent
    version: "1.0.0"
    resolved: 1.0.0
    sha256: ghi789
environment_snapshot:
  mcps_count: 1
  agents_count: 1
`

	createTestEnv(t, home, "e2e-cur", yamlContent, lockfileContent)
	fixFrameworkInState(t, home, "e2e-cur", "cursor")
	populateStore(t, home, "mcp", "npm_at_test_e2e-server", "1.0.0", []byte("dummy-mcp-data"))
	populateStore(t, home, "agent", "github_test_e2e-agent", "1.0.0", fullCycleAgentTarGz())

	internalActivateCmd.SetOut(&bytes.Buffer{})
	if err := internalActivateCmd.RunE(internalActivateCmd, []string{"e2e-cur"}); err != nil {
		t.Fatalf("activate cursor: %v", err)
	}

	verifyActiveLock(t, home, "e2e-cur:cursor")
	paths := e2ePathsForFramework(home, "cursor")
	verifyMCPInstalled(t, paths)
	verifyAgentInstalled(t, paths)
	verifyManifestHasItems(t, home, "cursor", 0, 1, 1)

	internalDeactivateCmd.SetOut(&bytes.Buffer{})
	if err := internalDeactivateCmd.RunE(internalDeactivateCmd, nil); err != nil {
		t.Fatalf("deactivate cursor: %v", err)
	}

	verifyActiveLockRemoved(t, home)
	verifyAgentRemoved(t, paths)
	verifyManifestHasItems(t, home, "cursor", 0, 0, 0)
}

func TestE2E_Codex_FullCycle(t *testing.T) {
	home := newTestHome(t)
	setupAdapterForFramework(t, home, "codex")

	yamlContent := `name: e2e-co
description: "Codex E2E test"
mcps:
  e2e-mcp:
    source: npm:@test/e2e-server
    version: "1.0.0"
    config:
      command: npx
      args: ["-y", "@test/e2e-server"]
`
	lockfileContent := `version: 1
generated: "2026-05-24T12:00:00Z"
packages:
  - name: e2e-mcp
    type: mcp
    source: npm:@test/e2e-server
    version: "1.0.0"
    resolved: 1.0.0
    sha256: abc123
environment_snapshot:
  mcps_count: 1
`

	createAgent = "codex"
	createFrom = ""
	createEmpty = false
	createTestEnv(t, home, "e2e-co", yamlContent, lockfileContent)
	fixFrameworkInState(t, home, "e2e-co", "codex")
	populateStore(t, home, "mcp", "npm_at_test_e2e-server", "1.0.0", []byte("dummy-mcp-data"))

	internalActivateCmd.SetOut(&bytes.Buffer{})
	if err := internalActivateCmd.RunE(internalActivateCmd, []string{"e2e-co"}); err != nil {
		t.Fatalf("activate codex: %v", err)
	}

	verifyActiveLock(t, home, "e2e-co:codex")
	paths := e2ePathsForFramework(home, "codex")
	verifyMCPInstalled(t, paths)
	verifyManifestHasItems(t, home, "codex", 0, 1, 0)

	internalDeactivateCmd.SetOut(&bytes.Buffer{})
	if err := internalDeactivateCmd.RunE(internalDeactivateCmd, nil); err != nil {
		t.Fatalf("deactivate codex: %v", err)
	}

	verifyActiveLockRemoved(t, home)
	verifyMCPRemoved(t, paths)
	verifyManifestHasItems(t, home, "codex", 0, 0, 0)
}

func TestE2E_CrossFramework_Isolation(t *testing.T) {
	home := newTestHome(t)
	setupMultiAdapter(t, home)

	ccYAML := `name: cc-isolated
description: "Claude Code isolated env"
mcps:
  e2e-mcp-a:
    source: npm:@test/e2e-mcp-a
    version: "1.0.0"
    config:
      command: npx
      args: ["-y", "@test/mcp-a"]
`
	ccLockfile := `version: 1
generated: "2026-05-24T12:00:00Z"
packages:
  - name: e2e-mcp-a
    type: mcp
    source: npm:@test/e2e-mcp-a
    version: "1.0.0"
    resolved: 1.0.0
    sha256: aaa111
environment_snapshot:
  mcps_count: 1
`

	createAgent = "claude-code"
	createFrom = ""
	createEmpty = false
	createTestEnv(t, home, "cc-isolated", ccYAML, ccLockfile)
	fixFrameworkInState(t, home, "cc-isolated", "claude-code")
	populateStore(t, home, "mcp", "npm_at_test_e2e-mcp-a", "1.0.0", []byte("mcp-a-data"))

	ocYAML := `name: oc-isolated
description: "OpenCode isolated env"
mcps:
  e2e-mcp-b:
    source: npm:@test/e2e-mcp-b
    version: "1.0.0"
    config:
      command: node
      args: ["mcp-b.js"]
`
	ocLockfile := `version: 1
generated: "2026-05-24T12:00:00Z"
packages:
  - name: e2e-mcp-b
    type: mcp
    source: npm:@test/e2e-mcp-b
    version: "1.0.0"
    resolved: 1.0.0
    sha256: bbb222
environment_snapshot:
  mcps_count: 1
`

	createAgent = "opencode"
	createTestEnv(t, home, "oc-isolated", ocYAML, ocLockfile)
	fixFrameworkInState(t, home, "oc-isolated", "opencode")
	populateStore(t, home, "mcp", "npm_at_test_e2e-mcp-b", "1.0.0", []byte("mcp-b-data"))

	internalActivateCmd.SetOut(&bytes.Buffer{})
	if err := internalActivateCmd.RunE(internalActivateCmd, []string{"cc-isolated"}); err != nil {
		t.Fatalf("activate cc-isolated: %v", err)
	}
	verifyActiveLock(t, home, "cc-isolated:claude-code")

	ccMCPPath := filepath.Join(home, ".claude", ".mcp.json")
	ccMCPData, err := os.ReadFile(ccMCPPath)
	if err != nil {
		t.Fatalf("read cc mcp: %v", err)
	}
	if !strings.Contains(string(ccMCPData), "e2e-mcp-a") {
		t.Error("mcp config missing e2e-mcp-a for claude-code")
	}

	internalActivateCmd.SetOut(&bytes.Buffer{})
	if err := internalActivateCmd.RunE(internalActivateCmd, []string{"oc-isolated"}); err != nil {
		t.Fatalf("activate oc-isolated: %v", err)
	}

	verifyActiveLock(t, home, "oc-isolated:opencode")

	ocMCPPath := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	ocMCPData, err := os.ReadFile(ocMCPPath)
	if err != nil {
		t.Fatalf("read oc mcp: %v", err)
	}
	if !strings.Contains(string(ocMCPData), "e2e-mcp-b") {
		t.Error("mcp config missing e2e-mcp-b for opencode")
	}

	internalDeactivateCmd.SetOut(&bytes.Buffer{})
	if err := internalDeactivateCmd.RunE(internalDeactivateCmd, nil); err != nil {
		t.Fatalf("deactivate oc-isolated: %v", err)
	}
	verifyActiveLockRemoved(t, home)
}

func TestE2E_EmptyEnvActivation(t *testing.T) {
	home := newTestHome(t)
	setupAdapterForFramework(t, home, "claude-code")

	yamlContent := `name: empty-e2e
description: "Empty e2e env"
`
	lockfileContent := `version: 1
generated: "2026-05-24T12:00:00Z"
packages: []
environment_snapshot:
  skills_count: 0
  mcps_count: 0
  agents_count: 0
`

	createAgent = "claude-code"
	createFrom = ""
	createEmpty = false
	createTestEnv(t, home, "empty-e2e", yamlContent, lockfileContent)

	var stdoutBuf bytes.Buffer
	internalActivateCmd.SetOut(&stdoutBuf)
	if err := internalActivateCmd.RunE(internalActivateCmd, []string{"empty-e2e"}); err != nil {
		t.Fatalf("activate empty env: %v", err)
	}

	verifyActiveLock(t, home, "empty-e2e:claude-code")

	var out activeOutput
	if err := json.Unmarshal(stdoutBuf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.Active != "empty-e2e" {
		t.Errorf("output.Active = %q, want 'empty-e2e'", out.Active)
	}

	internalDeactivateCmd.SetOut(&bytes.Buffer{})
	if err := internalDeactivateCmd.RunE(internalDeactivateCmd, nil); err != nil {
		t.Fatalf("deactivate empty env: %v", err)
	}
	verifyActiveLockRemoved(t, home)
}

func TestE2E_FailedInstallRollback(t *testing.T) {
	home := newTestHome(t)
	setupAdapterForFramework(t, home, "claude-code")

	yamlContent := `name: rollback-e2e
description: "Rollback e2e test"
skills:
  broken-skill:
    source: github:test/broken-skill
    version: "1.0.0"
`
	lockfileContent := `version: 1
generated: "2026-05-24T12:00:00Z"
packages:
  - name: broken-skill
    type: skill
    source: github:test/broken-skill
    version: "1.0.0"
    resolved: 1.0.0
    sha256: badbad
environment_snapshot:
  skills_count: 1
`

	createAgent = "claude-code"
	createFrom = ""
	createEmpty = false
	createTestEnv(t, home, "rollback-e2e", yamlContent, lockfileContent)

	populateStore(t, home, "skill", "github_test_broken-skill", "1.0.0", []byte("not-a-valid-tar-gz"))

	paths := e2ePathsForFramework(home, "claude-code")

	internalActivateCmd.SetOut(&bytes.Buffer{})
	err := internalActivateCmd.RunE(internalActivateCmd, []string{"rollback-e2e"})
	if err == nil {
		t.Fatal("expected activation to fail with corrupted package data")
	}

	activeLock := filepath.Join(home, ".agentenv", "ACTIVE")
	if _, statErr := os.Stat(activeLock); statErr == nil {
		data, _ := os.ReadFile(activeLock)
		t.Logf("ACTIVE lock exists with content: %s (partial success scenario)", string(data))
	}

	if paths.skillDir != "" {
		skillPath := filepath.Join(paths.skillDir, "broken-skill")
		if info, statErr := os.Lstat(skillPath); statErr == nil {
			t.Errorf("broken-skill should not be installed, but exists: %v", info.Mode())
		}
	}

	statePath := filepath.Join(home, ".agentenv", "envs", "rollback-e2e", "state.json")
	if _, statErr := os.Stat(statePath); os.IsNotExist(statErr) {
		t.Error("state.json should still exist after failed activation")
	}
}

func TestE2E_AllFrameworks_ActivateDeactivateMCP(t *testing.T) {
	frameworkTests := []struct {
		framework string
		mcpKey    string
	}{
		{"claude-code", "mcpServers"},
		{"opencode", "mcp"},
		{"cursor", "mcpServers"},
		{"codex", "mcp_servers"},
	}

	for _, ft := range frameworkTests {
		t.Run(ft.framework, func(t *testing.T) {
			home := newTestHome(t)
			setupAdapterForFramework(t, home, ft.framework)

			yamlContent := `name: mcp-only
description: "MCP-only test"
mcps:
  e2e-mcp:
    source: npm:@test/e2e-server
    version: "1.0.0"
    config:
      command: npx
      args: ["-y", "@test/e2e-server"]
`
			lockfileContent := `version: 1
generated: "2026-05-24T12:00:00Z"
packages:
  - name: e2e-mcp
    type: mcp
    source: npm:@test/e2e-server
    version: "1.0.0"
    resolved: 1.0.0
    sha256: abc123
environment_snapshot:
  mcps_count: 1
`

			createAgent = ft.framework
			createFrom = ""
			createEmpty = false
			createTestEnv(t, home, "mcp-only", yamlContent, lockfileContent)
			if ft.framework != "claude-code" {
				fixFrameworkInState(t, home, "mcp-only", ft.framework)
			}
			populateStore(t, home, "mcp", "npm_at_test_e2e-server", "1.0.0", []byte("dummy"))

			internalActivateCmd.SetOut(&bytes.Buffer{})
			if err := internalActivateCmd.RunE(internalActivateCmd, []string{"mcp-only"}); err != nil {
				t.Fatalf("activate %s: %v", ft.framework, err)
			}

			verifyActiveLock(t, home, "mcp-only:"+ft.framework)
			paths := e2ePathsForFramework(home, ft.framework)
			verifyMCPInstalled(t, paths)

			internalDeactivateCmd.SetOut(&bytes.Buffer{})
			if err := internalDeactivateCmd.RunE(internalDeactivateCmd, nil); err != nil {
				t.Fatalf("deactivate %s: %v", ft.framework, err)
			}

			verifyActiveLockRemoved(t, home)
		})
	}
}

func TestE2E_RapidActivateDeactivate(t *testing.T) {
	home := newTestHome(t)
	setupAdapterForFramework(t, home, "claude-code")

	createTestEnv(t, home, "rapid-env", fullCycleYAML("rapid-env"), fullCycleLockfile)
	populateFullCycleStore(t, home)

	paths := e2ePathsForFramework(home, "claude-code")

	for i := 0; i < 5; i++ {
		internalActivateCmd.SetOut(&bytes.Buffer{})
		if err := internalActivateCmd.RunE(internalActivateCmd, []string{"rapid-env"}); err != nil {
			t.Fatalf("cycle %d activate: %v", i, err)
		}
		verifyActiveLock(t, home, "rapid-env:claude-code")
		verifySkillInstalled(t, paths, "claude-code")

		internalDeactivateCmd.SetOut(&bytes.Buffer{})
		if err := internalDeactivateCmd.RunE(internalDeactivateCmd, nil); err != nil {
			t.Fatalf("cycle %d deactivate: %v", i, err)
		}
		verifyActiveLockRemoved(t, home)
		verifySkillRemoved(t, paths)
	}
}

func TestE2E_NoLockfileActivation(t *testing.T) {
	home := newTestHome(t)
	setupAdapterForFramework(t, home, "claude-code")

	createAgent = "claude-code"
	createFrom = ""
	createEmpty = false

	yamlContent := `name: nolock-e2e
description: "No lockfile test"
skills:
  some-skill:
    source: github:test/some-skill
    version: "1.0.0"
`
	createTestEnv(t, home, "nolock-e2e", yamlContent, "")

	var stdoutBuf bytes.Buffer
	internalActivateCmd.SetOut(&stdoutBuf)
	if err := internalActivateCmd.RunE(internalActivateCmd, []string{"nolock-e2e"}); err != nil {
		t.Fatalf("activate without lockfile: %v", err)
	}

	verifyActiveLock(t, home, "nolock-e2e:claude-code")

	var out activeOutput
	if err := json.Unmarshal(stdoutBuf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Active != "nolock-e2e" {
		t.Errorf("output.Active = %q, want 'nolock-e2e'", out.Active)
	}
	if out.Prompt != "(agentenv:nolock-e2e)" {
		t.Errorf("output.Prompt = %q, want '(agentenv:nolock-e2e)'", out.Prompt)
	}

	internalDeactivateCmd.SetOut(&bytes.Buffer{})
	if err := internalDeactivateCmd.RunE(internalDeactivateCmd, nil); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	verifyActiveLockRemoved(t, home)
}

func TestE2E_MultiplePackagesPerType(t *testing.T) {
	home := newTestHome(t)
	setupAdapterForFramework(t, home, "claude-code")

	yamlContent := `name: multi-e2e
description: "Multiple packages test"
skills:
  skill-one:
    source: github:test/skill-one
    version: "1.0.0"
  skill-two:
    source: github:test/skill-two
    version: "1.0.0"
mcps:
  mcp-one:
    source: npm:@test/mcp-one
    version: "1.0.0"
    config:
      command: cmd1
      args: ["one"]
  mcp-two:
    source: npm:@test/mcp-two
    version: "1.0.0"
    config:
      command: cmd2
      args: ["two"]
agents:
  agent-one:
    source: github:test/agent-one
    version: "1.0.0"
  agent-two:
    source: github:test/agent-two
    version: "1.0.0"
`
	lockfileContent := `version: 1
generated: "2026-05-24T12:00:00Z"
packages:
  - name: skill-one
    type: skill
    source: github:test/skill-one
    version: "1.0.0"
    resolved: 1.0.0
    sha256: aaa
  - name: skill-two
    type: skill
    source: github:test/skill-two
    version: "1.0.0"
    resolved: 1.0.0
    sha256: bbb
  - name: mcp-one
    type: mcp
    source: npm:@test/mcp-one
    version: "1.0.0"
    resolved: 1.0.0
    sha256: ccc
  - name: mcp-two
    type: mcp
    source: npm:@test/mcp-two
    version: "1.0.0"
    resolved: 1.0.0
    sha256: ddd
  - name: agent-one
    type: agent
    source: github:test/agent-one
    version: "1.0.0"
    resolved: 1.0.0
    sha256: eee
  - name: agent-two
    type: agent
    source: github:test/agent-two
    version: "1.0.0"
    resolved: 1.0.0
    sha256: fff
environment_snapshot:
  skills_count: 2
  mcps_count: 2
  agents_count: 2
`

	createAgent = "claude-code"
	createFrom = ""
	createEmpty = false
	createTestEnv(t, home, "multi-e2e", yamlContent, lockfileContent)

	skillData := fullCycleSkillTarGz()
	agentData := fullCycleAgentTarGz()

	populateStore(t, home, "skill", "github_test_skill-one", "1.0.0", skillData)
	populateStore(t, home, "skill", "github_test_skill-two", "1.0.0", skillData)
	populateStore(t, home, "mcp", "npm_at_test_mcp-one", "1.0.0", []byte("dummy1"))
	populateStore(t, home, "mcp", "npm_at_test_mcp-two", "1.0.0", []byte("dummy2"))
	populateStore(t, home, "agent", "github_test_agent-one", "1.0.0", agentData)
	populateStore(t, home, "agent", "github_test_agent-two", "1.0.0", agentData)

	internalActivateCmd.SetOut(&bytes.Buffer{})
	if err := internalActivateCmd.RunE(internalActivateCmd, []string{"multi-e2e"}); err != nil {
		t.Fatalf("activate multi-e2e: %v", err)
	}

	verifyActiveLock(t, home, "multi-e2e:claude-code")
	verifyManifestHasItems(t, home, "claude-code", 2, 2, 2)

	mcpPath := filepath.Join(home, ".claude", ".mcp.json")
	mcpData, _ := os.ReadFile(mcpPath)
	if !strings.Contains(string(mcpData), "mcp-one") || !strings.Contains(string(mcpData), "mcp-two") {
		t.Error("MCP config should contain both mcp-one and mcp-two")
	}

	internalDeactivateCmd.SetOut(&bytes.Buffer{})
	if err := internalDeactivateCmd.RunE(internalDeactivateCmd, nil); err != nil {
		t.Fatalf("deactivate multi-e2e: %v", err)
	}

	verifyActiveLockRemoved(t, home)
	verifyManifestHasItems(t, home, "claude-code", 0, 0, 0)
}

func TestE2E_ActivateNonexistent(t *testing.T) {
	home := newTestHome(t)
	os.MkdirAll(filepath.Join(home, ".agentenv"), 0o755)

	err := activateEnv(context.Background(), "ghost-e2e", internalActivateCmd)
	if err == nil {
		t.Fatal("expected error for non-existent env")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestE2E_DeactivateWithoutActive(t *testing.T) {
	home := newTestHome(t)
	os.MkdirAll(filepath.Join(home, ".agentenv"), 0o755)

	var stdoutBuf bytes.Buffer
	internalDeactivateCmd.SetOut(&stdoutBuf)

	if err := internalDeactivateCmd.RunE(internalDeactivateCmd, nil); err != nil {
		t.Fatalf("deactivate without active env: %v", err)
	}

	var out deactivateOutput
	if err := json.Unmarshal(stdoutBuf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Active {
		t.Error("expected active=false, got true")
	}
}

func TestE2E_AdapterRegistryHasAll(t *testing.T) {
	want := []string{"claude-code", "opencode", "cursor", "codex"}
	got := adapter.List()

	for _, name := range want {
		found := false
		for _, g := range got {
			if g == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("adapter %q not found in registry, got: %v", name, got)
		}
	}

	for _, name := range want {
		a, err := adapter.Get(name)
		if err != nil {
			t.Errorf("adapter.Get(%q): %v", name, err)
			continue
		}
		if a == nil {
			t.Errorf("adapter.Get(%q) returned nil adapter", name)
		}
		if a.Name() != name {
			t.Errorf("adapter.Name() = %q, want %q", a.Name(), name)
		}
	}
}
