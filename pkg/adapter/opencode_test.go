package adapter

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/7emotions/agentenv/pkg/types"
)

func newTestOpenCodeAdapter(t *testing.T) (*OpenCodeAdapter, string) {
	t.Helper()
	basePath := t.TempDir()
	homePath := t.TempDir()
	return NewOpenCodeAdapterWithBaseAndHome(basePath, homePath), basePath
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "agentenv-test-*.md")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestOpenCode_Name(t *testing.T) {
	a, _ := newTestOpenCodeAdapter(t)

	if got := a.Name(); got != "opencode" {
		t.Errorf("Name() = %q, want %q", got, "opencode")
	}
}

func TestOpenCode_DisplayName(t *testing.T) {
	a, _ := newTestOpenCodeAdapter(t)

	if got := a.DisplayName(); got != "OpenCode" {
		t.Errorf("DisplayName() = %q, want %q", got, "OpenCode")
	}
}

func TestOpenCode_GetSkillBasePath(t *testing.T) {
	a, basePath := newTestOpenCodeAdapter(t)

	expected := filepath.Join(basePath, "skills")
	if got := a.GetSkillBasePath(); got != expected {
		t.Errorf("GetSkillBasePath() = %q, want %q", got, expected)
	}
}

func TestOpenCode_GetAgentBasePath(t *testing.T) {
	a, basePath := newTestOpenCodeAdapter(t)

	expected := filepath.Join(basePath, "agents")
	if got := a.GetAgentBasePath(); got != expected {
		t.Errorf("GetAgentBasePath() = %q, want %q", got, expected)
	}
}

