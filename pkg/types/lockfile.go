package types

// Lockfile records the exact resolved state of an environment.
type Lockfile struct {
	Version     int                 `json:"version" yaml:"version"`
	Generated   string              `json:"generated" yaml:"generated"` // ISO 8601 timestamp
	Packages    []LockedPackage     `json:"packages" yaml:"packages"`
	Environment LockfileEnvSnapshot `json:"environment_snapshot" yaml:"environment_snapshot"`
}

// LockedPackage records a single resolved package in the lockfile.
type LockedPackage struct {
	Name         string      `json:"name" yaml:"name"`
	Type         PackageType `json:"type" yaml:"type"`
	Source       string      `json:"source" yaml:"source"`
	Version      string      `json:"version" yaml:"version"`
	Resolved     string      `json:"resolved" yaml:"resolved"`
	SHA256       string      `json:"sha256,omitempty" yaml:"sha256,omitempty"`
	Dependencies []LockedDep `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	InstalledAt  string      `json:"installed_at,omitempty" yaml:"installed_at,omitempty"`
	ResolvedBy   string      `json:"resolved_by,omitempty" yaml:"resolved_by,omitempty"`
}

// LockedDep records a resolved dependency in the lockfile.
type LockedDep struct {
	Name     string      `json:"name" yaml:"name"`
	Type     PackageType `json:"type,omitempty" yaml:"type,omitempty"`
	Version  string      `json:"version" yaml:"version"`
	Resolved string      `json:"resolved" yaml:"resolved"`
	Source   string      `json:"source,omitempty" yaml:"source,omitempty"`
}

// LockfileEnvSnapshot captures the package counts at lock time.
type LockfileEnvSnapshot struct {
	SkillsCount  int `json:"skills_count" yaml:"skills_count"`
	MCPsCount    int `json:"mcps_count" yaml:"mcps_count"`
	AgentsCount  int `json:"agents_count" yaml:"agents_count"`
	ToolsCount   int `json:"tools_count" yaml:"tools_count"`
	HooksCount   int `json:"hooks_count" yaml:"hooks_count"`
	PromptsCount int `json:"prompts_count" yaml:"prompts_count"`
}
