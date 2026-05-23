package types

import (
	"encoding/json"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSourceURLParsing(t *testing.T) {
	tests := []struct {
		input string
		want  SourceURL
	}{
		{
			input: "github:owner/repo",
			want:  SourceURL{Raw: "github:owner/repo", Scheme: "github", Owner: "owner", Repo: "repo"},
		},
		{
			input: "github:owner/repo/path/to/skill",
			want:  SourceURL{Raw: "github:owner/repo/path/to/skill", Scheme: "github", Owner: "owner", Repo: "repo", SubPath: "path/to/skill"},
		},
		{
			input: "npm:@scope/package",
			want:  SourceURL{Raw: "npm:@scope/package", Scheme: "npm", Scope: "@scope", Name: "package"},
		},
		{
			input: "npm:package",
			want:  SourceURL{Raw: "npm:package", Scheme: "npm", Name: "package"},
		},
		{
			input: "local:./path",
			want:  SourceURL{Raw: "local:./path", Scheme: "local", Path: "./path"},
		},
		{
			input: "git:https://example.com/repo.git",
			want:  SourceURL{Raw: "git:https://example.com/repo.git", Scheme: "git", URL: "https://example.com/repo.git"},
		},
		{
			input: "file:/absolute/path",
			want:  SourceURL{Raw: "file:/absolute/path", Scheme: "file", Path: "/absolute/path"},
		},
		{
			input: "url:https://example.com/pkg.tar.gz",
			want:  SourceURL{Raw: "url:https://example.com/pkg.tar.gz", Scheme: "url", URL: "https://example.com/pkg.tar.gz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseSourceURL(tt.input)
			if err != nil {
				t.Fatalf("ParseSourceURL(%q) returned error: %v", tt.input, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSourceURL(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
			if got.String() != tt.input {
				t.Errorf("String() = %q, want %q", got.String(), tt.input)
			}
			if !got.IsValid() {
				t.Errorf("IsValid() = false, want true for %+v", got)
			}
		})
	}
}

func TestSourceURLInvalid(t *testing.T) {
	invalid := []string{
		"",
		"noscheme",
		"github:",
		"github:onlyowner",
		"npm:",
		"npm:@scope/",
		"unknown:foo",
	}

	for _, input := range invalid {
		t.Run(input, func(t *testing.T) {
			_, err := ParseSourceURL(input)
			if err == nil {
				t.Errorf("ParseSourceURL(%q) expected error, got nil", input)
			}
		})
	}
}

func TestSourceURLIsValidFalse(t *testing.T) {
	invalid := []SourceURL{
		{Scheme: "github", Owner: "", Repo: ""},
		{Scheme: "npm", Name: ""},
		{Scheme: "local", Path: ""},
		{Scheme: "git", URL: ""},
		{Scheme: "file", Path: ""},
		{Scheme: "url", URL: ""},
		{Scheme: ""},
	}

	for _, s := range invalid {
		t.Run(s.Scheme, func(t *testing.T) {
			if s.IsValid() {
				t.Errorf("IsValid() = true, want false for %+v", s)
			}
		})
	}
}

func TestJSONRoundTripSourceURL(t *testing.T) {
	original, err := ParseSourceURL("github:owner/repo")
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded SourceURL
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("JSON round-trip: got %+v, want %+v", decoded, original)
	}
}

func TestJSONRoundTripPackage(t *testing.T) {
	src, _ := ParseSourceURL("github:owner/repo")
	original := Package{
		Name:    "test-pkg",
		Type:    PackageTypeSkill,
		Version: "1.0.0",
		Source:  src,
		SHA256:  "abc123",
		Dependencies: []PackageDependency{
			{Name: "dep1", Type: PackageTypeSkill, Constraint: ">=1.0.0"},
		},
		RuntimeReqs: []RuntimeRequirement{
			{Type: "binary", Name: "node", MinVersion: "18", Optional: false},
		},
		Config: map[string]interface{}{"key": "val"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded Package
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("JSON round-trip:\n  got:  %+v\n  want: %+v", decoded, original)
	}
}

func TestJSONRoundTripEnvironment(t *testing.T) {
	original := Environment{
		Name:        "test-env",
		Description: "a test environment",
		Skills: map[string]PackageSpec{
			"skill-1": {Source: "github:owner/repo", Version: "1.0.0"},
		},
		MCPs: map[string]PackageSpec{
			"mcp-1": {Source: "npm:@scope/mcp", Version: "^2.0"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded Environment
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("JSON round-trip:\n  got:  %+v\n  want: %+v", decoded, original)
	}
}

func TestJSONRoundTripLockfile(t *testing.T) {
	original := Lockfile{
		Version:   1,
		Generated: "2025-05-24T10:00:00Z",
		Packages: []LockedPackage{
			{
				Name:     "pkg-1",
				Type:     PackageTypeSkill,
				Source:   "github:owner/repo",
				Version:  "1.0.0",
				Resolved: "https://github.com/owner/repo/archive/v1.0.0.tar.gz",
				SHA256:   "def456",
				Dependencies: []LockedDep{
					{Name: "dep1", Version: "2.0.0", Resolved: "resolved-url"},
				},
				InstalledAt: "2025-05-24T10:00:05Z",
			},
		},
		Environment: LockfileEnvSnapshot{
			SkillsCount: 1, MCPsCount: 0, AgentsCount: 0,
			ToolsCount: 0, HooksCount: 0, PromptsCount: 0,
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded Lockfile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("JSON round-trip:\n  got:  %+v\n  want: %+v", decoded, original)
	}
}

func TestJSONRoundTripMCPConfig(t *testing.T) {
	original := UniversalMCPConfig{
		Servers: map[string]UniversalMCPServer{
			"server-1": {
				Command:     "npx",
				Args:        []string{"-y", "mcp-server"},
				Env:         map[string]string{"KEY": "val"},
				Transport:   "stdio",
				Description: "test server",
			},
			"server-2": {
				URL:       "https://example.com/sse",
				Transport: "sse",
				Headers:   map[string]string{"Authorization": "Bearer token"},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded UniversalMCPConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("JSON round-trip:\n  got:  %+v\n  want: %+v", decoded, original)
	}
}

func TestJSONRoundTripAgentSpec(t *testing.T) {
	src, _ := ParseSourceURL("github:owner/agent-repo")
	original := AgentSpec{
		Name:        "test-agent",
		Version:     "0.1.0",
		Description: "a test agent",
		Source:      src,
		Dependencies: []PackageDependency{
			{Name: "skill-dep", Type: PackageTypeSkill, Constraint: ">=1.0"},
		},
		Config: map[string]interface{}{"model": "gpt-4"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded AgentSpec
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("JSON round-trip:\n  got:  %+v\n  want: %+v", decoded, original)
	}
}

func TestYAMLRoundTripSourceURL(t *testing.T) {
	original, err := ParseSourceURL("github:owner/repo")
	if err != nil {
		t.Fatal(err)
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal error: %v", err)
	}

	var decoded SourceURL
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("YAML round-trip: got %+v, want %+v", decoded, original)
	}
}

func TestYAMLRoundTripPackage(t *testing.T) {
	src, _ := ParseSourceURL("npm:test-package")
	original := Package{
		Name:    "yaml-pkg",
		Type:    PackageTypeMCP,
		Version: "2.0.0",
		Source:  src,
		Config:  map[string]interface{}{"timeout": 30},
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal error: %v", err)
	}

	var decoded Package
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("YAML round-trip:\n  got:  %+v\n  want: %+v", decoded, original)
	}
}

func TestYAMLRoundTripEnvironment(t *testing.T) {
	original := Environment{
		Name: "yaml-env",
		Agents: map[string]PackageSpec{
			"agent-1": {Source: "file:/path/to/agent", Version: "0.1.0"},
		},
		Hooks: map[string]PackageSpec{
			"preinstall": {Source: "local:./hook.sh"},
		},
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal error: %v", err)
	}

	var decoded Environment
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("YAML round-trip:\n  got:  %+v\n  want: %+v", decoded, original)
	}
}

func TestYAMLRoundTripLockfile(t *testing.T) {
	original := Lockfile{
		Version:   1,
		Generated: "2025-05-24T10:00:00Z",
		Packages: []LockedPackage{
			{
				Name:     "pkg-1",
				Type:     PackageTypeSkill,
				Source:   "github:owner/repo",
				Version:  "1.0.0",
				Resolved: "https://example.com/pkg.tar.gz",
			},
		},
		Environment: LockfileEnvSnapshot{SkillsCount: 1},
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal error: %v", err)
	}

	var decoded Lockfile
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("YAML round-trip:\n  got:  %+v\n  want: %+v", decoded, original)
	}
}

func TestYAMLRoundTripMCPConfig(t *testing.T) {
	original := UniversalMCPConfig{
		Servers: map[string]UniversalMCPServer{
			"srv": {
				URL:       "http://localhost:8080/sse",
				Transport: "http",
				Headers:   map[string]string{"X-Custom": "val"},
			},
		},
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal error: %v", err)
	}

	var decoded UniversalMCPConfig
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("YAML round-trip:\n  got:  %+v\n  want: %+v", decoded, original)
	}
}

func TestYAMLRoundTripAgentSpec(t *testing.T) {
	src, _ := ParseSourceURL("git:https://github.com/owner/agent.git")
	original := AgentSpec{
		Name:        "yaml-agent",
		Version:     "0.2.0",
		Description: "agent from yaml test",
		Source:      src,
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal error: %v", err)
	}

	var decoded AgentSpec
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("YAML round-trip:\n  got:  %+v\n  want: %+v", decoded, original)
	}
}
