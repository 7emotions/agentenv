package parser

import (
	"fmt"
	"strconv"

	"github.com/agentenv/agentenv/pkg/types"
	"gopkg.in/yaml.v3"
)

func ParseLockfile(data []byte) (*types.Lockfile, error) {
	var raw struct {
		Version     yaml.Node `yaml:"version"`
		Generated   yaml.Node `yaml:"generated"`
		Packages    yaml.Node `yaml:"packages"`
		Environment yaml.Node `yaml:"environment_snapshot"`
	}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse agent.lock: %w", err)
	}

	lf := &types.Lockfile{}

	if raw.Version.Value == "" {
		return lf, fmt.Errorf("required field 'version' is missing")
	}
	v, err := strconv.Atoi(raw.Version.Value)
	if err != nil {
		return lf, fmt.Errorf("line %d: invalid version %q: must be an integer", raw.Version.Line, raw.Version.Value)
	}
	lf.Version = v

	if raw.Generated.Value == "" {
		return lf, fmt.Errorf("required field 'generated' is missing")
	}
	lf.Generated = raw.Generated.Value

	pkgs, err := decodeLockedPackages(raw.Packages)
	if err != nil {
		return lf, fmt.Errorf("packages: %w", err)
	}
	lf.Packages = pkgs

	if raw.Environment.Kind != 0 {
		envSnap, err := decodeEnvSnapshot(raw.Environment)
		if err != nil {
			return lf, fmt.Errorf("environment_snapshot: %w", err)
		}
		lf.Environment = *envSnap
	}

	return lf, nil
}

func decodeLockedPackages(node yaml.Node) ([]types.LockedPackage, error) {
	if node.Kind == 0 {
		return nil, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("line %d: expected a sequence, got %s", node.Line, kindName(node.Kind))
	}

	var result []types.LockedPackage
	for _, item := range node.Content {
		pkg, err := decodeLockedPackage(*item)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", item.Line, err)
		}
		result = append(result, *pkg)
	}
	return result, nil
}

func decodeLockedPackage(node yaml.Node) (*types.LockedPackage, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected a mapping, got %s", kindName(node.Kind))
	}

	pkg := &types.LockedPackage{}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]

		switch key {
		case "name":
			pkg.Name = val.Value
		case "type":
			pkg.Type = types.PackageType(val.Value)
		case "version":
			pkg.Version = val.Value
		case "source":
			pkg.Source = val.Value
		case "resolved":
			pkg.Resolved = val.Value
		case "sha256":
			pkg.SHA256 = val.Value
		case "dependencies":
			deps, err := decodeLockedDeps(*val)
			if err != nil {
				return nil, fmt.Errorf("dependencies: %w", err)
			}
			pkg.Dependencies = deps
		case "installed_at":
			pkg.InstalledAt = val.Value
		}
	}
	return pkg, nil
}

func decodeLockedDeps(node yaml.Node) ([]types.LockedDep, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("line %d: expected a sequence, got %s", node.Line, kindName(node.Kind))
	}
	var result []types.LockedDep
	for _, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("line %d: expected a mapping in dependencies, got %s", item.Line, kindName(item.Kind))
		}
		dep := types.LockedDep{}
		for j := 0; j < len(item.Content); j += 2 {
			switch item.Content[j].Value {
			case "name":
				dep.Name = item.Content[j+1].Value
			case "version":
				dep.Version = item.Content[j+1].Value
			case "resolved":
				dep.Resolved = item.Content[j+1].Value
			}
		}
		result = append(result, dep)
	}
	return result, nil
}

func decodeEnvSnapshot(node yaml.Node) (*types.LockfileEnvSnapshot, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: expected a mapping, got %s", node.Line, kindName(node.Kind))
	}

	snap := &types.LockfileEnvSnapshot{}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1].Value
		n, _ := strconv.Atoi(val)
		switch key {
		case "skills_count":
			snap.SkillsCount = n
		case "mcps_count":
			snap.MCPsCount = n
		case "agents_count":
			snap.AgentsCount = n
		case "tools_count":
			snap.ToolsCount = n
		case "hooks_count":
			snap.HooksCount = n
		case "prompts_count":
			snap.PromptsCount = n
		}
	}
	return snap, nil
}

func WriteLockfile(lf *types.Lockfile) ([]byte, error) {
	return yaml.Marshal(lf)
}
