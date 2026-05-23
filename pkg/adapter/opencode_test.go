package adapter

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestOpenCodeAdapter(t *testing.T) (*OpenCodeAdapter, string) {
	t.Helper()
	basePath := t.TempDir()
	return NewOpenCodeAdapterWithBase(basePath), basePath
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

func TestOpenCode_StubsReturnErrors(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestOpenCodeAdapter(t)

	if err := a.InstallSkill(ctx, "x", "/tmp"); err == nil {
		t.Error("InstallSkill should return error (not implemented)")
	}
	if err := a.RemoveSkill(ctx, "x"); err == nil {
		t.Error("RemoveSkill should return error (not implemented)")
	}
	if _, err := a.ListSkills(ctx); err == nil {
		t.Error("ListSkills should return error (not implemented)")
	}
	if err := a.InstallAgent(ctx, "x", "/tmp"); err == nil {
		t.Error("InstallAgent should return error (not implemented)")
	}
	if err := a.RemoveAgent(ctx, "x"); err == nil {
		t.Error("RemoveAgent should return error (not implemented)")
	}
	if _, err := a.ListAgents(ctx); err == nil {
		t.Error("ListAgents should return error (not implemented)")
	}
}
