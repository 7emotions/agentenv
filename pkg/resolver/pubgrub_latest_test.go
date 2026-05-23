package resolver

import (
	"context"
	"strings"
	"testing"

	"github.com/7emotions/agentenv/pkg/types"
)

type mockLatestHandler struct {
	versions  []string
	listErr   error
	fetchData []byte
	fetchSHA  string
	fetchErr  error
}

func (m *mockLatestHandler) ListVersions(src types.SourceURL) ([]string, error) {
	return m.versions, m.listErr
}

func (m *mockLatestHandler) Fetch(src types.SourceURL, version string) ([]byte, string, error) {
	return m.fetchData, m.fetchSHA, m.fetchErr
}

func TestPubGrubLatest(t *testing.T) {
	t.Run("latest constraint with no versions resolves to latest", func(t *testing.T) {
		handler := &mockLatestHandler{
			versions: []string{},
		}
		_ = handler

		resolver := NewPubGrubResolver()
		srcURL, err := types.ParseSourceURL("github:test/no-tags")
		if err != nil {
			t.Fatalf("ParseSourceURL() error: %v", err)
		}
		_ = srcURL

		result, err := resolver.Resolve(context.Background(), []PackageRequest{
			{
				Name:       "no-tags",
				Type:       "skill",
				Source:     "github:test/no-tags",
				Constraint: "latest",
			},
		}, DefaultResolveOptions())

		if err != nil {
			if strings.Contains(err.Error(), "not yet implemented") {
				t.Logf("Expected RED: Resolve() returned: %v", err)
				t.FailNow()
			}
			t.Fatalf("Resolve() unexpected error: %v", err)
		}

		if len(result.Packages) != 1 {
			t.Fatalf("expected 1 package, got %d", len(result.Packages))
		}
		if result.Packages[0].Resolved != "latest" {
			t.Errorf("resolved = %q, want %q", result.Packages[0].Resolved, "latest")
		}
	})

	t.Run("star constraint matches any version", func(t *testing.T) {
		handler := &mockLatestHandler{
			versions: []string{"1.0.0", "2.0.0"},
		}
		_ = handler

		resolver := NewPubGrubResolver()

		result, err := resolver.Resolve(context.Background(), []PackageRequest{
			{
				Name:       "any-pkg",
				Type:       "skill",
				Source:     "github:test/any-pkg",
				Constraint: "*",
			},
		}, DefaultResolveOptions())

		if err != nil {
			if strings.Contains(err.Error(), "not yet implemented") {
				t.Logf("Expected RED: Resolve() returned: %v", err)
				t.FailNow()
			}
			t.Fatalf("Resolve() unexpected error: %v", err)
		}

		if len(result.Packages) != 1 {
			t.Fatalf("expected 1 package, got %d", len(result.Packages))
		}
		if result.Packages[0].Resolved == "" {
			t.Error("resolved version should not be empty for '*' constraint")
		}
	})
}
