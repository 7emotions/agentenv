// Package envfile reads, validates, and writes agent.yaml environment files.
package envfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/7emotions/agentenv/pkg/parser"
	"github.com/7emotions/agentenv/pkg/types"
	"gopkg.in/yaml.v3"
)

// knownTopLevelKeys is the set of recognized top-level keys in agent.yaml.
var knownTopLevelKeys = map[string]bool{
	"name":        true,
	"description": true,
	"skills":      true,
	"mcps":        true,
	"agents":      true,
	"tools":       true,
	"hooks":       true,
	"prompts":     true,
}

// EnvironmentSpec is the validated representation of an agent.yaml file.
type EnvironmentSpec struct {
	Name        string                `json:"name" yaml:"name"`
	Description string                `json:"description" yaml:"description"`
	Skills      map[string]PackageRef `json:"skills,omitempty" yaml:"skills,omitempty"`
	MCPs        map[string]PackageRef `json:"mcps,omitempty" yaml:"mcps,omitempty"`
	Agents      map[string]PackageRef `json:"agents,omitempty" yaml:"agents,omitempty"`
	Tools       map[string]PackageRef `json:"tools,omitempty" yaml:"tools,omitempty"`
	Hooks       map[string]PackageRef `json:"hooks,omitempty" yaml:"hooks,omitempty"`
	Prompts     map[string]PackageRef `json:"prompts,omitempty" yaml:"prompts,omitempty"`
}

// PackageRef is a lightweight reference to a package within an environment spec.
type PackageRef struct {
	Source  string                 `json:"source" yaml:"source"`
	Version string                 `json:"version,omitempty" yaml:"version,omitempty"`
	Config  map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

// ReadFile reads and validates an agent.yaml file at the given path.
// Local source paths (local:./...) are resolved relative to the file's directory.
func ReadFile(path string) (*EnvironmentSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	spec, warnKeys, err := parseAndValidate(data)
	if err != nil {
		// Return partial spec if available alongside the error
		return spec, err
	}

	// Resolve local: paths relative to the file's directory
	dir := filepath.Dir(path)
	resolveLocalPaths(spec, dir)

	// Collect unknown-key warnings after resolution (so they show on final spec)
	result := Validate(spec)
	for _, k := range warnKeys {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   k,
			Message: fmt.Sprintf("unknown top-level key %q", k),
		})
	}
	sort.Slice(result.Warnings, func(i, j int) bool {
		return result.Warnings[i].Field < result.Warnings[j].Field
	})

	if !result.Valid {
		return spec, formatValidationErrors(result)
	}

	return spec, nil
}

// Read reads and validates agent.yaml from raw bytes.
func Read(data []byte) (*EnvironmentSpec, error) {
	spec, warnKeys, err := parseAndValidate(data)
	if err != nil {
		return spec, err
	}

	result := Validate(spec)
	for _, k := range warnKeys {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   k,
			Message: fmt.Sprintf("unknown top-level key %q", k),
		})
	}
	sort.Slice(result.Warnings, func(i, j int) bool {
		return result.Warnings[i].Field < result.Warnings[j].Field
	})

	if !result.Valid {
		return spec, formatValidationErrors(result)
	}

	return spec, nil
}

// parseAndValidate performs YAML parsing, unknown-key detection, and type conversion.
// Returns the spec, any unknown top-level keys (for warnings), and any parse/convert error.
func parseAndValidate(data []byte) (*EnvironmentSpec, []string, error) {
	// First pass: detect unknown top-level keys using yaml.Node
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("failed to parse agent.yaml: %w", err)
	}

	var unknownKeys []string
	for k := range raw {
		if !knownTopLevelKeys[k] {
			unknownKeys = append(unknownKeys, k)
		}
	}
	sort.Strings(unknownKeys)

	// Second pass: use existing parser for typed result
	env, err := parser.ParseAgentYAML(data)
	if err != nil {
		return nil, unknownKeys, err
	}

	// Convert types.Environment -> EnvironmentSpec
	spec := convertToSpec(env)

	return spec, unknownKeys, nil
}

// convertToSpec converts a types.Environment to an EnvironmentSpec.
func convertToSpec(env *types.Environment) *EnvironmentSpec {
	spec := &EnvironmentSpec{
		Name:        env.Name,
		Description: env.Description,
	}
	if env.Skills != nil {
		spec.Skills = convertPkgMap(env.Skills)
	}
	if env.MCPs != nil {
		spec.MCPs = convertPkgMap(env.MCPs)
	}
	if env.Agents != nil {
		spec.Agents = convertPkgMap(env.Agents)
	}
	if env.Tools != nil {
		spec.Tools = convertPkgMap(env.Tools)
	}
	if env.Hooks != nil {
		spec.Hooks = convertPkgMap(env.Hooks)
	}
	if env.Prompts != nil {
		spec.Prompts = convertPkgMap(env.Prompts)
	}
	return spec
}

// convertPkgMap converts a map of types.PackageSpec to PackageRef.
func convertPkgMap(in map[string]types.PackageSpec) map[string]PackageRef {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]PackageRef, len(in))
	for k, v := range in {
		out[k] = PackageRef{
			Source:  v.Source,
			Version: v.Version,
			Config:  v.Config,
		}
	}
	return out
}

// resolveLocalPaths resolves local:./... and local:../... paths in the spec
// relative to baseDir. Absolute and other scheme paths are left unchanged.
func resolveLocalPaths(spec *EnvironmentSpec, baseDir string) {
	resolvePkgMap(spec.Skills, baseDir)
	resolvePkgMap(spec.MCPs, baseDir)
	resolvePkgMap(spec.Agents, baseDir)
	resolvePkgMap(spec.Tools, baseDir)
	resolvePkgMap(spec.Hooks, baseDir)
	resolvePkgMap(spec.Prompts, baseDir)
}

// resolvePkgMap resolves local: relative paths in a single package map.
func resolvePkgMap(pkgs map[string]PackageRef, baseDir string) {
	for k, pkg := range pkgs {
		if isRelativeLocalPath(pkg.Source) {
			relPath := pkg.Source[6:] // strip "local:"
			absPath := filepath.Join(baseDir, relPath)
			pkg.Source = "local:" + filepath.Clean(absPath)
			pkgs[k] = pkg
		}
	}
}

// isRelativeLocalPath returns true if the source starts with "local:./" or "local:../".
func isRelativeLocalPath(source string) bool {
	return strings.HasPrefix(source, "local:./") || strings.HasPrefix(source, "local:../")
}

// formatValidationErrors formats errors from a ValidationResult into a single error.
func formatValidationErrors(result *ValidationResult) error {
	var sb strings.Builder
	sb.WriteString("validation failed:")
	for _, e := range result.Errors {
		sb.WriteString(fmt.Sprintf("\n  - %s: %s", e.Field, e.Message))
	}
	return fmt.Errorf("%s", sb.String())
}
