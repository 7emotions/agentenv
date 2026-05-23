package resolver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"

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
