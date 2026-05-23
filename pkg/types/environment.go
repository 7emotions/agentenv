package types

// Environment defines a named set of packages (skills, MCPs, agents, etc.)
// that together form a complete agent runtime environment.
type Environment struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Skills      map[string]PackageSpec `json:"skills,omitempty" yaml:"skills,omitempty"`
	MCPs        map[string]PackageSpec `json:"mcps,omitempty" yaml:"mcps,omitempty"`
	Agents      map[string]PackageSpec `json:"agents,omitempty" yaml:"agents,omitempty"`
	Tools       map[string]PackageSpec `json:"tools,omitempty" yaml:"tools,omitempty"`
	Hooks       map[string]PackageSpec `json:"hooks,omitempty" yaml:"hooks,omitempty"`
	Prompts     map[string]PackageSpec `json:"prompts,omitempty" yaml:"prompts,omitempty"`
}

// PackageSpec is a lightweight reference to a package within an environment.
type PackageSpec struct {
	Source  string                 `json:"source" yaml:"source"`
	Version string                 `json:"version,omitempty" yaml:"version,omitempty"`
	Config  map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}
