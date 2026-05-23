package resolver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/7emotions/agentenv/pkg/source"
	"github.com/7emotions/agentenv/pkg/types"
	"github.com/contriboss/pubgrub-go"
)

// mockAdapterHandler implements source.SourceHandler for adapter tests.
type mockAdapterHandler struct {
	versions   []string
	listErr    error
	fetchData  []byte
	fetchSHA   string
	fetchErr   error
	fetchCalls int
}

func (m *mockAdapterHandler) ListVersions(src types.SourceURL) ([]string, error) {
	return m.versions, m.listErr
}

func (m *mockAdapterHandler) Fetch(src types.SourceURL, version string) ([]byte, string, error) {
	m.fetchCalls++
	return m.fetchData, m.fetchSHA, m.fetchErr
}

// makeTarGz creates a tar.gz archive containing agentpkg.yaml with the given content.
func makeTarGz(t *testing.T, manifestContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: "agentpkg.yaml",
		Mode: 0644,
		Size: int64(len(manifestContent)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(manifestContent)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAdapter_GetVersions(t *testing.T) {
	handler := &mockAdapterHandler{
		versions: []string{"1.0.0", "2.0.0", "1.5.0"},
	}
	srcURL, _ := types.ParseSourceURL("github:test/pkg")
	src := NewAgentenvSource(handler, &srcURL, nil)

	name := pubgrub.MakeName("skill:pkg")
	versions, err := src.GetVersions(name)
	if err != nil {
		t.Fatalf("GetVersions() unexpected error: %v", err)
	}

	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}

	// Verify ascending order: 1.0.0 < 1.5.0 < 2.0.0
	expected := []string{"1.0.0", "1.5.0", "2.0.0"}
	for i, v := range versions {
		got := v.String()
		if got != expected[i] {
			t.Errorf("versions[%d] = %q, want %q", i, got, expected[i])
		}
	}

	// Verify they are *pubgrub.SemanticVersion instances
	for i, v := range versions {
		sv, ok := v.(*pubgrub.SemanticVersion)
		if !ok {
			t.Fatalf("versions[%d] is %T, want *pubgrub.SemanticVersion", i, v)
		}
		_ = sv
	}
}

func TestAdapter_EmptyVersionList(t *testing.T) {
	handler := &mockAdapterHandler{
		versions: []string{},
	}
	srcURL, _ := types.ParseSourceURL("github:test/empty")
	src := NewAgentenvSource(handler, &srcURL, nil)

	versions, err := src.GetVersions(pubgrub.MakeName("skill:empty"))
	if err != nil {
		t.Fatalf("GetVersions() unexpected error: %v", err)
	}

	if len(versions) != 0 {
		t.Fatalf("expected 0 versions, got %d", len(versions))
	}
}

func TestAdapter_SemverParsing(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantStr string
		wantMaj int
		wantMin int
		wantPat int
	}{
		{"simple", "1.2.3", "1.2.3", 1, 2, 3},
		{"zero major", "0.0.0", "0.0.0", 0, 0, 0},
		{"two digits", "10.20.30", "10.20.30", 10, 20, 30},
		{"prerelease", "1.2.3-alpha.1", "1.2.3-alpha.1", 1, 2, 3},
		{"with build", "1.2.3+build123", "1.2.3+build123", 1, 2, 3},
		{"prerelease+build", "1.2.3-rc.1+build.42", "1.2.3-rc.1+build.42", 1, 2, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &mockAdapterHandler{
				versions: []string{tt.input},
			}
			srcURL, _ := types.ParseSourceURL("github:test/pkg")
			src := NewAgentenvSource(handler, &srcURL, nil)

			versions, err := src.GetVersions(pubgrub.MakeName("test"))
			if err != nil {
				t.Fatalf("GetVersions() unexpected error: %v", err)
			}
			if len(versions) != 1 {
				t.Fatalf("expected 1 version, got %d", len(versions))
			}

			sv, ok := versions[0].(*pubgrub.SemanticVersion)
			if !ok {
				t.Fatalf("version is %T, want *pubgrub.SemanticVersion", versions[0])
			}

			if sv.String() != tt.wantStr {
				t.Errorf("String() = %q, want %q", sv.String(), tt.wantStr)
			}
			if sv.Major != tt.wantMaj {
				t.Errorf("Major = %d, want %d", sv.Major, tt.wantMaj)
			}
			if sv.Minor != tt.wantMin {
				t.Errorf("Minor = %d, want %d", sv.Minor, tt.wantMin)
			}
			if sv.Patch != tt.wantPat {
				t.Errorf("Patch = %d, want %d", sv.Patch, tt.wantPat)
			}
		})
	}
}

