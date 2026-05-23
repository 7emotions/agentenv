package resolver

import (
	"testing"

	"github.com/7emotions/agentenv/pkg/types"
	"github.com/contriboss/pubgrub-go"
)

// mockAdapterHandler implements source.SourceHandler for adapter tests.
type mockAdapterHandler struct {
	versions []string
	listErr  error
}

func (m *mockAdapterHandler) ListVersions(src types.SourceURL) ([]string, error) {
	return m.versions, m.listErr
}

func (m *mockAdapterHandler) Fetch(src types.SourceURL, version string) ([]byte, string, error) {
	return nil, "", nil
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

	if versions == nil {
		t.Fatal("GetVersions() returned nil, want non-nil empty slice")
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

	if deps == nil {
		t.Fatal("GetDependencies() returned nil, want non-nil empty slice")
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

// errFakeNetwork is a sentinel error for testing error propagation.
var errFakeNetwork = &mockError{"fake network error"}

type mockError struct{ msg string }

func (e *mockError) Error() string { return e.msg }
