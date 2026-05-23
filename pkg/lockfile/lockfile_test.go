package lockfile

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/7emotions/agentenv/pkg/parser"
	"github.com/7emotions/agentenv/pkg/resolver"
	"github.com/7emotions/agentenv/pkg/types"
)

func TestGenerate(t *testing.T) {
	pkgs := []resolver.ResolvedPackage{
		{
			Name:     "my-skill",
			Type:     "skill",
			Source:   "github:owner/repo",
			Version:  "1.0.0",
			Resolved: "https://github.com/owner/repo/archive/v1.0.0.tar.gz",
			SHA256:   "abc123",
		},
		{
			Name:     "my-mcp",
			Type:     "mcp",
			Source:   "npm:@scope/mcp",
			Version:  "^2.0",
			Resolved: "2.1.0",
		},
	}

	lf, err := Generate(pkgs)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if lf.Version != 1 {
		t.Errorf("Version = %d, want 1", lf.Version)
	}
	if lf.Generated == "" {
		t.Error("Generated timestamp is empty")
	}
	if len(lf.Packages) != 2 {
		t.Fatalf("got %d packages, want 2", len(lf.Packages))
	}

	// Packages should be sorted by type: "mcp" < "skill"
	if lf.Packages[0].Type != types.PackageTypeMCP {
		t.Errorf("First package type = %s, want mcp", lf.Packages[0].Type)
	}
	if lf.Packages[1].Type != types.PackageTypeSkill {
		t.Errorf("Second package type = %s, want skill", lf.Packages[1].Type)
	}
}

func TestRoundTrip(t *testing.T) {
	pkgs := []resolver.ResolvedPackage{
		{
			Name:     "test-skill",
			Type:     "skill",
			Source:   "github:owner/repo",
			Version:  "1.0.0",
			Resolved: "https://example.com/pkg.tar.gz",
			SHA256:   "def456",
		},
	}

	lf, err := Generate(pkgs)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	data, err := parser.WriteLockfile(lf)
	if err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "agent.lock")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ReadFromFile(path)
	if err != nil {
		t.Fatalf("ReadFromFile: %v", err)
	}

	if got.Version != lf.Version {
		t.Errorf("Version = %d, want %d", got.Version, lf.Version)
	}
	if len(got.Packages) != len(lf.Packages) {
		t.Fatalf("len(Packages) = %d, want %d", len(got.Packages), len(lf.Packages))
	}
	if got.Packages[0].Name != lf.Packages[0].Name {
		t.Errorf("Name = %s, want %s", got.Packages[0].Name, lf.Packages[0].Name)
	}
	if got.Packages[0].SHA256 != lf.Packages[0].SHA256 {
		t.Errorf("SHA256 = %s, want %s", got.Packages[0].SHA256, lf.Packages[0].SHA256)
	}
}

func TestDeterminism(t *testing.T) {
	pkgs := []resolver.ResolvedPackage{
		{Name: "b-pkg", Type: "skill", Source: "github:b/repo", Version: "1.0.0", Resolved: "url-b"},
		{Name: "a-pkg", Type: "skill", Source: "github:a/repo", Version: "2.0.0", Resolved: "url-a"},
		{Name: "c-pkg", Type: "mcp", Source: "npm:c/mcp", Version: "1.0.0", Resolved: "url-c"},
	}

	lf1, err := Generate(pkgs)
	if err != nil {
		t.Fatalf("Generate 1: %v", err)
	}
	lf2, err := Generate(pkgs)
	if err != nil {
		t.Fatalf("Generate 2: %v", err)
	}

	data1, _ := parser.WriteLockfile(lf1)
	data2, _ := parser.WriteLockfile(lf2)

	if string(data1) != string(data2) {
		t.Error("Generated lockfiles are not byte-identical")
	}

	h1 := fmt.Sprintf("%x", sha256.Sum256(data1))
	h2 := fmt.Sprintf("%x", sha256.Sum256(data2))
	if h1 != h2 {
		t.Errorf("SHA256 mismatch: %s != %s", h1, h2)
	}
}

