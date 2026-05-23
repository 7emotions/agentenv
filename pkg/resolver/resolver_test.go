package resolver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/agentenv/agentenv/pkg/types"
)

type mockSource struct {
	versions map[string][]string
	pkgs     map[string][]byte
	listErr  map[string]error
	fetchErr map[string]error
}

func (m *mockSource) ListVersions(src types.SourceURL) ([]string, error) {
	key := src.String()
	if err, ok := m.listErr[key]; ok {
		return nil, err
	}
	v, ok := m.versions[key]
	if !ok {
		return nil, fmt.Errorf("unknown package: %s", key)
	}
	return v, nil
}

func (m *mockSource) Fetch(src types.SourceURL, version string) ([]byte, string, error) {
	key := src.String()
	if err, ok := m.fetchErr[key]; ok {
		return nil, "", err
	}
	data, ok := m.pkgs[key]
	if !ok {
		return nil, "", fmt.Errorf("unknown package: %s", key)
	}
	h := sha256.Sum256(data)
	return data, fmt.Sprintf("%x", h), nil
}

func makeTarGzManifest(data []byte) []byte {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	hdr := &tar.Header{
		Name: "agentpkg.yaml",
		Size: int64(len(data)),
		Mode: 0644,
	}
	tarWriter.WriteHeader(hdr)
	tarWriter.Write(data)
	tarWriter.Close()
	gzWriter.Close()
	return buf.Bytes()
}

// makeAgentPkg creates a test agentpkg manifest wrapped in a tar.gz archive,
// as returned by real Fetch() implementations.
func makeAgentPkg(name, version string, deps ...string) []byte {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("name: %s\n", name))
	sb.WriteString(fmt.Sprintf("version: %q\n", version))
	sb.WriteString(fmt.Sprintf("source: github:test/%s\n", name))
	if len(deps) > 0 {
		sb.WriteString("dependencies:\n")
	}
	for _, d := range deps {
		parts := strings.SplitN(d, "@", 2)
		depName := parts[0]
		constraint := "*"
		if len(parts) > 1 {
			constraint = parts[1]
		}
		sb.WriteString(fmt.Sprintf("  - name: %s\n", depName))
		sb.WriteString("    type: skill\n")
		sb.WriteString(fmt.Sprintf("    constraint: %q\n", constraint))
	}
	return makeTarGzManifest([]byte(sb.String()))
}

func newTestResolver(m *mockSource) *TopologicalBacktrackResolver {
	r := NewResolver()
	r.RegisterHandler("github", m)
	return r
}

func req(name, source, constraint string) PackageRequest {
	return PackageRequest{
		Name:       name,
		Type:       "skill",
		Source:     source,
		Constraint: constraint,
	}
}

func TestSinglePackage(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"2.0.0", "1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "2.0.0"),
		},
	}

	r := newTestResolver(m)
	result, err := r.Resolve(context.Background(), []PackageRequest{req("A", "github:test/A", "^1.0.0")}, DefaultResolveOptions())
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	if len(result.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(result.Packages))
	}
	pkg := result.Packages[0]
	if pkg.Name != "A" {
		t.Errorf("name = %q, want %q", pkg.Name, "A")
	}
	if pkg.Resolved != "1.0.0" {
		t.Errorf("resolved = %q, want %q", pkg.Resolved, "1.0.0")
	}
	if pkg.Type != "skill" {
		t.Errorf("type = %q, want %q", pkg.Type, "skill")
	}
	if pkg.Version != "^1.0.0" {
		t.Errorf("version constraint = %q, want %q", pkg.Version, "^1.0.0")
	}
}

