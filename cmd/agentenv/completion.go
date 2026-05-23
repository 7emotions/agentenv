package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for agentenv commands.

The completion script can be sourced to enable tab completion:

  # Bash
  source <(agentenv completion bash)

  # Zsh
  source <(agentenv completion zsh)

To make it permanent:

  # Bash (Linux)
  agentenv completion bash > /etc/bash_completion.d/agentenv

  # Bash (macOS)
  agentenv completion bash > /usr/local/etc/bash_completion.d/agentenv

  # Zsh
  agentenv completion zsh > "${fpath[1]}/_agentenv"
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh"},
	Args:                  cobra.ExactValidArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			return rootCmd.GenZshCompletion(cmd.OutOrStdout())
		default:
			return fmt.Errorf("unsupported shell: %s (use bash or zsh)", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
