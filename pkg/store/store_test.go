package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ----- Slugify -----

func TestSlugify(t *testing.T) {
	tests := []struct {
		raw, want string
	}{
		{"github:owner/repo", "github_owner_repo"},
		{"npm:@scope/package", "npm_at_scope_package"},
		{"npm:simple-pkg", "npm_simple-pkg"},
		{"git:https://example.com/repo.git", "git_https___example.com_repo.git"},
		{"github:owner/repo with spaces", "github_owner_repo_with_spaces"},
		{"simple", "simple"},
		{"has.dot.v1", "has.dot.v1"},
	}
	for _, tc := range tests {
		got := Slugify(tc.raw)
		if got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

// ----- Store -----

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	storeRoot := filepath.Join(root, "store")
	envsRoot := filepath.Join(root, "envs")

	s, err := NewStore(storeRoot)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	s.SetEnvBasePath(envsRoot)
	return s, root
}

func TestStore_PutGet(t *testing.T) {
	s, _ := newTestStore(t)

	data := []byte("hello store")
	digest, err := s.Put("skill", "github_owner_repo", "1.0.0", data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	expected := sha256Hash(data)
	if digest != expected {
		t.Errorf("digest = %q, want %q", digest, expected)
	}

	got, err := s.Get("skill", "github_owner_repo", "1.0.0")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("Get returned %q, want %q", string(got), string(data))
	}
}

func TestStore_PutGet_DifferentType(t *testing.T) {
	s, _ := newTestStore(t)

	data := []byte("skill-data")
	_, err := s.Put("skill", "pkg", "v1", data)
	if err != nil {
		t.Fatalf("Put skill: %v", err)
	}

	mcpData := []byte("mcp-data")
	_, err = s.Put("mcp", "pkg", "v1", mcpData)
	if err != nil {
		t.Fatalf("Put mcp: %v", err)
	}

	got, err := s.Get("skill", "pkg", "v1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "skill-data" {
		t.Errorf("expected skill-data, got %q", string(got))
	}

	got, err = s.Get("mcp", "pkg", "v1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "mcp-data" {
		t.Errorf("expected mcp-data, got %q", string(got))
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	s, _ := newTestStore(t)

	_, err := s.Get("skill", "nonexistent", "1.0.0")
	if err == nil {
		t.Fatal("expected error for missing package")
	}
}

func TestStore_Exists(t *testing.T) {
	s, _ := newTestStore(t)

	if s.Exists("skill", "gh_repo", "1.0.0") {
		t.Error("Exists returned true before Put")
	}

	_, err := s.Put("skill", "gh_repo", "1.0.0", []byte("data"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if !s.Exists("skill", "gh_repo", "1.0.0") {
		t.Error("Exists returned false after Put")
	}
	if s.Exists("skill", "other", "1.0.0") {
		t.Error("Exists returned true for wrong sourceSlug")
	}
	if s.Exists("mcp", "gh_repo", "1.0.0") {
		t.Error("Exists returned true for wrong type")
	}
}

func TestStore_LinkUnlink(t *testing.T) {
	s, root := newTestStore(t)

	_, err := s.Put("skill", "gh_repo", "1.0.0", []byte("pkg-data"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	envPath := filepath.Join(root, "envs", "myenv")
	os.MkdirAll(envPath, 0o755)

	// Link
	if err := s.Link(envPath, "skill", "my-skill", "gh_repo", "1.0.0"); err != nil {
		t.Fatalf("Link: %v", err)
	}

	linkPath := filepath.Join(envPath, "packages", "skill", "my-skill")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}

	expectedTarget := s.PackagePath("skill", "gh_repo", "1.0.0")
	if target != expectedTarget {
		t.Errorf("symlink target = %q, want %q", target, expectedTarget)
	}

	// Data accessible via symlink
	linkData, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatalf("read via symlink: %v", err)
	}
	if string(linkData) != "pkg-data" {
		t.Errorf("data via link = %q, want %q", string(linkData), "pkg-data")
	}

	// Unlink
	if err := s.Unlink(envPath, "skill", "my-skill"); err != nil {
		t.Fatalf("Unlink: %v", err)
	}

	if _, err := os.Stat(linkPath); !os.IsNotExist(err) {
		t.Error("link still exists after Unlink")
	}

	// Store entry still intact
	if !s.Exists("skill", "gh_repo", "1.0.0") {
		t.Error("store entry removed by Unlink")
	}

	// Get still works
	data, err := s.Get("skill", "gh_repo", "1.0.0")
	if err != nil {
		t.Fatalf("Get after Unlink: %v", err)
	}
	if string(data) != "pkg-data" {
		t.Errorf("store data corrupted after Unlink")
	}
}

func TestStore_Unlink_NotExist(t *testing.T) {
	s, root := newTestStore(t)
	envPath := filepath.Join(root, "envs", "myenv")
	os.MkdirAll(envPath, 0o755)

	// Unlink non-existent should not error
	if err := s.Unlink(envPath, "skill", "nonexistent"); err != nil {
		t.Errorf("Unlink non-existent returned error: %v", err)
	}
}

func TestStore_Link_Overwrite(t *testing.T) {
	s, root := newTestStore(t)

	_, err := s.Put("skill", "gh_repo", "1.0.0", []byte("v1"))
	if err != nil {
		t.Fatalf("Put v1: %v", err)
	}
	_, err = s.Put("skill", "gh_repo", "2.0.0", []byte("v2"))
	if err != nil {
		t.Fatalf("Put v2: %v", err)
	}

	envPath := filepath.Join(root, "envs", "myenv")
	os.MkdirAll(envPath, 0o755)

	// Link to v1
	if err := s.Link(envPath, "skill", "my-skill", "gh_repo", "1.0.0"); err != nil {
		t.Fatalf("Link v1: %v", err)
	}

	// Overwrite link to v2
	if err := s.Link(envPath, "skill", "my-skill", "gh_repo", "2.0.0"); err != nil {
		t.Fatalf("Link v2: %v", err)
	}

	linkPath := filepath.Join(envPath, "packages", "skill", "my-skill")
	data, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "v2" {
		t.Errorf("data = %q, want %q", string(data), "v2")
	}
}

func TestStore_GC(t *testing.T) {
	s, root := newTestStore(t)

	_, err := s.Put("skill", "gh_a", "1.0.0", []byte("a"))
	if err != nil {
		t.Fatalf("Put a: %v", err)
	}
	_, err = s.Put("skill", "gh_b", "1.0.0", []byte("b"))
	if err != nil {
		t.Fatalf("Put b: %v", err)
	}

	// Link only pkg-a
	envPath := filepath.Join(root, "envs", "myenv")
	os.MkdirAll(envPath, 0o755)
	if err := s.Link(envPath, "skill", "skill-a", "gh_a", "1.0.0"); err != nil {
		t.Fatalf("Link: %v", err)
	}

	removed, err := s.GC()
	if err != nil {
		t.Fatalf("GC: %v", err)
	}

	if len(removed) != 1 {
		t.Fatalf("expected 1 removed entry, got %d: %v", len(removed), removed)
	}

	// pkg-b should be removed, pkg-a should still exist
	if !strings.Contains(removed[0], "gh_b") {
		t.Errorf("expected gh_b to be removed, got %v", removed)
	}
	if !s.Exists("skill", "gh_a", "1.0.0") {
		t.Error("linked pkg-a was removed by GC")
	}
	if s.Exists("skill", "gh_b", "1.0.0") {
		t.Error("unlinked pkg-b still exists after GC")
	}
}

func TestStore_GC_NoSideEffects(t *testing.T) {
	s, root := newTestStore(t)

	_, err := s.Put("skill", "gh_a", "1.0.0", []byte("a"))
	if err != nil {
		t.Fatalf("Put a: %v", err)
	}
	_, err = s.Put("skill", "gh_b", "1.0.0", []byte("b"))
	if err != nil {
		t.Fatalf("Put b: %v", err)
	}

	// Link pkg-a in env1, pkg-b in env2
	env1 := filepath.Join(root, "envs", "env1")
	env2 := filepath.Join(root, "envs", "env2")
	os.MkdirAll(env1, 0o755)
	os.MkdirAll(env2, 0o755)

	if err := s.Link(env1, "skill", "a", "gh_a", "1.0.0"); err != nil {
		t.Fatalf("Link env1: %v", err)
	}
	if err := s.Link(env2, "skill", "b", "gh_b", "1.0.0"); err != nil {
		t.Fatalf("Link env2: %v", err)
	}

	removed, err := s.GC()
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removed, got %v", removed)
	}

	// Now unlink from env2
	if err := s.Unlink(env2, "skill", "b"); err != nil {
		t.Fatalf("Unlink env2: %v", err)
	}

	removed, err = s.GC()
	if err != nil {
		t.Fatalf("second GC: %v", err)
	}
	if len(removed) != 1 {
		t.Errorf("expected 1 removed, got %d: %v", len(removed), removed)
	}
	if !s.Exists("skill", "gh_a", "1.0.0") {
		t.Error("env1's pkg was removed by GC")
	}
	if s.Exists("skill", "gh_b", "1.0.0") {
		t.Error("env2's unlinked pkg still exists")
	}
}

func TestStore_GC_NoEnvs(t *testing.T) {
	s, _ := newTestStore(t)
	// Store has no env base — but SetEnvBasePath sets a non-existent dir.
	// This tests that GC handles missing envs dir gracefully.

	_, err := s.Put("skill", "gh_a", "1.0.0", []byte("a"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	removed, err := s.GC()
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(removed) != 1 {
		t.Errorf("expected 1 removed (no envs exist), got %d", len(removed))
	}
}

func TestStore_ReferencedBy(t *testing.T) {
	s, root := newTestStore(t)

	_, err := s.Put("skill", "gh_a", "1.0.0", []byte("a"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	env1 := filepath.Join(root, "envs", "env1")
	env2 := filepath.Join(root, "envs", "env2")
	os.MkdirAll(env1, 0o755)
	os.MkdirAll(env2, 0o755)

	if err := s.Link(env1, "skill", "a", "gh_a", "1.0.0"); err != nil {
		t.Fatalf("Link env1: %v", err)
	}
	if err := s.Link(env2, "skill", "a", "gh_a", "1.0.0"); err != nil {
		t.Fatalf("Link env2: %v", err)
	}

	refs, err := s.ReferencedBy("skill", "gh_a", "1.0.0")
	if err != nil {
		t.Fatalf("ReferencedBy: %v", err)
	}
	if len(refs) != 2 {
		t.Errorf("expected 2 refs, got %d: %v", len(refs), refs)
	}

	// ReferencedBy for non-existent package
	refs, err = s.ReferencedBy("skill", "nonexistent", "1.0.0")
	if err != nil {
		t.Fatalf("ReferencedBy nonexistent: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs for nonexistent, got %d", len(refs))
	}
}

func TestStore_Size(t *testing.T) {
	s, _ := newTestStore(t)

	initialSize, err := s.Size()
	if err != nil {
		t.Fatalf("initial Size: %v", err)
	}

	data := []byte("hello world")
	_, err = s.Put("skill", "gh_a", "1.0.0", data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	newSize, err := s.Size()
	if err != nil {
		t.Fatalf("Size after Put: %v", err)
	}
	if newSize <= initialSize {
		t.Errorf("size %d should be > initial %d", newSize, initialSize)
	}
}

func TestStore_PackagePath(t *testing.T) {
	s, _ := newTestStore(t)

	got := s.PackagePath("skill", "github_owner_repo", "1.0.0")
	if !strings.HasSuffix(got, filepath.Join("store", "skill", "github_owner_repo", "1.0.0")) {
		t.Errorf("PackagePath = %q, unexpected", got)
	}
}

func TestStore_CrossFS_CopyFallback(t *testing.T) {
	// Verify copyFile directly as a fallback for cross-FS scenarios.
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	src := filepath.Join(srcDir, "src.dat")
	dst := filepath.Join(dstDir, "dst.dat")

	data := []byte("cross-fs-fallback")
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	copied, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(copied) != string(data) {
		t.Errorf("copied data = %q, want %q", string(copied), string(data))
	}
}

func TestStore_CrossFS_SymlinkWorks(t *testing.T) {
	// On a single filesystem (typical test env), symlink should succeed.
	s, root := newTestStore(t)

	_, err := s.Put("skill", "gh_a", "1.0.0", []byte("data"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	envPath := filepath.Join(root, "envs", "myenv")
	os.MkdirAll(envPath, 0o755)

	if err := s.Link(envPath, "skill", "s", "gh_a", "1.0.0"); err != nil {
		t.Fatalf("Link on same filesystem: %v", err)
	}

	linkPath := filepath.Join(envPath, "packages", "skill", "s")
	fi, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file")
	}
}

func TestStore_NewStore_CreatesRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "new", "nested", "store")
	s, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	info, err := os.Stat(s.root)
	if err != nil {
		t.Fatalf("Stat root: %v", err)
	}
	if !info.IsDir() {
		t.Error("root is not a directory")
	}
}

func TestStore_Put_Overwrite(t *testing.T) {
	s, _ := newTestStore(t)

	d1, _ := s.Put("skill", "gh_a", "1.0.0", []byte("v1"))
	d2, _ := s.Put("skill", "gh_a", "1.0.0", []byte("v2"))

	if d1 == d2 {
		t.Error("digests should differ for different data")
	}

	data, _ := s.Get("skill", "gh_a", "1.0.0")
	if string(data) != "v2" {
		t.Errorf("expected v2 after overwrite, got %q", string(data))
	}
}

// ----- Cache -----

func TestCache_Download(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "downloaded-content")
	}))
	defer srv.Close()

	c, err := NewCache(t.TempDir())
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	path, err := c.Download(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cached file: %v", err)
	}
	if string(data) != "downloaded-content" {
		t.Errorf("cached data = %q, want %q", string(data), "downloaded-content")
	}
}

func TestCache_Download_Cached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, w2 *http.Request) {
		fmt.Fprint(w, "content-v1")
	}))
	defer srv.Close()

	c, err := NewCache(t.TempDir())
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	path1, err := c.Download(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("first Download: %v", err)
	}

	// Second download should return same file without hitting server.
	// We verify by rewriting the cached file and checking the returned path.
	if err := os.WriteFile(path1, []byte("tampered"), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	path2, err := c.Download(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("second Download: %v", err)
	}
	if path1 != path2 {
		t.Errorf("cached path differs: %q vs %q", path1, path2)
	}

	data, _ := os.ReadFile(path2)
	if string(data) != "tampered" {
		t.Error("expected tampered content (cache hit, not re-download)")
	}
}

