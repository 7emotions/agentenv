package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	agentenvError "github.com/7emotions/agentenv/pkg/errors"
	"github.com/7emotions/agentenv/pkg/envfile"
	"github.com/7emotions/agentenv/pkg/types"
	"github.com/spf13/cobra"
)

var (
	addSource  string
	addVersion string
	addDev     bool
	addDryRun  bool
)

var validPkgName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

var typeToSection = map[string]string{
	"skill":  "skills",
	"mcp":    "mcps",
	"agent":  "agents",
	"tool":   "tools",
	"hook":   "hooks",
	"prompt": "prompts",
}

var addCmd = &cobra.Command{
	Use:   "add <type> <name> --source <url>",
	Short: "Add a package to the active environment",
	Long: `Add a package to the currently active environment's agent.yaml.

Supported types: skill, mcp, agent, tool, hook, prompt

The --source flag is required and must use a supported scheme:
  github:owner/repo[/path]
  npm:@scope/package or npm:package
  local:./path or local:/absolute/path
  git:https://example.com/repo.git
  file:/absolute/path
  url:https://example.com/pkg.tar.gz

Examples:
  agentenv add skill code-review --source github:vercel-labs/agent-skills/skills/code-review
  agentenv add mcp filesystem --source npm:@modelcontextprotocol/server-filesystem --version ^1.0.0
  agentenv add agent assistant --source github:agent-hub/assistant --dev`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		pkgType := args[0]
		name := args[1]

		sectionKey, ok := typeToSection[pkgType]
		if !ok {
			return agentenvError.UserError(
				fmt.Sprintf("Invalid type %q", pkgType),
				"Supported types: skill, mcp, agent, tool, hook, prompt")
		}

		if len(name) > 64 || !validPkgName.MatchString(name) {
			return agentenvError.UserError(
				fmt.Sprintf("Invalid package name %q", name),
				"Use kebab-case (lowercase letters, digits, hyphens), max 64 characters.")
		}

		if addSource == "" {
			return agentenvError.UserError(
				"Package source is required",
				"Use --source to specify the package source URL (e.g., --source github:owner/repo)")
		}

		src, err := types.ParseSourceURL(addSource)
		if err != nil {
			return agentenvError.UserError(
				fmt.Sprintf("Invalid source URL %q", addSource),
				"Use a supported scheme: github:, npm:, local:, git:, file:, or url:").WithCause(err)
		}
		if !src.IsValid() {
			return agentenvError.UserError(
				fmt.Sprintf("Invalid source URL %q", addSource),
				"The URL format is not valid. Check the documentation for supported formats.")
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

		if pkgExistsInSection(spec, sectionKey, name) {
			return agentenvError.UserError(
				fmt.Sprintf("Package %q already exists in %s", name, sectionKey),
				"Use a different name or remove the existing package first.")
		}

		ref := envfile.PackageRef{
			Source:  addSource,
			Version: addVersion,
		}
		if addDev {
			ref.Config = map[string]interface{}{"dev": true}
		}

		if addDryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "Would add %s %q to environment %q:\n", pkgType, name, envName)
			fmt.Fprintf(cmd.OutOrStdout(), "  source: %s\n", addSource)
			if addVersion != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  version: %s\n", addVersion)
			}
			if addDev {
				fmt.Fprintf(cmd.OutOrStdout(), "  dev: true\n")
			}
			return nil
		}

		addPkgToSpec(spec, sectionKey, name, ref)

		if err := envfile.WriteFile(yamlPath, spec); err != nil {
			return agentenvError.SystemError(
				fmt.Sprintf("Cannot write agent.yaml for %q", envName),
				"Check file permissions.").WithCause(err)
		}

		versionInfo := ""
		if addVersion != "" {
			versionInfo = fmt.Sprintf(" version: %s", addVersion)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added %s %q (source: %s%s)\n", pkgType, name, addSource, versionInfo)
		fmt.Fprintln(cmd.OutOrStdout(), "Run 'agentenv lock' to lock dependencies")

		return nil
	},
}

func pkgExistsInSection(spec *envfile.EnvironmentSpec, section, name string) bool {
	m := getSectionMap(spec, section)
	if m == nil {
		return false
	}
	_, ok := m[name]
	return ok
}

func getSectionMap(spec *envfile.EnvironmentSpec, section string) map[string]envfile.PackageRef {
	switch section {
	case "skills":
		return spec.Skills
	case "mcps":
		return spec.MCPs
	case "agents":
		return spec.Agents
	case "tools":
		return spec.Tools
	case "hooks":
		return spec.Hooks
	case "prompts":
		return spec.Prompts
	}
	return nil
}

func addPkgToSpec(spec *envfile.EnvironmentSpec, section, name string, ref envfile.PackageRef) {
	switch section {
	case "skills":
		if spec.Skills == nil {
			spec.Skills = make(map[string]envfile.PackageRef)
		}
		spec.Skills[name] = ref
	case "mcps":
		if spec.MCPs == nil {
			spec.MCPs = make(map[string]envfile.PackageRef)
		}
		spec.MCPs[name] = ref
	case "agents":
		if spec.Agents == nil {
			spec.Agents = make(map[string]envfile.PackageRef)
		}
		spec.Agents[name] = ref
	case "tools":
		if spec.Tools == nil {
			spec.Tools = make(map[string]envfile.PackageRef)
		}
		spec.Tools[name] = ref
	case "hooks":
		if spec.Hooks == nil {
			spec.Hooks = make(map[string]envfile.PackageRef)
		}
		spec.Hooks[name] = ref
	case "prompts":
		if spec.Prompts == nil {
			spec.Prompts = make(map[string]envfile.PackageRef)
		}
		spec.Prompts[name] = ref
	}
}

func init() {
	addCmd.Flags().StringVar(&addSource, "source", "", "Package source URL (required)")
	addCmd.Flags().StringVar(&addVersion, "version", "", "Version constraint (e.g., ^1.0.0, >=2.0)")
	addCmd.Flags().BoolVar(&addDev, "dev", false, "Mark as development dependency")
	addCmd.Flags().BoolVar(&addDryRun, "dry-run", false, "Show what would be added without modifying files")
	rootCmd.AddCommand(addCmd)
}
