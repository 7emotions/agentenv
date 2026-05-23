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

func newTestCursorAdapter(t *testing.T) (*CursorAdapter, string) {
	t.Helper()
	basePath := t.TempDir()
	homePath := t.TempDir()
	return NewCursorAdapterWithBaseAndHome(basePath, homePath), basePath
}

func writeTempCursorFile(t *testing.T, content string) string {
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

func TestCursor_Name(t *testing.T) {
	a, _ := newTestCursorAdapter(t)

	if got := a.Name(); got != "cursor" {
		t.Errorf("Name() = %q, want %q", got, "cursor")
	}
}

func TestCursor_DisplayName(t *testing.T) {
	a, _ := newTestCursorAdapter(t)

	if got := a.DisplayName(); got != "Cursor" {
		t.Errorf("DisplayName() = %q, want %q", got, "Cursor")
	}
}

func TestCursor_GetSkillBasePath(t *testing.T) {
	a, basePath := newTestCursorAdapter(t)

	expected := filepath.Join(basePath, "skills")
	if got := a.GetSkillBasePath(); got != expected {
		t.Errorf("GetSkillBasePath() = %q, want %q", got, expected)
	}
}

func TestCursor_GetAgentBasePath(t *testing.T) {
	a, basePath := newTestCursorAdapter(t)

	expected := filepath.Join(basePath, "agents")
	if got := a.GetAgentBasePath(); got != expected {
		t.Errorf("GetAgentBasePath() = %q, want %q", got, expected)
	}
}

func TestCursor_ManifestRoundTrip(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestCursorAdapter(t)

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

func TestCursor_ReadManifest_NotExists(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestCursorAdapter(t)

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

// ---- Skills ----

func TestCursor_InstallSkill(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestCursorAdapter(t)

	sourcePath := writeTempCursorFile(t, "# My Cursor SKILL.md content")
	if err := a.InstallSkill(ctx, "my-skill", sourcePath); err != nil {
		t.Fatalf("InstallSkill: %v", err)
	}

	skillFile := filepath.Join(basePath, "skills", "my-skill", "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", skillFile, err)
	}
	if string(data) != "# My Cursor SKILL.md content" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestCursor_InstallSkill_RefusesUnmanaged(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestCursorAdapter(t)

	skillDir := filepath.Join(basePath, "skills", "existing")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	source := writeTempCursorFile(t, "new content")
	err := a.InstallSkill(ctx, "existing", source)
	if err == nil {
		t.Fatal("expected error for unmanaged skill, got nil")
	}
	if !strings.Contains(err.Error(), "not managed by agentenv") {
		t.Errorf("expected 'not managed' error, got: %v", err)
	}
}

func TestCursor_RemoveSkill(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestCursorAdapter(t)

	source := writeTempCursorFile(t, "skill content")
	if err := a.InstallSkill(ctx, "to-remove", source); err != nil {
		t.Fatalf("InstallSkill: %v", err)
	}

	if err := a.RemoveSkill(ctx, "to-remove"); err != nil {
		t.Fatalf("RemoveSkill: %v", err)
	}

	skillDir := filepath.Join(basePath, "skills", "to-remove")
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Error("skill directory should be removed")
	}

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

func TestCursor_ListSkills(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestCursorAdapter(t)

	s1 := writeTempCursorFile(t, "skill one")
	s2 := writeTempCursorFile(t, "skill two")
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

func TestCursor_InstallAgent(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestCursorAdapter(t)

	sourcePath := writeTempCursorFile(t, "# Cursor Agent definition")
	if err := a.InstallAgent(ctx, "my-agent", sourcePath); err != nil {
		t.Fatalf("InstallAgent: %v", err)
	}

	agentFile := filepath.Join(basePath, "agents", "my-agent.md")
	data, err := os.ReadFile(agentFile)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", agentFile, err)
	}
	if string(data) != "# Cursor Agent definition" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestCursor_RemoveAgent(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestCursorAdapter(t)

	source := writeTempCursorFile(t, "agent content")
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

func TestCursor_ListAgents(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestCursorAdapter(t)

	s1 := writeTempCursorFile(t, "agent one")
	s2 := writeTempCursorFile(t, "agent two")
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

// ---- MCP ----

func TestCursor_MCPConfig_RoundTrip(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestCursorAdapter(t)

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
}

func TestCursor_ReadMCPConfig_NotExists(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestCursorAdapter(t)

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

func TestCursor_InstallMCP(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestCursorAdapter(t)

	server := types.UniversalMCPServer{
		Command:   "python",
		Args:      []string{"-m", "my_mcp_server"},
		Transport: "stdio",
		Env:       map[string]string{"DEBUG": "1"},
	}

	if err := a.InstallMCP(ctx, "test-mcp", server); err != nil {
		t.Fatalf("InstallMCP: %v", err)
	}

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

	configPath := filepath.Join(basePath, "mcp.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read mcp.json: %v", err)
	}
	var cc claudeMCPConfig
	if err := json.Unmarshal(data, &cc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	srv, ok := cc.MCPServers["test-mcp"]
	if !ok {
		t.Fatal("expected 'test-mcp' in mcp.json")
	}
	if srv.Command != "python" {
		t.Errorf("raw Command = %q, want python", srv.Command)
	}
	// Verify transport is NOT in the raw file (Claude Code / Cursor format)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	rawServers := raw["mcpServers"].(map[string]interface{})
	rawSrv := rawServers["test-mcp"].(map[string]interface{})
	if _, ok := rawSrv["transport"]; ok {
		t.Error("transport field should not appear in Cursor MCP format")
	}
}

func TestCursor_RemoveMCP(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestCursorAdapter(t)

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

	config, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig: %v", err)
	}
	if _, ok := config.Servers["to-remove"]; ok {
		t.Error("server should be removed from config")
	}

	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if _, ok := manifest.MCPServers["to-remove"]; ok {
		t.Error("server should be removed from manifest")
	}
}

// ---- Backup / Restore ----

func TestCursor_Backup(t *testing.T) {
	ctx := context.Background()
	a, basePath := newTestCursorAdapter(t)

	configPath := filepath.Join(basePath, "mcp.json")
	if err := os.WriteFile(configPath, []byte(`{"key": "value"}`), 0o644); err != nil {
		t.Fatalf("write mcp.json: %v", err)
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

	backupDir := filepath.Join(a.homeDir(), ".agentenv", "backups", "cursor")
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

func TestCursor_Restore(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestCursorAdapter(t)

	configPath := a.mcpConfigPath()

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

func TestCursor_Restore_MissingFile(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestCursorAdapter(t)

	err := a.Restore(ctx, "nonexistent-backup-id")
	if err != nil {
		t.Fatalf("Restore with missing backup should be non-fatal, got: %v", err)
	}
}

// ---- Registry ----

func TestRegistry_GetCursor(t *testing.T) {
	a, err := Get("cursor")
	if err != nil {
		t.Fatalf("Get('cursor'): %v", err)
	}
	if a.Name() != "cursor" {
		t.Errorf("Name = %q, want 'cursor'", a.Name())
	}
	if a.DisplayName() != "Cursor" {
		t.Errorf("DisplayName = %q, want 'Cursor'", a.DisplayName())
	}
}

func TestRegistry_ListIncludesCursor(t *testing.T) {
	names := List()
	found := false
	for _, n := range names {
		if n == "cursor" {
			found = true
		}
	}
	if !found {
		t.Errorf("List() should include 'cursor', got %v", names)
	}
}

// Ensure CursorAdapter interface compliance.
var _ AgentAdapter = (*CursorAdapter)(nil)