func TestCache_Download_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c, err := NewCache(t.TempDir())
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	_, err = c.Download(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestCache_Download_InvalidURL(t *testing.T) {
	c, err := NewCache(t.TempDir())
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	_, err = c.Download(context.Background(), "http://\x01.invalid")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestReferencedBy_WalkError(t *testing.T) {
	s, root := newTestStore(t)

	_, err := s.Put("skill", "gh_a", "1.0.0", []byte("a"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	envPath := filepath.Join(root, "envs", "myenv")
	os.MkdirAll(envPath, 0o755)

	if err := s.Link(envPath, "skill", "my-skill", "gh_a", "1.0.0"); err != nil {
		t.Fatalf("Link: %v", err)
	}

	// Create an unreadable subdirectory to trigger WalkDir error during ReferencedBy.
	// Name it so it sorts after the valid symlink to ensure the ref is found first.
	badDir := filepath.Join(envPath, "packages", "skill", "z_unreadable")
	os.MkdirAll(badDir, 0o000)
	defer os.Chmod(badDir, 0o755) // restore so temp dir cleanup works

	// ReferencedBy should still find the valid symlink (visited before error)
	refs, err := s.ReferencedBy("skill", "gh_a", "1.0.0")
	if err != nil {
		t.Fatalf("ReferencedBy: %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("expected 1 ref (myenv), got %d: %v", len(refs), refs)
	}
	if len(refs) > 0 && refs[0] != "myenv" {
		t.Errorf("expected ref 'myenv', got %q", refs[0])
	}
}

// ----- Helpers -----

func sha256Hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
