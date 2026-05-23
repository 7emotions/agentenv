package main

import (
	"fmt"
	"os"
	"path/filepath"

	agentenvError "github.com/7emotions/agentenv/pkg/errors"
	"github.com/spf13/cobra"
)

var deleteForce bool

var deleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete an environment",
	Long: `Delete an environment and all its files from ~/.agentenv/envs/<name>/.

If the environment is currently active, deactivation is required first,
unless --force is used to override.

This operation is irreversible.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		root, err := agentenvRoot()
		if err != nil {
			return err
		}

		envDir := filepath.Join(root, "envs", name)

		if _, err := os.Stat(envDir); os.IsNotExist(err) {
			return agentenvError.UserError(
				fmt.Sprintf("Environment %q not found", name),
				"Check the name with 'agentenv list'.")
		}

		activeLock := filepath.Join(root, "ACTIVE")
		if data, err := os.ReadFile(activeLock); err == nil {
			activeName := parseActiveName(string(data))
			if activeName == name && !deleteForce {
				return agentenvError.UserError(
					fmt.Sprintf("Cannot delete active environment %q", name),
					"Deactivate it first, or use --force to override.")
			}
		}

		if err := os.RemoveAll(envDir); err != nil {
			return agentenvError.SystemError(
				fmt.Sprintf("Cannot delete environment %q", name),
				"Check file permissions.").WithCause(err)
		}

		if jsonOutput {
			writeJSON(cmd, map[string]string{"deleted": name})
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted environment %q\n", name)
		}

		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVar(&deleteForce, "force", false, "Force deletion even if active")
	rootCmd.AddCommand(deleteCmd)
}
