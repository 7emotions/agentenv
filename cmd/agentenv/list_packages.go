package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	agentenvError "github.com/7emotions/agentenv/pkg/errors"
	"github.com/7emotions/agentenv/pkg/envfile"
	"github.com/spf13/cobra"
)

var (
	listPkgType string
	listPkgTree bool
)

type packageEntry struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Source  string `json:"source,omitempty"`
	Version string `json:"version,omitempty"`
}

var listPackagesCmd = &cobra.Command{
	Use:   "list-packages [name]",
	Short: "List packages in an environment",
	Long: `List all packages (skills, MCPs, agents, tools, hooks, prompts)
defined in an environment's agent.yaml.

If name is omitted, the currently active environment is used.
Use --type to filter by package type.
Use --tree to show a hierarchical view grouped by type.
Use --json for machine-readable output.

Package types: skill, mcp, agent, tool, hook, prompt`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := agentenvRoot()
		if err != nil {
			return err
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			activeLock := filepath.Join(root, "ACTIVE")
			data, err := os.ReadFile(activeLock)
			if err != nil {
				if os.IsNotExist(err) {
					return agentenvError.UserError(
						"No environment specified and no active environment found",
						"Specify an environment name, or activate one first.")
				}
				return agentenvError.SystemError(
					"Cannot read active lock",
					"Check file permissions in ~/.agentenv/.").WithCause(err)
			}
		name = parseActiveName(string(data))
		if name == "" {
				return agentenvError.UserError(
					"No environment specified and no active environment found",
					"Specify an environment name, or activate one first.")
			}
		}

		envDir := filepath.Join(root, "envs", name)
		if _, err := os.Stat(envDir); os.IsNotExist(err) {
			return agentenvError.UserError(
				fmt.Sprintf("Environment %q not found", name),
				"Check the name with 'agentenv list'.")
		}

		yamlPath := filepath.Join(envDir, "agent.yaml")
		data, err := os.ReadFile(yamlPath)
		if err != nil {
			if os.IsNotExist(err) {
				return agentenvError.UserError(
					fmt.Sprintf("Environment %q has no agent.yaml", name),
					"Recreate the environment or add packages with 'agentenv add'.")
			}
			return agentenvError.SystemError(
				fmt.Sprintf("Cannot read agent.yaml for %q", name),
				"Check file permissions.").WithCause(err)
		}

		spec, err := envfile.Read(data)
		if err != nil && spec == nil {
			return agentenvError.UserError(
				fmt.Sprintf("Cannot parse agent.yaml for %q", name),
				"Check that the file is valid YAML.").WithCause(err)
		}

		groups := make(map[string][]packageEntry)
		addPkgs := func(typeName string, pkgs map[string]envfile.PackageRef) {
			if listPkgType != "" && !strings.EqualFold(typeName, listPkgType) {
				return
			}
			var entries []packageEntry
			keys := make([]string, 0, len(pkgs))
			for k := range pkgs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				p := pkgs[k]
				entries = append(entries, packageEntry{
					Name:    k,
					Type:    typeName,
					Source:  p.Source,
					Version: p.Version,
				})
			}
			groups[typeName] = entries
		}

		if spec.Skills != nil {
			addPkgs("skill", spec.Skills)
		}
		if spec.MCPs != nil {
			addPkgs("mcp", spec.MCPs)
		}
		if spec.Agents != nil {
			addPkgs("agent", spec.Agents)
		}
		if spec.Tools != nil {
			addPkgs("tool", spec.Tools)
		}
		if spec.Hooks != nil {
			addPkgs("hook", spec.Hooks)
		}
		if spec.Prompts != nil {
			addPkgs("prompt", spec.Prompts)
		}

		if jsonOutput {
			var all []packageEntry
			typeOrder := []string{"skill", "mcp", "agent", "tool", "hook", "prompt"}
			for _, t := range typeOrder {
				if entries, ok := groups[t]; ok {
					all = append(all, entries...)
				}
			}
			writeJSON(cmd, all)
			return nil
		}

		if len(groups) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No packages defined.")
			return nil
		}

		out := cmd.OutOrStdout()
		if listPkgTree {
			typeOrder := []string{"skill", "mcp", "agent", "tool", "hook", "prompt"}
			for _, t := range typeOrder {
				entries, ok := groups[t]
				if !ok {
					continue
				}
				fmt.Fprintf(out, "%s:\n", t)
				for _, p := range entries {
					sourceInfo := ""
					if p.Source != "" {
						sourceInfo = fmt.Sprintf(" (%s)", p.Source)
					}
					fmt.Fprintf(out, "  %s%s\n", p.Name, sourceInfo)
				}
			}
		} else {
			typeOrder := []string{"skill", "mcp", "agent", "tool", "hook", "prompt"}
			for _, t := range typeOrder {
				entries, ok := groups[t]
				if !ok {
					continue
				}
				for _, p := range entries {
					sourceInfo := ""
					if p.Source != "" {
						sourceInfo = fmt.Sprintf(" (%s)", p.Source)
					}
					fmt.Fprintf(out, "%-16s %s%s\n", t, p.Name, sourceInfo)
				}
			}
		}

		return nil
	},
}

func init() {
	listPackagesCmd.Flags().StringVar(&listPkgType, "type", "", "Filter by package type (skill, mcp, agent, tool, hook, prompt)")
	listPackagesCmd.Flags().BoolVar(&listPkgTree, "tree", false, "Show hierarchical view grouped by type")
	rootCmd.AddCommand(listPackagesCmd)
}
