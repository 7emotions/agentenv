package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/7emotions/agentenv/pkg/envfile"
	agentenvError "github.com/7emotions/agentenv/pkg/errors"
	"github.com/spf13/cobra"
)

type envInfo struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	AgentFramework string `json:"agent_framework"`
	PackagesCount  int    `json:"packages_count"`
	CreatedAt      string `json:"created_at,omitempty"`
	LastActivated  string `json:"last_activated,omitempty"`
	Active         bool   `json:"active"`
}

var infoCmd = &cobra.Command{
	Use:   "info [name]",
	Short: "Show environment details",
	Long: `Show detailed information about an environment.

If name is omitted, the currently active environment is shown.
Use --json for machine-readable output.`,
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

		state, err := readState(envDir)
		if err != nil {
			return err
		}

		pkgCount := 0
		yamlPath := filepath.Join(envDir, "agent.yaml")
		if data, err := os.ReadFile(yamlPath); err == nil {
			spec, err := envfile.Read(data)
			if err == nil && spec != nil {
				if spec.Skills != nil {
					pkgCount += len(spec.Skills)
				}
				if spec.MCPs != nil {
					pkgCount += len(spec.MCPs)
				}
				if spec.Agents != nil {
					pkgCount += len(spec.Agents)
				}
				if spec.Tools != nil {
					pkgCount += len(spec.Tools)
				}
				if spec.Hooks != nil {
					pkgCount += len(spec.Hooks)
				}
				if spec.Prompts != nil {
					pkgCount += len(spec.Prompts)
				}
			}
		}

		activeName := ""
		if data, err := os.ReadFile(filepath.Join(root, "ACTIVE")); err == nil {
			activeName = parseActiveName(string(data))
		}

		info := envInfo{
			Name:          name,
			Path:          envDir,
			PackagesCount: pkgCount,
			Active:        name == activeName,
		}
		if f, ok := state["agent_framework"].(string); ok {
			info.AgentFramework = f
		}
		if c, ok := state["created_at"].(string); ok {
			info.CreatedAt = c
		}
		if l, ok := state["last_activated"].(string); ok {
			info.LastActivated = l
		}

		if jsonOutput {
			writeJSON(cmd, info)
			return nil
		}

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Name:           %s\n", info.Name)
		fmt.Fprintf(out, "Path:           %s\n", info.Path)
		fmt.Fprintf(out, "Agent:          %s\n", info.AgentFramework)
		fmt.Fprintf(out, "Packages:       %d\n", info.PackagesCount)
		fmt.Fprintf(out, "Created:        %s\n", info.CreatedAt)
		if info.LastActivated != "" {
			fmt.Fprintf(out, "Last Activated: %s\n", info.LastActivated)
		}
		fmt.Fprintf(out, "Active:         %t\n", info.Active)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
