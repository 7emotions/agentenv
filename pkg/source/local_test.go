package source

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentenv/agentenv/pkg/types"
)

func TestLocalListVersionsWithAgentPkg(t *testing.T) {
	dir := t.TempDir()

	agentPkg := []byte("name: test-skill\ntype: skill\nversion: \"1.2.3\"\nsource: local:./\n")
	if err := os.WriteFile(filepath.Join(dir, "agentpkg.yaml"), agentPkg, 0o644); err != nil {
		t.Fatal(err)
	}

	ls := &LocalSource{}
	src := types.SourceURL{Scheme: "local", Path: dir}

	versions, err := ls.ListVersions(src)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	if versions[0] != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %q", versions[0])
	}
}

func TestLocalListVersionsWithoutAgentPkg(t *testing.T) {
	dir := t.TempDir()

	ls := &LocalSource{}
	src := types.SourceURL{Scheme: "local", Path: dir}

	versions, err := ls.ListVersions(src)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	if versions[0] != "0.0.0-dev" {
		t.Errorf("expected default version 0.0.0-dev, got %q", versions[0])
	}
}

func TestLocalFetch(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "nested.txt"), []byte("nested content"), 0o644); err != nil {
		t.Fatal(err)
	}

	ls := &LocalSource{}
	src := types.SourceURL{Scheme: "local", Path: dir}

	data, sha256, err := ls.Fetch(src, "0.0.0-dev")
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

func TestLocalFetchEmptyPath(t *testing.T) {
	ls := &LocalSource{}
	src := types.SourceURL{Scheme: "local", Path: ""}

	_, _, err := ls.Fetch(src, "0.0.0-dev")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestLocalFetchNonexistentDir(t *testing.T) {
	ls := &LocalSource{}
	src := types.SourceURL{Scheme: "local", Path: "/nonexistent/path/12345"}

	_, _, err := ls.Fetch(src, "0.0.0-dev")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestLocalFetchNotADirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	ls := &LocalSource{}
	src := types.SourceURL{Scheme: "local", Path: filePath}

	_, _, err := ls.Fetch(src, "0.0.0-dev")
	if err == nil {
		t.Error("expected error for non-directory path")
	}
}

func TestLocalFetchEmptyDir(t *testing.T) {
	dir := t.TempDir()

	ls := &LocalSource{}
	src := types.SourceURL{Scheme: "local", Path: dir}

	_, _, err := ls.Fetch(src, "0.0.0-dev")
	if err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestLocalListVersionsEmptyPath(t *testing.T) {
	ls := &LocalSource{}
	src := types.SourceURL{Scheme: "local", Path: ""}

	_, err := ls.ListVersions(src)
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestLocalVersionFromQuotedYAML(t *testing.T) {
	dir := t.TempDir()

	agentPkg := []byte("version: '2.0.0'\n")
	if err := os.WriteFile(filepath.Join(dir, "agentpkg.yaml"), agentPkg, 0o644); err != nil {
		t.Fatal(err)
	}

	ls := &LocalSource{}
	src := types.SourceURL{Scheme: "local", Path: dir}

	versions, err := ls.ListVersions(src)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if versions[0] != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %q", versions[0])
	}
}

func TestLocalVersionFromDoubleQuotedYAML(t *testing.T) {
	dir := t.TempDir()

	agentPkg := []byte("version: \"3.0.0\"\n")
	if err := os.WriteFile(filepath.Join(dir, "agentpkg.yaml"), agentPkg, 0o644); err != nil {
		t.Fatal(err)
	}

	ls := &LocalSource{}
	src := types.SourceURL{Scheme: "local", Path: dir}

	versions, err := ls.ListVersions(src)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if versions[0] != "3.0.0" {
		t.Errorf("expected version 3.0.0, got %q", versions[0])
	}
}
