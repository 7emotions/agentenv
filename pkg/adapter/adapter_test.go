package adapter

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/7emotions/agentenv/pkg/types"
)

func newTestAdapter(t *testing.T) (*ClaudeCodeAdapter, string) {
	t.Helper()
	basePath := t.TempDir()
	return NewClaudeCodeAdapterWithBase(basePath), basePath
}

func TestInstallSkill(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	storePath := filepath.Join(basePath, "store", "code-review")
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := a.InstallSkill(ctx, "code-review", storePath); err != nil {
		t.Fatalf("InstallSkill: %v", err)
	}

	linkPath := filepath.Join(basePath, "skills", "code-review")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if target != storePath {
		t.Errorf("symlink target = %q, want %q", target, storePath)
	}

	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	item, ok := manifest.Skills["code-review"]
	if !ok {
		t.Fatal("code-review not in manifest")
	}
	if item.SourcePath != storePath {
		t.Errorf("SourcePath = %q, want %q", item.SourcePath, storePath)
	}
	if item.InstalledAt == "" {
		t.Error("InstalledAt should not be empty")
	}
}

func TestRemoveSkill(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	storePath := filepath.Join(basePath, "store", "code-review")
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := a.InstallSkill(ctx, "code-review", storePath); err != nil {
		t.Fatalf("InstallSkill: %v", err)
	}

	if err := a.RemoveSkill(ctx, "code-review"); err != nil {
		t.Fatalf("RemoveSkill: %v", err)
	}

	linkPath := filepath.Join(basePath, "skills", "code-review")
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Errorf("symlink should be removed, err = %v", err)
	}

	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if _, ok := manifest.Skills["code-review"]; ok {
		t.Error("code-review should not be in manifest after removal")
	}
}

func TestRemoveSkill_NotInManifest(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	skillsDir := filepath.Join(basePath, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userSkillPath := filepath.Join(skillsDir, "user-skill")
	if err := os.MkdirAll(userSkillPath, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := a.RemoveSkill(ctx, "user-skill"); err != nil {
		t.Fatalf("RemoveSkill should not error for non-managed skill: %v", err)
	}

	if _, err := os.Stat(userSkillPath); os.IsNotExist(err) {
		t.Error("user skill should not have been removed")
	}
}

func TestInstallSkill_ExistingUserSkill(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	skillsDir := filepath.Join(basePath, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userSkillPath := filepath.Join(skillsDir, "user-skill")
	if err := os.MkdirAll(userSkillPath, 0o755); err != nil {
		t.Fatal(err)
	}

	storePath := filepath.Join(basePath, "store", "user-skill")
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		t.Fatal(err)
	}

	err := a.InstallSkill(ctx, "user-skill", storePath)
	if err == nil {
		t.Fatal("expected error when installing over user skill")
	}

	if _, err := os.Stat(userSkillPath); os.IsNotExist(err) {
		t.Error("user skill should still exist")
	}
}

func TestListSkills(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	for _, name := range []string{"skill-a", "skill-b"} {
		storePath := filepath.Join(basePath, "store", name)
		if err := os.MkdirAll(storePath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := a.InstallSkill(ctx, name, storePath); err != nil {
			t.Fatalf("InstallSkill(%q): %v", name, err)
		}
	}

	names, err := a.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("got %d skills, want 2: %v", len(names), names)
	}
}

func TestListSkills_Empty(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestAdapter(t)

	names, err := a.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("got %d skills, want 0", len(names))
	}
}

func TestInstallSkill_ReplacesManaged(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	storePath1 := filepath.Join(basePath, "store", "v1")
	if err := os.MkdirAll(storePath1, 0o755); err != nil {
		t.Fatal(err)
	}
	storePath2 := filepath.Join(basePath, "store", "v2")
	if err := os.MkdirAll(storePath2, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := a.InstallSkill(ctx, "my-skill", storePath1); err != nil {
		t.Fatalf("first InstallSkill: %v", err)
	}

	if err := a.InstallSkill(ctx, "my-skill", storePath2); err != nil {
		t.Fatalf("second InstallSkill (replace): %v", err)
	}

	linkPath := filepath.Join(basePath, "skills", "my-skill")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if target != storePath2 {
		t.Errorf("symlink target = %q, want %q", target, storePath2)
	}
}

func TestRegistry_Get(t *testing.T) {
	a, err := Get("claude-code")
	if err != nil {
		t.Fatalf("Get('claude-code'): %v", err)
	}
	if a.Name() != "claude-code" {
		t.Errorf("Name = %q, want 'claude-code'", a.Name())
	}
	if a.DisplayName() != "Claude Code" {
		t.Errorf("DisplayName = %q, want 'Claude Code'", a.DisplayName())
	}
}

func TestRegistry_GetNonexistent(t *testing.T) {
	_, err := Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent adapter")
	}
}

func TestRegistry_List(t *testing.T) {
	names := List()
	found := false
	for _, n := range names {
		if n == "claude-code" {
			found = true
		}
	}
	if !found {
		t.Errorf("List() should include 'claude-code', got %v", names)
	}
}

func TestRegistry_DoubleRegisterPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on double register")
		}
	}()
	Register("claude-code", NewClaudeCodeAdapterWithBase(t.TempDir()))
}