func TestOpenCode_ManifestRoundTrip(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

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

func TestOpenCode_ReadManifest_NotExists(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

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

func TestOpenCode_ManifestRoundTrip_Agents(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

	orig := &Manifest{
		Version: 1,
		EnvName: "test-env",
		Agents: map[string]ManifestItem{
			"agent-x": {
				Name:        "agent-x",
				SourcePath:  "/store/agents/agent-x.md",
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

	if len(got.Agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(got.Agents))
	}
	item := got.Agents["agent-x"]
	if item.Name != "agent-x" || item.SourcePath != "/store/agents/agent-x.md" {
		t.Errorf("unexpected manifest item: %+v", item)
	}
}

func TestOpenCode_ManifestRoundTrip_MCPServers(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

	orig := &Manifest{
		Version: 1,
		EnvName: "test-env",
		MCPServers: map[string]ManifestItem{
			"my-server": {
				Name:        "my-server",
				SourcePath:  "npx",
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

	if len(got.MCPServers) != 1 {
		t.Fatalf("got %d mcp_servers, want 1", len(got.MCPServers))
	}
	item := got.MCPServers["my-server"]
	if item.Name != "my-server" || item.SourcePath != "npx" {
		t.Errorf("unexpected manifest item: %+v", item)
	}
}

func TestRegistry_GetOpenCode(t *testing.T) {
	a, err := Get("opencode")
	if err != nil {
		t.Fatalf("Get('opencode'): %v", err)
	}
	if a.Name() != "opencode" {
		t.Errorf("Name = %q, want 'opencode'", a.Name())
	}
	if a.DisplayName() != "OpenCode" {
		t.Errorf("DisplayName = %q, want 'OpenCode'", a.DisplayName())
	}
}

func TestRegistry_ListIncludesOpenCode(t *testing.T) {
	names := List()
	found := false
	for _, n := range names {
		if n == "opencode" {
			found = true
		}
	}
	if !found {
		t.Errorf("List() should include 'opencode', got %v", names)
	}
}

// ---- Skills ----

func TestOpenCode_InstallSkill(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	sourcePath := writeTempFile(t, "# My SKILL.md content")
	if err := a.InstallSkill(ctx, "my-skill", sourcePath); err != nil {
		t.Fatalf("InstallSkill: %v", err)
	}

	skillFile := filepath.Join(basePath, "skills", "my-skill", "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", skillFile, err)
	}
	if string(data) != "# My SKILL.md content" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestOpenCode_InstallSkill_OverwriteManaged(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	source1 := writeTempFile(t, "content v1")
	if err := a.InstallSkill(ctx, "reinstall", source1); err != nil {
		t.Fatalf("first InstallSkill: %v", err)
	}

	source2 := writeTempFile(t, "content v2")
	if err := a.InstallSkill(ctx, "reinstall", source2); err != nil {
		t.Fatalf("second InstallSkill (overwrite): %v", err)
	}

	skillFile := filepath.Join(basePath, "skills", "reinstall", "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "content v2" {
		t.Errorf("expected v2 content, got %q", string(data))
	}
}

func TestOpenCode_InstallSkill_RefusesUnmanaged(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	// Pre-create a skill dir without it being in the manifest
	skillDir := filepath.Join(basePath, "skills", "existing")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	source := writeTempFile(t, "new content")
	err := a.InstallSkill(ctx, "existing", source)
	if err == nil {
		t.Fatal("expected error for unmanaged skill, got nil")
	}
	if !strings.Contains(err.Error(), "not managed by agentenv") {
		t.Errorf("expected 'not managed' error, got: %v", err)
	}
}

func TestOpenCode_RemoveSkill(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	source := writeTempFile(t, "skill content")
	if err := a.InstallSkill(ctx, "to-remove", source); err != nil {
		t.Fatalf("InstallSkill: %v", err)
	}

	if err := a.RemoveSkill(ctx, "to-remove"); err != nil {
		t.Fatalf("RemoveSkill: %v", err)
	}

	// Verify dir is gone
	skillDir := filepath.Join(basePath, "skills", "to-remove")
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Error("skill directory should be removed")
	}

	// Verify not in manifest
	names, err := a.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	for _, n := range names {
		if n == "to-remove" {
			t.Error("removed skill still in manifest")
		}
	}
}

func TestOpenCode_RemoveSkill_NonManagedPreserved(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	// Pre-create a skill dir without manifest entry
	skillDir := filepath.Join(basePath, "skills", "user-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// RemoveSkill should be a no-op for non-managed
	if err := a.RemoveSkill(ctx, "user-skill"); err != nil {
		t.Fatalf("RemoveSkill (non-managed): %v", err)
	}

	// Directory should still exist
	if _, err := os.Stat(skillDir); err != nil {
		t.Errorf("non-managed skill dir should be preserved: %v", err)
	}
}

func TestOpenCode_ListSkills(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

	s1 := writeTempFile(t, "skill one")
	s2 := writeTempFile(t, "skill two")
	a.InstallSkill(ctx, "skill-a", s1)
	a.InstallSkill(ctx, "skill-b", s2)

	names, err := a.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}

	foundA, foundB := false, false
	for _, n := range names {
		if n == "skill-a" {
			foundA = true
		}
		if n == "skill-b" {
			foundB = true
		}
	}
	if !foundA {
		t.Error("skill-a not found in list")
	}
	if !foundB {
		t.Error("skill-b not found in list")
	}
}

// ---- Agents ----

func TestOpenCode_InstallAgent(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	sourcePath := writeTempFile(t, "# Agent definition")
	if err := a.InstallAgent(ctx, "my-agent", sourcePath); err != nil {
		t.Fatalf("InstallAgent: %v", err)
	}

	agentFile := filepath.Join(basePath, "agents", "my-agent.md")
	data, err := os.ReadFile(agentFile)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", agentFile, err)
	}
	if string(data) != "# Agent definition" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestOpenCode_RemoveAgent(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	source := writeTempFile(t, "agent content")
	if err := a.InstallAgent(ctx, "to-remove", source); err != nil {
		t.Fatalf("InstallAgent: %v", err)
	}

	if err := a.RemoveAgent(ctx, "to-remove"); err != nil {
		t.Fatalf("RemoveAgent: %v", err)
	}

	agentFile := filepath.Join(basePath, "agents", "to-remove.md")
	if _, err := os.Stat(agentFile); !os.IsNotExist(err) {
		t.Error("agent file should be removed")
	}

	names, err := a.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	for _, n := range names {
		if n == "to-remove" {
			t.Error("removed agent still in manifest")
		}
	}
}

func TestOpenCode_RemoveAgent_NonManagedPreserved(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	agentsDir := filepath.Join(basePath, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	agentFile := filepath.Join(agentsDir, "user-agent.md")
	if err := os.WriteFile(agentFile, []byte("user content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := a.RemoveAgent(ctx, "user-agent"); err != nil {
		t.Fatalf("RemoveAgent (non-managed): %v", err)
	}

	if _, err := os.Stat(agentFile); err != nil {
		t.Errorf("non-managed agent file should be preserved: %v", err)
	}
}

func TestOpenCode_ListAgents(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

	s1 := writeTempFile(t, "agent one")
	s2 := writeTempFile(t, "agent two")
	a.InstallAgent(ctx, "agent-a", s1)
	a.InstallAgent(ctx, "agent-b", s2)

	names, err := a.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}

	foundA, foundB := false, false
	for _, n := range names {
		if n == "agent-a" {
			foundA = true
		}
		if n == "agent-b" {
			foundB = true
		}
	}
	if !foundA {
		t.Error("agent-a not found in list")
	}
	if !foundB {
		t.Error("agent-b not found in list")
	}
}

// ---- Backup / Restore ----

func TestOpenCode_Backup(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	configPath := filepath.Join(basePath, "opencode.jsonc")
	if err := os.WriteFile(configPath, []byte(`{"key": "value"}`), 0o644); err != nil {
		t.Fatalf("write opencode.jsonc: %v", err)
	}

	m := &Manifest{Version: 1, EnvName: "test-backup"}
	if err := a.WriteManifest(ctx, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	backupID, err := a.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if backupID == "" {
		t.Fatal("Backup returned empty ID")
	}

	backupDir := filepath.Join(a.homeDir(), ".agentenv", "backups", "opencode")
	configBackup := filepath.Join(backupDir, backupID+".json")
	manifestBackup := filepath.Join(backupDir, backupID+"-manifest.json")

	data, err := os.ReadFile(configBackup)
	if err != nil {
		t.Errorf("config backup not found: %v", err)
	}
	if string(data) != `{"key": "value"}` {
		t.Errorf("config backup = %q, want %q", string(data), `{"key": "value"}`)
	}
	if _, err := os.Stat(manifestBackup); err != nil {
		t.Errorf("manifest backup not found: %v", err)
	}
}

func TestOpenCode_Restore(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

	configPath := a.configPath()

	if err := os.WriteFile(configPath, []byte(`{"original": true}`), 0o644); err != nil {
		t.Fatalf("write original config: %v", err)
	}
	if err := a.WriteManifest(ctx, &Manifest{Version: 1, EnvName: "original"}); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	backupID, err := a.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	if err := os.WriteFile(configPath, []byte(`{"modified": true}`), 0o644); err != nil {
		t.Fatalf("modify config: %v", err)
	}
	if err := a.WriteManifest(ctx, &Manifest{Version: 1, EnvName: "modified"}); err != nil {
		t.Fatalf("WriteManifest modified: %v", err)
	}

	if err := a.Restore(ctx, backupID); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read restored config: %v", err)
	}
	if string(data) != `{"original": true}` {
		t.Errorf("restored config = %q, want %q", string(data), `{"original": true}`)
	}

	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest after restore: %v", err)
	}
	if manifest.EnvName != "original" {
		t.Errorf("EnvName = %q, want %q", manifest.EnvName, "original")
	}
}

func TestOpenCode_Backup_FirstRun(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	configPath := filepath.Join(basePath, "opencode.jsonc")
	if err := os.WriteFile(configPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write opencode.jsonc: %v", err)
	}

	backupID, err := a.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup (first run): %v", err)
	}
	if backupID == "" {
		t.Fatal("Backup returned empty ID on first run")
	}

	backupDir := filepath.Join(a.homeDir(), ".agentenv", "backups", "opencode")
	configBackup := filepath.Join(backupDir, backupID+".json")
	if _, err := os.ReadFile(configBackup); err != nil {
		t.Errorf("config backup should exist after first-run backup: %v", err)
	}
	manifestBackup := filepath.Join(backupDir, backupID+"-manifest.json")
	if _, err := os.ReadFile(manifestBackup); err != nil {
		t.Errorf("manifest backup should exist after first-run backup: %v", err)
	}
}

func TestOpenCode_Restore_MissingFile(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

	err := a.Restore(ctx, "nonexistent-backup-id")
	if err != nil {
		t.Fatalf("Restore with missing backup should be non-fatal, got: %v", err)
	}
}

// ---- MCP ----

func TestOpenCode_ReadMCPConfig_NotExists(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

	config, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if len(config.Servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(config.Servers))
	}
}

func TestOpenCode_ReadMCPConfig_EmptyMCPKey(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	// File exists but has no "mcp" key.
	configPath := filepath.Join(basePath, "opencode.jsonc")
	if err := os.WriteFile(configPath, []byte(`{"skills": {}}`), 0o644); err != nil {
		t.Fatalf("write opencode.jsonc: %v", err)
	}

	config, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}
	if len(config.Servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(config.Servers))
	}
}

func TestOpenCode_ReadMCPConfig_LocalServer(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	configPath := filepath.Join(basePath, "opencode.jsonc")
	content := `{
  "mcp": {
    "codegraph": {
      "type": "local",
      "command": ["/usr/local/bin/node", "/path/to/codegraph", "serve", "--mcp"],
      "enabled": true
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write opencode.jsonc: %v", err)
	}

	config, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}

	server, ok := config.Servers["codegraph"]
	if !ok {
		t.Fatal("expected 'codegraph' server")
	}
	if server.Transport != "stdio" {
		t.Errorf("Transport = %q, want %q", server.Transport, "stdio")
	}
	if server.Command != "/usr/local/bin/node" {
		t.Errorf("Command = %q, want %q", server.Command, "/usr/local/bin/node")
	}
	if len(server.Args) != 3 || server.Args[0] != "/path/to/codegraph" {
		t.Errorf("Args = %v, want [/path/to/codegraph serve --mcp]", server.Args)
	}
}

func TestOpenCode_ReadMCPConfig_RemoteServer(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	configPath := filepath.Join(basePath, "opencode.jsonc")
	content := `{
  "mcp": {
    "remote-api": {
      "type": "remote",
      "url": "https://api.example.com/mcp",
      "headers": {"Authorization": "Bearer token123"},
      "environment": {"LOG_LEVEL": "debug"},
      "timeout": 10000,
      "description": "Remote MCP server"
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write opencode.jsonc: %v", err)
	}

	config, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}

	server, ok := config.Servers["remote-api"]
	if !ok {
		t.Fatal("expected 'remote-api' server")
	}
	if server.Transport != "sse" {
		t.Errorf("Transport = %q, want %q", server.Transport, "sse")
	}
	if server.URL != "https://api.example.com/mcp" {
		t.Errorf("URL = %q", server.URL)
	}
	if server.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("Headers = %v", server.Headers)
	}
	if server.Env["LOG_LEVEL"] != "debug" {
		t.Errorf("Env = %v", server.Env)
	}
	if server.Description != "Remote MCP server" {
		t.Errorf("Description = %q", server.Description)
	}
}

func TestOpenCode_MCPConfig_RoundTrip(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

	original := &types.UniversalMCPConfig{
		Servers: map[string]types.UniversalMCPServer{
			"my-server": {
				Command:     "npx",
				Args:        []string{"-y", "@scope/mcp-package"},
				Transport:   "stdio",
				Env:         map[string]string{"NODE_ENV": "production"},
				Description: "Test server",
			},
		},
	}

	if err := a.WriteMCPConfig(ctx, original); err != nil {
		t.Fatalf("WriteMCPConfig: %v", err)
	}

	read, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}

	if len(read.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(read.Servers))
	}

	s, ok := read.Servers["my-server"]
	if !ok {
		t.Fatal("expected 'my-server'")
	}
	if s.Command != "npx" {
		t.Errorf("Command = %q", s.Command)
	}
	if s.Transport != "stdio" {
		t.Errorf("Transport = %q", s.Transport)
	}
	if len(s.Args) != 2 || s.Args[0] != "-y" {
		t.Errorf("Args = %v", s.Args)
	}
	if s.Env["NODE_ENV"] != "production" {
		t.Errorf("Env = %v", s.Env)
	}
}

