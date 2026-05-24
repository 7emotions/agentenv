package envfile

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ---- Reader Tests ----

func TestRead_Minimal(t *testing.T) {
	input := `name: test-env
description: A test environment
`
	spec, err := Read([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Name != "test-env" {
		t.Errorf("Name = %q, want %q", spec.Name, "test-env")
	}
	if spec.Description != "A test environment" {
		t.Errorf("Description = %q, want %q", spec.Description, "A test environment")
	}
	if spec.Skills != nil {
		t.Errorf("Skills should be nil, got %v", spec.Skills)
	}
}

func TestRead_NameOnly(t *testing.T) {
	input := `name: minimal
`
	spec, err := Read([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Name != "minimal" {
		t.Errorf("Name = %q", spec.Name)
	}
}

func TestRead_Full(t *testing.T) {
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
  github-mcp:
    source: npm:@modelcontextprotocol/server-github
    version: ">=0.1.0"
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
	spec, err := Read([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if l := len(spec.Skills); l != 2 {
		t.Errorf("Skills len = %d, want 2", l)
	}
	if l := len(spec.MCPs); l != 2 {
		t.Errorf("MCPs len = %d, want 2", l)
	}
	if l := len(spec.Agents); l != 1 {
		t.Errorf("Agents len = %d, want 1", l)
	}
	if l := len(spec.Tools); l != 1 {
		t.Errorf("Tools len = %d, want 1", l)
	}
	if l := len(spec.Hooks); l != 1 {
		t.Errorf("Hooks len = %d, want 1", l)
	}
	if l := len(spec.Prompts); l != 1 {
		t.Errorf("Prompts len = %d, want 1", l)
	}

	cr := spec.Skills["code-review"]
	if cr.Source != "github:vercel-labs/agent-skills/skills/code-review" {
		t.Errorf("code-review source = %q", cr.Source)
	}
	if cr.Version != "^1.0.0" {
		t.Errorf("code-review version = %q", cr.Version)
	}

	fs := spec.MCPs["filesystem"]
	if fs.Config["transport"] != "stdio" {
		t.Errorf("filesystem transport = %v", fs.Config["transport"])
	}
}

func TestRead_EmptyName(t *testing.T) {
	input := `description: no name here
`
	_, err := Read([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention 'name': %v", err)
	}
}

func TestRead_UnknownKey(t *testing.T) {
	input := `name: test-env
description: test
version: "2.0"
foo: bar
skills:
  test-skill:
    source: github:a/b
`
	spec, err := Read([]byte(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	// The Read function should warn about unknown top-level keys via Validate
	// and the returned spec should still be valid
	if spec == nil {
		t.Fatal("expected non-nil spec despite unknown keys")
	}
	if spec.Name != "test-env" {
		t.Errorf("Name = %q", spec.Name)
	}
}

func TestReadFile_LocalPathResolution(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "agent.yaml")

	content := `name: test-env
description: test
tools:
  my-tool:
    source: local:./tools/my-tool
  other-tool:
    source: local:../other-tool
`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	spec, err := ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	myTool := spec.Tools["my-tool"]
	expectedMy := "local:" + filepath.Join(dir, "tools", "my-tool")
	if myTool.Source != expectedMy {
		t.Errorf("my-tool source = %q, want %q", myTool.Source, expectedMy)
	}

	otherTool := spec.Tools["other-tool"]
	expectedOther := "local:" + filepath.Join(filepath.Dir(dir), "other-tool")
	if otherTool.Source != expectedOther {
		t.Errorf("other-tool source = %q, want %q", otherTool.Source, expectedOther)
	}
}

func TestRead_InvalidYAML(t *testing.T) {
	input := "name: broken\n\x09bad: tab\n"
	_, err := Read([]byte(input))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to parse agent.yaml") {
		t.Errorf("error should wrap YAML error: %v", err)
	}
}

// ---- Validator Tests ----

func TestValidate_ValidSpec(t *testing.T) {
	spec := &EnvironmentSpec{
		Name:        "my-project",
		Description: "A project",
		Skills: map[string]PackageRef{
			"code-review": {Source: "github:vercel/repo", Version: "^1.0.0"},
		},
	}
	result := Validate(spec)
	if !result.Valid {
		t.Errorf("expected Valid=true, got false. Errors: %v", result.Errors)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestValidate_MissingName(t *testing.T) {
	spec := &EnvironmentSpec{
		Description: "no name",
	}
	result := Validate(spec)
	if result.Valid {
		t.Fatal("expected Valid=false for missing name")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "name") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing name, got: %v", result.Errors)
	}
}

func TestValidate_NonKebabName(t *testing.T) {
	spec := &EnvironmentSpec{
		Name:        "MyProject",
		Description: "bad name",
	}
	result := Validate(spec)
	if result.Valid {
		t.Fatal("expected Valid=false for non-kebab name")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "kebab-case") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected kebab-case error, got: %v", result.Errors)
	}
}

func TestValidate_MissingDescription(t *testing.T) {
	spec := &EnvironmentSpec{
		Name: "my-env",
	}
	result := Validate(spec)
	if !result.Valid {
		t.Fatal("expected Valid=true even without description")
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "description") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about empty description, got: %v", result.Warnings)
	}
}

func TestValidate_InvalidSource(t *testing.T) {
	spec := &EnvironmentSpec{
		Name:        "test-env",
		Description: "test",
		Skills: map[string]PackageRef{
			"bad-skill": {Source: "no-scheme", Version: "*"},
		},
	}
	result := Validate(spec)
	if result.Valid {
		t.Fatal("expected Valid=false for invalid source")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "scheme") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about unsupported scheme, got: %v", result.Errors)
	}
}

func TestValidate_UnsupportedScheme(t *testing.T) {
	spec := &EnvironmentSpec{
		Name:        "test-env",
		Description: "test",
		MCPs: map[string]PackageRef{
			"bad-mcp": {Source: "ftp://example.com/pkg", Version: "*"},
		},
	}
	result := Validate(spec)
	if result.Valid {
		t.Fatal("expected Valid=false for unsupported scheme")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "unsupported source scheme") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unsupported scheme error, got: %v", result.Errors)
	}
}

func TestValidate_EmptySource(t *testing.T) {
	spec := &EnvironmentSpec{
		Name:        "test-env",
		Description: "test",
		Tools: map[string]PackageRef{
			"my-tool": {Source: "", Version: "*"},
		},
	}
	result := Validate(spec)
	if result.Valid {
		t.Fatal("expected Valid=false for empty source")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "source is required") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'source is required' error, got: %v", result.Errors)
	}
}

func TestValidate_InvalidVersion(t *testing.T) {
	spec := &EnvironmentSpec{
		Name:        "test-env",
		Description: "test",
		Skills: map[string]PackageRef{
			"test-skill": {Source: "github:a/b", Version: ">< 2.0"},
		},
	}
	result := Validate(spec)
	if result.Valid {
		t.Fatal("expected Valid=false for invalid version constraint")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "invalid version constraint") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected version constraint error, got: %v", result.Errors)
	}
}

func TestValidate_ValidVersions(t *testing.T) {
	validVersions := []string{"", "*", "latest", "^1.0.0", "~1.0", ">=1.0 <2.0", "1.2.3"}
	for _, v := range validVersions {
		spec := &EnvironmentSpec{
			Name:        "test-env",
			Description: "test",
			Skills: map[string]PackageRef{
				"test-skill": {Source: "github:a/b", Version: v},
			},
		}
		result := Validate(spec)
		if !result.Valid {
			t.Errorf("version %q: expected Valid=true, got false. Errors: %v", v, result.Errors)
		}
	}
}

func TestValidate_DuplicatePackageNames(t *testing.T) {
	spec := &EnvironmentSpec{
		Name:        "test-env",
		Description: "test",
		Skills: map[string]PackageRef{
			"deploy": {Source: "github:a/b", Version: "*"},
		},
		Agents: map[string]PackageRef{
			"deploy": {Source: "github:c/d", Version: "*"},
		},
	}
	result := Validate(spec)
	if !result.Valid {
		t.Errorf("expected Valid=true (duplicate is warning not error), got false")
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "deploy") && strings.Contains(w.Message, "skills") && strings.Contains(w.Message, "agents") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about duplicate 'deploy', got: %v", result.Warnings)
	}
}

func TestValidate_NilSpec(t *testing.T) {
	result := Validate(nil)
	if result.Valid {
		t.Fatal("expected Valid=false for nil spec")
	}
}

func TestValidateAndWarn_NoErrors(t *testing.T) {
	spec := &EnvironmentSpec{
		Name:        "test-env",
		Description: "test",
	}
	result, err := ValidateAndWarn(spec)
	if err != nil {
		t.Fatalf("expected no error for valid spec: %v", err)
	}
	if !result.Valid {
		t.Fatal("expected Valid=true")
	}
}

func TestValidateAndWarn_WithErrors(t *testing.T) {
	spec := &EnvironmentSpec{
		Description: "no name",
	}
	_, err := ValidateAndWarn(spec)
	if err == nil {
		t.Fatal("expected error for invalid spec")
	}
}

func TestValidateAndWarn_WithWarningsOnly(t *testing.T) {
	spec := &EnvironmentSpec{
		Name: "test-env",
	}
	result, err := ValidateAndWarn(spec)
	if err != nil {
		t.Fatalf("expected no error for warnings only: %v", err)
	}
	if !result.HasWarnings() {
		t.Fatal("expected warnings for missing description")
	}
}

// ---- Writer Tests ----

func TestWrite_RoundTrip(t *testing.T) {
	original := &EnvironmentSpec{
		Name:        "test-env",
		Description: "A test environment",
		Skills: map[string]PackageRef{
			"code-review": {Source: "github:vercel/repo", Version: "^1.0.0"},
			"linting":     {Source: "github:some-org/linting", Version: "latest"},
		},
		MCPs: map[string]PackageRef{
			"filesystem": {
				Source:  "npm:@mcp/filesystem",
				Version: "latest",
				Config: map[string]interface{}{
					"transport": "stdio",
					"command":   "npx",
					"args":      []interface{}{"-y", "mcp-server"},
				},
			},
		},
		Agents: map[string]PackageRef{
			"assistant": {Source: "github:agent-hub/assistant", Version: "~2.0"},
		},
	}

	data, err := Write(original)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	parsed, err := Read(data)
	if err != nil {
		t.Fatalf("Read after Write error: %v", err)
	}

	if parsed.Name != original.Name {
		t.Errorf("Name = %q, want %q", parsed.Name, original.Name)
	}
	if parsed.Description != original.Description {
		t.Errorf("Description = %q, want %q", parsed.Description, original.Description)
	}
	if len(parsed.Skills) != len(original.Skills) {
		t.Errorf("Skills len = %d, want %d", len(parsed.Skills), len(original.Skills))
	}
	if len(parsed.MCPs) != len(original.MCPs) {
		t.Errorf("MCPs len = %d, want %d", len(parsed.MCPs), len(original.MCPs))
	}
	if len(parsed.Agents) != len(original.Agents) {
		t.Errorf("Agents len = %d, want %d", len(parsed.Agents), len(original.Agents))
	}

	// Check specific values
	cr := parsed.Skills["code-review"]
	if cr.Source != "github:vercel/repo" {
		t.Errorf("code-review source = %q", cr.Source)
	}
	if cr.Version != "^1.0.0" {
		t.Errorf("code-review version = %q", cr.Version)
	}

	fs := parsed.MCPs["filesystem"]
	if fs.Config["transport"] != "stdio" {
		t.Errorf("filesystem transport = %v", fs.Config["transport"])
	}
}

func TestWrite_Minimal(t *testing.T) {
	spec := &EnvironmentSpec{
		Name:        "minimal",
		Description: "minimal env",
	}
	data, err := Write(spec)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	// Should contain required fields
	if !strings.Contains(string(data), "name: minimal") {
		t.Errorf("output missing 'name':\n%s", string(data))
	}
	if !strings.Contains(string(data), "description:") {
		t.Errorf("output missing 'description':\n%s", string(data))
	}
}

func TestWrite_Deterministic(t *testing.T) {
	spec := &EnvironmentSpec{
		Name:        "test-env",
		Description: "test",
		Skills: map[string]PackageRef{
			"b-skill": {Source: "github:a/b", Version: "*"},
			"a-skill": {Source: "github:a/a", Version: "*"},
		},
	}

	data1, err := Write(spec)
	if err != nil {
		t.Fatalf("first Write error: %v", err)
	}
	data2, err := Write(spec)
	if err != nil {
		t.Fatalf("second Write error: %v", err)
	}

	if !bytes.Equal(data1, data2) {
		t.Errorf("deterministic output failed:\n%s\nvs\n%s", string(data1), string(data2))
	}
}

func TestWrite_NilSpec(t *testing.T) {
	_, err := Write(nil)
	if err == nil {
		t.Fatal("expected error for nil spec")
	}
}

func TestMerge_CombinesPackages(t *testing.T) {
	base := &EnvironmentSpec{
		Name:        "base-env",
		Description: "base",
		Skills: map[string]PackageRef{
			"skill-a": {Source: "github:a/a", Version: "*"},
		},
	}
	overlay := &EnvironmentSpec{
		Name:        "overlay-env",
		Description: "overlay desc",
		Skills: map[string]PackageRef{
			"skill-b": {Source: "github:b/b", Version: "*"},
		},
		MCPs: map[string]PackageRef{
			"mcp-a": {Source: "npm:mcp-a", Version: "^1.0"},
		},
	}

	merged := Merge(base, overlay)

	if merged.Name != "overlay-env" {
		t.Errorf("Name = %q, want %q", merged.Name, "overlay-env")
	}
	if merged.Description != "overlay desc" {
		t.Errorf("Description = %q", merged.Description)
	}
	if len(merged.Skills) != 2 {
		t.Errorf("Skills len = %d, want 2", len(merged.Skills))
	}
	if len(merged.MCPs) != 1 {
		t.Errorf("MCPs len = %d, want 1", len(merged.MCPs))
	}

	// Base should not be modified
	if len(base.Skills) != 1 {
		t.Errorf("base Skills mutated: len = %d", len(base.Skills))
	}
	if base.Name != "base-env" {
		t.Errorf("base Name mutated: %q", base.Name)
	}
}

func TestMerge_OverlayWins(t *testing.T) {
	base := &EnvironmentSpec{
		Name: "base",
		Skills: map[string]PackageRef{
			"same-skill": {Source: "github:old/version", Version: "1.0.0"},
		},
	}
	overlay := &EnvironmentSpec{
		Skills: map[string]PackageRef{
			"same-skill": {Source: "github:new/version", Version: "2.0.0"},
		},
	}

	merged := Merge(base, overlay)

	skill := merged.Skills["same-skill"]
	if skill.Source != "github:new/version" {
		t.Errorf("source = %q, want %q", skill.Source, "github:new/version")
	}
	if skill.Version != "2.0.0" {
		t.Errorf("version = %q", skill.Version)
	}
}

func TestMerge_NilInputs(t *testing.T) {
	// Both nil
	merged := Merge(nil, nil)
	if merged == nil {
		t.Fatal("Merge(nil, nil) returned nil")
	}
	if merged.Name != "" {
		t.Errorf("Name = %q, want empty", merged.Name)
	}

	// Base nil, overlay not
	overlay := &EnvironmentSpec{
		Name:        "overlay-only",
		Description: "desc",
		Skills: map[string]PackageRef{
			"skill-x": {Source: "github:x/x", Version: "*"},
		},
	}
	merged = Merge(nil, overlay)
	if merged.Name != "overlay-only" {
		t.Errorf("Name = %q", merged.Name)
	}
	if len(merged.Skills) != 1 {
		t.Errorf("Skills len = %d", len(merged.Skills))
	}

	// Overlay nil, base not
	base := &EnvironmentSpec{
		Name: "base-only",
		Skills: map[string]PackageRef{
			"skill-b": {Source: "github:b/b", Version: "*"},
		},
	}
	merged = Merge(base, nil)
	if merged.Name != "base-only" {
		t.Errorf("Name = %q", merged.Name)
	}
	if len(merged.Skills) != 1 {
		t.Errorf("Skills len = %d", len(merged.Skills))
	}
}

func TestMerge_SixTypes(t *testing.T) {
	base := &EnvironmentSpec{
		Name:   "base",
		Skills: map[string]PackageRef{"s1": {Source: "github:a/s1", Version: "*"}},
		MCPs:   map[string]PackageRef{"m1": {Source: "npm:m1", Version: "*"}},
	}
	overlay := &EnvironmentSpec{
		Agents:  map[string]PackageRef{"a1": {Source: "github:a/a1", Version: "*"}},
		Tools:   map[string]PackageRef{"t1": {Source: "local:./t1", Version: "*"}},
		Hooks:   map[string]PackageRef{"h1": {Source: "github:a/h1", Version: "*"}},
		Prompts: map[string]PackageRef{"p1": {Source: "npm:p1", Version: "*"}},
	}

	merged := Merge(base, overlay)

	if len(merged.Skills) != 1 || len(merged.MCPs) != 1 || len(merged.Agents) != 1 ||
		len(merged.Tools) != 1 || len(merged.Hooks) != 1 || len(merged.Prompts) != 1 {
		t.Errorf("expected 1 in each type, got: skills=%d mcps=%d agents=%d tools=%d hooks=%d prompts=%d",
			len(merged.Skills), len(merged.MCPs), len(merged.Agents),
			len(merged.Tools), len(merged.Hooks), len(merged.Prompts))
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "agent.yaml")

	spec := &EnvironmentSpec{
		Name:        "test-env",
		Description: "test write",
		Skills: map[string]PackageRef{
			"skill-a": {Source: "github:a/a", Version: "*"},
		},
	}

	if err := WriteFile(filePath, spec); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Read it back
	readSpec, err := ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile after WriteFile error: %v", err)
	}

	if readSpec.Name != spec.Name {
		t.Errorf("Name = %q, want %q", readSpec.Name, spec.Name)
	}
	if len(readSpec.Skills) != 1 {
		t.Errorf("Skills len = %d, want 1", len(readSpec.Skills))
	}
}

