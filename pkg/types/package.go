package types

// PackageType enumerates the kinds of packages agentenv manages.
type PackageType string

const (
	// PackageTypeSkill is an installable skill.
	PackageTypeSkill PackageType = "skill"
	// PackageTypeMCP is an MCP server configuration.
	PackageTypeMCP PackageType = "mcp"
	// PackageTypeAgent is an agent specification.
	PackageTypeAgent PackageType = "agent"
	// PackageTypeTool is a tool package.
	PackageTypeTool PackageType = "tool"
	// PackageTypeHook is a lifecycle hook.
	PackageTypeHook PackageType = "hook"
	// PackageTypePrompt is a prompt template package.
	PackageTypePrompt PackageType = "prompt"
)

// Package describes a single installable package in agentenv.
type Package struct {
	Name         string                 `json:"name" yaml:"name"`
	Type         PackageType            `json:"type" yaml:"type"`
	Version      string                 `json:"version" yaml:"version"`
	Source       SourceURL              `json:"source" yaml:"source"`
	SHA256       string                 `json:"sha256,omitempty" yaml:"sha256,omitempty"`
	Dependencies []PackageDependency    `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	RuntimeReqs  []RuntimeRequirement   `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

// PackageDependency describes a dependency of a package.
type PackageDependency struct {
	Name       string      `json:"name" yaml:"name"`
	Type       PackageType `json:"type" yaml:"type"`
	Constraint string      `json:"constraint" yaml:"constraint"` // semver constraint
	Source     string      `json:"source,omitempty" yaml:"source,omitempty"`
}

// RuntimeRequirement describes a runtime prerequisite for a package.
type RuntimeRequirement struct {
	Type        string `json:"type" yaml:"type"`                   // "binary", "env"
	Name        string `json:"name" yaml:"name"`
	MinVersion  string `json:"min_version,omitempty" yaml:"min_version,omitempty"`
	Optional    bool   `json:"optional,omitempty" yaml:"optional,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}
