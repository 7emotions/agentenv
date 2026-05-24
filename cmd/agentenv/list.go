package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	agentenvError "github.com/7emotions/agentenv/pkg/errors"
	"github.com/spf13/cobra"
)

type envEntry struct {
	Name    string `json:"name"`
	Agent   string `json:"agent"`
	Active  bool   `json:"active"`
	Created string `json:"created_at,omitempty"`
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all environments",
	Long: `List all agent environments under ~/.agentenv/envs/.

Displays the name, agent framework, and activation status for each environment.
Use --json for machine-readable output.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := agentenvRoot()
		if err != nil {
			return err
		}

		envRoot := filepath.Join(root, "envs")
		entries, err := os.ReadDir(envRoot)
		if err != nil {
			if os.IsNotExist(err) {
				if jsonOutput {
					writeJSON(cmd, []envEntry{})
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No environments. Create one with: agentenv create <name>")
				}
				return nil
			}
			return agentenvError.SystemError(
				"Cannot list environments",
				"Check that ~/.agentenv/envs/ exists and is readable.").WithCause(err)
		}

		activeName := ""
		if data, err := os.ReadFile(filepath.Join(root, "ACTIVE")); err == nil {
			activeName = parseActiveName(string(data))
		}

		var envs []envEntry
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			envDir := filepath.Join(envRoot, entry.Name())

			state, err := readState(envDir)
			if err != nil {
				continue
			}

			e := envEntry{
				Name:   entry.Name(),
				Active: entry.Name() == activeName,
			}
			if f, ok := state["agent_framework"].(string); ok {
				e.Agent = f
			}
			if c, ok := state["created_at"].(string); ok {
				e.Created = c
			}
			envs = append(envs, e)
		}

		sort.Slice(envs, func(i, j int) bool {
			return envs[i].Name < envs[j].Name
		})

		if jsonOutput {
			writeJSON(cmd, envs)
			return nil
		}

		if len(envs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No environments. Create one with: agentenv create <name>")
			return nil
		}

		out := cmd.OutOrStdout()
		for _, e := range envs {
			status := "inactive"
			if e.Active {
				status = "active"
			}
			agent := e.Agent
			if agent == "" {
				agent = "claude-code"
			}
			fmt.Fprintf(out, "%-24s %-16s %s\n", e.Name, agent, status)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
