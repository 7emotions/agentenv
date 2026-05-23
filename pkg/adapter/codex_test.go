package adapter

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/7emotions/agentenv/pkg/types"
)

// newTestCodexAdapter creates a CodexAdapter with isolated temp directories
// for both basePath and homePath so tests don't touch real ~/.codex/.
func newTestCodexAdapter(t *testing.T) *CodexAdapter {
	t.Helper()
	basePath := t.TempDir()
	homePath := t.TempDir()
	return NewCodexAdapterWithBaseAndHome(basePath, homePath)
}

// writeConfigTOML writes a TOML string to the adapter's config.toml path.
func writeCodexConfig(t *testing.T, a *CodexAdapter, content string) {
	t.Helper()
	if err := os.MkdirAll(a.basePath, 0o755); err != nil {
		t.Fatalf("create base dir: %v", err)
	}
	if err := os.WriteFile(a.configPath(), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// ---- Identity ----

func TestCodex_Name(t *testing.T) {
	a := newTestCodexAdapter(t)
	if got := a.Name(); got != "codex" {
		t.Errorf("Name() = %q, want %q", got, "codex")
	}
}

func TestCodex_DisplayName(t *testing.T) {
	a := newTestCodexAdapter(t)
	if got := a.DisplayName(); got != "Codex" {
		t.Errorf("DisplayName() = %q, want %q", got, "Codex")
	}
}

func TestCodex_GetSkillBasePath(t *testing.T) {
	a := newTestCodexAdapter(t)
	if got := a.GetSkillBasePath(); got != "" {
		t.Errorf("GetSkillBasePath() = %q, want %q", got, "")
	}
}

func TestCodex_GetAgentBasePath(t *testing.T) {
	a := newTestCodexAdapter(t)
	if got := a.GetAgentBasePath(); got != "" {
		t.Errorf("GetAgentBasePath() = %q, want %q", got, "")
	}
}

// ---- Manifest ----

func TestCodex_ReadManifest_NotExist(t *testing.T) {
	a := newTestCodexAdapter(t)
	m, err := a.ReadManifest(context.Background())
	if err != nil {
		t.Fatalf("ReadManifest() error: %v", err)
	}
	if m.Version != 1 {
		t.Errorf("Version = %d, want 1", m.Version)
	}
}

func TestCodex_Manifest_RoundTrip(t *testing.T) {
	a := newTestCodexAdapter(t)
	ctx := context.Background()

	m := &Manifest{
		Version: 1,
		EnvName: "test-env",
		MCPServers: map[string]ManifestItem{
			"test-mcp": {
				Name:        "test-mcp",
				SourcePath:  "/tmp/test-mcp",
				InstalledAt: "2024-01-01T00:00:00Z",
			},
		},
	}
	if err := a.WriteManifest(ctx, m); err != nil {
		t.Fatalf("WriteManifest() error: %v", err)
	}

	got, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest() error: %v", err)
	}
	if got.EnvName != m.EnvName {
		t.Errorf("EnvName = %q, want %q", got.EnvName, m.EnvName)
	}
	if len(got.MCPServers) != 1 {
		t.Fatalf("MCPServers count = %d, want 1", len(got.MCPServers))
	}
	if item, ok := got.MCPServers["test-mcp"]; !ok {
		t.Error("MCPServers missing key test-mcp")
	} else if item.Name != "test-mcp" {
		t.Errorf("MCPServers[test-mcp].Name = %q", item.Name)
	}
}

// ---- Skills (unsupported) ----

func TestCodex_InstallSkill_ReturnsError(t *testing.T) {
	a := newTestCodexAdapter(t)
	err := a.InstallSkill(context.Background(), "test-skill", "/tmp/skill.md")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCodex_RemoveSkill_ReturnsError(t *testing.T) {
	a := newTestCodexAdapter(t)
	err := a.RemoveSkill(context.Background(), "test-skill")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCodex_ListSkills_ReturnsEmpty(t *testing.T) {
	a := newTestCodexAdapter(t)
	skills, err := a.ListSkills(context.Background())
	if err != nil {
		t.Fatalf("ListSkills() error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("ListSkills() = %v, want empty", skills)
	}
}

// ---- Agents (unsupported) ----

func TestCodex_InstallAgent_ReturnsError(t *testing.T) {
	a := newTestCodexAdapter(t)
	err := a.InstallAgent(context.Background(), "test-agent", "/tmp/agent.md")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCodex_RemoveAgent_ReturnsError(t *testing.T) {
	a := newTestCodexAdapter(t)
	err := a.RemoveAgent(context.Background(), "test-agent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCodex_ListAgents_ReturnsEmpty(t *testing.T) {
	a := newTestCodexAdapter(t)
	agents, err := a.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents() error: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("ListAgents() = %v, want empty", agents)
	}
}

// ---- MCP Config ----

func TestCodex_ReadMCPConfig_NotExist(t *testing.T) {
	a := newTestCodexAdapter(t)
	cfg, err := a.ReadMCPConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	if len(cfg.Servers) != 0 {
		t.Errorf("Servers count = %d, want 0", len(cfg.Servers))
	}
}

func TestCodex_ReadMCPConfig_EmptyMCPSection(t *testing.T) {
	a := newTestCodexAdapter(t)
	writeCodexConfig(t, a, `
model = "gpt-5.1"
[mcp_servers]
`)
	cfg, err := a.ReadMCPConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	if len(cfg.Servers) != 0 {
		t.Errorf("Servers count = %d, want 0", len(cfg.Servers))
	}
}

func TestCodex_ReadMCPConfig_FlatStdioServer(t *testing.T) {
	a := newTestCodexAdapter(t)
	writeCodexConfig(t, a, `
[mcp_servers.docs]
command = "docs-server"
args = ["--port", "4000"]
enabled = true
`)
	cfg, err := a.ReadMCPConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	srv, ok := cfg.Servers["docs"]
	if !ok {
		t.Fatal("missing server 'docs'")
	}
	if srv.Transport != "stdio" {
		t.Errorf("Transport = %q, want stdio", srv.Transport)
	}
	if srv.Command != "docs-server" {
		t.Errorf("Command = %q, want docs-server", srv.Command)
	}
	if len(srv.Args) != 2 || srv.Args[0] != "--port" || srv.Args[1] != "4000" {
		t.Errorf("Args = %v, want [--port 4000]", srv.Args)
	}
}

func TestCodex_ReadMCPConfig_FlatRemoteServer(t *testing.T) {
	a := newTestCodexAdapter(t)
	writeCodexConfig(t, a, `
[mcp_servers.github]
url = "https://github-mcp.example.com/mcp"
enabled = true
`)
	cfg, err := a.ReadMCPConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	srv, ok := cfg.Servers["github"]
	if !ok {
		t.Fatal("missing server 'github'")
	}
	if srv.Transport != "sse" {
		t.Errorf("Transport = %q, want sse", srv.Transport)
	}
	if srv.URL != "https://github-mcp.example.com/mcp" {
		t.Errorf("URL = %q", srv.URL)
	}
}

func TestCodex_ReadMCPConfig_TransportStdio(t *testing.T) {
	a := newTestCodexAdapter(t)
	writeCodexConfig(t, a, `
[mcp_servers.my-server]
enabled = true
[mcp_servers.my-server.transport]
type = "stdio"
command = "npx"
args = ["-y", "@my-org/my-mcp-server"]
`)
	cfg, err := a.ReadMCPConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	srv, ok := cfg.Servers["my-server"]
	if !ok {
		t.Fatal("missing server 'my-server'")
	}
	if srv.Transport != "stdio" {
		t.Errorf("Transport = %q, want stdio", srv.Transport)
	}
	if srv.Command != "npx" {
		t.Errorf("Command = %q, want npx", srv.Command)
	}
}

func TestCodex_ReadMCPConfig_TransportHTTP(t *testing.T) {
	a := newTestCodexAdapter(t)
	writeCodexConfig(t, a, `
[mcp_servers.remote-api]
enabled = true
[mcp_servers.remote-api.transport]
type = "streamable_http"
url = "https://api.example.com"
`)
	cfg, err := a.ReadMCPConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	srv, ok := cfg.Servers["remote-api"]
	if !ok {
		t.Fatal("missing server 'remote-api'")
	}
	if srv.Transport != "sse" {
		t.Errorf("Transport = %q, want sse", srv.Transport)
	}
	if srv.URL != "https://api.example.com" {
		t.Errorf("URL = %q", srv.URL)
	}
}

func TestCodex_ReadMCPConfig_DisabledServer(t *testing.T) {
	a := newTestCodexAdapter(t)
	writeCodexConfig(t, a, `
[mcp_servers.enabled-one]
command = "enabled-server"
enabled = true

[mcp_servers.disabled-one]
command = "disabled-server"
enabled = false
`)
	cfg, err := a.ReadMCPConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	if _, ok := cfg.Servers["enabled-one"]; !ok {
		t.Error("missing enabled server")
	}
	if _, ok := cfg.Servers["disabled-one"]; ok {
		t.Error("disabled server should not appear")
	}
}

func TestCodex_ReadMCPConfig_EnvAndHeaders(t *testing.T) {
	a := newTestCodexAdapter(t)
	writeCodexConfig(t, a, `
[mcp_servers.with-env]
command = "server"
env = { DEBUG = "mcp:*", TOKEN = "abc" }
http_headers = { "X-API" = "v1" }
enabled = true
`)
	cfg, err := a.ReadMCPConfig(context.Background())
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	srv := cfg.Servers["with-env"]
	if srv.Env["DEBUG"] != "mcp:*" {
		t.Errorf("Env[DEBUG] = %q", srv.Env["DEBUG"])
	}
	if srv.Env["TOKEN"] != "abc" {
		t.Errorf("Env[TOKEN] = %q", srv.Env["TOKEN"])
	}
	if srv.Headers["X-API"] != "v1" {
		t.Errorf("Headers[X-API] = %q", srv.Headers["X-API"])
	}
}

// ---- MCP Write / Round-trip ----

func TestCodex_MCPConfig_RoundTripStdio(t *testing.T) {
	a := newTestCodexAdapter(t)
	ctx := context.Background()

	cfg := &types.UniversalMCPConfig{
		Servers: map[string]types.UniversalMCPServer{
			"my-server": {
				Transport:   "stdio",
				Command:     "npx",
				Args:        []string{"-y", "@my-org/mcp-server"},
				Description: "A test server",
			},
		},
	}
	if err := a.WriteMCPConfig(ctx, cfg); err != nil {
		t.Fatalf("WriteMCPConfig() error: %v", err)
	}

	got, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	srv, ok := got.Servers["my-server"]
	if !ok {
		t.Fatal("missing server 'my-server'")
	}
	if srv.Transport != "stdio" {
		t.Errorf("Transport = %q, want stdio", srv.Transport)
	}
	if srv.Command != "npx" {
		t.Errorf("Command = %q, want npx", srv.Command)
	}
	if srv.Description != "A test server" {
		t.Errorf("Description = %q", srv.Description)
	}
}

func TestCodex_MCPConfig_RoundTripSSE(t *testing.T) {
	a := newTestCodexAdapter(t)
	ctx := context.Background()

	cfg := &types.UniversalMCPConfig{
		Servers: map[string]types.UniversalMCPServer{
			"remote": {
				Transport: "sse",
				URL:       "https://mcp.example.com",
			},
		},
	}
	if err := a.WriteMCPConfig(ctx, cfg); err != nil {
		t.Fatalf("WriteMCPConfig() error: %v", err)
	}

	got, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	srv, ok := got.Servers["remote"]
	if !ok {
		t.Fatal("missing server 'remote'")
	}
	if srv.Transport != "sse" {
		t.Errorf("Transport = %q, want sse", srv.Transport)
	}
	if srv.URL != "https://mcp.example.com" {
		t.Errorf("URL = %q", srv.URL)
	}
}

func TestCodex_MCPConfig_RoundTripEmpty(t *testing.T) {
	a := newTestCodexAdapter(t)
	ctx := context.Background()

	cfg := &types.UniversalMCPConfig{
		Servers: make(map[string]types.UniversalMCPServer),
	}
	if err := a.WriteMCPConfig(ctx, cfg); err != nil {
		t.Fatalf("WriteMCPConfig() error: %v", err)
	}

	got, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	if len(got.Servers) != 0 {
		t.Errorf("Servers count = %d, want 0", len(got.Servers))
	}
}

func TestCodex_MCPConfig_PreservesOtherKeys(t *testing.T) {
	a := newTestCodexAdapter(t)
	ctx := context.Background()

	// Write initial config with non-MCP settings.
	writeCodexConfig(t, a, `
model = "gpt-5.1"
approval_policy = "on-request"
[mcp_servers.old-server]
command = "old"
`)

	// Write new MCP config — should preserve model and approval_policy.
	cfg := &types.UniversalMCPConfig{
		Servers: map[string]types.UniversalMCPServer{
			"new-server": {
				Transport: "stdio",
				Command:   "new",
			},
		},
	}
	if err := a.WriteMCPConfig(ctx, cfg); err != nil {
		t.Fatalf("WriteMCPConfig() error: %v", err)
	}

	// Verify new MCP servers are present.
	got, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	if _, ok := got.Servers["new-server"]; !ok {
		t.Error("missing new-server")
	}
	if _, ok := got.Servers["old-server"]; ok {
		t.Error("old-server should have been replaced")
	}

	// Verify preserved keys by reading raw file content.
	data, err := os.ReadFile(a.configPath())
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	content := string(data)
	if !containsStr(content, "model") {
		t.Error("config.toml should contain 'model' key")
	}
	if !containsStr(content, "approval_policy") {
		t.Error("config.toml should contain 'approval_policy' key")
	}
}

func TestCodex_MCPConfig_MultipleServers(t *testing.T) {
	a := newTestCodexAdapter(t)
	ctx := context.Background()

	cfg := &types.UniversalMCPConfig{
		Servers: map[string]types.UniversalMCPServer{
			"filesystem": {
				Transport: "stdio",
				Command:   "npx",
				Args:      []string{"-y", "@mcp/server-filesystem"},
				Env:       map[string]string{"HOME": "/tmp"},
			},
			"github": {
				Transport: "stdio",
				Command:   "npx",
				Args:      []string{"-y", "@mcp/server-github"},
			},
			"custom-api": {
				Transport: "sse",
				URL:       "https://mcp.mycompany.com/api",
				Headers:   map[string]string{"X-API-Version": "2024-01"},
			},
		},
	}
	if err := a.WriteMCPConfig(ctx, cfg); err != nil {
		t.Fatalf("WriteMCPConfig() error: %v", err)
	}

	got, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	if len(got.Servers) != 3 {
		t.Fatalf("Servers count = %d, want 3", len(got.Servers))
	}
	for _, name := range []string{"filesystem", "github", "custom-api"} {
		if _, ok := got.Servers[name]; !ok {
			t.Errorf("missing server %q", name)
		}
	}
	if got.Servers["filesystem"].Env["HOME"] != "/tmp" {
		t.Errorf("Env[HOME] = %q", got.Servers["filesystem"].Env["HOME"])
	}
}

// ---- Install / Remove MCP ----

func TestCodex_InstallMCP(t *testing.T) {
	a := newTestCodexAdapter(t)
	ctx := context.Background()

	srv := types.UniversalMCPServer{
		Transport: "stdio",
		Command:   "test-server",
	}
	if err := a.InstallMCP(ctx, "test-mcp", srv); err != nil {
		t.Fatalf("InstallMCP() error: %v", err)
	}

	// Verify config.
	cfg, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	if _, ok := cfg.Servers["test-mcp"]; !ok {
		t.Error("server not in config")
	}

	// Verify manifest.
	m, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest() error: %v", err)
	}
	if _, ok := m.MCPServers["test-mcp"]; !ok {
		t.Error("server not in manifest")
	}
}

func TestCodex_RemoveMCP(t *testing.T) {
	a := newTestCodexAdapter(t)
	ctx := context.Background()

	// Install then remove.
	srv := types.UniversalMCPServer{
		Transport: "stdio",
		Command:   "test-server",
	}
	if err := a.InstallMCP(ctx, "test-mcp", srv); err != nil {
		t.Fatalf("InstallMCP() error: %v", err)
	}
	if err := a.RemoveMCP(ctx, "test-mcp"); err != nil {
		t.Fatalf("RemoveMCP() error: %v", err)
	}

	cfg, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	if _, ok := cfg.Servers["test-mcp"]; ok {
		t.Error("server should have been removed from config")
	}

	m, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest() error: %v", err)
	}
	if _, ok := m.MCPServers["test-mcp"]; ok {
		t.Error("server should have been removed from manifest")
	}
}

func TestCodex_InstallMCP_Update(t *testing.T) {
	a := newTestCodexAdapter(t)
	ctx := context.Background()

	srv1 := types.UniversalMCPServer{
		Transport: "stdio",
		Command:   "old-cmd",
	}
	if err := a.InstallMCP(ctx, "test-mcp", srv1); err != nil {
		t.Fatalf("InstallMCP() error: %v", err)
	}

	srv2 := types.UniversalMCPServer{
		Transport: "sse",
		URL:       "https://new.example.com",
	}
	if err := a.InstallMCP(ctx, "test-mcp", srv2); err != nil {
		t.Fatalf("InstallMCP() update error: %v", err)
	}

	cfg, err := a.ReadMCPConfig(ctx)
	if err != nil {
		t.Fatalf("ReadMCPConfig() error: %v", err)
	}
	s := cfg.Servers["test-mcp"]
	if s.Transport != "sse" {
		t.Errorf("Transport = %q, want sse", s.Transport)
	}
	if s.URL != "https://new.example.com" {
		t.Errorf("URL = %q", s.URL)
	}
}

// ---- Backup / Restore ----

func TestCodex_Backup(t *testing.T) {
	a := newTestCodexAdapter(t)
	ctx := context.Background()

	writeCodexConfig(t, a, `model = "gpt-5.1"`)

	m := &Manifest{Version: 1, EnvName: "test"}
	if err := a.WriteManifest(ctx, m); err != nil {
		t.Fatalf("WriteManifest() error: %v", err)
	}

	backupID, err := a.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error: %v", err)
	}
	if backupID == "" {
		t.Fatal("Backup() returned empty backupID")
	}

	// Verify backup files exist.
	backupDir := a.backupDir()
	if _, err := os.Stat(filepath.Join(backupDir, backupID+".toml")); os.IsNotExist(err) {
		t.Error("config.toml backup not found")
	}
	if _, err := os.Stat(filepath.Join(backupDir, backupID+"-manifest.json")); os.IsNotExist(err) {
		t.Error("manifest backup not found")
	}
}

func TestCodex_Restore(t *testing.T) {
	a := newTestCodexAdapter(t)
	ctx := context.Background()

	writeCodexConfig(t, a, `model = "original"`)
	m := &Manifest{Version: 1, EnvName: "original"}
	if err := a.WriteManifest(ctx, m); err != nil {
		t.Fatalf("WriteManifest() error: %v", err)
	}

	backupID, err := a.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error: %v", err)
	}

	// Modify the files.
	if err := a.WriteManifest(ctx, &Manifest{Version: 1, EnvName: "modified"}); err != nil {
		t.Fatalf("WriteManifest() error: %v", err)
	}
	writeCodexConfig(t, a, `model = "modified"`)

	// Restore.
	if err := a.Restore(ctx, backupID); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}

	// Verify restored values.
	data, err := os.ReadFile(a.configPath())
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	if !containsStr(string(data), "original") {
		t.Error("config.toml should contain 'original' after restore")
	}

	got, err := a.ReadManifest(ctx)
	if err != nil {
		t.Fatalf("ReadManifest() error: %v", err)
	}
	if got.EnvName != "original" {
		t.Errorf("EnvName = %q, want original", got.EnvName)
	}
}