func TestWrite_OmitEmptySections(t *testing.T) {
	spec := &EnvironmentSpec{
		Name:        "minimal",
		Description: "only name and desc",
	}
	data, err := Write(spec)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	output := string(data)
	// Should not contain empty section headers
	for _, section := range []string{"skills:", "mcps:", "agents:", "tools:", "hooks:", "prompts:"} {
		if strings.Contains(output, section) {
			t.Errorf("output contains empty section %q:\n%s", section, output)
		}
	}
}

func TestWrite_DeterministicSortedKeys(t *testing.T) {
	// Create spec with keys in non-alphabetical order
	spec := &EnvironmentSpec{
		Name:        "test",
		Description: "test",
		Skills: map[string]PackageRef{
			"z-skill": {Source: "github:a/z", Version: "*"},
			"a-skill": {Source: "github:a/a", Version: "*"},
			"m-skill": {Source: "github:a/m", Version: "*"},
		},
	}
	data, err := Write(spec)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	// Check that keys appear in alphabetical order
	output := string(data)
	aIdx := strings.Index(output, "a-skill:")
	zIdx := strings.Index(output, "z-skill:")
	mIdx := strings.Index(output, "m-skill:")
	if aIdx < 0 || zIdx < 0 || mIdx < 0 {
		t.Fatalf("keys not found in output:\n%s", output)
	}
	if !(aIdx < mIdx && mIdx < zIdx) {
		t.Errorf("keys not sorted alphabetically:\n%s", output)
	}
}

