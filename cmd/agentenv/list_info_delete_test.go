package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type jsonResponse struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
}

func TestList_NoEnvironments(t *testing.T) {
	home := newTestHome(t)
	_ = home

	var stdoutBuf bytes.Buffer
	listCmd.SetOut(&stdoutBuf)
	listCmd.SetErr(&bytes.Buffer{})

	err := listCmd.RunE(listCmd, nil)
	if err != nil {
		t.Fatalf("list with no envs should not error: %v", err)
	}
	output := stdoutBuf.String()
	if !strings.Contains(output, "No environments") {
		t.Errorf("expected 'No environments' message, got: %s", output)
	}
}

func TestList_MultipleEnvironments(t *testing.T) {
	home := newTestHome(t)

	createTestEnv(t, home, "env-alpha", "name: env-alpha\ndescription: alpha env\n", "")
	createTestEnv(t, home, "env-beta", "name: env-beta\ndescription: beta env\n", "")

	var stdoutBuf bytes.Buffer
	listCmd.SetOut(&stdoutBuf)
	listCmd.SetErr(&bytes.Buffer{})

	err := listCmd.RunE(listCmd, nil)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "env-alpha") {
		t.Errorf("output should contain env-alpha, got: %s", output)
	}
	if !strings.Contains(output, "env-beta") {
		t.Errorf("output should contain env-beta, got: %s", output)
	}
}

func TestList_JSONOutput(t *testing.T) {
	home := newTestHome(t)

	createTestEnv(t, home, "json-env", "name: json-env\ndescription: json\n", "")

	activeLock := filepath.Join(home, ".agentenv", "ACTIVE")
	os.WriteFile(activeLock, []byte("json-env:claude-code"), 0o644)

	jsonOutput = true
	defer func() { jsonOutput = false }()

	var stdoutBuf bytes.Buffer
	listCmd.SetOut(&stdoutBuf)
	listCmd.SetErr(&bytes.Buffer{})

	err := listCmd.RunE(listCmd, nil)
	if err != nil {
		t.Fatalf("list --json failed: %v", err)
	}

	var resp jsonResponse
	if err := json.Unmarshal(stdoutBuf.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok", resp.Status)
	}
	var entries []envEntry
	if err := json.Unmarshal(resp.Data, &entries); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "json-env" {
		t.Errorf("name = %q, want json-env", entries[0].Name)
	}
	if !entries[0].Active {
		t.Error("expected active=true")
	}
}

