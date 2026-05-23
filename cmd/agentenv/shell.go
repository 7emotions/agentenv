package main

import (
	"fmt"

	agentenvError "github.com/7emotions/agentenv/pkg/errors"
	"github.com/7emotions/agentenv/internal/shell"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init <shell>",
	Short: "Generate shell init script for shell integration",
	Long: `Generate a shell init script that integrates agentenv with your shell.

This command outputs a shell script that defines the 'agentenv()' shell function,
enabling environment activation/deactivation within the current shell session.

Usage in your shell config:
  eval "$(agentenv init zsh)"   # add to ~/.zshrc
  eval "$(agentenv init bash)"  # add to ~/.bashrc

Supported shells: zsh, bash`,
	Args: cobra.ExactArgs(1),
	ValidArgs: []string{"zsh", "bash"},
	RunE: func(cmd *cobra.Command, args []string) error {
		shellName := args[0]
		if shellName != "zsh" && shellName != "bash" {
			return agentenvError.UserError(
				fmt.Sprintf("Unsupported shell: %s", shellName),
				"Supported shells: zsh, bash. Usage: agentenv init zsh")
		}

		if jsonOutput {
			writeJSON(cmd, map[string]string{"shell": shellName, "version": rootCmd.Version})
			return nil
		}

		script, err := shell.GenerateInitScript(shellName)
		if err != nil {
			return agentenvError.SystemError(
				fmt.Sprintf("Cannot generate init script for %s", shellName),
				"This is a bug. Please report it.").WithCause(err)
		}
		fmt.Print(script)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
