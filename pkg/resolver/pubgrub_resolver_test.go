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

func TestPubGrubEdgeCases(t *testing.T) {
	// --- Cancel: pre-canceled context → Resolve stops immediately ---
	t.Run("Cancel", func(t *testing.T) {
		m := &mockSource{
			versions: map[string][]string{
				"github:test/A": {"1.0.0"},
			},
			pkgs: map[string][]byte{
				"github:test/A": makeAgentPkg("A", "1.0.0"),
			},
		}
		r := NewPubGrubResolver()
		r.RegisterHandler("github", m)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := r.Resolve(ctx, []PackageRequest{
			req("A", "github:test/A", "^1.0"),
		}, DefaultResolveOptions())

		if err == nil {
			t.Fatal("expected context cancellation error, got nil")
		}
		if !strings.Contains(err.Error(), "canceled") {
			t.Errorf("error should contain 'canceled', got: %v", err)
		}
	})

	// --- Empty: 0 requests → empty result, no error ---
	t.Run("Empty", func(t *testing.T) {
		r := NewPubGrubResolver()

		// nil slice
		result, err := r.Resolve(context.Background(), nil, DefaultResolveOptions())
		if err != nil {
			t.Fatalf("nil requests: unexpected error: %v", err)
		}
		if len(result.Packages) != 0 {
			t.Errorf("nil requests: expected 0 packages, got %d", len(result.Packages))
		}

		// empty slice
		result2, err2 := r.Resolve(context.Background(), []PackageRequest{}, DefaultResolveOptions())
		if err2 != nil {
			t.Fatalf("empty requests: unexpected error: %v", err2)
		}
		if len(result2.Packages) != 0 {
			t.Errorf("empty requests: expected 0 packages, got %d", len(result2.Packages))
		}
	})

	// --- No Version: package has no releases → error ---
	t.Run("NoVersion", func(t *testing.T) {
		m := &mockSource{
			versions: map[string][]string{
				"github:test/empty-pkg": {},
			},
		}
		r := NewPubGrubResolver()
		r.RegisterHandler("github", m)

		_, err := r.Resolve(context.Background(), []PackageRequest{
			req("empty-pkg", "github:test/empty-pkg", "^1.0"),
		}, DefaultResolveOptions())

		if err == nil {
			t.Fatal("expected error for package with no versions, got nil")
		}
		if !strings.Contains(err.Error(), "empty-pkg") {
			t.Errorf("error should mention package name 'empty-pkg', got: %s", err.Error())
		}
	})

	// --- No Tags: github repo with no tags → "latest" constraint works ---
	t.Run("NoTags", func(t *testing.T) {
		m := &mockSource{
			versions: map[string][]string{
				"github:test/no-tags": {},
			},
		}
		r := NewPubGrubResolver()
		r.RegisterHandler("github", m)

		result, err := r.Resolve(context.Background(), []PackageRequest{
			{Name: "no-tags", Type: "skill", Source: "github:test/no-tags", Constraint: "latest"},
		}, DefaultResolveOptions())

		if err != nil {
			t.Fatalf("latest constraint with no tags: unexpected error: %v", err)
		}
		if len(result.Packages) != 1 {
			t.Fatalf("expected 1 package, got %d", len(result.Packages))
		}
		pkg := result.Packages[0]
		if pkg.Resolved != "latest" {
			t.Errorf("Resolved = %q, want %q", pkg.Resolved, "latest")
		}
		if pkg.Name != "no-tags" {
			t.Errorf("Name = %q, want %q", pkg.Name, "no-tags")
		}
	})

	// --- Name Collision: same name different type → both resolve, collision warning ---
	t.Run("NameCollision", func(t *testing.T) {
		m := &mockSource{
			versions: map[string][]string{
				"github:test/run": {"1.0.0"},
			},
			pkgs: map[string][]byte{
				"github:test/run": makeAgentPkg("run", "1.0.0"),
			},
		}
		r := NewPubGrubResolver()
		r.RegisterHandler("github", m)

		result, err := r.Resolve(context.Background(), []PackageRequest{
			{Name: "run", Type: "skill", Source: "github:test/run", Constraint: "^1.0"},
			{Name: "run", Type: "agent", Source: "github:test/run", Constraint: "^1.0"},
		}, DefaultResolveOptions())

		if err != nil {
			t.Fatalf("name collision: unexpected error: %v", err)
		}

		// Both types should appear in the result (encoded as skill:run and agent:run)
		if len(result.Packages) != 2 {
			t.Fatalf("expected 2 packages, got %d", len(result.Packages))
		}

		foundSkill, foundAgent := false, false
		for _, pkg := range result.Packages {
			if pkg.Name != "run" {
				t.Errorf("unexpected package name: %q", pkg.Name)
				continue
			}
			switch pkg.Type {
			case "skill":
				foundSkill = true
			case "agent":
				foundAgent = true
			default:
				t.Errorf("unexpected package type: %q", pkg.Type)
			}
		}
		if !foundSkill {
			t.Error("expected skill type package in result")
		}
		if !foundAgent {
			t.Error("expected agent type package in result")
		}

		// Collision warning
		if len(result.Warnings) != 1 {
			t.Fatalf("expected 1 collision warning, got %d: %v", len(result.Warnings), result.Warnings)
		}
		if !strings.Contains(result.Warnings[0], "collision") {
			t.Errorf("warning should mention 'collision', got: %q", result.Warnings[0])
		}
		if !strings.Contains(result.Warnings[0], `"run"`) {
			t.Errorf("warning should mention 'run', got: %q", result.Warnings[0])
		}
	})
}

func TestPubGrubDiamondConflict(t *testing.T) {
	// Diamond dependency conflict (PubGrub algorithm):
	//   Root depends on A@^1.0 and C@^1.0
	//   A@1.0 depends on B@>=2.0
	//   C@1.0 depends on B@<2.0
	//   B has versions 1.0.0, 2.0.0, 3.0.0
	//   Expected: Resolve fails with error mentioning "A", "C", "B"
	//     (PubGrub's default conflict chain)

	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
			"github:test/B": {"3.0.0", "2.0.0", "1.0.0"},
			"github:test/C": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "1.0.0", "B@>=2.0"),
			"github:test/B": makeAgentPkg("B", "2.0.0"),
			"github:test/C": makeAgentPkg("C", "1.0.0", "B@<2.0"),
		},
	}

	r := NewPubGrubResolver()
	r.RegisterHandler("github", m)
	_, err := r.Resolve(context.Background(), []PackageRequest{
		req("A", "github:test/A", "^1.0"),
		req("C", "github:test/C", "^1.0"),
	}, DefaultResolveOptions())

	if err == nil {
		t.Fatal("expected error (diamond dependency conflict), got nil")
	}

	// PubGrub's default conflict chain should mention all three packages involved.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "A") {
		t.Error("error should mention 'A'")
	}
	if !strings.Contains(errMsg, "C") {
		t.Error("error should mention 'C'")
	}
	if !strings.Contains(errMsg, "B") {
		t.Error("error should mention 'B'")
	}
}