func TestInfo_ByName(t *testing.T) {
	home := newTestHome(t)

	createTestEnv(t, home, "info-env", "name: info-env\ndescription: info test env\n", "")

	var stdoutBuf bytes.Buffer
	infoCmd.SetOut(&stdoutBuf)
	infoCmd.SetErr(&bytes.Buffer{})

	err := infoCmd.RunE(infoCmd, []string{"info-env"})
	if err != nil {
		t.Fatalf("info failed: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "info-env") {
		t.Errorf("output should contain env name, got: %s", output)
	}
	if !strings.Contains(output, "Packages:") {
		t.Errorf("output should contain packages count, got: %s", output)
	}
}

func TestInfo_ActiveDefault(t *testing.T) {
	home := newTestHome(t)

	createTestEnv(t, home, "active-env", "name: active-env\ndescription: active\n", "")

	activeLock := filepath.Join(home, ".agentenv", "ACTIVE")
	os.WriteFile(activeLock, []byte("active-env:claude-code"), 0o644)

	var stdoutBuf bytes.Buffer
	infoCmd.SetOut(&stdoutBuf)
	infoCmd.SetErr(&bytes.Buffer{})

	err := infoCmd.RunE(infoCmd, nil)
	if err != nil {
		t.Fatalf("info without args should use active env: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "active-env") {
		t.Errorf("output should contain active-env, got: %s", output)
	}
}

func TestInfo_NoActiveEnv(t *testing.T) {
	home := newTestHome(t)
	_ = home

	infoCmd.SetOut(&bytes.Buffer{})
	infoCmd.SetErr(&bytes.Buffer{})

	err := infoCmd.RunE(infoCmd, nil)
	if err == nil {
		t.Fatal("expected error with no active env, got nil")
	}
	if !strings.Contains(err.Error(), "no active environment") {
		t.Errorf("error should mention no active env, got: %v", err)
	}
}

func TestInfo_JSONOutput(t *testing.T) {
	home := newTestHome(t)

	createTestEnv(t, home, "json-info", "name: json-info\ndescription: json info\n", "")

	jsonOutput = true
	defer func() { jsonOutput = false }()

	var stdoutBuf bytes.Buffer
	infoCmd.SetOut(&stdoutBuf)
	infoCmd.SetErr(&bytes.Buffer{})

	err := infoCmd.RunE(infoCmd, []string{"json-info"})
	if err != nil {
		t.Fatalf("info --json failed: %v", err)
	}

	var resp jsonResponse
	if err := json.Unmarshal(stdoutBuf.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok", resp.Status)
	}
	var info envInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if info.Name != "json-info" {
		t.Errorf("name = %q, want json-info", info.Name)
	}
	if info.PackagesCount != 0 {
		t.Errorf("packages = %d, want 0", info.PackagesCount)
	}
}

func TestInfo_PackagesCounted(t *testing.T) {
	home := newTestHome(t)

	yamlContent := `name: pkg-env
description: env with packages
skills:
  skill-a:
    source: github:test/a
    version: "1.0.0"
mcps:
  mcp-b:
    source: npm:@test/b
    version: "2.0.0"
agents:
  agent-c:
    source: github:test/c
    version: "3.0.0"
`

	createTestEnv(t, home, "pkg-env", yamlContent, "")

	jsonOutput = true
	defer func() { jsonOutput = false }()

	var stdoutBuf bytes.Buffer
	infoCmd.SetOut(&stdoutBuf)
	infoCmd.SetErr(&bytes.Buffer{})

	err := infoCmd.RunE(infoCmd, []string{"pkg-env"})
	if err != nil {
		t.Fatalf("info failed: %v", err)
	}

	var resp jsonResponse
	if err := json.Unmarshal(stdoutBuf.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok", resp.Status)
	}
	var info envInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if info.PackagesCount != 3 {
		t.Errorf("packages = %d, want 3", info.PackagesCount)
	}
}

func TestDelete_Basic(t *testing.T) {
	home := newTestHome(t)

	createTestEnv(t, home, "delete-me", "name: delete-me\ndescription: gonna be deleted\n", "")

	envDir := filepath.Join(home, ".agentenv", "envs", "delete-me")
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		t.Fatal("env should exist before delete")
	}

	var stdoutBuf bytes.Buffer
	deleteCmd.SetOut(&stdoutBuf)
	deleteCmd.SetErr(&bytes.Buffer{})

	err := deleteCmd.RunE(deleteCmd, []string{"delete-me"})
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if _, err := os.Stat(envDir); !os.IsNotExist(err) {
		t.Error("env directory should be removed after delete")
	}
}

func TestDelete_ActiveWithoutForce(t *testing.T) {
	home := newTestHome(t)

	createTestEnv(t, home, "active-del", "name: active-del\ndescription: active for delete test\n", "")

	activeLock := filepath.Join(home, ".agentenv", "ACTIVE")
	os.WriteFile(activeLock, []byte("active-del:claude-code"), 0o644)

	deleteCmd.SetOut(&bytes.Buffer{})
	deleteCmd.SetErr(&bytes.Buffer{})

	err := deleteCmd.RunE(deleteCmd, []string{"active-del"})
	if err == nil {
		t.Fatal("expected error deleting active env without --force, got nil")
	}
	if !strings.Contains(err.Error(), "active") || !strings.Contains(err.Error(), "Deactivate") {
		t.Errorf("error should mention active/Deactivate, got: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "active-del")
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		t.Error("env should NOT be deleted without --force")
	}
}

func TestDelete_ActiveWithForce(t *testing.T) {
	home := newTestHome(t)

	createTestEnv(t, home, "force-del", "name: force-del\ndescription: force delete\n", "")

	activeLock := filepath.Join(home, ".agentenv", "ACTIVE")
	os.WriteFile(activeLock, []byte("force-del:claude-code"), 0o644)

	deleteForce = true
	defer func() { deleteForce = false }()

	var stdoutBuf bytes.Buffer
	deleteCmd.SetOut(&stdoutBuf)
	deleteCmd.SetErr(&bytes.Buffer{})

	err := deleteCmd.RunE(deleteCmd, []string{"force-del"})
	if err != nil {
		t.Fatalf("delete --force should work on active env: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "force-del")
	if _, err := os.Stat(envDir); !os.IsNotExist(err) {
		t.Error("env should be deleted with --force")
	}
}

func TestDelete_NonExistent(t *testing.T) {
	home := newTestHome(t)
	os.MkdirAll(filepath.Join(home, ".agentenv"), 0o755)

	deleteCmd.SetOut(&bytes.Buffer{})
	deleteCmd.SetErr(&bytes.Buffer{})

	err := deleteCmd.RunE(deleteCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for non-existent env, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should say 'not found', got: %v", err)
	}
}

func TestDelete_JSONOutput(t *testing.T) {
	home := newTestHome(t)

	createTestEnv(t, home, "json-del", "name: json-del\ndescription: json delete\n", "")

	jsonOutput = true
	deleteForce = false
	defer func() {
		jsonOutput = false
	}()

	var stdoutBuf bytes.Buffer
	deleteCmd.SetOut(&stdoutBuf)
	deleteCmd.SetErr(&bytes.Buffer{})

	err := deleteCmd.RunE(deleteCmd, []string{"json-del"})
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	var resp jsonResponse
	if err := json.Unmarshal(stdoutBuf.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok", resp.Status)
	}
	var result map[string]string
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if result["deleted"] != "json-del" {
		t.Errorf("deleted = %q, want json-del", result["deleted"])
	}
}

func TestListPackages_Basic(t *testing.T) {
	home := newTestHome(t)

	yamlContent := `name: pkg-list
description: test packages
skills:
  my-skill:
    source: github:user/skill
    version: "1.0.0"
mcps:
  my-mcp:
    source: npm:@user/mcp
    version: "2.0.0"
    config:
      command: npx
      args: ["-y", "@user/mcp"]
`

	createTestEnv(t, home, "pkg-list", yamlContent, "")

	var stdoutBuf bytes.Buffer
	listPackagesCmd.SetOut(&stdoutBuf)
	listPackagesCmd.SetErr(&bytes.Buffer{})

	err := listPackagesCmd.RunE(listPackagesCmd, []string{"pkg-list"})
	if err != nil {
		t.Fatalf("list-packages failed: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "my-skill") {
		t.Errorf("output should contain my-skill, got: %s", output)
	}
	if !strings.Contains(output, "my-mcp") {
		t.Errorf("output should contain my-mcp, got: %s", output)
	}
	if !strings.Contains(output, "skill") {
		t.Errorf("output should contain 'skill', got: %s", output)
	}
	if !strings.Contains(output, "mcp") {
		t.Errorf("output should contain 'mcp', got: %s", output)
	}
}

func TestListPackages_TypeFilter(t *testing.T) {
	home := newTestHome(t)

	yamlContent := `name: filter-env
skills:
  only-skill:
    source: github:user/skill
mcps:
  only-mcp:
    source: npm:@user/mcp
`

	createTestEnv(t, home, "filter-env", yamlContent, "")

	listPkgType = "skill"
	defer func() { listPkgType = "" }()

	var stdoutBuf bytes.Buffer
	listPackagesCmd.SetOut(&stdoutBuf)
	listPackagesCmd.SetErr(&bytes.Buffer{})

	err := listPackagesCmd.RunE(listPackagesCmd, []string{"filter-env"})
	if err != nil {
		t.Fatalf("list-packages --type skill failed: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "only-skill") {
		t.Errorf("output should contain only-skill, got: %s", output)
	}
	if strings.Contains(output, "only-mcp") {
		t.Errorf("output should NOT contain only-mcp when filtered to skill, got: %s", output)
	}
}

func TestListPackages_TreeView(t *testing.T) {
	home := newTestHome(t)

	yamlContent := `name: tree-env
skills:
  skill-a:
    source: github:user/a
agents:
  agent-b:
    source: github:user/b
`

	createTestEnv(t, home, "tree-env", yamlContent, "")

	listPkgTree = true
	defer func() { listPkgTree = false }()

	var stdoutBuf bytes.Buffer
	listPackagesCmd.SetOut(&stdoutBuf)
	listPackagesCmd.SetErr(&bytes.Buffer{})

	err := listPackagesCmd.RunE(listPackagesCmd, []string{"tree-env"})
	if err != nil {
		t.Fatalf("list-packages --tree failed: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "skill:") {
		t.Errorf("tree view should have 'skill:' heading, got: %s", output)
	}
	if !strings.Contains(output, "agent:") {
		t.Errorf("tree view should have 'agent:' heading, got: %s", output)
	}
	if !strings.Contains(output, "  skill-a") {
		t.Errorf("tree view should indent skill-a, got: %s", output)
	}
}

func TestListPackages_JSONOutput(t *testing.T) {
	home := newTestHome(t)

	yamlContent := `name: json-pkgs
skills:
  skill-x:
    source: github:user/x
    version: "1.0.0"
`

	createTestEnv(t, home, "json-pkgs", yamlContent, "")

	jsonOutput = true
	defer func() { jsonOutput = false }()

	var stdoutBuf bytes.Buffer
	listPackagesCmd.SetOut(&stdoutBuf)
	listPackagesCmd.SetErr(&bytes.Buffer{})

	err := listPackagesCmd.RunE(listPackagesCmd, []string{"json-pkgs"})
	if err != nil {
		t.Fatalf("list-packages --json failed: %v", err)
	}

	var resp jsonResponse
	if err := json.Unmarshal(stdoutBuf.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok", resp.Status)
	}
	var pkgs []packageEntry
	if err := json.Unmarshal(resp.Data, &pkgs); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	if pkgs[0].Name != "skill-x" {
		t.Errorf("name = %q, want skill-x", pkgs[0].Name)
	}
	if pkgs[0].Type != "skill" {
		t.Errorf("type = %q, want skill", pkgs[0].Type)
	}
}

func TestListPackages_NoPackages(t *testing.T) {
	home := newTestHome(t)

	createTestEnv(t, home, "no-pkgs", "name: no-pkgs\ndescription: empty\n", "")

	var stdoutBuf bytes.Buffer
	listPackagesCmd.SetOut(&stdoutBuf)
	listPackagesCmd.SetErr(&bytes.Buffer{})

	err := listPackagesCmd.RunE(listPackagesCmd, []string{"no-pkgs"})
	if err != nil {
		t.Fatalf("list-packages on empty env: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "No packages") {
		t.Errorf("expected 'No packages defined', got: %s", output)
	}
}

func TestListPackages_ActiveDefault(t *testing.T) {
	home := newTestHome(t)

	yamlContent := `name: active-pkg
skills:
  active-skill:
    source: github:user/skill
`
	createTestEnv(t, home, "active-pkg", yamlContent, "")

	activeLock := filepath.Join(home, ".agentenv", "ACTIVE")
	os.WriteFile(activeLock, []byte("active-pkg:claude-code"), 0o644)

	var stdoutBuf bytes.Buffer
	listPackagesCmd.SetOut(&stdoutBuf)
	listPackagesCmd.SetErr(&bytes.Buffer{})

	err := listPackagesCmd.RunE(listPackagesCmd, nil)
	if err != nil {
		t.Fatalf("list-packages without args should use active env: %v", err)
	}

	output := stdoutBuf.String()
	if !strings.Contains(output, "active-skill") {
		t.Errorf("output should contain active-skill, got: %s", output)
	}
}

func TestActiveLockFormat_RoundTrip(t *testing.T) {
	home := newTestHome(t)

	yamlContent := `name: fmt-env
description: format roundtrip test
skills:
  fmt-skill:
    source: github:test/fmt-skill
mcps:
  fmt-mcp:
    source: npm:@test/fmt-mcp
`
	createTestEnv(t, home, "fmt-env", yamlContent, "")

	activeLock := filepath.Join(home, ".agentenv", "ACTIVE")
	os.WriteFile(activeLock, []byte("fmt-env:claude-code"), 0o644)

	t.Run("info reads name:framework format", func(t *testing.T) {
		var stdoutBuf bytes.Buffer
		infoCmd.SetOut(&stdoutBuf)
		infoCmd.SetErr(&bytes.Buffer{})

		err := infoCmd.RunE(infoCmd, nil)
		if err != nil {
			t.Fatalf("info (no args) with name:framework ACTIVE: %v", err)
		}
		output := stdoutBuf.String()
		if !strings.Contains(output, "fmt-env") {
			t.Errorf("output should contain env name, got: %s", output)
		}
		if !strings.Contains(output, "Active:         true") {
			t.Errorf("expected Active: true, got: %s", output)
		}
	})

	t.Run("list-packages reads name:framework format", func(t *testing.T) {
		var stdoutBuf bytes.Buffer
		listPackagesCmd.SetOut(&stdoutBuf)
		listPackagesCmd.SetErr(&bytes.Buffer{})

		err := listPackagesCmd.RunE(listPackagesCmd, nil)
		if err != nil {
			t.Fatalf("list-packages (no args) with name:framework ACTIVE: %v", err)
		}
		output := stdoutBuf.String()
		if !strings.Contains(output, "fmt-skill") {
			t.Errorf("output should contain fmt-skill, got: %s", output)
		}
		if !strings.Contains(output, "fmt-mcp") {
			t.Errorf("output should contain fmt-mcp, got: %s", output)
		}
	})
}
