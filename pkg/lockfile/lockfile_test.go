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

	if lf.Version != 3 {
		t.Errorf("Version = %d, want 3", lf.Version)
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

	if lf.Version != 3 {
		t.Errorf("Version = %d, want 3", lf.Version)
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

func TestLockfileV3Roundtrip(t *testing.T) {
	// Build a Lockfile with v3 fields: ResolvedBy, Dep.Type, Dep.Source.
	deps := []types.LockedDep{
		{
			Name:     "dep-a",
			Type:     types.PackageTypeSkill,
			Version:  "1.0.0",
			Resolved: "sha256:abc",
			Source:   "npm:@scope/pkg",
		},
		{
			Name:     "dep-b",
			Type:     types.PackageTypeMCP,
			Version:  "2.0.0",
			Resolved: "sha256:def",
			Source:   "github:owner/mcp",
		},
	}

	pkgs := []types.LockedPackage{
		{
			Name:         "test-pkg",
			Type:         types.PackageTypeSkill,
			Source:       "github:owner/repo",
			Version:      "2.0.0",
			Resolved:     "https://example.com/pkg.tar.gz",
			SHA256:       "sha256:123",
			ResolvedBy:   "(root)",
			Dependencies: deps,
		},
		{
			Name:       "test-mcp",
			Type:       types.PackageTypeMCP,
			Source:     "npm:@scope/mcp",
			Version:    "1.0.0",
			Resolved:   "1.0.0",
			ResolvedBy: "test-skill@1.0.0",
		},
	}

	lf := &types.Lockfile{
		Version:   3,
		Generated: "2025-06-01T00:00:00Z",
		Packages:  pkgs,
		Environment: types.LockfileEnvSnapshot{
			SkillsCount: 1,
			MCPsCount:   1,
		},
	}

	// Write v3 lockfile to bytes.
	data, err := parser.WriteLockfile(lf)
	if err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}

	// Read it back.
	got, err := Read(data)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	// Verify top-level fields.
	if got.Version != 3 {
		t.Errorf("Version = %d, want 3", got.Version)
	}
	if got.Generated != "2025-06-01T00:00:00Z" {
		t.Errorf("Generated = %q, want %q", got.Generated, "2025-06-01T00:00:00Z")
	}
	if len(got.Packages) != 2 {
		t.Fatalf("len(Packages) = %d, want 2", len(got.Packages))
	}

	// Verify first package (skill with deps).
	p0 := got.Packages[0]
	if p0.Name != "test-pkg" {
		t.Errorf("Package[0] Name = %q, want %q", p0.Name, "test-pkg")
	}
	if p0.Type != types.PackageTypeSkill {
		t.Errorf("Package[0] Type = %q, want %q", p0.Type, types.PackageTypeSkill)
	}
	if p0.Source != "github:owner/repo" {
		t.Errorf("Package[0] Source = %q, want %q", p0.Source, "github:owner/repo")
	}
	if p0.Version != "2.0.0" {
		t.Errorf("Package[0] Version = %q, want %q", p0.Version, "2.0.0")
	}
	if p0.Resolved != "https://example.com/pkg.tar.gz" {
		t.Errorf("Package[0] Resolved = %q, want %q", p0.Resolved, "https://example.com/pkg.tar.gz")
	}
	if p0.SHA256 != "sha256:123" {
		t.Errorf("Package[0] SHA256 = %q, want %q", p0.SHA256, "sha256:123")
	}
	if p0.ResolvedBy != "(root)" {
		t.Errorf("Package[0] ResolvedBy = %q, want %q", p0.ResolvedBy, "(root)")
	}

	// Verify dependencies on first package.
	if len(p0.Dependencies) != 2 {
		t.Fatalf("Package[0] len(Dependencies) = %d, want 2", len(p0.Dependencies))
	}

	gotDepA := p0.Dependencies[0]
	if gotDepA.Name != "dep-a" {
		t.Errorf("Dep[0] Name = %q, want %q", gotDepA.Name, "dep-a")
	}
	if gotDepA.Type != types.PackageTypeSkill {
		t.Errorf("Dep[0] Type = %q, want %q", gotDepA.Type, types.PackageTypeSkill)
	}
	if gotDepA.Version != "1.0.0" {
		t.Errorf("Dep[0] Version = %q, want %q", gotDepA.Version, "1.0.0")
	}
	if gotDepA.Resolved != "sha256:abc" {
		t.Errorf("Dep[0] Resolved = %q, want %q", gotDepA.Resolved, "sha256:abc")
	}
	if gotDepA.Source != "npm:@scope/pkg" {
		t.Errorf("Dep[0] Source = %q, want %q", gotDepA.Source, "npm:@scope/pkg")
	}

	gotDepB := p0.Dependencies[1]
	if gotDepB.Name != "dep-b" {
		t.Errorf("Dep[1] Name = %q, want %q", gotDepB.Name, "dep-b")
	}
	if gotDepB.Type != types.PackageTypeMCP {
		t.Errorf("Dep[1] Type = %q, want %q", gotDepB.Type, types.PackageTypeMCP)
	}
	if gotDepB.Version != "2.0.0" {
		t.Errorf("Dep[1] Version = %q, want %q", gotDepB.Version, "2.0.0")
	}
	if gotDepB.Resolved != "sha256:def" {
		t.Errorf("Dep[1] Resolved = %q, want %q", gotDepB.Resolved, "sha256:def")
	}
	if gotDepB.Source != "github:owner/mcp" {
		t.Errorf("Dep[1] Source = %q, want %q", gotDepB.Source, "github:owner/mcp")
	}

	// Verify second package (mcp with ResolvedBy).
	p1 := got.Packages[1]
	if p1.Name != "test-mcp" {
		t.Errorf("Package[1] Name = %q, want %q", p1.Name, "test-mcp")
	}
	if p1.Type != types.PackageTypeMCP {
		t.Errorf("Package[1] Type = %q, want %q", p1.Type, types.PackageTypeMCP)
	}
	if p1.ResolvedBy != "test-skill@1.0.0" {
		t.Errorf("Package[1] ResolvedBy = %q, want %q", p1.ResolvedBy, "test-skill@1.0.0")
	}
}

func TestLockfileV1ToV3Migration(t *testing.T) {
	// v1 lockfile YAML without v3 fields (ResolvedBy, Dep.Type, Dep.Source).
	v1YAML := `version: 1
generated: "2025-01-01T00:00:00Z"
packages:
  - name: test-pkg
    type: skill
    source: github:owner/repo
    version: "1.0.0"
    resolved: https://example.com/pkg.tar.gz
    dependencies:
      - name: dep-a
        version: "1.0.0"
        resolved: sha256:abc
  - name: test-mcp
    type: mcp
    source: npm:@scope/mcp
    version: "^2.0"
    resolved: "2.1.0"
`

	// Read v1 lockfile.
	lf, err := parser.ParseLockfile([]byte(v1YAML))
	if err != nil {
		t.Fatalf("ParseLockfile(v1): %v", err)
	}

	if lf.Version != 1 {
		t.Errorf("Version = %d, want 1", lf.Version)
	}

	// Upgrade to v3 and write.
	lf.Version = 3
	data, err := parser.WriteLockfile(lf)
	if err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}

	// Read back the v3 lockfile.
	got, err := Read(data)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.Version != 3 {
		t.Errorf("Version = %d, want 3", got.Version)
	}

	// Verify packages exist.
	if len(got.Packages) != 2 {
		t.Fatalf("len(Packages) = %d, want 2", len(got.Packages))
	}

	// v1 packages should have empty ResolvedBy.
	for i, pkg := range got.Packages {
		if pkg.ResolvedBy != "" {
			t.Errorf("Package[%d] ResolvedBy = %q, want empty string for migrated v1 package", i, pkg.ResolvedBy)
		}
	}

	// v1 deps should have empty Type (default PackageType) and empty Source.
	p0 := got.Packages[0]
	if len(p0.Dependencies) != 1 {
		t.Fatalf("len(Dependencies) = %d, want 1", len(p0.Dependencies))
	}

	gotDep := p0.Dependencies[0]
	if gotDep.Name != "dep-a" {
		t.Errorf("Dep Name = %q, want %q", gotDep.Name, "dep-a")
	}
	if gotDep.Type != "" {
		t.Errorf("Dep Type for v1 = %q, want empty string", gotDep.Type)
	}
	if gotDep.Source != "" {
		t.Errorf("Dep Source for v1 = %q, want empty string", gotDep.Source)
	}
	if gotDep.Version != "1.0.0" {
		t.Errorf("Dep Version = %q, want %q", gotDep.Version, "1.0.0")
	}
	if gotDep.Resolved != "sha256:abc" {
		t.Errorf("Dep Resolved = %q, want %q", gotDep.Resolved, "sha256:abc")
	}
}