func TestRegistry_EmptyNamePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty name")
		}
	}()
	Register("", NewClaudeCodeAdapterWithBase(t.TempDir()))
}

func TestRegistryRace(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := Get("claude-code")
			if err != nil {
				t.Errorf("Get('claude-code'): %v", err)
			}
			_ = List()
		}()
	}
	wg.Wait()
}

func TestManifestRoundTrip(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestAdapter(t)

	orig := &Manifest{
		Version: 1,
		EnvName: "test-env",
		Skills: map[string]ManifestItem{
			"skill-a": {
				Name:        "skill-a",
				Version:     "1.0.0",
				SourcePath:  "/store/skills/skill-a",
				InstalledAt: "2026-05-24T12:00:00Z",
			},
		},
	}

	if err := a.WriteManifest(ctx, orig); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	got, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	if got.Version != orig.Version {
		t.Errorf("Version = %d, want %d", got.Version, orig.Version)
	}
	if got.EnvName != orig.EnvName {
		t.Errorf("EnvName = %q, want %q", got.EnvName, orig.EnvName)
	}
	if len(got.Skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(got.Skills))
	}
	item := got.Skills["skill-a"]
	if item.Name != "skill-a" || item.Version != "1.0.0" || item.SourcePath != "/store/skills/skill-a" {
		t.Errorf("unexpected manifest item: %+v", item)
	}
}

func TestReadManifest_NotExists(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestAdapter(t)

	m, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if m.Version != 1 {
		t.Errorf("Version = %d, want 1", m.Version)
	}
	if len(m.Skills) != 0 {
		t.Errorf("expected empty skills, got %d", len(m.Skills))
	}
}

// --- Agent tests ---

func TestInstallAgent(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	sourcePath := filepath.Join(basePath, "store", "my-agent.md")
	sourceContent := "# My Agent\n\nThis is a test agent definition."
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.InstallAgent(ctx, "my-agent", sourcePath); err != nil {
		t.Fatalf("InstallAgent: %v", err)
	}

	agentPath := filepath.Join(basePath, "agents", "my-agent.md")
	data, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != sourceContent {
		t.Errorf("content = %q, want %q", string(data), sourceContent)
	}

	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	item, ok := manifest.Agents["my-agent"]
	if !ok {
		t.Fatal("my-agent not in manifest")
	}
	if item.SourcePath != sourcePath {
		t.Errorf("SourcePath = %q, want %q", item.SourcePath, sourcePath)
	}
	if item.InstalledAt == "" {
		t.Error("InstalledAt should not be empty")
	}
}

