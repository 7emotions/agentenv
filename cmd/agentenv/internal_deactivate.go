package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	agentenvError "github.com/7emotions/agentenv/pkg/errors"
	"github.com/spf13/cobra"
)

type deactivateOutput struct {
	Active   bool     `json:"active"`
	Warnings []string `json:"warnings,omitempty"`
}

var internalDeactivateCmd = &cobra.Command{
	Use:    "_internal_deactivate",
	Short:  "INTERNAL: deactivate the current environment (called by shell function)",
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := agentenvRoot()
		if err != nil {
			return err
		}

		activeLock := filepath.Join(root, "ACTIVE")
		data, err := os.ReadFile(activeLock)
		if err != nil {
			if os.IsNotExist(err) {
				output := deactivateOutput{Active: false}
				enc := json.NewEncoder(cmd.OutOrStdout())
				return enc.Encode(output)
			}
			return agentenvError.SystemError("Cannot read active lock file",
				"Check file permissions in ~/.agentenv/.").WithCause(err)
		}

		activeName := strings.TrimSpace(string(data))
		if activeName == "" {
			output := deactivateOutput{Active: false}
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode(output)
		}

		if err := deactivateEnv(cmd.Context(), activeName, root); err != nil {
			os.Remove(activeLock)
			output := deactivateOutput{
				Active:   false,
				Warnings: []string{err.Error()},
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			_ = enc.Encode(output)
			return nil
		}

		output := deactivateOutput{Active: false}
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(output)
	},
}

func init() {
	rootCmd.AddCommand(internalDeactivateCmd)
}