func TestTransitiveResolution(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
			"github:test/B": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "1.0.0", "B@^1.0.0"),
			"github:test/B": makeAgentPkg("B", "1.0.0"),
		},
	}

	r := newTestResolver(m)
	result, err := r.Resolve(context.Background(), []PackageRequest{req("A", "github:test/A", "^1.0.0")}, DefaultResolveOptions())
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	if len(result.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(result.Packages))
	}

	names := make(map[string]bool)
	for _, pkg := range result.Packages {
		names[pkg.Name] = true
	}
	if !names["A"] || !names["B"] {
		t.Errorf("expected packages A and B, got names: %v", names)
	}

	var pkgA ResolvedPackage
	for _, pkg := range result.Packages {
		if pkg.Name == "A" {
			pkgA = pkg
			break
		}
	}
	if len(pkgA.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency for A, got %d", len(pkgA.Dependencies))
	}
	if pkgA.Dependencies[0].Name != "B" {
		t.Errorf("dep name = %q, want %q", pkgA.Dependencies[0].Name, "B")
	}
	if pkgA.Dependencies[0].Resolved != "1.0.0" {
		t.Errorf("dep resolved = %q, want %q", pkgA.Dependencies[0].Resolved, "1.0.0")
	}
}

func TestCircularDependency(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
			"github:test/B": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "1.0.0", "B@^1.0.0"),
			"github:test/B": makeAgentPkg("B", "1.0.0", "A@^1.0.0"),
		},
	}

	r := newTestResolver(m)
	_, err := r.Resolve(context.Background(), []PackageRequest{req("A", "github:test/A", "^1.0.0")}, DefaultResolveOptions())
	if err == nil {
		t.Fatal("expected circular dependency error, got nil")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("error should mention 'circular', got: %v", err)
	}
	if !strings.Contains(err.Error(), "A → B → A") {
		t.Errorf("error should contain cycle path 'A → B → A', got: %v", err)
	}
}

func TestVersionConflict(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
			"github:test/B": {"1.0.0"},
			"github:test/C": {"3.0.0", "2.5.0", "2.0.0", "1.9.0", "1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "1.0.0", "C@>=2.0.0"),
			"github:test/B": makeAgentPkg("B", "1.0.0", "C@<2.0.0"),
			"github:test/C": makeAgentPkg("C", "2.5.0"),
		},
	}

	r := newTestResolver(m)
	_, err := r.Resolve(context.Background(), []PackageRequest{
		req("A", "github:test/A", "^1.0.0"),
		req("B", "github:test/B", "^1.0.0"),
	}, DefaultResolveOptions())
	if err == nil {
		t.Fatal("expected version conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "version conflict") {
		t.Errorf("error should mention 'version conflict', got: %v", err)
	}
}

func TestCompatibleConstraints(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
			"github:test/B": {"1.0.0"},
			"github:test/C": {"3.0.0", "2.5.0", "2.0.0", "1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "1.0.0", "C@>=1.0.0"),
			"github:test/B": makeAgentPkg("B", "1.0.0", "C@<3.0.0"),
			"github:test/C": makeAgentPkg("C", "2.5.0"),
		},
	}

	r := newTestResolver(m)
	result, err := r.Resolve(context.Background(), []PackageRequest{
		req("A", "github:test/A", "^1.0.0"),
		req("B", "github:test/B", "^1.0.0"),
	}, DefaultResolveOptions())
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	var pkgC ResolvedPackage
	for _, pkg := range result.Packages {
		if pkg.Name == "C" {
			pkgC = pkg
			break
		}
	}
	if pkgC.Resolved == "" {
		t.Fatal("C not found in resolved packages")
	}
	if pkgC.Resolved != "2.5.0" {
		t.Errorf("C resolved = %q, want %q", pkgC.Resolved, "2.5.0")
	}
}

func TestMaxDepth(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
			"github:test/B": {"1.0.0"},
			"github:test/C": {"1.0.0"},
			"github:test/D": {"1.0.0"},
			"github:test/E": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "1.0.0", "B@^1.0.0"),
			"github:test/B": makeAgentPkg("B", "1.0.0", "C@^1.0.0"),
			"github:test/C": makeAgentPkg("C", "1.0.0", "D@^1.0.0"),
			"github:test/D": makeAgentPkg("D", "1.0.0", "E@^1.0.0"),
			"github:test/E": makeAgentPkg("E", "1.0.0"),
		},
	}

	r := newTestResolver(m)
	_, err := r.Resolve(context.Background(), []PackageRequest{req("A", "github:test/A", "^1.0.0")}, DefaultResolveOptions())
	if err == nil {
		t.Fatal("expected max depth error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds max 3") {
		t.Errorf("error should mention 'exceeds max 3', got: %v", err)
	}
}