func TestRemoveAgent(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	sourcePath := filepath.Join(basePath, "store", "my-agent.md")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("# Agent"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.InstallAgent(ctx, "my-agent", sourcePath); err != nil {
		t.Fatalf("InstallAgent: %v", err)
	}

	if err := a.RemoveAgent(ctx, "my-agent"); err != nil {
		t.Fatalf("RemoveAgent: %v", err)
	}

	agentPath := filepath.Join(basePath, "agents", "my-agent.md")
	if _, err := os.Stat(agentPath); !os.IsNotExist(err) {
		t.Errorf("agent file should be removed, err = %v", err)
	}

	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if _, ok := manifest.Agents["my-agent"]; ok {
		t.Error("my-agent should not be in manifest after removal")
	}
}

func TestRemoveAgent_NotInManifest(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	agentsDir := filepath.Join(basePath, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	agentPath := filepath.Join(agentsDir, "user-agent.md")
	if err := os.WriteFile(agentPath, []byte("# User Agent"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.RemoveAgent(ctx, "user-agent"); err != nil {
		t.Fatalf("RemoveAgent should not error for non-managed agent: %v", err)
	}

	if _, err := os.Stat(agentPath); os.IsNotExist(err) {
		t.Error("user agent should not have been removed")
	}
}

func TestListAgents(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	for _, name := range []string{"agent-a", "agent-b"} {
		sourcePath := filepath.Join(basePath, "store", name+".md")
		if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sourcePath, []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := a.InstallAgent(ctx, name, sourcePath); err != nil {
			t.Fatalf("InstallAgent(%q): %v", name, err)
		}
	}

	names, err := a.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("got %d agents, want 2: %v", len(names), names)
	}
}

func TestListAgents_Empty(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestAdapter(t)

	names, err := a.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("got %d agents, want 0", len(names))
	}
}

func TestGetAgentBasePath(t *testing.T) {
	a, basePath := newTestAdapter(t)

	expected := filepath.Join(basePath, "agents")
	if got := a.GetAgentBasePath(); got != expected {
		t.Errorf("GetAgentBasePath() = %q, want %q", got, expected)
	}
}

func readFileJSON(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	var v map[string]interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return v
}

func TestInstallMCP(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	server := types.UniversalMCPServer{
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "."},
		Env:     map[string]string{"NODE_ENV": "production"},
	}
	if err := a.InstallMCP(ctx, "filesystem", server); err != nil {
		t.Fatalf("InstallMCP: %v", err)
	}

	// Verify .mcp.json
	mcpPath := filepath.Join(basePath, ".mcp.json")
	cfg := readFileJSON(t, mcpPath)
	mcpServers, ok := cfg["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal(".mcp.json missing mcpServers key")
	}
	fs, ok := mcpServers["filesystem"].(map[string]interface{})
	if !ok {
		t.Fatal("filesystem not in mcpServers")
	}
	if fs["command"] != "npx" {
		t.Errorf("command = %q, want npx", fs["command"])
	}
	if _, ok := fs["transport"]; ok {
		t.Error("transport field should not appear in Claude Code format")
	}

	// Verify manifest
	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if _, ok := manifest.MCPServers["filesystem"]; !ok {
		t.Error("filesystem not in manifest MCPServers")
	}

	// Verify backup exists
	home := filepath.Dir(basePath)
	backupDir := filepath.Join(home, ".agentenv", "backups", "claude-code")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("ReadDir backup: %v", err)
	}
	if len(entries) < 1 {
		t.Fatal("no backup files found")
	}
}

