package parser

import (
	"fmt"
	"strconv"

	"github.com/agentenv/agentenv/pkg/types"
	"gopkg.in/yaml.v3"
)

func ParseAgentYAML(data []byte) (*types.Environment, error) {
	var raw struct {
		Name        yaml.Node `yaml:"name"`
		Description yaml.Node `yaml:"description"`
		Skills      yaml.Node `yaml:"skills"`
		MCPs        yaml.Node `yaml:"mcps"`
		Agents      yaml.Node `yaml:"agents"`
		Tools       yaml.Node `yaml:"tools"`
		Hooks       yaml.Node `yaml:"hooks"`
		Prompts     yaml.Node `yaml:"prompts"`
	}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse agent.yaml: %w", err)
	}

	env := &types.Environment{}

	if raw.Name.Kind == 0 || raw.Name.Value == "" {
		return env, fmt.Errorf("required field 'name' is missing")
	}
	env.Name = raw.Name.Value

	if raw.Description.Kind != 0 {
		env.Description = raw.Description.Value
	}

	var err error
	env.Skills, err = decodePkgMap(raw.Skills)
	if err != nil {
		return env, fmt.Errorf("skills: %w", err)
	}
	env.MCPs, err = decodePkgMap(raw.MCPs)
	if err != nil {
		return env, fmt.Errorf("mcps: %w", err)
	}
	env.Agents, err = decodePkgMap(raw.Agents)
	if err != nil {
		return env, fmt.Errorf("agents: %w", err)
	}
	env.Tools, err = decodePkgMap(raw.Tools)
	if err != nil {
		return env, fmt.Errorf("tools: %w", err)
	}
	env.Hooks, err = decodePkgMap(raw.Hooks)
	if err != nil {
		return env, fmt.Errorf("hooks: %w", err)
	}
	env.Prompts, err = decodePkgMap(raw.Prompts)
	if err != nil {
		return env, fmt.Errorf("prompts: %w", err)
	}

	return env, nil
}

func decodePkgMap(node yaml.Node) (map[string]types.PackageSpec, error) {
	if node.Kind == 0 {
		return nil, nil
	}
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: expected a mapping, got %s", node.Line, kindName(node.Kind))
	}

	result := make(map[string]types.PackageSpec, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		pkg, err := decodePkgSpec(*valNode)
		if err != nil {
			return nil, fmt.Errorf("line %d: package %q: %w", valNode.Line, keyNode.Value, err)
		}
		if pkg.Version == "" {
			pkg.Version = "*"
		}
		result[keyNode.Value] = *pkg
	}
	return result, nil
}

func decodePkgSpec(node yaml.Node) (*types.PackageSpec, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected a mapping, got %s", kindName(node.Kind))
	}

	pkg := &types.PackageSpec{Version: "*"}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]

		switch key {
		case "source":
			pkg.Source = val.Value
		case "version":
			pkg.Version = val.Value
		case "config":
			cfg, err := decodeConfig(*val)
			if err != nil {
				return nil, fmt.Errorf("line %d: config: %w", val.Line, err)
			}
			pkg.Config = cfg
		}
	}
	return pkg, nil
}

func decodeConfig(node yaml.Node) (map[string]interface{}, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected a mapping, got %s", kindName(node.Kind))
	}

	result := make(map[string]interface{}, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]

		v, err := nodeToInterface(*val)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		result[key] = v
	}
	return result, nil
}

func nodeToInterface(node yaml.Node) (interface{}, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		return decodeScalar(node)
	case yaml.MappingNode:
		return decodeConfig(node)
	case yaml.SequenceNode:
		var arr []interface{}
		for _, item := range node.Content {
			v, err := nodeToInterface(*item)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("unsupported YAML node kind: %s", kindName(node.Kind))
	}
}

func decodeScalar(node yaml.Node) (interface{}, error) {
	switch node.Tag {
	case "!!null":
		return nil, nil
	case "!!bool":
		return node.Value == "true" || node.Value == "yes" || node.Value == "on", nil
	case "!!int":
		return strconv.Atoi(node.Value)
	case "!!float":
		return strconv.ParseFloat(node.Value, 64)
	default:
		return node.Value, nil
	}
}

func kindName(kind yaml.Kind) string {
	switch kind {
	case yaml.DocumentNode:
		return "document"
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return fmt.Sprintf("unknown(%d)", kind)
	}
}
