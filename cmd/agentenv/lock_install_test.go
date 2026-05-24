package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/7emotions/agentenv/pkg/envfile"
	"github.com/7emotions/agentenv/pkg/lockfile"
	"github.com/7emotions/agentenv/pkg/parser"
	"github.com/7emotions/agentenv/pkg/resolver"
	"github.com/7emotions/agentenv/pkg/source"
	"github.com/7emotions/agentenv/pkg/store"
	"github.com/7emotions/agentenv/pkg/types"
)

type testSource struct {
	versions map[string][]string
	pkgs     map[string][]byte
}

func (m *testSource) ListVersions(src types.SourceURL) ([]string, error) {
	v, ok := m.versions[src.String()]
	if !ok {
		return nil, fmt.Errorf("unknown package: %s", src.String())
	}
	return v, nil
}

func (m *testSource) Fetch(src types.SourceURL, version string) ([]byte, string, error) {
	data, ok := m.pkgs[src.String()]
	if !ok {
		return nil, "", fmt.Errorf("unknown package: %s", src.String())
	}
	h := sha256.Sum256(data)
	return data, fmt.Sprintf("%x", h), nil
}

func makePkgYAML(name, version string, deps ...string) []byte {
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
	// Wrap in tar.gz so the resolver's extractManifestFromTarGz can discover
	// transitive dependencies from agentpkg.yaml.
	return createTestTarGz(map[string]string{
		"agentpkg.yaml": sb.String(),
	})
}

func registerTestSources(t *testing.T, ts *testSource) {
	t.Helper()
	orig := make(map[string]source.SourceHandler)
	for k, v := range source.Registry {
		orig[k] = v
	}
	source.Registry["github"] = ts
	source.Registry["npm"] = ts
	t.Cleanup(func() {
		for k, v := range orig {
			source.Registry[k] = v
		}
	})
}

