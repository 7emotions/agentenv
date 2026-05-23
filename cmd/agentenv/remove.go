package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	agentenvError "github.com/7emotions/agentenv/pkg/errors"
	"github.com/7emotions/agentenv/pkg/envfile"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [<type>] <name>",
	Short: "Remove a package from the active environment",
	Long: `Remove a package from the currently active environment's agent.yaml.

If <type> is omitted, all sections are searched. If the package name is found
in more than one section, you must specify the type to disambiguate.

Examples:
  agentenv remove my-skill
  agentenv remove skill my-skill`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var pkgType, name string
		if len(args) == 2 {
			pkgType = args[0]
			name = args[1]

			if _, ok := typeToSection[pkgType]; !ok {
				return agentenvError.UserError(
					fmt.Sprintf("Invalid type %q", pkgType),
					"Supported types: skill, mcp, agent, tool, hook, prompt")
			}
		} else {
			name = args[0]
		}

		root, err := agentenvRoot()
		if err != nil {
			return err
		}

		activeLock := filepath.Join(root, "ACTIVE")
		activeData, err := os.ReadFile(activeLock)
		if err != nil {
			return agentenvError.UserError(
				"No active environment",
				"Activate one first with 'agentenv activate <name>'")
		}
		envName := parseActiveName(string(activeData))
		if envName == "" {
			return agentenvError.UserError(
				"No active environment",
				"Activate one first with 'agentenv activate <name>'")
		}

		envDir := filepath.Join(root, "envs", envName)
		yamlPath := filepath.Join(envDir, "agent.yaml")

		if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
			return agentenvError.UserError(
				fmt.Sprintf("agent.yaml not found for environment %q", envName),
				"The environment may be incomplete. Try recreating it.")
		}

		spec, err := envfile.ReadFile(yamlPath)
		if err != nil {
			return agentenvError.SystemError(
				fmt.Sprintf("Cannot read agent.yaml for %q", envName),
				"Check that the file is valid YAML.").WithCause(err)
		}

		if pkgType != "" {
			sectionKey := typeToSection[pkgType]
			if !pkgExistsInSection(spec, sectionKey, name) {
				return agentenvError.UserError(
					fmt.Sprintf("Package %q not found in %s of environment %q", name, sectionKey, envName),
					"Check the package name with 'agentenv list-packages'.")
			}
			removePkgFromSpec(spec, sectionKey, name)
		} else {
			sections := findPkgSections(spec, name)
			if len(sections) == 0 {
				return agentenvError.UserError(
					fmt.Sprintf("Package %q not found in environment %q", name, envName),
					"Check the package name with 'agentenv list-packages'.")
			}
			if len(sections) > 1 {
				return agentenvError.UserError(
					fmt.Sprintf("Package %q found in both %s", name, strings.Join(sections, " and ")),
					fmt.Sprintf("Use 'agentenv remove <type> %s' to disambiguate.", name))
			}
			removePkgFromSpec(spec, sections[0], name)
		}

		if err := envfile.WriteFile(yamlPath, spec); err != nil {
			return agentenvError.SystemError(
				fmt.Sprintf("Cannot write agent.yaml for %q", envName),
				"Check file permissions.").WithCause(err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Removed %q from environment %q\n", name, envName)
		fmt.Fprintln(cmd.OutOrStdout(), "Run 'agentenv lock' to lock dependencies")

		return nil
	},
}

func findPkgSections(spec *envfile.EnvironmentSpec, name string) []string {
	var sections []string
	check := func(m map[string]envfile.PackageRef, section string) {
		if m != nil {
			if _, ok := m[name]; ok {
				sections = append(sections, section)
			}
		}
	}
	check(spec.Skills, "skills")
	check(spec.MCPs, "mcps")
	check(spec.Agents, "agents")
	check(spec.Tools, "tools")
	check(spec.Hooks, "hooks")
	check(spec.Prompts, "prompts")
	return sections
}

func removePkgFromSpec(spec *envfile.EnvironmentSpec, section, name string) {
	switch section {
	case "skills":
		delete(spec.Skills, name)
		if len(spec.Skills) == 0 {
			spec.Skills = nil
		}
	case "mcps":
		delete(spec.MCPs, name)
		if len(spec.MCPs) == 0 {
			spec.MCPs = nil
		}
	case "agents":
		delete(spec.Agents, name)
		if len(spec.Agents) == 0 {
			spec.Agents = nil
		}
	case "tools":
		delete(spec.Tools, name)
		if len(spec.Tools) == 0 {
			spec.Tools = nil
		}
	case "hooks":
		delete(spec.Hooks, name)
		if len(spec.Hooks) == 0 {
			spec.Hooks = nil
		}
	case "prompts":
		delete(spec.Prompts, name)
		if len(spec.Prompts) == 0 {
			spec.Prompts = nil
		}
	}
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