func TestCodex_Backup_FirstRun(t *testing.T) {
	a := newTestCodexAdapter(t)
	ctx := context.Background()

	backupID, err := a.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error: %v", err)
	}
	if backupID == "" {
		t.Fatal("Backup() returned empty backupID")
	}

	backupDir := a.backupDir()
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		t.Error("backup dir should have been created")
	}
}

func TestCodex_Restore_MissingFile(t *testing.T) {
	a := newTestCodexAdapter(t)
	ctx := context.Background()

	writeCodexConfig(t, a, `model = "exists"`)

	// Backup, then delete a backup file to simulate partial backup.
	backupID, err := a.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error: %v", err)
	}
	os.Remove(filepath.Join(a.backupDir(), backupID+".toml"))

	// Restore should not fail — missing file is non-fatal.
	if err := a.Restore(ctx, backupID); err != nil {
		t.Fatalf("Restore() should not fail on missing backup file: %v", err)
	}
	// The config.toml should be removed since backup is missing.
	if _, err := os.Stat(a.configPath()); !os.IsNotExist(err) {
		t.Error("config.toml should have been removed")
	}
}

// ---- Registry ----

func TestCodex_Registry(t *testing.T) {
	a, err := Get("codex")
	if err != nil {
		t.Fatalf("Get(codex) error: %v", err)
	}
	if a.Name() != "codex" {
		t.Errorf("Name() = %q", a.Name())
	}

	names := List()
	found := false
	for _, n := range names {
		if n == "codex" {
			found = true
			break
		}
	}
	if !found {
		t.Error("codex not found in List()")
	}
}

// ---- Interface compliance ----

func TestCodex_ImplementsAgentAdapter(t *testing.T) {
	var a AgentAdapter = NewCodexAdapter()
	_ = a
}

// ---- Helpers ----

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
