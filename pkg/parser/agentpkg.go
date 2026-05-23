package parser

import (
	"fmt"

	"github.com/7emotions/agentenv/pkg/types"
	"gopkg.in/yaml.v3"
)

func ParseAgentPkg(data []byte) (*types.AgentSpec, error) {
	var raw struct {
		Name         yaml.Node `yaml:"name"`
		Version      yaml.Node `yaml:"version"`
		Description  yaml.Node `yaml:"description"`
		Source       yaml.Node `yaml:"source"`
		Dependencies yaml.Node `yaml:"dependencies"`
		Config       yaml.Node `yaml:"config"`
	}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse agentpkg.yaml: %w", err)
	}

	spec := &types.AgentSpec{}

	if raw.Name.Value == "" {
		return spec, fmt.Errorf("required field 'name' is missing")
	}
	spec.Name = raw.Name.Value

	if raw.Version.Value == "" {
		return spec, fmt.Errorf("required field 'version' is missing")
	}
	spec.Version = raw.Version.Value

	spec.Description = raw.Description.Value

	if raw.Source.Kind != 0 && raw.Source.Value != "" {
		src, err := types.ParseSourceURL(raw.Source.Value)
		if err != nil {
			return spec, fmt.Errorf("source: line %d: %w", raw.Source.Line, err)
		}
		spec.Source = src
	}

	if raw.Dependencies.Kind != 0 {
		deps, err := decodePkgDeps(raw.Dependencies)
		if err != nil {
			return spec, fmt.Errorf("dependencies: %w", err)
		}
		spec.Dependencies = deps
	}

	if raw.Config.Kind != 0 {
		cfg, err := decodeConfig(raw.Config)
		if err != nil {
			return spec, fmt.Errorf("config: %w", err)
		}
		spec.Config = cfg
	}

	return spec, nil
}

func decodePkgDeps(node yaml.Node) ([]types.PackageDependency, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("line %d: expected a sequence, got %s", node.Line, kindName(node.Kind))
	}
	var result []types.PackageDependency
	for _, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("line %d: expected a mapping, got %s", item.Line, kindName(item.Kind))
		}
		dep := types.PackageDependency{}
		for j := 0; j < len(item.Content); j += 2 {
			switch item.Content[j].Value {
			case "name":
				dep.Name = item.Content[j+1].Value
			case "type":
				dep.Type = types.PackageType(item.Content[j+1].Value)
			case "constraint":
				dep.Constraint = item.Content[j+1].Value
			case "source":
				dep.Source = item.Content[j+1].Value
			}
		}
		if dep.Source != "" {
			if _, err := types.ParseSourceURL(dep.Source); err != nil {
				return nil, fmt.Errorf("line %d: invalid source %q: %w", item.Line, dep.Source, err)
			}
		}
		result = append(result, dep)
	}
	return result, nil
}