func TestOpenCode_MCPConfig_RoundTrip_RemoteServer(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

	original := &types.UniversalMCPConfig{
		Servers: map[string]types.UniversalMCPServer{
			"api-server": {
				Transport:   "sse",
				URL:         "https://mcp.example.com/events",
				Headers:     map[string]string{"X-API-Key": "secret"},
				Description: "SSE-based server",
			},
		},
	}

	if err := a.WriteMCPConfig(ctx, original); err != nil {
		t.Fatalf("WriteMCPConfig: %v", err)
	}

	read, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}

	s, ok := read.Servers["api-server"]
	if !ok {
		t.Fatal("expected 'api-server'")
	}
	if s.Transport != "sse" {
		t.Errorf("Transport = %q", s.Transport)
	}
	if s.URL != "https://mcp.example.com/events" {
		t.Errorf("URL = %q", s.URL)
	}
	if s.Headers["X-API-Key"] != "secret" {
		t.Errorf("Headers = %v", s.Headers)
	}
}

func TestOpenCode_MCPConfig_RoundTrip_EmptyConfig(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

	empty := &types.UniversalMCPConfig{
		Servers: make(map[string]types.UniversalMCPServer),
	}

	if err := a.WriteMCPConfig(ctx, empty); err != nil {
		t.Fatalf("WriteMCPConfig empty: %v", err)
	}

	read, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}
	if len(read.Servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(read.Servers))
	}
}