func TestMultipleDeps(t *testing.T) {
	pkgs := []resolver.ResolvedPackage{
		{
			Name:     "main-pkg",
			Type:     "skill",
			Source:   "github:owner/repo",
			Version:  "1.0.0",
			Resolved: "url-main",
			Dependencies: []resolver.ResolvedDep{
				{Name: "dep-b", Type: "skill", Version: ">=2.0", Resolved: "2.5.0"},
				{Name: "dep-a", Type: "skill", Version: ">=1.0", Resolved: "1.2.0"},
			},
		},
	}

	lf, err := Generate(pkgs)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(lf.Packages) != 1 {
		t.Fatalf("got %d packages, want 1", len(lf.Packages))
	}
	deps := lf.Packages[0].Dependencies
	if len(deps) != 2 {
		t.Fatalf("got %d deps, want 2", len(deps))
	}
	// Dependencies should be sorted by name
	if deps[0].Name != "dep-a" {
		t.Errorf("First dep name = %s, want dep-a", deps[0].Name)
	}
	if deps[1].Name != "dep-b" {
		t.Errorf("Second dep name = %s, want dep-b", deps[1].Name)
	}
	if deps[0].Resolved != "1.2.0" {
		t.Errorf("First dep resolved = %s, want 1.2.0", deps[0].Resolved)
	}
	if deps[1].Resolved != "2.5.0" {
		t.Errorf("Second dep resolved = %s, want 2.5.0", deps[1].Resolved)
	}
}

func TestEmptyLockfile(t *testing.T) {
	lf, err := Generate(nil)
	if err != nil {
		t.Fatalf("Generate(nil): %v", err)
	}

	if lf.Version != 1 {
		t.Errorf("Version = %d, want 1", lf.Version)
	}
	if len(lf.Packages) != 0 {
		t.Errorf("len(Packages) = %d, want 0", len(lf.Packages))
	}

	// Empty lockfile should serialize as valid YAML
	data, err := parser.WriteLockfile(lf)
	if err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}

	// Empty lockfile should be readable
	got, err := Read(data)
	if err != nil {
		t.Fatalf("Read of empty lockfile: %v", err)
	}
	if len(got.Packages) != 0 {
		t.Errorf("len(Packages) = %d, want 0", len(got.Packages))
	}
}

func TestSnapshotCounts(t *testing.T) {
	pkgs := []resolver.ResolvedPackage{
		{Name: "s1", Type: "skill", Source: "github:a/s1", Version: "1.0", Resolved: "url"},
		{Name: "s2", Type: "skill", Source: "github:a/s2", Version: "1.0", Resolved: "url"},
		{Name: "m1", Type: "mcp", Source: "npm:@scope/m1", Version: "1.0", Resolved: "url"},
		{Name: "a1", Type: "agent", Source: "github:b/a1", Version: "1.0", Resolved: "url"},
		{Name: "t1", Type: "tool", Source: "github:c/t1", Version: "1.0", Resolved: "url"},
		{Name: "h1", Type: "hook", Source: "local:./h1", Version: "1.0", Resolved: "url"},
		{Name: "p1", Type: "prompt", Source: "github:d/p1", Version: "1.0", Resolved: "url"},
	}

	lf, err := Generate(pkgs)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if lf.Environment.SkillsCount != 2 {
		t.Errorf("SkillsCount = %d, want 2", lf.Environment.SkillsCount)
	}
	if lf.Environment.MCPsCount != 1 {
		t.Errorf("MCPsCount = %d, want 1", lf.Environment.MCPsCount)
	}
	if lf.Environment.AgentsCount != 1 {
		t.Errorf("AgentsCount = %d, want 1", lf.Environment.AgentsCount)
	}
	if lf.Environment.ToolsCount != 1 {
		t.Errorf("ToolsCount = %d, want 1", lf.Environment.ToolsCount)
	}
	if lf.Environment.HooksCount != 1 {
		t.Errorf("HooksCount = %d, want 1", lf.Environment.HooksCount)
	}
	if lf.Environment.PromptsCount != 1 {
		t.Errorf("PromptsCount = %d, want 1", lf.Environment.PromptsCount)
	}
}
