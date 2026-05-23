package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/7emotions/agentenv/pkg/envfile"
)

func writeActiveLock(t *testing.T, home, envName string) {
	t.Helper()
	lockPath := filepath.Join(home, ".agentenv", "ACTIVE")
	if err := os.WriteFile(lockPath, []byte(envName+":claude-code"), 0o644); err != nil {
		t.Fatalf("write ACTIVE lock: %v", err)
	}
}

func TestAdd_Basic(t *testing.T) {
	home := newTestHome(t)
	createTestEnv(t, home, "test-env", "name: test-env\ndescription: test env\n", "")
	writeActiveLock(t, home, "test-env")

	addSource = "github:owner/repo"
	addVersion = ""
	addDev = false
	addDryRun = false

	var stdoutBuf bytes.Buffer
	addCmd.SetOut(&stdoutBuf)
	addCmd.SetErr(&bytes.Buffer{})

	err := addCmd.RunE(addCmd, []string{"skill", "my-skill"})
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "Added") {
		t.Errorf("expected 'Added' in output, got: %s", output)
	}
	if !strings.Contains(output, "agentenv lock") {
		t.Errorf("expected lock hint in output, got: %s", output)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "test-env")
	yamlPath := filepath.Join(envDir, "agent.yaml")
	spec, err := envfile.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("reading updated agent.yaml: %v", err)
	}
	if spec.Skills == nil {
		t.Fatal("expected skills section")
	}
	pkg, ok := spec.Skills["my-skill"]
	if !ok {
		t.Fatal("expected my-skill in skills")
	}
	if pkg.Source != "github:owner/repo" {
		t.Errorf("source = %q, want github:owner/repo", pkg.Source)
	}
}

func TestAdd_WithVersion(t *testing.T) {
	home := newTestHome(t)
	createTestEnv(t, home, "test-env", "name: test-env\ndescription: test\n", "")
	writeActiveLock(t, home, "test-env")

	addSource = "npm:@scope/pkg"
	addVersion = "^1.0.0"
	addDev = false
	addDryRun = false

	var stdoutBuf bytes.Buffer
	addCmd.SetOut(&stdoutBuf)
	addCmd.SetErr(&bytes.Buffer{})

	err := addCmd.RunE(addCmd, []string{"mcp", "my-mcp"})
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "test-env")
	spec, err := envfile.ReadFile(filepath.Join(envDir, "agent.yaml"))
	if err != nil {
		t.Fatalf("reading agent.yaml: %v", err)
	}
	pkg, ok := spec.MCPs["my-mcp"]
	if !ok {
		t.Fatal("expected my-mcp in mcps")
	}
	if pkg.Source != "npm:@scope/pkg" {
		t.Errorf("source = %q", pkg.Source)
	}
	if pkg.Version != "^1.0.0" {
		t.Errorf("version = %q, want ^1.0.0", pkg.Version)
	}
}

