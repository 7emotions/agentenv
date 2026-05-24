package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	runtimedebug "runtime/debug"

	agentenvError "github.com/7emotions/agentenv/pkg/errors"
	"github.com/spf13/cobra"
)

// Set via ldflags at build time (goreleaser)
var (
	version = ""
	commit  = ""
	date    = ""
)

var rootCmd = &cobra.Command{
	Use:   "agentenv",
	Short: "AI agent environment management CLI",
	Long: `agentenv is a CLI tool for managing AI agent environments.

It helps create, configure, and manage isolated environments
for running AI agents with precise dependency control.

Examples:
  agentenv create my-env
  agentenv list
  agentenv info my-env
  agentenv list-packages my-env --type skill
  agentenv delete my-env`,
	Version:       getVersion(),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var jsonOutput bool
var debug bool

func getVersion() string {
	if version != "" {
		return version
	}
	if v := os.Getenv("VERSION"); v != "" {
		return v
	}
	return "dev"
}

func writeJSON(cmd *cobra.Command, data interface{}) {
	json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
		"status": "ok",
		"data":   data,
	})
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "enable JSON output format")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug output with stack traces")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		if jsonOutput {
			json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"status": "error",
				"error": map[string]interface{}{
					"code":    exitCode(err),
					"message": err.Error(),
				},
			})
		} else {
			fmt.Fprintln(os.Stderr, err.Error())
		}

		if debug {
			fmt.Fprintf(os.Stderr, "\nStack trace:\n%s\n", runtimedebug.Stack())
		}

		os.Exit(exitCode(err))
	}
}

func exitCode(err error) int {
	var ae *agentenvError.AgentError
	if errors.As(err, &ae) {
		return ae.ExitCode()
	}
	return 1
}