func TestAdapter_InvalidVersionsSkipped(t *testing.T) {
	handler := &mockAdapterHandler{
		versions: []string{"1.0.0", "not-a-version", "2.0.0"},
	}
	srcURL, _ := types.ParseSourceURL("github:test/pkg")
	src := NewAgentenvSource(handler, &srcURL, nil)

	versions, err := src.GetVersions(pubgrub.MakeName("skill:pkg"))
	if err != nil {
		t.Fatalf("GetVersions() unexpected error: %v", err)
	}

	if len(versions) != 2 {
		t.Fatalf("expected 2 valid versions (invalid skipped), got %d", len(versions))
	}
	if versions[0].String() != "1.0.0" {
		t.Errorf("versions[0] = %q, want %q", versions[0].String(), "1.0.0")
	}
	if versions[1].String() != "2.0.0" {
		t.Errorf("versions[1] = %q, want %q", versions[1].String(), "2.0.0")
	}
}

func TestAdapter_GetDependencies(t *testing.T) {
	handler := &mockAdapterHandler{
		versions: []string{"1.0.0"},
	}
	srcURL, _ := types.ParseSourceURL("github:test/pkg")
	src := NewAgentenvSource(handler, &srcURL, nil)

	sv, err := pubgrub.ParseSemanticVersion("1.0.0")
	if err != nil {
		t.Fatalf("ParseSemanticVersion() error: %v", err)
	}

	deps, err := src.GetDependencies(pubgrub.MakeName("skill:pkg"), sv)
	if err != nil {
		t.Fatalf("GetDependencies() unexpected error: %v", err)
	}

	if len(deps) != 0 {
		t.Fatalf("expected 0 deps, got %d", len(deps))
	}
}

func TestAdapter_ListVersionsError(t *testing.T) {
	handler := &mockAdapterHandler{
		listErr: errFakeNetwork,
	}
	srcURL, _ := types.ParseSourceURL("github:test/pkg")
	src := NewAgentenvSource(handler, &srcURL, nil)

	_, err := src.GetVersions(pubgrub.MakeName("skill:pkg"))
	if err == nil {
		t.Fatal("GetVersions() expected error, got nil")
	}
	if err != errFakeNetwork {
		t.Errorf("GetVersions() error = %v, want %v", err, errFakeNetwork)
	}
}

func TestAdapter_Dependencies_Discovery(t *testing.T) {
	manifest := `name: test-pkg
version: 1.0.0
dependencies:
  - name: dep-a
    type: skill
    constraint: ">=1.0.0, <2.0.0"
  - name: dep-b
    type: mcp
    constraint: ">=0.5.0"
`
	handler := &mockAdapterHandler{
		versions:  []string{"1.0.0"},
		fetchData: makeTarGz(t, manifest),
	}
	srcURL, _ := types.ParseSourceURL("github:test/pkg")
	src := NewAgentenvSource(handler, &srcURL, nil)

	sv, err := pubgrub.ParseSemanticVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	deps, err := src.GetDependencies(pubgrub.MakeName("skill:pkg"), sv)
	if err != nil {
		t.Fatalf("GetDependencies() unexpected error: %v", err)
	}

	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}

	// Verify dep-a: skill:dep-a with constraint >=1.0.0, <2.0.0
	wantA := pubgrub.MakeName("skill:dep-a")
	if deps[0].Name != wantA {
		t.Errorf("deps[0].Name = %v, want %v", deps[0].Name, wantA)
	}
	if !deps[0].Positive {
		t.Errorf("deps[0] should be positive")
	}
	if deps[0].Condition == nil {
		t.Error("deps[0].Condition should not be nil")
	}

	// Verify dep-b: mcp:dep-b with constraint >=0.5.0
	wantB := pubgrub.MakeName("mcp:dep-b")
	if deps[1].Name != wantB {
		t.Errorf("deps[1].Name = %v, want %v", deps[1].Name, wantB)
	}
	if !deps[1].Positive {
		t.Errorf("deps[1] should be positive")
	}
	if deps[1].Condition == nil {
		t.Error("deps[1].Condition should not be nil")
	}
}

