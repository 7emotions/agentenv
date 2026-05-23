package resolver

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestPubGrubSimpleGraph(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
			"github:test/B": {"2.1.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "1.0.0", "B@>=2.0"),
			"github:test/B": makeAgentPkg("B", "2.1.0"),
		},
	}
	r := NewPubGrubResolver()
	r.RegisterHandler("github", m)

	result, err := r.Resolve(context.Background(), []PackageRequest{
		req("A", "github:test/A", "^1.0"),
	}, DefaultResolveOptions())

	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	if len(result.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(result.Packages))
	}

	pkgMap := make(map[string]ResolvedPackage)
	for _, pkg := range result.Packages {
		pkgMap[pkg.Name] = pkg
	}

	pkgA, ok := pkgMap["A"]
	if !ok {
		t.Fatal("expected package A in result")
	}
	if pkgA.Resolved == "" {
		t.Error("A.Resolved is empty")
	}
	if pkgA.Type != "skill" {
		t.Errorf("A.Type = %q, want %q", pkgA.Type, "skill")
	}
	if pkgA.Version != "^1.0" {
		t.Errorf("A.Version = %q, want %q", pkgA.Version, "^1.0")
	}

	pkgB, ok := pkgMap["B"]
	if !ok {
		t.Fatal("expected package B in result")
	}
	if pkgB.Resolved != "2.1.0" {
		t.Errorf("B.Resolved = %q, want %q", pkgB.Resolved, "2.1.0")
	}
	if pkgB.Type != "skill" {
		t.Errorf("B.Type = %q, want %q", pkgB.Type, "skill")
	}

	if len(pkgA.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency for A, got %d", len(pkgA.Dependencies))
	}
	dep := pkgA.Dependencies[0]
	if dep.Name != "B" {
		t.Errorf("A dep name = %q, want %q", dep.Name, "B")
	}
	if dep.Resolved != "2.1.0" {
		t.Errorf("A dep Resolved = %q, want %q", dep.Resolved, "2.1.0")
	}

	if len(result.Warnings) > 0 {
		t.Errorf("expected no warnings, got %d: %s", len(result.Warnings),
			strings.Join(result.Warnings, "; "))
	}

	if result.Duration <= 0 {
		t.Errorf("expected non-zero Duration, got %v", result.Duration)
	}

	_ = fmt.Sprintf("A@%s + B@%s resolved successfully", pkgA.Resolved, pkgB.Resolved)
}

func TestPubGrubUnsatisfiable(t *testing.T) {
	// Scenario:
	// - Root depends on A@^1.0
	// - A@1.0 depends on B@>=2.0
	// - B only has version 1.0.0  (no version satisfies >=2.0)
	// Expected: Resolve fails because constraint is unsatisfiable.

	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
			"github:test/B": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "1.0.0", "B@>=2.0"),
		},
	}

	r := NewPubGrubResolver()
	r.RegisterHandler("github", m)

	_, err := r.Resolve(context.Background(), []PackageRequest{
		req("A", "github:test/A", "^1.0.0"),
	}, DefaultResolveOptions())

	if err == nil {
		t.Fatal("expected unsatisfiable constraint error, got nil")
	}
	if !strings.Contains(err.Error(), "no version satisfies") &&
		!strings.Contains(err.Error(), "No versions of") {
		t.Errorf("error should mention version satisfaction issue, got: %v", err)
	}
}

func TestPubGrubResolvedBy(t *testing.T) {
	// Verify ResolvedBy is correct for a 3-deep transitive chain:
	//   root → A (ResolvedBy: "(root)")
	//   root → A → B (ResolvedBy: "A")
	//   root → A → B → C (ResolvedBy: "B")

	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
			"github:test/B": {"1.0.0"},
			"github:test/C": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "1.0.0", "B@^1.0"),
			"github:test/B": makeAgentPkg("B", "1.0.0", "C@^1.0"),
			"github:test/C": makeAgentPkg("C", "1.0.0"),
		},
	}

	r := NewPubGrubResolver()
	r.RegisterHandler("github", m)

	result, err := r.Resolve(context.Background(), []PackageRequest{
		req("A", "github:test/A", "^1.0"),
	}, DefaultResolveOptions())

	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	pkgBy := make(map[string]ResolvedPackage)
	for _, pkg := range result.Packages {
		pkgBy[pkg.Name] = pkg
	}

	// Root package A → ResolvedBy = "(root)"
	if pkgA, ok := pkgBy["A"]; !ok {
		t.Fatal("expected package A in result")
	} else if pkgA.ResolvedBy != "(root)" {
		t.Errorf("A.ResolvedBy = %q, want %q", pkgA.ResolvedBy, "(root)")
	}

	// Transitive B (depended on by A) → ResolvedBy = "A"
	if pkgB, ok := pkgBy["B"]; !ok {
		t.Fatal("expected package B in result")
	} else if pkgB.ResolvedBy != "A" {
		t.Errorf("B.ResolvedBy = %q, want %q", pkgB.ResolvedBy, "A")
	}

	// Transitive C (depended on by B) → ResolvedBy = "B"
	if pkgC, ok := pkgBy["C"]; !ok {
		t.Fatal("expected package C in result")
	} else if pkgC.ResolvedBy != "B" {
		t.Errorf("C.ResolvedBy = %q, want %q", pkgC.ResolvedBy, "B")
	}

	if len(result.Packages) != 3 {
		t.Errorf("expected 3 packages, got %d", len(result.Packages))
	}
}