func TestMaxPackages(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{},
		pkgs:     map[string][]byte{},
	}

	for i := 0; i < 52; i++ {
		name := fmt.Sprintf("pkg%d", i)
		src := fmt.Sprintf("github:test/%s", name)
		m.versions[src] = []string{"1.0.0"}
		m.pkgs[src] = makeAgentPkg(name, "1.0.0")
	}

	requests := make([]PackageRequest, 52)
	for i := 0; i < 52; i++ {
		name := fmt.Sprintf("pkg%d", i)
		requests[i] = req(name, fmt.Sprintf("github:test/%s", name), "^1.0.0")
	}

	r := newTestResolver(m)
	_, err := r.Resolve(context.Background(), requests, DefaultResolveOptions())
	if err == nil {
		t.Fatal("expected max packages error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds max 50") {
		t.Errorf("error should mention 'exceeds max 50', got: %v", err)
	}
}

func TestToolCollision(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/run": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/run": makeAgentPkg("run", "1.0.0"),
		},
	}

	r := newTestResolver(m)
	result, err := r.Resolve(context.Background(), []PackageRequest{
		{Name: "run", Type: "skill", Source: "github:test/run", Constraint: "^1.0.0"},
		{Name: "run", Type: "tool", Source: "github:test/run", Constraint: "^1.0.0"},
	}, DefaultResolveOptions())
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
	}
	if !strings.Contains(result.Warnings[0], "collision") {
		t.Errorf("warning should mention 'collision', got: %q", result.Warnings[0])
	}
}

func TestDefaultConstraint(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"2.0.0", "1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "2.0.0"),
		},
	}

	r := newTestResolver(m)
	result, err := r.Resolve(context.Background(), []PackageRequest{req("A", "github:test/A", "")}, DefaultResolveOptions())
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	if result.Packages[0].Resolved != "2.0.0" {
		t.Errorf("resolved = %q, want %q (highest)", result.Packages[0].Resolved, "2.0.0")
	}
}

func TestDependencyChainWithSharedDep(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
			"github:test/B": {"1.0.0"},
			"github:test/C": {"1.0.0"},
			"github:test/D": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "1.0.0", "B@^1.0.0", "C@^1.0.0"),
			"github:test/B": makeAgentPkg("B", "1.0.0", "D@^1.0.0"),
			"github:test/C": makeAgentPkg("C", "1.0.0", "D@^1.0.0"),
			"github:test/D": makeAgentPkg("D", "1.0.0"),
		},
	}

	r := newTestResolver(m)
	result, err := r.Resolve(context.Background(), []PackageRequest{req("A", "github:test/A", "^1.0.0")}, DefaultResolveOptions())
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	if len(result.Packages) != 4 {
		t.Errorf("expected 4 packages, got %d", len(result.Packages))
	}

	dCount := 0
	for _, pkg := range result.Packages {
		if pkg.Name == "D" {
			dCount++
		}
	}
	if dCount != 1 {
		t.Errorf("expected D resolved once, got %d times", dCount)
	}
}

func TestUnknownSourceScheme(t *testing.T) {
	r := NewResolver()
	_, err := r.Resolve(context.Background(), []PackageRequest{
		req("A", "unknown:some/thing", "^1.0.0"),
	}, DefaultResolveOptions())
	if err == nil {
		t.Fatal("expected error for unknown scheme, got nil")
	}
	if !strings.Contains(err.Error(), "unknown source scheme") {
		t.Errorf("error should mention 'unknown source scheme', got: %v", err)
	}
}