func TestAdd_DevFlag(t *testing.T) {
	home := newTestHome(t)
	createTestEnv(t, home, "test-env", "name: test-env\ndescription: test\n", "")
	writeActiveLock(t, home, "test-env")

	addSource = "github:owner/repo"
	addVersion = ""
	addDev = true
	addDryRun = false

	var stdoutBuf bytes.Buffer
	addCmd.SetOut(&stdoutBuf)
	addCmd.SetErr(&bytes.Buffer{})

	err := addCmd.RunE(addCmd, []string{"skill", "dev-skill"})
	if err != nil {
		t.Fatalf("add --dev failed: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "test-env")
	spec, err := envfile.ReadFile(filepath.Join(envDir, "agent.yaml"))
	if err != nil {
		t.Fatalf("reading agent.yaml: %v", err)
	}
	pkg, ok := spec.Skills["dev-skill"]
	if !ok {
		t.Fatal("expected dev-skill in skills")
	}
	if pkg.Config == nil {
		t.Fatal("expected config for dev package")
	}
	devVal, ok := pkg.Config["dev"]
	if !ok || devVal != true {
		t.Errorf("expected dev=true in config, got %v", pkg.Config)
	}
}

func TestAdd_DryRun(t *testing.T) {
	home := newTestHome(t)
	createTestEnv(t, home, "test-env", "name: test-env\ndescription: test\n", "")
	writeActiveLock(t, home, "test-env")

	addSource = "github:owner/repo"
	addVersion = ""
	addDev = false
	addDryRun = true

	var stdoutBuf bytes.Buffer
	addCmd.SetOut(&stdoutBuf)
	addCmd.SetErr(&bytes.Buffer{})

	err := addCmd.RunE(addCmd, []string{"skill", "dry-skill"})
	if err != nil {
		t.Fatalf("add --dry-run failed: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "Would add") {
		t.Errorf("expected dry-run message, got: %s", output)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "test-env")
	spec, err := envfile.ReadFile(filepath.Join(envDir, "agent.yaml"))
	if err != nil {
		t.Fatalf("reading agent.yaml: %v", err)
	}
	if spec.Skills != nil {
		if _, ok := spec.Skills["dry-skill"]; ok {
			t.Error("dry-skill should not be added during dry-run")
		}
	}
}

func TestAdd_InvalidType(t *testing.T) {
	home := newTestHome(t)
	os.MkdirAll(filepath.Join(home, ".agentenv"), 0o755)

	addSource = "github:owner/repo"
	addDryRun = false

	err := addCmd.RunE(addCmd, []string{"invalid", "pkg"})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "Invalid type") {
		t.Errorf("expected 'Invalid type' error, got: %v", err)
	}
}

func TestAdd_InvalidName(t *testing.T) {
	home := newTestHome(t)
	createTestEnv(t, home, "test-env", "name: test-env\ndescription: test\n", "")
	writeActiveLock(t, home, "test-env")

	addSource = "github:owner/repo"
	addDryRun = false

	err := addCmd.RunE(addCmd, []string{"skill", "UpperCase"})
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
	if !strings.Contains(err.Error(), "Invalid package name") {
		t.Errorf("expected 'Invalid package name' error, got: %v", err)
	}
}

func TestAdd_MissingSource(t *testing.T) {
	home := newTestHome(t)
	createTestEnv(t, home, "test-env", "name: test-env\ndescription: test\n", "")
	writeActiveLock(t, home, "test-env")

	addSource = ""
	addDryRun = false

	err := addCmd.RunE(addCmd, []string{"skill", "my-skill"})
	if err == nil {
		t.Fatal("expected error for missing --source")
	}
	if !strings.Contains(err.Error(), "source is required") {
		t.Errorf("expected 'source is required' error, got: %v", err)
	}
}

func TestAdd_Duplicate(t *testing.T) {
	home := newTestHome(t)
	createTestEnv(t, home, "test-env",
		"name: test-env\ndescription: test\nskills:\n  existing:\n    source: github:a/b\n", "")
	writeActiveLock(t, home, "test-env")

	addSource = "github:other/repo"
	addDryRun = false

	err := addCmd.RunE(addCmd, []string{"skill", "existing"})
	if err == nil {
		t.Fatal("expected error for duplicate package")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestAdd_AllTypes(t *testing.T) {
	home := newTestHome(t)
	createTestEnv(t, home, "test-env", "name: test-env\ndescription: test\n", "")
	writeActiveLock(t, home, "test-env")

	type testCase struct {
		pkgType string
		name    string
		source  string
	}

	tests := []struct {
		pkgType string
		name    string
		source  string
	}{
		{"skill", "my-skill", "github:a/skill"},
		{"mcp", "my-mcp", "npm:@scope/mcp"},
		{"agent", "my-agent", "github:a/agent"},
		{"tool", "my-tool", "local:./tool"},
		{"hook", "my-hook", "github:a/hook"},
		{"prompt", "my-prompt", "github:a/prompt"},
	}

	for _, tc := range tests {
		t.Run(tc.pkgType, func(t *testing.T) {
			addSource = tc.source
			addVersion = ""
			addDev = false
			addDryRun = false

			err := addCmd.RunE(addCmd, []string{tc.pkgType, tc.name})
			if err != nil {
				t.Fatalf("add %s failed: %v", tc.pkgType, err)
			}

			envDir := filepath.Join(home, ".agentenv", "envs", "test-env")
			spec, err := envfile.ReadFile(filepath.Join(envDir, "agent.yaml"))
			if err != nil {
				t.Fatalf("reading agent.yaml: %v", err)
			}

			section := typeToSection[tc.pkgType]
			m := getSectionMap(spec, section)
			if m == nil {
				t.Fatalf("section %q is nil", section)
			}
			pkg, ok := m[tc.name]
			if !ok {
				t.Errorf("package %q not found in %s", tc.name, section)
			} else if pkg.Source != tc.source && !strings.HasPrefix(tc.source, "local:") {
				t.Errorf("source = %q, want %q", pkg.Source, tc.source)
			}
		})
	}
}

func TestRemove_Basic(t *testing.T) {
	home := newTestHome(t)
	createTestEnv(t, home, "test-env",
		"name: test-env\ndescription: test\nskills:\n  my-skill:\n    source: github:a/b\n", "")
	writeActiveLock(t, home, "test-env")

	var stdoutBuf bytes.Buffer
	removeCmd.SetOut(&stdoutBuf)
	removeCmd.SetErr(&bytes.Buffer{})

	err := removeCmd.RunE(removeCmd, []string{"my-skill"})
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "test-env")
	spec, err := envfile.ReadFile(filepath.Join(envDir, "agent.yaml"))
	if err != nil {
		t.Fatalf("reading agent.yaml: %v", err)
	}
	if spec.Skills != nil {
		if _, ok := spec.Skills["my-skill"]; ok {
			t.Error("my-skill should be removed from skills")
		}
	}
}

func TestRemove_WithType(t *testing.T) {
	home := newTestHome(t)
	content := `name: test-env
description: test
skills:
  common:
    source: github:a/b
agents:
  common:
    source: github:c/d
`
	createTestEnv(t, home, "test-env", content, "")
	writeActiveLock(t, home, "test-env")

	var stdoutBuf bytes.Buffer
	removeCmd.SetOut(&stdoutBuf)
	removeCmd.SetErr(&bytes.Buffer{})

	err := removeCmd.RunE(removeCmd, []string{"skill", "common"})
	if err != nil {
		t.Fatalf("remove skill common failed: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "test-env")
	spec, err := envfile.ReadFile(filepath.Join(envDir, "agent.yaml"))
	if err != nil {
		t.Fatalf("reading agent.yaml: %v", err)
	}
	if spec.Skills != nil {
		if _, ok := spec.Skills["common"]; ok {
			t.Error("common should be removed from skills")
		}
	}
	if spec.Agents == nil || spec.Agents["common"].Source != "github:c/d" {
		t.Error("common should remain in agents")
	}
}

func TestRemove_Ambiguous(t *testing.T) {
	home := newTestHome(t)
	content := `name: test-env
description: test
skills:
  shared:
    source: github:a/b
agents:
  shared:
    source: github:c/d
`
	createTestEnv(t, home, "test-env", content, "")
	writeActiveLock(t, home, "test-env")

	err := removeCmd.RunE(removeCmd, []string{"shared"})
	if err == nil {
		t.Fatal("expected error for ambiguous name")
	}
	if !strings.Contains(err.Error(), "both") {
		t.Errorf("expected 'both' in error, got: %v", err)
	}
}

func TestRemove_NotFound(t *testing.T) {
	home := newTestHome(t)
	createTestEnv(t, home, "test-env",
		"name: test-env\ndescription: test\nskills:\n  existing:\n    source: github:a/b\n", "")
	writeActiveLock(t, home, "test-env")

	err := removeCmd.RunE(removeCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for non-existent package")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestRemove_AllTypes(t *testing.T) {
	home := newTestHome(t)
	content := `name: test-env
description: test
skills:
  skill-pkg:
    source: github:a/skill
mcps:
  mcp-pkg:
    source: npm:@scope/mcp
agents:
  agent-pkg:
    source: github:a/agent
tools:
  tool-pkg:
    source: local:./tool
hooks:
  hook-pkg:
    source: github:a/hook
prompts:
  prompt-pkg:
    source: github:a/prompt
`
	createTestEnv(t, home, "test-env", content, "")
	writeActiveLock(t, home, "test-env")

	envDir := filepath.Join(home, ".agentenv", "envs", "test-env")

	type testCase struct {
		pkgType string
		name    string
		section string
	}

	tests := []testCase{
		{"skill", "skill-pkg", "skills"},
		{"mcp", "mcp-pkg", "mcps"},
		{"agent", "agent-pkg", "agents"},
		{"tool", "tool-pkg", "tools"},
		{"hook", "hook-pkg", "hooks"},
		{"prompt", "prompt-pkg", "prompts"},
	}

	for _, tc := range tests {
		t.Run(tc.pkgType, func(t *testing.T) {
			err := removeCmd.RunE(removeCmd, []string{tc.pkgType, tc.name})
			if err != nil {
				t.Fatalf("remove %s failed: %v", tc.pkgType, err)
			}

			spec, err := envfile.ReadFile(filepath.Join(envDir, "agent.yaml"))
			if err != nil {
				t.Fatalf("reading agent.yaml: %v", err)
			}

			m := getSectionMap(spec, tc.section)
			if m != nil {
				if _, ok := m[tc.name]; ok {
					t.Errorf("%s should be removed from %s", tc.name, tc.section)
				}
			}
		})
	}
}

func TestAdd_NoActiveEnv(t *testing.T) {
	_ = newTestHome(t)

	addSource = "github:owner/repo"
	addDryRun = false

	err := addCmd.RunE(addCmd, []string{"skill", "my-skill"})
	if err == nil {
		t.Fatal("expected error when no active env")
	}
	if !strings.Contains(err.Error(), "no active") && !strings.Contains(err.Error(), "Activate") {
		t.Errorf("expected 'no active' error, got: %v", err)
	}
}

func TestRemove_NoActiveEnv(t *testing.T) {
	_ = newTestHome(t)

	err := removeCmd.RunE(removeCmd, []string{"my-skill"})
	if err == nil {
		t.Fatal("expected error when no active env")
	}
	if !strings.Contains(err.Error(), "no active") && !strings.Contains(err.Error(), "Activate") {
		t.Errorf("expected 'no active' error, got: %v", err)
	}
}
