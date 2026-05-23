package types

// AgentSpec describes a single agent configuration.
type AgentSpec struct {
	Name         string                 `json:"name" yaml:"name"`
	Version      string                 `json:"version" yaml:"version"`
	Description  string                 `json:"description" yaml:"description"`
	Source       SourceURL              `json:"source" yaml:"source"`
	Dependencies []PackageDependency    `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}
