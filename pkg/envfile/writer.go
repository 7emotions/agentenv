package envfile

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// packageTypeOrder defines the canonical order of package-type sections in the YAML output.
var packageTypeOrder = []string{"skills", "mcps", "agents", "tools", "hooks", "prompts"}

// Write serializes an EnvironmentSpec to deterministic YAML bytes.
// Maps are sorted alphabetically and empty sections are omitted.
func Write(spec *EnvironmentSpec) ([]byte, error) {
	if spec == nil {
		return nil, fmt.Errorf("cannot write nil spec")
	}

	doc := &yaml.Node{Kind: yaml.DocumentNode}
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	doc.Content = []*yaml.Node{root}

	// name (required, always first)
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "name", Tag: "!!str"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: spec.Name, Tag: "!!str"},
	)

	// description (included even if empty for round-trip fidelity)
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "description", Tag: "!!str"},
		scalarNode(spec.Description),
	)

	// Package type sections in canonical order, sorted by key
	sections := map[string]map[string]PackageRef{
		"skills":  spec.Skills,
		"mcps":    spec.MCPs,
		"agents":  spec.Agents,
		"tools":   spec.Tools,
		"hooks":   spec.Hooks,
		"prompts": spec.Prompts,
	}

	for _, section := range packageTypeOrder {
		pkgs := sections[section]
		if len(pkgs) == 0 {
			continue
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: section, Tag: "!!str"},
			encodePkgMap(pkgs),
		)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("failed to marshal yaml: %w", err)
	}
	enc.Close()

	return buf.Bytes(), nil
}

// WriteFile writes an EnvironmentSpec to the given path as agent.yaml.
func WriteFile(path string, spec *EnvironmentSpec) error {
	data, err := Write(spec)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Merge adds packages from overlay to base, returning a new EnvironmentSpec.
// Overlay entries take precedence over base entries with the same name.
// Neither input is modified.
func Merge(base, overlay *EnvironmentSpec) *EnvironmentSpec {
	if base == nil {
		if overlay == nil {
			return &EnvironmentSpec{}
		}
		cp := *overlay
		cp.Skills = copyPkgMap(overlay.Skills)
		cp.MCPs = copyPkgMap(overlay.MCPs)
		cp.Agents = copyPkgMap(overlay.Agents)
		cp.Tools = copyPkgMap(overlay.Tools)
		cp.Hooks = copyPkgMap(overlay.Hooks)
		cp.Prompts = copyPkgMap(overlay.Prompts)
		return &cp
	}

	if overlay == nil {
		cp := *base
		cp.Skills = copyPkgMap(base.Skills)
		cp.MCPs = copyPkgMap(base.MCPs)
		cp.Agents = copyPkgMap(base.Agents)
		cp.Tools = copyPkgMap(base.Tools)
		cp.Hooks = copyPkgMap(base.Hooks)
		cp.Prompts = copyPkgMap(base.Prompts)
		return &cp
	}

	result := &EnvironmentSpec{
		Name:        overlay.Name,
		Description: overlay.Description,
	}

	mergePkgMap := func(baseMap, overlayMap map[string]PackageRef) map[string]PackageRef {
		out := copyPkgMap(baseMap)
		if out == nil {
			out = make(map[string]PackageRef)
		}
		for k, v := range overlayMap {
			out[k] = v
		}
		return out
	}

	if len(base.Skills) > 0 || len(overlay.Skills) > 0 {
		result.Skills = mergePkgMap(base.Skills, overlay.Skills)
	}
	if len(base.MCPs) > 0 || len(overlay.MCPs) > 0 {
		result.MCPs = mergePkgMap(base.MCPs, overlay.MCPs)
	}
	if len(base.Agents) > 0 || len(overlay.Agents) > 0 {
		result.Agents = mergePkgMap(base.Agents, overlay.Agents)
	}
	if len(base.Tools) > 0 || len(overlay.Tools) > 0 {
		result.Tools = mergePkgMap(base.Tools, overlay.Tools)
	}
	if len(base.Hooks) > 0 || len(overlay.Hooks) > 0 {
		result.Hooks = mergePkgMap(base.Hooks, overlay.Hooks)
	}
	if len(base.Prompts) > 0 || len(overlay.Prompts) > 0 {
		result.Prompts = mergePkgMap(base.Prompts, overlay.Prompts)
	}

	// If overlay has empty Name (shouldn't happen), use base
	if result.Name == "" {
		result.Name = base.Name
	}
	if result.Description == "" {
		result.Description = base.Description
	}

	return result
}

// encodePkgMap encodes a map of PackageRef into a sorted YAML mapping node.
func encodePkgMap(pkgs map[string]PackageRef) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(pkgs))
	for k := range pkgs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		pkg := pkgs[key]
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
			encodePkgRef(pkg),
		)
	}

	return node
}

// encodePkgRef encodes a single PackageRef into a YAML mapping node.
func encodePkgRef(pkg PackageRef) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	// source (required)
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "source", Tag: "!!str"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: pkg.Source, Tag: "!!str"},
	)

	// version (omit if empty or "*" — the default)
	if pkg.Version != "" && pkg.Version != "*" {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "version", Tag: "!!str"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: pkg.Version, Tag: "!!str"},
		)
	}

	// config (omit if empty)
	if len(pkg.Config) > 0 {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "config", Tag: "!!str"},
			encodeConfig(pkg.Config),
		)
	}

	return node
}

// encodeConfig encodes a map[string]interface{} into a sorted YAML mapping node.
func encodeConfig(cfg map[string]interface{}) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		val := cfg[key]
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
			encodeValue(val),
		)
	}

	return node
}

// encodeValue encodes an arbitrary Go value into a YAML node.
func encodeValue(v interface{}) *yaml.Node {
	switch val := v.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: fmt.Sprintf("%v", val)}
	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprintf("%d", val)}
	case float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: fmt.Sprintf("%v", val)}
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: val}
	case []interface{}:
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, item := range val {
			seq.Content = append(seq.Content, encodeValue(item))
		}
		return seq
	case map[string]interface{}:
		return encodeConfig(val)
	default:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: fmt.Sprintf("%v", val)}
	}
}

// scalarNode creates a scalar YAML node, using !!null for empty strings.
func scalarNode(value string) *yaml.Node {
	if value == "" {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Value: value, Tag: "!!str"}
}

// copyPkgMap makes a shallow copy of a package map.
func copyPkgMap(in map[string]PackageRef) map[string]PackageRef {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]PackageRef, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
