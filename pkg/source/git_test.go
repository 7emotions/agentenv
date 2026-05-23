package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/7emotions/agentenv/pkg/types"
)

// setupGitRepo creates a temporary git repository with tags and returns the path.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	// Create a file and commit it.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "initial commit")

	// Create tags.
	runGit(t, dir, "tag", "v1.0.0")
	runGit(t, dir, "tag", "v0.9.0")
	runGit(t, dir, "tag", "v2.0.0")
	runGit(t, dir, "tag", "not-a-version")

	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestGitListVersions(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := setupGitRepo(t)

	gs := &GitSource{}
	src := types.SourceURL{Scheme: "git", URL: "file://" + repoDir}

	versions, err := gs.ListVersions(src)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}

	// Should find 3 semver tags: v2.0.0, v1.0.0, v0.9.0 (sorted descending).
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d: %v", len(versions), versions)
	}
	if versions[0] != "2.0.0" {
		t.Errorf("expected highest version 2.0.0, got %q", versions[0])
	}
	if versions[1] != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %q", versions[1])
	}
	if versions[2] != "0.9.0" {
		t.Errorf("expected version 0.9.0, got %q", versions[2])
	}
}

func TestGitFetch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := setupGitRepo(t)

	gs := &GitSource{}
	src := types.SourceURL{Scheme: "git", URL: "file://" + repoDir}

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

func TestGitFetchWithSubPath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := setupGitRepo(t)

	// Create a subdirectory with files.
	subDir := filepath.Join(repoDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "test.go"), []byte("package test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", "subdir/test.go")
	runGit(t, repoDir, "commit", "-m", "add subdir")
	runGit(t, repoDir, "tag", "v1.1.0")

	gs := &GitSource{}
	src := types.SourceURL{Scheme: "git", URL: "file://" + repoDir, SubPath: "subdir"}

	data, sha256, err := gs.Fetch(src, "1.1.0")
	if err != nil {
		t.Fatalf("Fetch with subpath: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data for subpath")
	}
	if sha256 == "" {
		t.Error("expected non-empty sha256")
	}
}

func TestGitFetchNonexistentTag(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := setupGitRepo(t)

	gs := &GitSource{}
	src := types.SourceURL{Scheme: "git", URL: "file://" + repoDir}

	_, _, err := gs.Fetch(src, "99.0.0")
	if err == nil {
		t.Error("expected error for nonexistent tag")
	}
}

func TestGitFetchInvalidURL(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	gs := &GitSource{}
	src := types.SourceURL{Scheme: "git", URL: "https://example.com/nonexistent.git"}

	_, _, err := gs.Fetch(src, "1.0.0")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestGitSemverRe(t *testing.T) {
	valid := []string{"v1.0.0", "0.9.0", "v2.0.0-alpha", "1.0.0-beta.1"}
	invalid := []string{"not-a-version", "v1.0", "latest", "1.0.0.0"}

	for _, v := range valid {
		if !gitSemverRe.MatchString(v) {
			t.Errorf("gitSemverRe should match %q", v)
		}
	}
	for _, v := range invalid {
		if gitSemverRe.MatchString(v) {
			t.Errorf("gitSemverRe should not match %q", v)
		}
	}
}

func TestGitListVersionsInvalidURL(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	gs := &GitSource{}
	src := types.SourceURL{Scheme: "git", URL: "https://example.com/nonexistent.git"}

	_, err := gs.ListVersions(src)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}