func TestInvalidSourceURL(t *testing.T) {
	r := NewResolver()
	_, err := r.Resolve(context.Background(), []PackageRequest{
		{Name: "A", Type: "skill", Source: "not-a-valid-source", Constraint: "^1.0.0"},
	}, DefaultResolveOptions())
	if err == nil {
		t.Fatal("expected error for invalid source, got nil")
	}
}

func TestNoCompatibleVersion(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"0.9.0", "0.8.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "0.9.0"),
		},
	}

	r := newTestResolver(m)
	_, err := r.Resolve(context.Background(), []PackageRequest{
		req("A", "github:test/A", "^1.0.0"),
	}, DefaultResolveOptions())
	if err == nil {
		t.Fatal("expected 'no version satisfies' error, got nil")
	}
	if !strings.Contains(err.Error(), "no version satisfies") {
		t.Errorf("error should mention 'no version satisfies', got: %v", err)
	}
}

func TestListVersionsError(t *testing.T) {
	m := &mockSource{
		listErr: map[string]error{
			"github:test/A": fmt.Errorf("network error"),
		},
	}

	r := newTestResolver(m)
	_, err := r.Resolve(context.Background(), []PackageRequest{
		req("A", "github:test/A", "^1.0.0"),
	}, DefaultResolveOptions())
	if err == nil {
		t.Fatal("expected ListVersions error, got nil")
	}
	if !strings.Contains(err.Error(), "listing versions") {
		t.Errorf("error should mention 'listing versions', got: %v", err)
	}
}

func TestFetchError(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
		},
		fetchErr: map[string]error{
			"github:test/A": fmt.Errorf("download failed"),
		},
	}

	r := newTestResolver(m)
	_, err := r.Resolve(context.Background(), []PackageRequest{
		req("A", "github:test/A", "^1.0.0"),
	}, DefaultResolveOptions())
	if err == nil {
		t.Fatal("expected Fetch error, got nil")
	}
	if !strings.Contains(err.Error(), "fetching version") {
		t.Errorf("error should mention 'fetching version', got: %v", err)
	}
}

func TestParseError(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeTarGzManifest([]byte(":: invalid yaml ::")),
		},
	}

	r := newTestResolver(m)
	result, err := r.Resolve(context.Background(), []PackageRequest{
		req("A", "github:test/A", "^1.0.0"),
	}, DefaultResolveOptions())
	if err != nil {
		t.Fatalf("expected resolution to succeed with warning, got error: %v", err)
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "cannot parse") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about parse error, got warnings: %v", result.Warnings)
	}
}

func TestContextCancellation(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "1.0.0", "B@^1.0.0"),
			"github:test/B": makeAgentPkg("B", "1.0.0"),
		},
	}

	r := newTestResolver(m)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := r.Resolve(ctx, []PackageRequest{req("A", "github:test/A", "^1.0.0")}, DefaultResolveOptions())
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}

func TestResultDuration(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/A": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/A": makeAgentPkg("A", "1.0.0"),
		},
	}

	r := newTestResolver(m)
	result, err := r.Resolve(context.Background(), []PackageRequest{req("A", "github:test/A", "^1.0.0")}, DefaultResolveOptions())
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if result.Duration <= 0 {
		t.Errorf("expected non-zero duration, got %v", result.Duration)
	}
}

func TestSameNameDifferentTypeCollision(t *testing.T) {
	m := &mockSource{
		versions: map[string][]string{
			"github:test/run": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/run": makeAgentPkg("run", "1.0.0"),
		},
	}

	r := newTestResolver(m)
	result, err := r.Resolve(context.Background(), []PackageRequest{
		{Name: "run", Type: "skill", Source: "github:test/run", Constraint: "^1.0.0"},
		{Name: "run", Type: "tool", Source: "github:test/run", Constraint: "^1.0.0"},
	}, DefaultResolveOptions())
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 collision warning, got %d: %v", len(result.Warnings), result.Warnings)
	}
	if !strings.Contains(result.Warnings[0], `"run"`) {
		t.Errorf("warning should mention 'run', got: %q", result.Warnings[0])
	}
}