func TestMerge_NoMutation(t *testing.T) {
	base := &EnvironmentSpec{
		Name: "base",
		Skills: map[string]PackageRef{
			"skill-x": {Source: "github:a/x", Version: "*"},
		},
		MCPs: map[string]PackageRef{
			"mcp-x": {Source: "npm:mcp-x", Version: "*"},
		},
	}
	overlay := &EnvironmentSpec{
		Name: "overlay",
		Skills: map[string]PackageRef{
			"skill-y": {Source: "github:a/y", Version: "*"},
		},
	}

	// Capture original state
	origBaseSkills := len(base.Skills)
	origBaseMCPs := len(base.MCPs)
	origOverlaySkills := len(overlay.Skills)

	_ = Merge(base, overlay)

	// Verify base unchanged
	if len(base.Skills) != origBaseSkills {
		t.Fatal("base.Skills was mutated by Merge")
	}
	if len(base.MCPs) != origBaseMCPs {
		t.Fatal("base.MCPs was mutated by Merge")
	}
	if len(overlay.Skills) != origOverlaySkills {
		t.Fatal("overlay.Skills was mutated by Merge")
	}
}

func TestRead_StarsVersionDefaults(t *testing.T) {
	input := `
name: test-env
description: test
skills:
  no-version:
    source: github:some/repo
`
	spec, err := Read([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pkg := spec.Skills["no-version"]
	if pkg.Version != "*" {
		t.Errorf("Version = %q, want %q", pkg.Version, "*")
	}
}

func TestValidate_PackageNameNotKebab(t *testing.T) {
	spec := &EnvironmentSpec{
		Name:        "my-project",
		Description: "test",
		Skills: map[string]PackageRef{
			"Bad-Skill": {Source: "github:a/b", Version: "*"},
		},
	}
	result := Validate(spec)
	if result.Valid {
		t.Fatal("expected Valid=false for non-kebab package name")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "Bad-Skill") && strings.Contains(e.Message, "kebab-case") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected kebab-case error for Bad-Skill, got: %v", result.Errors)
	}
}

func TestValidate_SourceEmptyPath(t *testing.T) {
	spec := &EnvironmentSpec{
		Name:        "test-env",
		Description: "test",
		Skills: map[string]PackageRef{
			"skill": {Source: "github:", Version: "*"},
		},
	}
	result := Validate(spec)
	if result.Valid {
		t.Fatal("expected Valid=false for empty path after scheme")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "empty path") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'empty path' error, got: %v", result.Errors)
	}
}