func TestLockfileV2RoundTrip(t *testing.T) {
	// Build a LockedDep with a Source field (v2 format).
	deps := []types.LockedDep{
		{Name: "dep-a", Version: "1.0.0", Resolved: "sha256:abc", Source: "npm:@scope/pkg"},
	}

	pkgs := []types.LockedPackage{
		{
			Name:         "test-pkg",
			Type:         types.PackageTypeSkill,
			Source:       "github:owner/repo",
			Version:      "2.0.0",
			Resolved:     "https://example.com/pkg.tar.gz",
			Dependencies: deps,
		},
	}

	lf := &types.Lockfile{
		Version:   3,
		Generated: "2025-01-01T00:00:00Z",
		Packages:  pkgs,
		Environment: types.LockfileEnvSnapshot{
			SkillsCount: 1,
		},
	}

	data, err := parser.WriteLockfile(lf)
	if err != nil {
		t.Fatalf("WriteLockfile: %v", err)
	}

	got, err := Read(data)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(got.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(got.Packages))
	}
	if len(got.Packages[0].Dependencies) != 1 {
		t.Fatalf("len(Dependencies) = %d, want 1", len(got.Packages[0].Dependencies))
	}

	gotDep := got.Packages[0].Dependencies[0]
	if gotDep.Source != "npm:@scope/pkg" {
		t.Errorf("Dep Source = %q, want %q", gotDep.Source, "npm:@scope/pkg")
	}
	if gotDep.Name != "dep-a" {
		t.Errorf("Dep Name = %q, want %q", gotDep.Name, "dep-a")
	}
	if gotDep.Version != "1.0.0" {
		t.Errorf("Dep Version = %q, want %q", gotDep.Version, "1.0.0")
	}
	if gotDep.Resolved != "sha256:abc" {
		t.Errorf("Dep Resolved = %q, want %q", gotDep.Resolved, "sha256:abc")
	}
}

func TestLockfileV1WithoutSource(t *testing.T) {
	// v1 lockfile YAML without source in dependency entries should still parse.
	v1YAML := `version: 1
generated: "2025-01-01T00:00:00Z"
packages:
  - name: test-pkg
    type: skill
    source: github:owner/repo
    version: "1.0.0"
    resolved: https://example.com/pkg.tar.gz
    dependencies:
      - name: dep-a
        version: "1.0.0"
        resolved: sha256:abc
`

	lf, err := parser.ParseLockfile([]byte(v1YAML))
	if err != nil {
		t.Fatalf("ParseLockfile(v1): %v", err)
	}

	if len(lf.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(lf.Packages))
	}
	if len(lf.Packages[0].Dependencies) != 1 {
		t.Fatalf("len(Dependencies) = %d, want 1", len(lf.Packages[0].Dependencies))
	}

	gotDep := lf.Packages[0].Dependencies[0]
	if gotDep.Source != "" {
		t.Errorf("Dep Source for v1 should be empty, got %q", gotDep.Source)
	}
	if gotDep.Name != "dep-a" {
		t.Errorf("Dep Name = %q, want %q", gotDep.Name, "dep-a")
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
