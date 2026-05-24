package lockfile

import (
	"sort"
	"time"

	"github.com/7emotions/agentenv/pkg/resolver"
	"github.com/7emotions/agentenv/pkg/types"
)

// Generate creates a Lockfile from a list of resolved packages.
// Packages are sorted deterministically by (type, name).
// Dependencies within each package are sorted by name.
// An EnvironmentSnapshot is populated with counts per package type.
func Generate(packages []resolver.ResolvedPackage) (*types.Lockfile, error) {
	sorted := make([]resolver.ResolvedPackage, len(packages))
	copy(sorted, packages)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Type != sorted[j].Type {
			return sorted[i].Type < sorted[j].Type
		}
		return sorted[i].Name < sorted[j].Name
	})

	locked := make([]types.LockedPackage, len(sorted))
	for i, pkg := range sorted {
		deps := make([]types.LockedDep, len(pkg.Dependencies))
		for j, dep := range pkg.Dependencies {
			deps[j] = types.LockedDep{
				Name:     dep.Name,
				Type:     types.PackageType(dep.Type),
				Version:  dep.Version,
				Resolved: dep.Resolved,
				Source:   dep.Source,
			}
		}
		sort.Slice(deps, func(a, b int) bool {
			return deps[a].Name < deps[b].Name
		})

		locked[i] = types.LockedPackage{
			Name:         pkg.Name,
			Type:         types.PackageType(pkg.Type),
			Source:       pkg.Source,
			Version:      pkg.Version,
			Resolved:     pkg.Resolved,
			SHA256:       pkg.SHA256,
			Dependencies: deps,
			ResolvedBy:   pkg.ResolvedBy,
		}
	}

	lf := &types.Lockfile{
		Version:     3,
		Generated:   time.Now().UTC().Format(time.RFC3339),
		Packages:    locked,
		Environment: countByType(sorted),
	}

	return lf, nil
}

func countByType(packages []resolver.ResolvedPackage) types.LockfileEnvSnapshot {
	var snap types.LockfileEnvSnapshot
	for _, pkg := range packages {
		switch types.PackageType(pkg.Type) {
		case types.PackageTypeSkill:
			snap.SkillsCount++
		case types.PackageTypeMCP:
			snap.MCPsCount++
		case types.PackageTypeAgent:
			snap.AgentsCount++
		case types.PackageTypeTool:
			snap.ToolsCount++
		case types.PackageTypeHook:
			snap.HooksCount++
		case types.PackageTypePrompt:
			snap.PromptsCount++
		}
	}
	return snap
}