func TestWrite_RoundTrip_DeepEquality(t *testing.T) {
	original := &EnvironmentSpec{
		Name:        "deep-env",
		Description: "deep equality test",
		Skills: map[string]PackageRef{
			"skill-a": {
				Source:  "github:owner/repo",
				Version: "~1.2.3",
				Config: map[string]interface{}{
					"timeout": 30,
					"retries": 3,
				},
			},
		},
		MCPs: map[string]PackageRef{
			"mcp-b": {
				Source:  "npm:@scope/package",
				Version: "^2.0",
				Config: map[string]interface{}{
					"transport": "stdio",
					"env": map[string]interface{}{
						"KEY": "value",
					},
				},
			},
		},
	}

	data, err := Write(original)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	parsed, err := Read(data)
	if err != nil {
		t.Fatalf("Read error: %v\nYAML output:\n%s", err, string(data))
	}

	if !reflect.DeepEqual(original, parsed) {
		t.Errorf("round-trip deep equality failed:\noriginal: %+v\nparsed:   %+v", original, parsed)
	}
}

func TestHasErrorsAndHasWarnings(t *testing.T) {
	result := &ValidationResult{Valid: true}
	if result.HasErrors() {
		t.Error("HasErrors should be false")
	}
	if result.HasWarnings() {
		t.Error("HasWarnings should be false")
	}

	result.Errors = append(result.Errors, ValidationError{Field: "test", Message: "err"})
	result.Warnings = append(result.Warnings, ValidationWarning{Field: "test", Message: "warn"})

	if !result.HasErrors() {
		t.Error("HasErrors should be true")
	}
	if !result.HasWarnings() {
		t.Error("HasWarnings should be true")
	}
}
