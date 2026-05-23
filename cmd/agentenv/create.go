package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	agentenvError "github.com/7emotions/agentenv/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	createAgent string
	createFrom  string
	createEmpty bool
)

var validEnvName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new agent environment",
	Long: `Create a new agent environment under ~/.agentenv/envs/<name>/.

This initializes the environment directory with an agent.yaml manifest
and a state.json tracking file.

Environment names must be kebab-case (lowercase letters, digits, hyphens),
starting with a lowercase letter, and at most 64 characters.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if err := validateName(name); err != nil {
			return err
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return agentenvError.SystemError("Cannot determine home directory",
				"Check that $HOME is set and accessible.").WithCause(err)
		}
		envRoot := filepath.Join(home, ".agentenv", "envs")
		envDir := filepath.Join(envRoot, name)

		if _, err := os.Stat(envDir); err == nil {
			return agentenvError.UserError(
				fmt.Sprintf("Environment %q already exists at %s", name, envDir),
				"Use a different name, or remove the existing environment first.")
		} else if !os.IsNotExist(err) {
			return agentenvError.SystemError(
				fmt.Sprintf("Cannot check environment directory %s", envDir),
				"Check file permissions.").WithCause(err)
		}

		if err := os.MkdirAll(envDir, 0o755); err != nil {
			return agentenvError.SystemError(
				fmt.Sprintf("Cannot create environment directory %s", envDir),
				"Check file permissions and disk space.").WithCause(err)
		}

		var failed bool
		defer func() {
			if failed {
				os.RemoveAll(envDir)
			}
		}()

		if createFrom != "" {
			srcDir := filepath.Join(envRoot, createFrom)
			srcYAML := filepath.Join(srcDir, "agent.yaml")
			if _, err := os.Stat(srcYAML); err != nil {
				failed = true
				return agentenvError.UserError(
					fmt.Sprintf("Source environment %q not found or missing agent.yaml", createFrom),
					"Check that the source environment exists: agentenv list")
			}
			data, err := os.ReadFile(srcYAML)
			if err != nil {
				failed = true
				return agentenvError.SystemError(
					fmt.Sprintf("Cannot read source agent.yaml from %q", createFrom),
					"Check file permissions.").WithCause(err)
			}
			if err := os.WriteFile(filepath.Join(envDir, "agent.yaml"), data, 0o644); err != nil {
				failed = true
				return agentenvError.SystemError(
					fmt.Sprintf("Cannot write agent.yaml to %s", envDir),
					"Check file permissions and disk space.").WithCause(err)
			}
		} else if !createEmpty {
			yamlContent := fmt.Sprintf("name: %s\ndescription: \"Environment created on %s\"\n",
				name, time.Now().UTC().Format(time.RFC3339))
			if err := os.WriteFile(filepath.Join(envDir, "agent.yaml"), []byte(yamlContent), 0o644); err != nil {
				failed = true
				return agentenvError.SystemError(
					fmt.Sprintf("Cannot write agent.yaml to %s", envDir),
					"Check file permissions and disk space.").WithCause(err)
			}
		}

		state := map[string]interface{}{
			"active":          false,
			"agent_framework": createAgent,
			"created_at":      time.Now().UTC().Format(time.RFC3339),
		}
		stateData, err := json.Marshal(state)
		if err != nil {
			failed = true
			return agentenvError.SystemError("Cannot marshal state.json",
				"This is a bug. Please report it.").WithCause(err)
		}
		if err := os.WriteFile(filepath.Join(envDir, "state.json"), stateData, 0o644); err != nil {
			failed = true
			return agentenvError.SystemError(
				fmt.Sprintf("Cannot write state.json to %s", envDir),
				"Check file permissions and disk space.").WithCause(err)
		}

		if jsonOutput {
			writeJSON(cmd, map[string]string{"created": name, "path": envDir})
		} else {
			fmt.Printf("Created environment %q at %s\n", name, envDir)
		}

		return nil
	},
}

func validateName(name string) error {
	if len(name) > 64 {
		return agentenvError.UserError(
			fmt.Sprintf("Invalid environment name %q: must be kebab-case, max 64 characters", name),
			"Use lowercase letters, digits, and hyphens only, starting with a letter.")
	}
	if !validEnvName.MatchString(name) {
		return agentenvError.UserError(
			fmt.Sprintf("Invalid environment name %q: must be kebab-case", name),
			"Use lowercase letters, digits, and hyphens only, starting with a letter (e.g. 'my-env-42').")
	}
	return nil
}

func init() {
	createCmd.Flags().StringVar(&createAgent, "agent", "claude-code", "Agent framework to use")
	createCmd.Flags().StringVar(&createFrom, "from", "", "Source environment or template to copy from")
	createCmd.Flags().BoolVar(&createEmpty, "empty", false, "Skip creating agent.yaml (cold-start env)")
	rootCmd.AddCommand(createCmd)
}
