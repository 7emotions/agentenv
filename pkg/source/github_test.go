package source

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/7emotions/agentenv/pkg/types"
)

func TestGitHubListVersionsWithMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		tags := []githubTag{
			{Name: "v1.0.0"},
			{Name: "v0.9.0"},
			{Name: "v2.0.0"},
			{Name: "v1.0.1-beta.1"},
			{Name: "not-semver"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tags)
	}))
	defer server.Close()

	gs := &GitHubSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "github", Owner: "owner", Repo: "repo"}

	versions, err := gs.ListVersions(src)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}

	if len(versions) != 4 {
		t.Fatalf("expected 4 versions, got %d: %v", len(versions), versions)
	}
	if versions[0] != "2.0.0" {
		t.Errorf("expected highest 2.0.0, got %q", versions[0])
	}
}

func TestGitHubListVersionsRateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer server.Close()

	gs := &GitHubSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "github", Owner: "owner", Repo: "repo"}

	_, err := gs.ListVersions(src)
	if err == nil {
		t.Error("expected error for rate limit response")
	}
}

func TestGitHubListVersions404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer server.Close()

	gs := &GitHubSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "github", Owner: "nonexistent", Repo: "repo"}

	_, err := gs.ListVersions(src)
	if err == nil {
		t.Error("expected error for 404")
	}
}

func TestGitHubFetch(t *testing.T) {
	files := map[string][]byte{
		"testdata/hello.txt": []byte("hello"),
	}
	tarGzData := makeTarGzWithPrefix(t, "owner-repo-abc123", files)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/tarball/v1.0.0" {
			w.Header().Set("Content-Type", "application/x-gzip")
			w.Write(tarGzData)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	gs := &GitHubSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "github", Owner: "owner", Repo: "repo"}

	data, sha256, err := gs.Fetch(src, "1.0.0")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
	if sha256 == "" {
		t.Error("expected non-empty sha256")
	}
	if len(sha256) != 64 {
		t.Errorf("expected 64-char sha256, got %d chars", len(sha256))
	}
}

func TestGitHubFetch404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer server.Close()

	gs := &GitHubSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "github", Owner: "owner", Repo: "repo"}

	_, _, err := gs.Fetch(src, "99.0.0")
	if err == nil {
		t.Error("expected error for 404")
	}
}

func TestGitHubFetchRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer server.Close()

	gs := &GitHubSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "github", Owner: "owner", Repo: "repo"}

	_, _, err := gs.Fetch(src, "1.0.0")
	if err == nil {
		t.Error("expected error for rate limit")
	}
}

func TestExtractTarballSubPath(t *testing.T) {
	files := map[string][]byte{
		"testdata/hello.txt":     []byte("hello"),
		"testdata/sub/world.txt": []byte("world"),
		"other/file.txt":         []byte("other"),
	}
	tarGzData := makeTarGzWithPrefix(t, "owner-repo-abc123", files)

	t.Run("extract all", func(t *testing.T) {
		result, err := extractTarballSubPath(bytes.NewReader(tarGzData), "")
		if err != nil {
			t.Fatalf("extractTarballSubPath: %v", err)
		}
		if len(result) == 0 {
			t.Error("expected non-empty result")
		}
	})

	t.Run("extract subpath", func(t *testing.T) {
		result, err := extractTarballSubPath(bytes.NewReader(tarGzData), "testdata")
		if err != nil {
			t.Fatalf("extractTarballSubPath: %v", err)
		}
		if len(result) == 0 {
			t.Error("expected non-empty result")
		}
	})

	t.Run("extract missing subpath", func(t *testing.T) {
		_, err := extractTarballSubPath(bytes.NewReader(tarGzData), "nonexistent")
		if err == nil {
			t.Error("expected error for missing subpath")
		}
	})
}

func TestGitHubAuthHeader(t *testing.T) {
	oldToken := os.Getenv("GITHUB_TOKEN")
	defer os.Setenv("GITHUB_TOKEN", oldToken)

	os.Setenv("GITHUB_TOKEN", "test-token")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		tags := []githubTag{{Name: "v1.0.0"}}
		json.NewEncoder(w).Encode(tags)
	}))
	defer server.Close()

	gs := &GitHubSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "github", Owner: "owner", Repo: "repo"}

	versions, err := gs.ListVersions(src)
	if err != nil {
		t.Fatalf("expected success with auth, got: %v", err)
	}
	if len(versions) == 0 {
		t.Error("expected at least one version")
	}

	os.Unsetenv("GITHUB_TOKEN")
}

func TestGitHubUnauthenticated(t *testing.T) {
	oldToken := os.Getenv("GITHUB_TOKEN")
	os.Unsetenv("GITHUB_TOKEN")
	defer os.Setenv("GITHUB_TOKEN", oldToken)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("expected no auth header when GITHUB_TOKEN is unset")
		}
		tags := []githubTag{{Name: "v1.0.0"}}
		json.NewEncoder(w).Encode(tags)
	}))
	defer server.Close()

	gs := &GitHubSource{client: server.Client(), baseURL: server.URL}
	src := types.SourceURL{Scheme: "github", Owner: "owner", Repo: "repo"}

	_, err := gs.ListVersions(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "0.9.0", 1},
		{"0.9.0", "1.0.0", -1},
		{"1.0.0", "1.0.0", 0},
		{"2.0.0", "1.9.9", 1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0-beta", "1.0.0", -1},
		{"1.0.0", "1.0.0-alpha", 1},
		{"1.0.0-alpha", "1.0.0-beta", 0},
	}

	for _, tt := range tests {
		got := compareSemver(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSemverRe(t *testing.T) {
	valid := []string{"v1.0.0", "0.9.0", "v2.0.0-alpha", "1.0.0-beta.1", "v1.2.3+build"}
	invalid := []string{"not-semver", "v1.0", "latest", "1.0.0.0", "v"}

	for _, v := range valid {
		if !semverRe.MatchString(v) {
			t.Errorf("semverRe should match %q", v)
		}
	}
	for _, v := range invalid {
		if semverRe.MatchString(v) {
			t.Errorf("semverRe should not match %q", v)
		}
	}
}

func TestGitHubHTTPTimeout(t *testing.T) {
	gs := &GitHubSource{}
	client := gs.httpClient()
	if client.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", client.Timeout)
	}
}

func makeTarGzWithPrefix(t *testing.T, prefix string, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: prefix + "/" + name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("write tar: %v", err)
		}
	}

	tw.Close()
	gw.Close()
	return buf.Bytes()
}