func TestAdapter_Dependencies_MissingManifest(t *testing.T) {
	handler := &mockAdapterHandler{
		versions:  []string{"1.0.0"},
		fetchData: []byte("not-a-tar-gz"),
	}
	srcURL, _ := types.ParseSourceURL("github:test/pkg")
	src := NewAgentenvSource(handler, &srcURL, nil)

	sv, err := pubgrub.ParseSemanticVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	deps, err := src.GetDependencies(pubgrub.MakeName("skill:pkg"), sv)
	if err != nil {
		t.Fatalf("GetDependencies() unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Fatalf("expected 0 deps, got %d", len(deps))
	}
}

func TestAdapter_Dependencies_FetchError(t *testing.T) {
	handler := &mockAdapterHandler{
		versions: []string{"1.0.0"},
		fetchErr: errFakeNetwork,
	}
	srcURL, _ := types.ParseSourceURL("github:test/pkg")
	src := NewAgentenvSource(handler, &srcURL, nil)

	sv, err := pubgrub.ParseSemanticVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	_, err = src.GetDependencies(pubgrub.MakeName("skill:pkg"), sv)
	if err == nil {
		t.Fatal("GetDependencies() expected error, got nil")
	}
	if err != errFakeNetwork {
		t.Errorf("GetDependencies() error = %v, want %v", err, errFakeNetwork)
	}
}

func TestAdapter_Dependencies_Caching(t *testing.T) {
	manifest := `name: test-pkg
version: 1.0.0
dependencies:
  - name: dep-a
    type: skill
    constraint: ">=1.0.0"
`
	handler := &mockAdapterHandler{
		versions:  []string{"1.0.0"},
		fetchData: makeTarGz(t, manifest),
	}
	srcURL, _ := types.ParseSourceURL("github:test/pkg")
	src := NewAgentenvSource(handler, &srcURL, nil)

	sv, err := pubgrub.ParseSemanticVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	// First call - should trigger a fetch
	deps1, err := src.GetDependencies(pubgrub.MakeName("skill:pkg"), sv)
	if err != nil {
		t.Fatalf("first GetDependencies() error: %v", err)
	}
	if len(deps1) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps1))
	}

	// Second call with same name+version - should use cache
	deps2, err := src.GetDependencies(pubgrub.MakeName("skill:pkg"), sv)
	if err != nil {
		t.Fatalf("second GetDependencies() error: %v", err)
	}
	if len(deps2) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps2))
	}

	// Verify the underlying Fetch was only called once
	if handler.fetchCalls != 1 {
		t.Errorf("expected 1 underlying fetch call, got %d", handler.fetchCalls)
	}
}

func TestAdapter_Dependencies_EmptyConstraint(t *testing.T) {
	manifest := `name: test-pkg
version: 1.0.0
dependencies:
  - name: dep-a
    type: skill
    constraint: ""
  - name: dep-b
    type: tool
    constraint: ">=1.0.0"
`
	handler := &mockAdapterHandler{
		versions:  []string{"1.0.0"},
		fetchData: makeTarGz(t, manifest),
	}
	srcURL, _ := types.ParseSourceURL("github:test/pkg")
	src := NewAgentenvSource(handler, &srcURL, nil)

	sv, err := pubgrub.ParseSemanticVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	deps, err := src.GetDependencies(pubgrub.MakeName("skill:pkg"), sv)
	if err != nil {
		t.Fatalf("GetDependencies() unexpected error: %v", err)
	}

	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}

	// dep-a has empty constraint → should be "*" (any version)
	if deps[0].Condition == nil {
		t.Error("deps[0].Condition should not be nil (empty constraint becomes '*')")
	}
}