func TestOpenCode_MCPConfig_PreservesOtherKeys(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	configPath := filepath.Join(basePath, "opencode.jsonc")
	// Write a file with extra top-level keys.
	initial := `{"skills": {"my-skill": true}, "agents": {}, "mcp": {}}`
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	// Write MCP config with one server.
	cfg := &types.UniversalMCPConfig{
		Servers: map[string]types.UniversalMCPServer{
			"test": {Command: "echo", Transport: "stdio"},
		},
	}
	if err := a.WriteMCPConfig(ctx, cfg); err != nil {
		t.Fatalf("WriteMCPConfig: %v", err)
	}

	// Read the file and check that the other keys survived.
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := m["skills"]; !ok {
		t.Error("'skills' key was lost during WriteMCPConfig")
	}
	if _, ok := m["agents"]; !ok {
		t.Error("'agents' key was lost during WriteMCPConfig")
	}
	if _, ok := m["mcp"]; !ok {
		t.Error("'mcp' key was lost during WriteMCPConfig")
	}
}

func TestOpenCode_InstallMCP(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestOpenCodeAdapter(t)

	server := types.UniversalMCPServer{
		Command:   "python",
		Args:      []string{"-m", "my_mcp_server"},
		Transport: "stdio",
		Env:       map[string]string{"DEBUG": "1"},
	}

	if err := a.InstallMCP(ctx, "test-mcp", server); err != nil {
		t.Fatalf("InstallMCP: %v", err)
	}

	// Verify the config was written.
	config, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}
	s, ok := config.Servers["test-mcp"]
	if !ok {
		t.Fatal("expected 'test-mcp' server in config")
	}
	if s.Command != "python" {
		t.Errorf("Command = %q", s.Command)
	}
	if s.Env["DEBUG"] != "1" {
		t.Errorf("Env = %v", s.Env)
	}

	// Verify the manifest was updated.
	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	item, ok := manifest.MCPServers["test-mcp"]
	if !ok {
		t.Fatal("expected 'test-mcp' in manifest")
	}
	if item.Name != "test-mcp" {
		t.Errorf("Name = %q", item.Name)
	}
	if item.InstalledAt == "" {
		t.Error("InstalledAt should not be empty")
	}

	// Verify the written file has OpenCode format.
	configPath := filepath.Join(basePath, "opencode.jsonc")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read opencode.jsonc: %v", err)
	}
	var wrapper opencodeJSONC
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	oc, ok := wrapper.MCP["test-mcp"]
	if !ok {
		t.Fatal("expected 'test-mcp' in opencode.jsonc")
	}
	if oc.Type != "local" {
		t.Errorf("Type = %q, want 'local'", oc.Type)
	}
	if len(oc.Command) != 3 || oc.Command[0] != "python" {
		t.Errorf("Command = %v", oc.Command)
	}
	if oc.Enabled == nil || !*oc.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestOpenCode_RemoveMCP(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

	server := types.UniversalMCPServer{
		Command:   "node",
		Args:      []string{"server.js"},
		Transport: "stdio",
	}

	if err := a.InstallMCP(ctx, "to-remove", server); err != nil {
		t.Fatalf("InstallMCP: %v", err)
	}

	if err := a.RemoveMCP(ctx, "to-remove"); err != nil {
		t.Fatalf("RemoveMCP: %v", err)
	}

	// Verify server is gone from config.
	config, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}
	if _, ok := config.Servers["to-remove"]; ok {
		t.Error("server should be removed from config")
	}

	// Verify server is gone from manifest.
	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if _, ok := manifest.MCPServers["to-remove"]; ok {
		t.Error("server should be removed from manifest")
	}
}