func TestRemoveMCP(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	// Install two servers
	srvA := types.UniversalMCPServer{Command: "a"}
	srvB := types.UniversalMCPServer{Command: "b"}
	if err := a.InstallMCP(ctx, "server-a", srvA); err != nil {
		t.Fatalf("InstallMCP server-a: %v", err)
	}
	if err := a.InstallMCP(ctx, "server-b", srvB); err != nil {
		t.Fatalf("InstallMCP server-b: %v", err)
	}

	// Verify both present
	cfg, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}
	if _, ok := cfg.Servers["server-a"]; !ok {
		t.Fatal("server-a not in config")
	}
	if _, ok := cfg.Servers["server-b"]; !ok {
		t.Fatal("server-b not in config")
	}

	// Remove server-a
	if err := a.RemoveMCP(ctx, "server-a"); err != nil {
		t.Fatalf("RemoveMCP: %v", err)
	}

	// Verify server-a gone, server-b preserved
	cfg, err = a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig after remove: %v", err)
	}
	if _, ok := cfg.Servers["server-a"]; ok {
		t.Error("server-a should be removed from config")
	}
	if _, ok := cfg.Servers["server-b"]; !ok {
		t.Error("server-b should still be in config")
	}

	// Verify file contains only server-b
	mcpPath := filepath.Join(basePath, ".mcp.json")
	raw := readFileJSON(t, mcpPath)
	mcpServers := raw["mcpServers"].(map[string]interface{})
	if _, ok := mcpServers["server-a"]; ok {
		t.Error("server-a should not be in .mcp.json")
	}
	if _, ok := mcpServers["server-b"]; !ok {
		t.Error("server-b should still be in .mcp.json")
	}
}

func TestReadWriteRoundTrip(t *testing.T) {
	a, basePath := newTestAdapter(t)

	// Write a Claude Code format .mcp.json directly
	claudeRaw := `{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "."],
      "env": {
        "NODE_ENV": "production"
      }
    }
  }
}`
	mcpPath := filepath.Join(basePath, ".mcp.json")
	if err := os.WriteFile(mcpPath, []byte(claudeRaw), 0o644); err != nil {
		t.Fatal(err)
	}

	// Read via adapter → universal format
	ctx := context.Background()
	uc, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}
	fs, ok := uc.Servers["filesystem"]
	if !ok {
		t.Fatal("filesystem not in universal config")
	}
	if fs.Command != "npx" {
		t.Errorf("Command = %q, want npx", fs.Command)
	}
	if fs.Transport != "stdio" {
		t.Errorf("Transport = %q, want stdio", fs.Transport)
	}

	// Write back via adapter → Claude format
	if err := a.WriteMCPConfig(ctx, uc); err != nil {
		t.Fatalf("WriteMCPConfig: %v", err)
	}

	// Read raw file and verify round-trip
	got := readFileJSON(t, mcpPath)

	gotServers, ok := got["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcpServers missing after round-trip")
	}
	gotFS, ok := gotServers["filesystem"].(map[string]interface{})
	if !ok {
		t.Fatal("filesystem missing after round-trip")
	}
	if gotFS["command"] != "npx" {
		t.Errorf("command = %q after round-trip", gotFS["command"])
	}
	// transport should NOT be present in Claude format
	if _, ok := gotFS["transport"]; ok {
		t.Error("transport should not appear in Claude Code format after round-trip")
	}
	// env should be preserved
	env, ok := gotFS["env"].(map[string]interface{})
	if !ok || env["NODE_ENV"] != "production" {
		t.Error("env not preserved after round-trip")
	}
}

func TestReadWriteRoundTrip_URL(t *testing.T) {
	a, basePath := newTestAdapter(t)

	// Write a Claude Code format with URL-based server
	claudeRaw := `{
  "mcpServers": {
    "remote": {
      "url": "https://example.com/mcp",
      "headers": {
        "Authorization": "Bearer token"
      }
    }
  }
}`
	mcpPath := filepath.Join(basePath, ".mcp.json")
	if err := os.WriteFile(mcpPath, []byte(claudeRaw), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	uc, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}
	remote, ok := uc.Servers["remote"]
	if !ok {
		t.Fatal("remote not in universal config")
	}
	if remote.Transport != "sse" {
		t.Errorf("Transport = %q, want sse for URL-based server", remote.Transport)
	}

	// Round-trip
	if err := a.WriteMCPConfig(ctx, uc); err != nil {
		t.Fatalf("WriteMCPConfig: %v", err)
	}

	got := readFileJSON(t, mcpPath)
	gotServers := got["mcpServers"].(map[string]interface{})
	gotRemote := gotServers["remote"].(map[string]interface{})
	if _, ok := gotRemote["transport"]; ok {
		t.Error("transport should not appear in Claude format")
	}
	if gotRemote["url"] != "https://example.com/mcp" {
		t.Errorf("url = %q", gotRemote["url"])
	}
}

