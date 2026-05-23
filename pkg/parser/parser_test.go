package parser

import (
	"strings"
	"testing"

	"github.com/agentenv/agentenv/pkg/types"
)

func TestParseAgentYAML_Minimal(t *testing.T) {
	input := `name: test-env
description: A test environment
`
	env, err := ParseAgentYAML([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Name != "test-env" {
		t.Errorf("Name = %q, want %q", env.Name, "test-env")
	}
	if env.Description != "A test environment" {
		t.Errorf("Description = %q, want %q", env.Description, "A test environment")
	}
}

func TestParseAgentYAML_NameOnly(t *testing.T) {
	input := `name: minimal
`
	env, err := ParseAgentYAML([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Name != "minimal" {
		t.Errorf("Name = %q, want %q", env.Name, "minimal")
	}
	if env.Description != "" {
		t.Errorf("Description = %q, want empty", env.Description)
	}
}

func TestParseAgentYAML_Full(t *testing.T) {
	input := `
name: my-project
description: My project environment
skills:
  code-review:
    source: github:vercel-labs/agent-skills/skills/code-review
    version: "^1.0.0"
  linting:
    source: github:some-org/linting-skill
    version: latest
mcps:
  filesystem:
    source: npm:@modelcontextprotocol/server-filesystem
    version: latest
    config:
      transport: stdio
      command: npx
      args:
        - "-y"
        - "@modelcontextprotocol/server-filesystem"
        - "."
  github:
    source: npm:@modelcontextprotocol/server-github
    version: ">=0.1.0"
    config:
      transport: stdio
      command: npx
      env:
        GITHUB_TOKEN: "${GITHUB_TOKEN}"
agents:
  code-assistant:
    source: github:agent-hub/code-assistant
    version: "~2.0"
tools:
  formatter:
    source: local:./tools/formatter
hooks:
  preinstall:
    source: github:org/hooks/preinstall
prompts:
  code-review-prompt:
    source: npm:@scope/prompt-pkg
`
	env, err := ParseAgentYAML([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Name != "my-project" {
		t.Errorf("Name = %q", env.Name)
	}
	if l := len(env.Skills); l != 2 {
		t.Errorf("Skills len = %d, want 2", l)
	}
	if l := len(env.MCPs); l != 2 {
		t.Errorf("MCPs len = %d, want 2", l)
	}
	if l := len(env.Agents); l != 1 {
		t.Errorf("Agents len = %d, want 1", l)
	}
	if l := len(env.Tools); l != 1 {
		t.Errorf("Tools len = %d, want 1", l)
	}
	if l := len(env.Hooks); l != 1 {
		t.Errorf("Hooks len = %d, want 1", l)
	}
	if l := len(env.Prompts); l != 1 {
		t.Errorf("Prompts len = %d, want 1", l)
	}

	cr, ok := env.Skills["code-review"]
	if !ok {
		t.Fatal("code-review skill missing")
	}
	if cr.Source != "github:vercel-labs/agent-skills/skills/code-review" {
		t.Errorf("code-review source = %q", cr.Source)
	}
	if cr.Version != "^1.0.0" {
		t.Errorf("code-review version = %q", cr.Version)
	}

	fs, ok := env.MCPs["filesystem"]
	if !ok {
		t.Fatal("filesystem MCP missing")
	}
	if fs.Version != "latest" {
		t.Errorf("filesystem version = %q", fs.Version)
	}
	if fs.Config == nil {
		t.Fatal("filesystem config missing")
	}
	if fs.Config["transport"] != "stdio" {
		t.Errorf("transport = %v", fs.Config["transport"])
	}
	args, ok := fs.Config["args"].([]interface{})
	if !ok || len(args) != 3 {
		t.Errorf("args = %v", fs.Config["args"])
	}
}

func TestParseAgentYAML_VersionDefaultsToStar(t *testing.T) {
	input := `
name: test
description: test
skills:
  no-version:
    source: github:some/repo
`
	env, err := ParseAgentYAML([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pkg := env.Skills["no-version"]
	if pkg.Version != "*" {
		t.Errorf("Version = %q, want %q", pkg.Version, "*")
	}
}

func TestParseAgentYAML_MissingName(t *testing.T) {
	input := `description: no name here
`
	_, err := ParseAgentYAML([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention 'name': %v", err)
	}
}

func TestParseAgentYAML_InvalidYAML(t *testing.T) {
	// \x09 is a tab character — tabs are not allowed in YAML indentation
	input := "name: broken\n\x09bad: tab\n"
	_, err := ParseAgentYAML([]byte(input))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to parse agent.yaml") {
		t.Errorf("error should wrap YAML error: %v", err)
	}
}

func TestLockfileRoundTrip(t *testing.T) {
	original := &types.Lockfile{
		Version:   1,
		Generated: "2025-05-24T10:00:00Z",
		Packages: []types.LockedPackage{
			{
				Name:     "code-review",
				Type:     types.PackageTypeSkill,
				Source:   "github:vercel-labs/agent-skills/skills/code-review",
				Version:  "1.0.0",
				Resolved: "https://github.com/vercel-labs/agent-skills/archive/v1.0.0.tar.gz",
				SHA256:   "abc123def456",
				Dependencies: []types.LockedDep{
					{Name: "utils", Version: "2.0.0", Resolved: "https://example.com/utils.tar.gz"},
				},
				InstalledAt: "2025-05-24T10:00:05Z",
			},
			{
				Name:     "filesystem-mcp",
				Type:     types.PackageTypeMCP,
				Source:   "npm:@modelcontextprotocol/server-filesystem",
				Version:  "0.5.0",
				Resolved: "https://registry.npmjs.org/@modelcontextprotocol/server-filesystem/-/server-filesystem-0.5.0.tgz",
			},
		},
		Environment: types.LockfileEnvSnapshot{
			SkillsCount: 1,
			MCPsCount:   1,
		},
	}

	data, err := WriteLockfile(original)
	if err != nil {
		t.Fatalf("WriteLockfile error: %v", err)
	}

	parsed, err := ParseLockfile(data)
	if err != nil {
		t.Fatalf("ParseLockfile error: %v", err)
	}

	if parsed.Version != original.Version {
		t.Errorf("Version = %d, want %d", parsed.Version, original.Version)
	}
	if parsed.Generated != original.Generated {
		t.Errorf("Generated = %q", parsed.Generated)
	}
	if len(parsed.Packages) != len(original.Packages) {
		t.Fatalf("Packages len = %d, want %d", len(parsed.Packages), len(original.Packages))
	}

	p0 := parsed.Packages[0]
	o0 := original.Packages[0]
	if p0.Name != o0.Name {
		t.Errorf("Package[0].Name = %q", p0.Name)
	}
	if p0.Type != o0.Type {
		t.Errorf("Package[0].Type = %q", p0.Type)
	}
	if p0.SHA256 != o0.SHA256 {
		t.Errorf("Package[0].SHA256 = %q", p0.SHA256)
	}
	if p0.InstalledAt != o0.InstalledAt {
		t.Errorf("Package[0].InstalledAt = %q", p0.InstalledAt)
	}
	if len(p0.Dependencies) != 1 {
		t.Fatalf("Dependencies len = %d, want 1", len(p0.Dependencies))
	}
	if p0.Dependencies[0].Name != "utils" {
		t.Errorf("Dep[0].Name = %q", p0.Dependencies[0].Name)
	}

	if parsed.Environment.SkillsCount != 1 {
		t.Errorf("Environment.SkillsCount = %d", parsed.Environment.SkillsCount)
	}
	if parsed.Environment.MCPsCount != 1 {
		t.Errorf("Environment.MCPsCount = %d", parsed.Environment.MCPsCount)
	}
}

func TestParseLockfile_MissingVersion(t *testing.T) {
	input := `generated: "2025-01-01T00:00:00Z"
packages: []
`
	_, err := ParseLockfile([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing version")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("error should mention 'version': %v", err)
	}
}

func TestParseLockfile_MissingGenerated(t *testing.T) {
	input := `version: 1
packages: []
`
	_, err := ParseLockfile([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing generated")
	}
	if !strings.Contains(err.Error(), "generated") {
		t.Errorf("error should mention 'generated': %v", err)
	}
}

func TestParseLockfile_InvalidVersion(t *testing.T) {
	input := `version: not-a-number
generated: "2025-01-01T00:00:00Z"
packages: []
`
	_, err := ParseLockfile([]byte(input))
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
	if !strings.Contains(err.Error(), "integer") {
		t.Errorf("error should mention integer: %v", err)
	}
}

func TestAgentPkgParsing(t *testing.T) {
	input := `
name: my-agent
version: "1.0.0"
description: A test agent
source: github:owner/repo
dependencies:
  - name: dep-skill
    type: skill
    constraint: ">=1.0"
  - name: dep-mcp
    type: mcp
    constraint: "^2.0"
config:
  model: gpt-4
  temperature: 0.7
`
	spec, err := ParseAgentPkg([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.Name != "my-agent" {
		t.Errorf("Name = %q", spec.Name)
	}
	if spec.Version != "1.0.0" {
		t.Errorf("Version = %q", spec.Version)
	}
	if spec.Description != "A test agent" {
		t.Errorf("Description = %q", spec.Description)
	}
	if spec.Source.String() != "github:owner/repo" {
		t.Errorf("Source = %q", spec.Source.String())
	}
	if len(spec.Dependencies) != 2 {
		t.Fatalf("Dependencies len = %d, want 2", len(spec.Dependencies))
	}
	if spec.Dependencies[0].Name != "dep-skill" {
		t.Errorf("Dep[0].Name = %q", spec.Dependencies[0].Name)
	}
	if spec.Dependencies[0].Type != types.PackageTypeSkill {
		t.Errorf("Dep[0].Type = %q", spec.Dependencies[0].Type)
	}
	if spec.Dependencies[0].Constraint != ">=1.0" {
		t.Errorf("Dep[0].Constraint = %q", spec.Dependencies[0].Constraint)
	}
	if spec.Config["model"] != "gpt-4" {
		t.Errorf("Config.model = %v", spec.Config["model"])
	}
	if spec.Config["temperature"] != 0.7 {
		t.Errorf("Config.temperature = %v", spec.Config["temperature"])
	}
}

func TestParseAgentPkg_Minimal(t *testing.T) {
	input := `name: minimal
version: 0.1.0
`
	spec, err := ParseAgentPkg([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Name != "minimal" {
		t.Errorf("Name = %q", spec.Name)
	}
	if spec.Version != "0.1.0" {
		t.Errorf("Version = %q", spec.Version)
	}
}

func TestParseAgentPkg_MissingName(t *testing.T) {
	input := `version: "1.0.0"
`
	_, err := ParseAgentPkg([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention 'name': %v", err)
	}
}

func TestParseAgentPkg_MissingVersion(t *testing.T) {
	input := `name: no-version
`
	_, err := ParseAgentPkg([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing version")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("error should mention 'version': %v", err)
	}
}

func TestParseAgentPkg_InvalidSource(t *testing.T) {
	input := `name: test
version: "1.0"
source: invalid-scheme
`
	_, err := ParseAgentPkg([]byte(input))
	if err == nil {
		t.Fatal("expected error for invalid source")
	}
	if !strings.Contains(err.Error(), "source") {
		t.Errorf("error should mention 'source': %v", err)
	}
}

func TestVersionConstraints(t *testing.T) {
	tests := []struct{ input, expected string }{
		{`"^1.2.3"`, "^1.2.3"},
		{`"~1.0"`, "~1.0"},
		{`">=1.0 <2.0"`, ">=1.0 <2.0"},
		{`"1.2.3"`, "1.2.3"},
		{`latest`, "latest"},
		{`"*"`, "*"},
	}
	for _, tc := range tests {
		input := "name: test\ndescription: test\nskills:\n  test-skill:\n    source: github:a/b\n    version: " + tc.input + "\n"
		env, err := ParseAgentYAML([]byte(input))
		if err != nil {
			t.Errorf("version %s should parse without error: %v", tc.input, err)
			continue
		}
		got := env.Skills["test-skill"].Version
		if got != tc.expected {
			t.Errorf("version %s parsed as %q, want %q", tc.input, got, tc.expected)
		}
	}
}