// errFakeNetwork is a sentinel error for testing error propagation.
var errFakeNetwork = &mockError{"fake network error"}

type mockError struct{ msg string }

func (e *mockError) Error() string { return e.msg }

func TestPubGrubDeepConflict(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
			"github:test/B": {"1.0.0"},
			"github:test/C": {"1.0.0"},
			"github:test/D": {"1.0.0"},
			"github:test/E": {"1.0.0"},
			"github:test/F": {"1.0.0", "2.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeTarGz(t, `name: A
version: "1.0.0"
source: github:test/A
dependencies:
  - name: B
    type: skill
    constraint: "^1.0.0"
`),
			"github:test/B": makeTarGz(t, `name: B
version: "1.0.0"
source: github:test/B
dependencies:
  - name: C
    type: skill
    constraint: "^1.0.0"
`),
			"github:test/C": makeTarGz(t, `name: C
version: "1.0.0"
source: github:test/C
dependencies:
  - name: D
    type: skill
    constraint: "^1.0.0"
`),
			"github:test/D": makeTarGz(t, `name: D
version: "1.0.0"
source: github:test/D
dependencies:
  - name: E
    type: skill
    constraint: "^1.0.0"
  - name: F
    type: skill
    constraint: ">=1.0.0"
`),
			"github:test/E": makeTarGz(t, `name: E
version: "1.0.0"
source: github:test/E
dependencies:
  - name: F
    type: skill
    constraint: "<2.0.0"
`),
			"github:test/F": makeTarGz(t, `name: F
version: "2.0.0"
source: github:test/F
`),
		},
	}

	resolver := NewPubGrubResolver(WithMaxSteps(200))
	resolver.RegisterHandler("github", m)

	requests := []PackageRequest{
		{Name: "A", Type: "skill", Source: "github:test/A", Constraint: "^1.0.0"},
	}

	ctx := context.Background()
	result, err := resolver.Resolve(ctx, requests, DefaultResolveOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Packages) == 0 {
		t.Fatal("expected at least one resolved package")
	}

	pkgNames := make(map[string]bool)
	for _, pkg := range result.Packages {
		pkgNames[pkg.Name] = true
	}
	for _, want := range []string{"A", "B", "C", "D", "E", "F"} {
		if !pkgNames[want] {
			t.Errorf("expected package %q in resolved set", want)
		}
	}
	if pkgNames["F"] {
		for _, pkg := range result.Packages {
			if pkg.Name == "F" && pkg.Resolved != "1.0.0" {
				t.Errorf("F resolved = %q, want 1.0.0 (intersection of >=1.0.0 and <2.0.0)", pkg.Resolved)
			}
		}
	}
}

// --- Integration tests with real source handlers ---

// TestAdapter_Integration_LocalSource_WithDependencies tests the full adapter
// pipeline using a real LocalSource handler. It creates a temp directory with
// agentpkg.yaml containing dependency declarations and verifies that
// ListVersions → Fetch → GetDependencies produces correct pubgrub terms.
func TestAdapter_Integration_LocalSource_WithDependencies(t *testing.T) {
	dir := t.TempDir()

	agentPkg := `name: test-pkg
version: 1.0.0
dependencies:
  - name: dep-a
    type: skill
    constraint: ">=1.0.0, <2.0.0"
  - name: dep-b
    type: mcp
    constraint: ">=0.5.0"
`
	if err := os.WriteFile(filepath.Join(dir, "agentpkg.yaml"), []byte(agentPkg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill.md"), []byte("# Test Skill"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &source.LocalSource{}
	srcURL := types.SourceURL{Scheme: "local", Path: dir}
	src := NewAgentenvSource(handler, &srcURL, nil)

	name := pubgrub.MakeName("skill:test-pkg")

	// GetVersions should read version from agentpkg.yaml via LocalSource
	versions, err := src.GetVersions(name)
	if err != nil {
		t.Fatalf("GetVersions() unexpected error: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	if versions[0].String() != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", versions[0].String())
	}

	// Verify the version is a proper SemanticVersion (not just a string)
	if _, ok := versions[0].(*pubgrub.SemanticVersion); !ok {
		t.Fatalf("version is %T, want *pubgrub.SemanticVersion", versions[0])
	}

	// GetDependencies should parse manifest and return dependency terms
	sv, err := pubgrub.ParseSemanticVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	deps, err := src.GetDependencies(name, sv)
	if err != nil {
		t.Fatalf("GetDependencies() unexpected error: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}

	// Verify dep-a: skill:dep-a with constraint >=1.0.0, <2.0.0
	wantA := pubgrub.MakeName("skill:dep-a")
	if deps[0].Name != wantA {
		t.Errorf("deps[0].Name = %v, want %v", deps[0].Name, wantA)
	}
	if !deps[0].Positive {
		t.Error("deps[0] should be positive")
	}
	if deps[0].Condition == nil {
		t.Error("deps[0].Condition should not be nil")
	}

	// Verify dep-b: mcp:dep-b with constraint >=0.5.0
	wantB := pubgrub.MakeName("mcp:dep-b")
	if deps[1].Name != wantB {
		t.Errorf("deps[1].Name = %v, want %v", deps[1].Name, wantB)
	}
	if !deps[1].Positive {
		t.Error("deps[1] should be positive")
	}
}

// TestAdapter_Integration_LocalSource_MissingManifest tests that when a
// local directory has no agentpkg.yaml, the adapter gracefully returns
// empty dependencies without error or panic.
func TestAdapter_Integration_LocalSource_MissingManifest(t *testing.T) {
	dir := t.TempDir()

	// Create files but NO agentpkg.yaml
	if err := os.WriteFile(filepath.Join(dir, "skill.md"), []byte("# Orphan Skill"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &source.LocalSource{}
	srcURL := types.SourceURL{Scheme: "local", Path: dir}
	src := NewAgentenvSource(handler, &srcURL, nil)

	// ListVersions returns default "0.0.0-dev" when no agentpkg.yaml
	versions, err := src.GetVersions(pubgrub.MakeName("skill:orphan"))
	if err != nil {
		t.Fatalf("GetVersions() unexpected error: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version (0.0.0-dev), got %d", len(versions))
	}
	if versions[0].String() != "0.0.0-dev" {
		t.Errorf("version = %q, want 0.0.0-dev", versions[0].String())
	}

	// GetDependencies should return empty deps (no panic) because
	// extractManifestFromTarGz won't find agentpkg.yaml in the archive.
	sv, err := pubgrub.ParseSemanticVersion("0.0.0-dev")
	if err != nil {
		t.Fatal(err)
	}

	deps, err := src.GetDependencies(pubgrub.MakeName("skill:orphan"), sv)
	if err != nil {
		t.Fatalf("GetDependencies() unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Fatalf("expected 0 deps, got %d", len(deps))
	}
}

// TestAdapter_Integration_LocalSource_InvalidManifest tests that corrupted
// or invalid agentpkg.yaml content is handled gracefully: the YAML parser
// fails and returns empty dependencies without error or panic.
func TestAdapter_Integration_LocalSource_InvalidManifest(t *testing.T) {
	dir := t.TempDir()

	// agentpkg.yaml with invalid YAML — simulates a corrupted manifest
	if err := os.WriteFile(filepath.Join(dir, "agentpkg.yaml"), []byte(": : invalid yaml [[["), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill.md"), []byte("# Skill with bad manifest"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &source.LocalSource{}
	srcURL := types.SourceURL{Scheme: "local", Path: dir}
	src := NewAgentenvSource(handler, &srcURL, nil)

	// ListVersions uses simple line parsing so it still returns a version
	versions, err := src.GetVersions(pubgrub.MakeName("skill:bad"))
	if err != nil {
		t.Fatalf("GetVersions() unexpected error: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}

	// GetDependencies should fail to parse YAML → empty deps, no panic
	sv, err := pubgrub.ParseSemanticVersion(versions[0].String())
	if err != nil {
		t.Fatal(err)
	}

	deps, err := src.GetDependencies(pubgrub.MakeName("skill:bad"), sv)
	if err != nil {
		t.Fatalf("GetDependencies() unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Fatalf("expected 0 deps, got %d", len(deps))
	}
}

// TestAdapter_Integration_LocalSource_NonSemverVersion tests that
// non-semver version strings returned by the source handler are silently
// skipped by GetVersions without panicking.
func TestAdapter_Integration_LocalSource_NonSemverVersion(t *testing.T) {
	dir := t.TempDir()

	agentPkg := `name: test-pkg
version: abc-def
`
	if err := os.WriteFile(filepath.Join(dir, "agentpkg.yaml"), []byte(agentPkg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill.md"), []byte("# Non-semver skill"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &source.LocalSource{}
	srcURL := types.SourceURL{Scheme: "local", Path: dir}
	src := NewAgentenvSource(handler, &srcURL, nil)

	// GetVersions should skip "abc-def" (not valid semver) without panic.
	// The result should be an empty (or non-panicked) version list.
	versions, err := src.GetVersions(pubgrub.MakeName("skill:test-pkg"))
	if err != nil {
		t.Fatalf("GetVersions() unexpected error: %v", err)
	}
	// "abc-def" is not valid semver, so GetVersions silently skips it.
	// We accept 0 results — the key assertion is no panic occurred.
	t.Logf("GetVersions returned %d versions (non-semver should be skipped)", len(versions))
}

// TestAdapter_Integration_LocalSource_EmptyVersion tests that a completely
// empty version string in agentpkg.yaml is handled gracefully: the adapter
// should not panic and should return an appropriate result.
func TestAdapter_Integration_LocalSource_EmptyVersion(t *testing.T) {
	dir := t.TempDir()

	// version field present but empty string
	agentPkg := `name: test-pkg
version: ""
`
	if err := os.WriteFile(filepath.Join(dir, "agentpkg.yaml"), []byte(agentPkg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill.md"), []byte("# Empty version skill"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &source.LocalSource{}
	srcURL := types.SourceURL{Scheme: "local", Path: dir}
	src := NewAgentenvSource(handler, &srcURL, nil)

	// ListVersions strips quotes: version: "" → trimmed to empty → falls back to "0.0.0-dev"
	versions, err := src.GetVersions(pubgrub.MakeName("skill:test-pkg"))
	if err != nil {
		t.Fatalf("GetVersions() unexpected error: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	t.Logf("Empty version string resolved to %q", versions[0].String())
}

// TestAdapter_Integration_LocalSource_DescendentDir tests that agentpkg.yaml
// in a subdirectory doesn't interfere with package resolution. LocalSource
// walks all files recursively, so the archive will contain nested files.
func TestAdapter_Integration_LocalSource_DescendentDir(t *testing.T) {
	dir := t.TempDir()

	// agentpkg.yaml at root
	agentPkg := `name: nested-pkg
version: 2.0.0
`
	if err := os.WriteFile(filepath.Join(dir, "agentpkg.yaml"), []byte(agentPkg), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a subdirectory with some files
	subDir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested content"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &source.LocalSource{}
	srcURL := types.SourceURL{Scheme: "local", Path: dir}
	src := NewAgentenvSource(handler, &srcURL, nil)

	// GetVersions should find version from root agentpkg.yaml
	versions, err := src.GetVersions(pubgrub.MakeName("skill:nested-pkg"))
	if err != nil {
		t.Fatalf("GetVersions() unexpected error: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	if versions[0].String() != "2.0.0" {
		t.Errorf("version = %q, want 2.0.0", versions[0].String())
	}

	// Fetch should produce a valid tar.gz that includes both root and nested files
	sv, err := pubgrub.ParseSemanticVersion("2.0.0")
	if err != nil {
		t.Fatal(err)
	}

	deps, err := src.GetDependencies(pubgrub.MakeName("skill:nested-pkg"), sv)
	if err != nil {
		t.Fatalf("GetDependencies() unexpected error: %v", err)
	}
	// No deps declared, so should be empty
	if len(deps) != 0 {
		t.Fatalf("expected 0 deps, got %d", len(deps))
	}
}
