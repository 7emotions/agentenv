package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCreate_BasicCreate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	createAgent = "claude-code"
	createFrom = ""
	createEmpty = false

	err := createCmd.RunE(createCmd, []string{"test-env"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "test-env")

	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		t.Fatal("env directory was not created")
	}

	yamlPath := filepath.Join(envDir, "agent.yaml")
	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		t.Fatal("agent.yaml was not created")
	}

	statePath := filepath.Join(envDir, "state.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("state.json was not created")
	}
}

func TestCreate_DuplicateName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	createAgent = "claude-code"
	createFrom = ""
	createEmpty = false

	err := createCmd.RunE(createCmd, []string{"dup-env"})
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	err = createCmd.RunE(createCmd, []string{"dup-env"})
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}

func TestCreate_InvalidName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		name    string
		wantErr bool
	}{
		{"Uppercase", true},
		{"my_env", true},
		{"1start-with-digit", true},
		{"has spaces", true},
		{"special!chars", true},
		{"a", false},
		{"valid-name", false},
		{"valid123", false},
		{"a-b-c", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createAgent = "claude-code"
			createFrom = ""
			createEmpty = false

			err := createCmd.RunE(createCmd, []string{tt.name})
			if tt.wantErr && err == nil {
				t.Errorf("expected error for name %q, got nil", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for name %q: %v", tt.name, err)
			}
		})
	}
}

func TestCreate_NameTooLong(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	createAgent = "claude-code"
	createFrom = ""
	createEmpty = false

	longName := ""
	for i := 0; i < 65; i++ {
		longName += "a"
	}

	err := createCmd.RunE(createCmd, []string{longName})
	if err == nil {
		t.Fatal("expected error for name >64 chars, got nil")
	}
}

func TestCreate_EmptyFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	createAgent = "claude-code"
	createFrom = ""
	createEmpty = true

	err := createCmd.RunE(createCmd, []string{"empty-env"})
	if err != nil {
		t.Fatalf("create --empty failed: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "empty-env")

	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		t.Fatal("env directory was not created")
	}

	yamlPath := filepath.Join(envDir, "agent.yaml")
	if _, err := os.Stat(yamlPath); !os.IsNotExist(err) {
		t.Fatal("agent.yaml should not exist with --empty flag")
	}

	statePath := filepath.Join(envDir, "state.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("state.json should exist even with --empty flag")
	}
}

func TestCreate_AgentFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	createAgent = "cursor"
	createFrom = ""
	createEmpty = false

	err := createCmd.RunE(createCmd, []string{"cursor-env"})
	if err != nil {
		t.Fatalf("create --agent cursor failed: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "cursor-env")
	statePath := filepath.Join(envDir, "state.json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("reading state.json: %v", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("unmarshaling state.json: %v", err)
	}

	if framework, ok := state["agent_framework"]; !ok {
		t.Fatal("state.json missing agent_framework field")
	} else if framework != "cursor" {
		t.Errorf("expected agent_framework=cursor, got %v", framework)
	}
}

func TestCreate_FromFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	createAgent = "claude-code"
	createFrom = ""
	createEmpty = false

	err := createCmd.RunE(createCmd, []string{"source-env"})
	if err != nil {
		t.Fatalf("creating source env failed: %v", err)
	}

	createFrom = "source-env"
	err = createCmd.RunE(createCmd, []string{"cloned-env"})
	if err != nil {
		t.Fatalf("create --from source-env failed: %v", err)
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "cloned-env")
	yamlPath := filepath.Join(envDir, "agent.yaml")

	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		t.Fatal("agent.yaml was not created via --from")
	}

	srcYAML := filepath.Join(home, ".agentenv", "envs", "source-env", "agent.yaml")
	srcData, _ := os.ReadFile(srcYAML)
	dstData, _ := os.ReadFile(yamlPath)

	if string(srcData) != string(dstData) {
		t.Error("agent.yaml content differs between source and clone")
	}
}

func TestCreate_FromFlagSourceNotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	createAgent = "claude-code"
	createFrom = "nonexistent-env"
	createEmpty = false

	err := createCmd.RunE(createCmd, []string{"new-env"})
	if err == nil {
		t.Fatal("expected error when --from source does not exist, got nil")
	}

	envDir := filepath.Join(home, ".agentenv", "envs", "new-env")
	if _, err := os.Stat(envDir); !os.IsNotExist(err) {
		t.Fatal("env directory should be cleaned up on --from failure")
	}
}