func TestOpenCode_FormatConversion_LocalToStdio(t *testing.T) {
	oc := &opencodeJSONC{
		MCP: map[string]opencodeMCPServer{
			"local-srv": {
				Type:        "local",
				Command:     []string{"/usr/bin/go", "run", "./main.go"},
				Environment: map[string]string{"GOARCH": "amd64"},
				Description: "A local Go server",
			},
		},
	}

	uc := opencodeToUniversal(oc)
	s, ok := uc.Servers["local-srv"]
	if !ok {
		t.Fatal("expected 'local-srv'")
	}
	if s.Transport != "stdio" {
		t.Errorf("Transport = %q, want stdio", s.Transport)
	}
	if s.Command != "/usr/bin/go" {
		t.Errorf("Command = %q, want /usr/bin/go", s.Command)
	}
	if len(s.Args) != 2 || s.Args[0] != "run" {
		t.Errorf("Args = %v", s.Args)
	}
	if s.Env["GOARCH"] != "amd64" {
		t.Errorf("Env = %v", s.Env)
	}

	// Round-trip back.
	oc2 := universalToOpenCode(uc)
	s2, ok := oc2.MCP["local-srv"]
	if !ok {
		t.Fatal("round-trip: expected 'local-srv'")
	}
	if s2.Type != "local" {
		t.Errorf("round-trip Type = %q, want local", s2.Type)
	}
	if len(s2.Command) != 3 {
		t.Errorf("round-trip Command len = %d, want 3", len(s2.Command))
	}
}

