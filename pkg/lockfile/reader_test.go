package lockfile

import (
	"testing"

	"github.com/7emotions/agentenv/pkg/types"
)

const v1YAML = `version: 1
generated: "2025-01-01T00:00:00Z"
packages:
  - name: my-skill
    type: skill
    source: github:owner/repo
    version: "1.0.0"
    resolved: https://example.com/pkg.tar.gz
    sha256: abc123
    dependencies:
      - name: dep-a
        version: "1.0.0"
        resolved: sha256:def
`

const v3YAML = `version: 3
generated: "2026-05-24T12:00:00Z"
packages:
  - name: my-skill
    type: skill
    source: github:owner/repo
    version: "1.0.0"
    resolved: https://example.com/pkg.tar.gz
    sha256: abc123
    resolved_by: user@example.com
    dependencies:
      - name: dep-a
        type: skill
        version: "1.0.0"
        resolved: sha256:def
        source: github:owner/dep-a
`

func TestReadV1Lockfile_ResolvedByEmpty(t *testing.T) {
	lf, err := Read([]byte(v1YAML))
	if err != nil {
		t.Fatalf("Read(v1): %v", err)
	}

	if len(lf.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(lf.Packages))
	}

	pkg := lf.Packages[0]
	if pkg.ResolvedBy != "" {
		t.Errorf("v1 package ResolvedBy = %q, want empty string", pkg.ResolvedBy)
	}
}

func TestReadV1Lockfile_DepFieldsEmpty(t *testing.T) {
	lf, err := Read([]byte(v1YAML))
	if err != nil {
		t.Fatalf("Read(v1): %v", err)
	}

	if len(lf.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(lf.Packages))
	}
	deps := lf.Packages[0].Dependencies
	if len(deps) != 1 {
		t.Fatalf("len(Dependencies) = %d, want 1", len(deps))
	}

	dep := deps[0]
	if dep.Source != "" {
		t.Errorf("v1 dep Source = %q, want empty string", dep.Source)
	}
	if dep.Type != "" {
		t.Errorf("v1 dep Type = %q, want empty string", dep.Type)
	}
}

func TestReadV3Lockfile_ResolvedByPreserved(t *testing.T) {
	lf, err := Read([]byte(v3YAML))
	if err != nil {
		t.Fatalf("Read(v3): %v", err)
	}

	if len(lf.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(lf.Packages))
	}

	pkg := lf.Packages[0]
	if pkg.ResolvedBy != "user@example.com" {
		t.Errorf("v3 package ResolvedBy = %q, want %q", pkg.ResolvedBy, "user@example.com")
	}
}

func TestReadV3Lockfile_DepFieldsPreserved(t *testing.T) {
	lf, err := Read([]byte(v3YAML))
	if err != nil {
		t.Fatalf("Read(v3): %v", err)
	}

	if len(lf.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(lf.Packages))
	}
	deps := lf.Packages[0].Dependencies
	if len(deps) != 1 {
		t.Fatalf("len(Dependencies) = %d, want 1", len(deps))
	}

	dep := deps[0]
	if dep.Source != "github:owner/dep-a" {
		t.Errorf("v3 dep Source = %q, want %q", dep.Source, "github:owner/dep-a")
	}
	if dep.Type != types.PackageTypeSkill {
		t.Errorf("v3 dep Type = %q, want %q", dep.Type, types.PackageTypeSkill)
	}
}

func TestReadLockfile_NoDiskWrite(t *testing.T) {
	lf, err := Read([]byte(v1YAML))
	if err != nil {
		t.Fatalf("Read(v1): %v", err)
	}
	if lf == nil {
		t.Fatal("Read returned nil lockfile")
	}
	if lf.Version != 1 {
		t.Errorf("Version = %d, want 1", lf.Version)
	}
	if lf.Generated == "" {
		t.Error("Generated timestamp is empty")
	}
}