func TestBackupRestore(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestAdapter(t)

	// Install initial server
	srv := types.UniversalMCPServer{Command: "original-cmd"}
	if err := a.InstallMCP(ctx, "test-srv", srv); err != nil {
		t.Fatalf("InstallMCP: %v", err)
	}

	// Read original state
	orig, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}

	// Backup (done inside InstallMCP, but let's do it explicitly)
	backupID, err := a.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if backupID == "" {
		t.Fatal("Backup returned empty ID")
	}

	// Modify state
	uc, _ := a.ReadMCPConfig(ctx)
	uc.Servers["test-srv"] = types.UniversalMCPServer{Command: "modified-cmd"}
	if err := a.WriteMCPConfig(ctx, uc); err != nil {
		t.Fatalf("WriteMCPConfig (modify): %v", err)
	}

	// Restore
	if err := a.Restore(ctx, backupID); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Verify restored to original
	restored, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig after restore: %v", err)
	}
	r := restored.Servers["test-srv"]
	if r.Command != "original-cmd" {
		t.Errorf("Command after restore = %q, want original-cmd", r.Command)
	}

	// Verify original equals restored
	if orig.Servers["test-srv"].Command != restored.Servers["test-srv"].Command {
		t.Error("restored config does not match original")
	}
}

func TestEmptyMcpJson(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	// Ensure .mcp.json doesn't exist
	mcpPath := filepath.Join(basePath, ".mcp.json")
	if _, err := os.Stat(mcpPath); !os.IsNotExist(err) {
		t.Fatal(".mcp.json already exists")
	}

	// Install should create the file
	server := types.UniversalMCPServer{
		Command: "node",
		Args:    []string{"server.js"},
	}
	if err := a.InstallMCP(ctx, "my-mcp", server); err != nil {
		t.Fatalf("InstallMCP: %v", err)
	}

	// Verify file created
	if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
		t.Fatal(".mcp.json was not created")
	}

	// Verify content
	cfg := readFileJSON(t, mcpPath)
	mcpServers := cfg["mcpServers"].(map[string]interface{})
	s := mcpServers["my-mcp"].(map[string]interface{})
	if s["command"] != "node" {
		t.Errorf("command = %q", s["command"])
	}
}

func TestMultipleServers(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestAdapter(t)

	servers := map[string]types.UniversalMCPServer{
		"filesystem": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
		},
		"memory": {
			Command: "node",
			Args:    []string{"memory-server.js"},
			Env:     map[string]string{"STORAGE": "/tmp"},
		},
	}

	for name, srv := range servers {
		if err := a.InstallMCP(ctx, name, srv); err != nil {
			t.Fatalf("InstallMCP(%q): %v", name, err)
		}
	}

	// Verify config
	cfg, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}
	if len(cfg.Servers) != 2 {
		t.Errorf("got %d servers, want 2", len(cfg.Servers))
	}
	for name := range servers {
		if _, ok := cfg.Servers[name]; !ok {
			t.Errorf("%q not in config", name)
		}
	}

	// Verify manifest
	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if len(manifest.MCPServers) != 2 {
		t.Errorf("manifest has %d MCP entries, want 2", len(manifest.MCPServers))
	}

	// Verify raw file
	mcpPath := filepath.Join(basePath, ".mcp.json")
	raw := readFileJSON(t, mcpPath)
	rawServers := raw["mcpServers"].(map[string]interface{})
	if len(rawServers) != 2 {
		t.Errorf(".mcp.json has %d servers, want 2", len(rawServers))
	}
}

func TestReadMCPConfig_NotExists(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestAdapter(t)

	cfg, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("config should not be nil")
	}
	if len(cfg.Servers) != 0 {
		t.Errorf("expected 0 servers for non-existent file, got %d", len(cfg.Servers))
	}
}