func TestOpenCode_FormatConversion_RemoteToSSE(t *testing.T) {
	oc := &opencodeJSONC{
		MCP: map[string]opencodeMCPServer{
			"remote-srv": {
				Type:    "remote",
				URL:     "https://mcp.cloud.example.com",
				Headers: map[string]string{"Authorization": "Bearer abc"},
			},
		},
	}

	uc := opencodeToUniversal(oc)
	s, ok := uc.Servers["remote-srv"]
	if !ok {
		t.Fatal("expected 'remote-srv'")
	}
	if s.Transport != "sse" {
		t.Errorf("Transport = %q, want sse", s.Transport)
	}
	if s.URL != "https://mcp.cloud.example.com" {
		t.Errorf("URL = %q", s.URL)
	}
	if s.Headers["Authorization"] != "Bearer abc" {
		t.Errorf("Headers = %v", s.Headers)
	}

	// Round-trip back.
	oc2 := universalToOpenCode(uc)
	s2, ok := oc2.MCP["remote-srv"]
	if !ok {
		t.Fatal("round-trip: expected 'remote-srv'")
	}
	if s2.Type != "remote" {
		t.Errorf("round-trip Type = %q, want remote", s2.Type)
	}
	if s2.URL != "https://mcp.cloud.example.com" {
		t.Errorf("round-trip URL = %q", s2.URL)
	}
}

func TestOpenCode_FormatConversion_HTTPToRemote(t *testing.T) {
	uc := &types.UniversalMCPConfig{
		Servers: map[string]types.UniversalMCPServer{
			"http-srv": {
				Transport: "http",
				URL:       "https://api.example.com/mcp",
			},
		},
	}

	oc := universalToOpenCode(uc)
	s, ok := oc.MCP["http-srv"]
	if !ok {
		t.Fatal("expected 'http-srv'")
	}
	if s.Type != "remote" {
		t.Errorf("Type = %q, want remote", s.Type)
	}
	if s.URL != "https://api.example.com/mcp" {
		t.Errorf("URL = %q", s.URL)
	}
}

func TestOpenCode_FormatConversion_UnknownTypeIsStdio(t *testing.T) {
	oc := &opencodeJSONC{
		MCP: map[string]opencodeMCPServer{
			"weird": {
				Type:    "", // empty or unknown
				Command: []string{"some-tool"},
			},
		},
	}

	uc := opencodeToUniversal(oc)
	s, ok := uc.Servers["weird"]
	if !ok {
		t.Fatal("expected 'weird'")
	}
	if s.Transport != "stdio" {
		t.Errorf("empty type should default to stdio, got %q", s.Transport)
	}
	if s.Command != "some-tool" {
		t.Errorf("Command = %q", s.Command)
	}
}

func TestOpenCode_WriteMCPConfig_MultipleServers(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

	cfg := &types.UniversalMCPConfig{
		Servers: map[string]types.UniversalMCPServer{
			"server-a": {Command: "cmd-a", Transport: "stdio"},
			"server-b": {Command: "cmd-b", Transport: "stdio"},
			"server-c": {URL: "https://c.example.com", Transport: "sse"},
		},
	}

	if err := a.WriteMCPConfig(ctx, cfg); err != nil {
		t.Fatalf("WriteMCPConfig: %v", err)
	}

	read, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}
	if len(read.Servers) != 3 {
		t.Fatalf("expected 3 servers, got %d", len(read.Servers))
	}
	for _, name := range []string{"server-a", "server-b", "server-c"} {
		if _, ok := read.Servers[name]; !ok {
			t.Errorf("expected server %q", name)
		}
	}
}