func TestLockGenerate(t *testing.T) {
	home := newTestHome(t)

	ts := &testSource{
		versions: map[string][]string{
			"github:test/code-review": {"1.0.0"},
			"github:test/security":    {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/code-review": makePkgYAML("code-review", "1.0.0"),
			"github:test/security":    makePkgYAML("security", "1.0.0"),
		},
	}
	registerTestSources(t, ts)

	yamlContent := `name: test-env
description: "Lock test"
skills:
  code-review:
    source: github:test/code-review
    version: "1.0.0"
  security:
    source: github:test/security
    version: "1.0.0"
`
	createTestEnv(t, home, "test-env", yamlContent, "")

	var stdoutBuf bytes.Buffer
	lockCmd.SetOut(&stdoutBuf)
	lockCmd.SetErr(&stdoutBuf)

	if err := lockCmd.RunE(lockCmd, []string{"test-env"}); err != nil {
		t.Fatalf("lock failed: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "test-env")
	lockPath := filepath.Join(envDir, "agent.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("agent.lock not created: %v", err)
	}

	lf, err := lockfile.Read(data)
	if err != nil {
		t.Fatalf("parse lockfile: %v", err)
	}
	if len(lf.Packages) != 2 {
		t.Errorf("expected 2 packages, got %d", len(lf.Packages))
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "agent.lock updated") {
		t.Errorf("output should mention agent.lock updated, got: %s", output)
	}
	if !strings.Contains(output, "2 packages") {
		t.Errorf("output should mention 2 packages, got: %s", output)
	}
}

func TestLockDeterminism(t *testing.T) {
	home := newTestHome(t)

	ts := &testSource{
		versions: map[string][]string{
			"github:test/det-pkg": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/det-pkg": makePkgYAML("det-pkg", "1.0.0"),
		},
	}
	registerTestSources(t, ts)

	yamlContent := `name: det-env
description: "Determinism test"
skills:
  det-pkg:
    source: github:test/det-pkg
    version: "1.0.0"
`

	lockCmd.SetOut(&bytes.Buffer{})
	lockCmd.SetErr(&bytes.Buffer{})

	createTestEnv(t, home, "det-env", yamlContent, "")
	if err := lockCmd.RunE(lockCmd, []string{"det-env"}); err != nil {
		t.Fatalf("first lock: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "det-env")
	lockPath := filepath.Join(envDir, "agent.lock")
	data1, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read first lock: %v", err)
	}
	h1 := fmt.Sprintf("%x", sha256.Sum256(data1))

	os.Remove(lockPath)
	if err := lockCmd.RunE(lockCmd, []string{"det-env"}); err != nil {
		t.Fatalf("second lock: %v", err)
	}

	data2, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read second lock: %v", err)
	}
	h2 := fmt.Sprintf("%x", sha256.Sum256(data2))

	if h1 != h2 {
		t.Errorf("SHA256 mismatch: %s != %s", h1, h2)
	}
}

func TestLockEmpty(t *testing.T) {
	home := newTestHome(t)
	createTestEnv(t, home, "empty-env", "name: empty-env\ndescription: no packages\n", "")

	var stdoutBuf bytes.Buffer
	lockCmd.SetOut(&stdoutBuf)
	lockCmd.SetErr(&stdoutBuf)

	if err := lockCmd.RunE(lockCmd, []string{"empty-env"}); err != nil {
		t.Fatalf("lock empty env: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "No packages defined") {
		t.Errorf("output should mention no packages, got: %s", output)
	}
}

func TestLockNotExist(t *testing.T) {
	home := newTestHome(t)
	os.MkdirAll(filepath.Join(home, ".agentenv"), 0o755)

	err := lockCmd.RunE(lockCmd, []string{"ghost-env"})
	if err == nil {
		t.Fatal("expected error for non-existent env")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestInstall(t *testing.T) {
	home := newTestHome(t)

	ts := &testSource{
		versions: map[string][]string{
			"github:test/code-review": {"1.0.0"},
			"github:test/security":    {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/code-review": makePkgYAML("code-review", "1.0.0"),
			"github:test/security":    makePkgYAML("security", "1.0.0"),
		},
	}
	registerTestSources(t, ts)

	yamlContent := `name: install-env
description: "Install test"
skills:
  code-review:
    source: github:test/code-review
    version: "1.0.0"
  security:
    source: github:test/security
    version: "1.0.0"
`
	createTestEnv(t, home, "install-env", yamlContent, "")

	lockCmd.SetOut(&bytes.Buffer{})
	lockCmd.SetErr(&bytes.Buffer{})
	if err := lockCmd.RunE(lockCmd, []string{"install-env"}); err != nil {
		t.Fatalf("lock: %v", err)
	}

	var stdoutBuf bytes.Buffer
	installCmd.SetOut(&stdoutBuf)
	installCmd.SetErr(&stdoutBuf)

	if err := installCmd.RunE(installCmd, []string{"install-env"}); err != nil {
		t.Fatalf("install: %v", err)
	}

	st, err := store.NewStore(filepath.Join(home, ".agentenv", "store"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	slug1 := store.Slugify("github:test/code-review")
	if !st.Exists("skill", slug1, "1.0.0") {
		t.Error("code-review package not in store")
	}

	slug2 := store.Slugify("github:test/security")
	if !st.Exists("skill", slug2, "1.0.0") {
		t.Error("security package not in store")
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "2 fetched") {
		t.Errorf("output should mention 2 fetched, got: %s", output)
	}
}

func TestInstallNoLock(t *testing.T) {
	home := newTestHome(t)

	ts := &testSource{
		versions: map[string][]string{
			"github:test/quick-pkg": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/quick-pkg": makePkgYAML("quick-pkg", "1.0.0"),
		},
	}
	registerTestSources(t, ts)

	yamlContent := `name: nolock-env
description: "No lock install"
skills:
  quick-pkg:
    source: github:test/quick-pkg
    version: "1.0.0"
`
	createTestEnv(t, home, "nolock-env", yamlContent, "")

	var stdoutBuf bytes.Buffer
	installCmd.SetOut(&stdoutBuf)
	installCmd.SetErr(&stdoutBuf)

	if err := installCmd.RunE(installCmd, []string{"nolock-env"}); err != nil {
		t.Fatalf("install without lock: %v", err)
	}

	st, err := store.NewStore(filepath.Join(home, ".agentenv", "store"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	slug := store.Slugify("github:test/quick-pkg")
	if !st.Exists("skill", slug, "1.0.0") {
		t.Error("package not in store after install without lock")
	}
}

func TestInstallCached(t *testing.T) {
	home := newTestHome(t)

	ts := &testSource{
		versions: map[string][]string{
			"github:test/cached-pkg": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/cached-pkg": makePkgYAML("cached-pkg", "1.0.0"),
		},
	}
	registerTestSources(t, ts)

	yamlContent := `name: cache-env
description: "Cache test"
skills:
  cached-pkg:
    source: github:test/cached-pkg
    version: "1.0.0"
`
	createTestEnv(t, home, "cache-env", yamlContent, "")

	lockCmd.SetOut(&bytes.Buffer{})
	lockCmd.SetErr(&bytes.Buffer{})
	if err := lockCmd.RunE(lockCmd, []string{"cache-env"}); err != nil {
		t.Fatalf("lock: %v", err)
	}

	installCmd.SetOut(&bytes.Buffer{})
	installCmd.SetErr(&bytes.Buffer{})

	if err := installCmd.RunE(installCmd, []string{"cache-env"}); err != nil {
		t.Fatalf("first install: %v", err)
	}

	var stdoutBuf bytes.Buffer
	installCmd.SetOut(&stdoutBuf)

	if err := installCmd.RunE(installCmd, []string{"cache-env"}); err != nil {
		t.Fatalf("second install: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "0 fetched") {
		t.Errorf("second install should fetch 0 (all cached), got: %s", output)
	}
	if !strings.Contains(output, "1 cached") {
		t.Errorf("second install should show 1 cached, got: %s", output)
	}
}

func TestLockWithTransitiveDeps(t *testing.T) {
	home := newTestHome(t)

	ts := &testSource{
		versions: map[string][]string{
			"github:test/main-pkg":  {"1.0.0"},
			"github:agentenv/dep-a": {"1.0.0"},
			"github:agentenv/dep-b": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/main-pkg":  makePkgYAML("main-pkg", "1.0.0", "dep-a@^1.0", "dep-b@^1.0"),
			"github:agentenv/dep-a": makePkgYAML("dep-a", "1.0.0"),
			"github:agentenv/dep-b": makePkgYAML("dep-b", "1.0.0"),
		},
	}
	registerTestSources(t, ts)

	yamlContent := `name: trans-env
description: "Transitive deps test"
skills:
  main-pkg:
    source: github:test/main-pkg
    version: "1.0.0"
`
	createTestEnv(t, home, "trans-env", yamlContent, "")

	lockCmd.SetOut(&bytes.Buffer{})
	lockCmd.SetErr(&bytes.Buffer{})

	if err := lockCmd.RunE(lockCmd, []string{"trans-env"}); err != nil {
		t.Fatalf("lock with transitive deps: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "trans-env")
	data, err := os.ReadFile(filepath.Join(envDir, "agent.lock"))
	if err != nil {
		t.Fatalf("read agent.lock: %v", err)
	}

	lf, err := lockfile.Read(data)
	if err != nil {
		t.Fatalf("parse lockfile: %v", err)
	}
	if len(lf.Packages) != 3 {
		t.Errorf("expected 3 packages (main + 2 deps), got %d", len(lf.Packages))
	}
}

func TestInstallWithActivate(t *testing.T) {
	home := newTestHome(t)
	setupTestAdapter(t, home)

	skillTarGz := createTestTarGz(map[string]string{
		"skill.md": "# Install Skill\n\nTest skill for install+activate.",
	})

	ts := &testSource{
		versions: map[string][]string{
			"github:test/install-skill": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/install-skill": makePkgYAML("install-skill", "1.0.0"),
		},
	}
	registerTestSources(t, ts)

	yamlContent := `name: iact-env
description: "Install+activate test"
skills:
  install-skill:
    source: github:test/install-skill
    version: "1.0.0"
`
	createTestEnv(t, home, "iact-env", yamlContent, "")

	lockCmd.SetOut(&bytes.Buffer{})
	lockCmd.SetErr(&bytes.Buffer{})
	if err := lockCmd.RunE(lockCmd, []string{"iact-env"}); err != nil {
		t.Fatalf("lock: %v", err)
	}

	populateStore(t, home, "skill", "github_test_install-skill", "1.0.0", skillTarGz)

	activeLock := filepath.Join(home, ".agentenv", "ACTIVE")
	os.WriteFile(activeLock, []byte("iact-env:claude-code"), 0o644)

	installCmd.SetOut(&bytes.Buffer{})
	installCmd.SetErr(&bytes.Buffer{})

	if err := installCmd.RunE(installCmd, []string{"iact-env"}); err != nil {
		t.Fatalf("install with active env: %v", err)
	}

	st, err := store.NewStore(filepath.Join(home, ".agentenv", "store"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	slug := store.Slugify("github:test/install-skill")
	if !st.Exists("skill", slug, "1.0.0") {
		t.Error("package not in store")
	}

	claudeDir := filepath.Join(home, ".claude")
	skillLink := filepath.Join(claudeDir, "skills", "install-skill")
	if _, err := os.Stat(skillLink); os.IsNotExist(err) {
		t.Error("skill not installed after activate")
	}
}

func TestLockJSONOutput(t *testing.T) {
	home := newTestHome(t)

	ts := &testSource{
		versions: map[string][]string{
			"github:test/json-pkg": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/json-pkg": makePkgYAML("json-pkg", "1.0.0"),
		},
	}
	registerTestSources(t, ts)

	yamlContent := `name: json-env
description: "JSON output test"
skills:
  json-pkg:
    source: github:test/json-pkg
    version: "1.0.0"
`
	createTestEnv(t, home, "json-env", yamlContent, "")

	var stdoutBuf bytes.Buffer
	lockCmd.SetOut(&stdoutBuf)
	lockCmd.SetErr(&stdoutBuf)

	jsonOutput = true
	defer func() { jsonOutput = false }()

	if err := lockCmd.RunE(lockCmd, []string{"json-env"}); err != nil {
		t.Fatalf("lock: %v", err)
	}

	var summary lockSummary
	if err := json.Unmarshal(stdoutBuf.Bytes(), &summary); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	if summary.Packages != 1 {
		t.Errorf("packages = %d, want 1", summary.Packages)
	}
	if summary.Conflicts != 0 {
		t.Errorf("conflicts = %d, want 0", summary.Conflicts)
	}
}

func TestLockWarnings(t *testing.T) {
	home := newTestHome(t)

	ts := &testSource{
		versions: map[string][]string{
			"github:test/collide": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/collide": makePkgYAML("collide", "1.0.0"),
		},
	}
	registerTestSources(t, ts)

	yamlContent := `name: warn-env
description: "Warning test"
skills:
  collide:
    source: github:test/collide
    version: "1.0.0"
tools:
  collide:
    source: github:test/collide
    version: "1.0.0"
`
	createTestEnv(t, home, "warn-env", yamlContent, "")

	var stdoutBuf bytes.Buffer
	lockCmd.SetOut(&stdoutBuf)
	lockCmd.SetErr(&stdoutBuf)

	if err := lockCmd.RunE(lockCmd, []string{"warn-env"}); err != nil {
		t.Fatalf("lock: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "Warnings") {
		t.Errorf("output should show Warnings section, got: %s", output)
	}
	if !strings.Contains(output, "collision") {
		t.Errorf("output should mention collision, got: %s", output)
	}
}

func TestFindEnvDir(t *testing.T) {
	home := newTestHome(t)

	root := filepath.Join(home, ".agentenv")
	name := "findme-env"
	createTestEnv(t, home, name, "name: findme-env\ndescription: test\n", "")

	t.Run("by name", func(t *testing.T) {
		got, err := findEnvDir(name)
		if err != nil {
			t.Fatalf("findEnvDir(%q): %v", name, err)
		}
		want := filepath.Join(root, "envs", name)
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("by active env", func(t *testing.T) {
		activeLock := filepath.Join(root, "ACTIVE")
		os.WriteFile(activeLock, []byte(name+":claude-code"), 0o644)
		defer os.Remove(activeLock)

		got, err := findEnvDir("")
		if err != nil {
			t.Fatalf("findEnvDir by active: %v", err)
		}
		want := filepath.Join(root, "envs", name)
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("not found", func(t *testing.T) {
		os.Remove(filepath.Join(root, "ACTIVE"))
		_, err := findEnvDir("ghost")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestLockfileRoundTrip(t *testing.T) {
	home := newTestHome(t)

	ts := &testSource{
		versions: map[string][]string{
			"github:test/rt-pkg": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/rt-pkg": makePkgYAML("rt-pkg", "1.0.0"),
		},
	}
	registerTestSources(t, ts)

	yamlContent := `name: rt-env
description: "Round trip"
skills:
  rt-pkg:
    source: github:test/rt-pkg
    version: "1.0.0"
`
	createTestEnv(t, home, "rt-env", yamlContent, "")

	lockCmd.SetOut(&bytes.Buffer{})
	lockCmd.SetErr(&bytes.Buffer{})
	if err := lockCmd.RunE(lockCmd, []string{"rt-env"}); err != nil {
		t.Fatalf("lock: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "rt-env")
	lockData, err := os.ReadFile(filepath.Join(envDir, "agent.lock"))
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}

	lf, err := parser.ParseLockfile(lockData)
	if err != nil {
		t.Fatalf("parse lock: %v", err)
	}

	if lf.Version != 3 {
		t.Errorf("version = %d, want 3", lf.Version)
	}
	if len(lf.Packages) != 1 {
		t.Errorf("packages = %d, want 1", len(lf.Packages))
	}
	if lf.Packages[0].Name != "rt-pkg" {
		t.Errorf("pkg name = %q, want rt-pkg", lf.Packages[0].Name)
	}
}

func TestLockStrictFlag(t *testing.T) {
	home := newTestHome(t)

	ts := &testSource{
		versions: map[string][]string{
			"github:test/strict-pkg": {"1.0.0"},
		},
		pkgs: map[string][]byte{
			"github:test/strict-pkg": makePkgYAML("strict-pkg", "1.0.0"),
		},
	}
	registerTestSources(t, ts)

	yamlContent := `name: strict-env
description: "Strict test"
skills:
  strict-pkg:
    source: github:test/strict-pkg
    version: "1.0.0"
`
	createTestEnv(t, home, "strict-env", yamlContent, "")

	lockCmd.SetOut(&bytes.Buffer{})
	lockCmd.SetErr(&bytes.Buffer{})

	lockStrict = true
	defer func() { lockStrict = false }()

	if err := lockCmd.RunE(lockCmd, []string{"strict-env"}); err != nil {
		t.Fatalf("lock --strict: %v", err)
	}
}

func TestBuildRequests(t *testing.T) {
	spec := createTestSpec(t)
	requests := buildRequests(spec)

	if len(requests) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(requests))
	}

	byName := make(map[string]resolver.PackageRequest)
	for _, r := range requests {
		byName[r.Name] = r
	}

	if r, ok := byName["test-skill"]; !ok || r.Type != "skill" {
		t.Error("test-skill missing or wrong type")
	}
	if r, ok := byName["test-mcp"]; !ok || r.Type != "mcp" {
		t.Error("test-mcp missing or wrong type")
	}
	if r, ok := byName["test-agent"]; !ok || r.Type != "agent" {
		t.Error("test-agent missing or wrong type")
	}
}

func TestDepSource(t *testing.T) {
	if s := depSource("my-pkg", "skill"); s != "github:agentenv/my-pkg" {
		t.Errorf("skill dep source = %q, want github:agentenv/my-pkg", s)
	}
	if s := depSource("my-mcp", "mcp"); s != "npm:@agentenv/my-mcp" {
		t.Errorf("mcp dep source = %q, want npm:@agentenv/my-mcp", s)
	}
	if s := depSource("my-agent", "agent"); s != "github:agentenv/my-agent" {
		t.Errorf("agent dep source = %q, want github:agentenv/my-agent", s)
	}
}

func TestInstallErrors(t *testing.T) {
	home := newTestHome(t)
	createTestEnv(t, home, "err-env", "name: err-env\ndescription: test\n", "")

	lockfileContent := `version: 1
generated: "2026-05-24T12:00:00Z"
packages:
  - name: bad-pkg
    type: skill
    source: not-a-valid-source
    version: "1.0.0"
    resolved: 1.0.0
    sha256: abc123
environment_snapshot:
  skills_count: 1
`
	envDir := filepath.Join(home, ".agentenv", "envs", "err-env")
	os.WriteFile(filepath.Join(envDir, "agent.lock"), []byte(lockfileContent), 0o644)

	var stdoutBuf bytes.Buffer
	installCmd.SetOut(&stdoutBuf)
	installCmd.SetErr(&stdoutBuf)

	if err := installCmd.RunE(installCmd, []string{"err-env"}); err != nil {
		t.Fatalf("install: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "1 errors") {
		t.Errorf("output should show 1 error, got: %s", output)
	}
	if !strings.Contains(output, "invalid source") {
		t.Errorf("output should mention invalid source, got: %s", output)
	}
}

func createTestSpec(t *testing.T) *envfile.EnvironmentSpec {
	t.Helper()
	return &envfile.EnvironmentSpec{
		Name: "test",
		Skills: map[string]envfile.PackageRef{
			"test-skill": {Source: "github:test/skill", Version: "1.0.0"},
		},
		MCPs: map[string]envfile.PackageRef{
			"test-mcp": {Source: "npm:@test/mcp", Version: "2.0.0"},
		},
		Agents: map[string]envfile.PackageRef{
			"test-agent": {Source: "github:test/agent", Version: "3.0.0"},
		},
	}
}
