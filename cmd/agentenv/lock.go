package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/7emotions/agentenv/pkg/envfile"
	"github.com/7emotions/agentenv/pkg/lockfile"
	"github.com/7emotions/agentenv/pkg/parser"
	"github.com/7emotions/agentenv/pkg/resolver"
	"github.com/7emotions/agentenv/pkg/types"
	"github.com/spf13/cobra"
)

var (
	lockUpdate bool
	lockStrict bool
)

type lockSummary struct {
	Path       string   `json:"path"`
	Packages   int      `json:"packages"`
	Conflicts  int      `json:"conflicts"`
	Warnings   []string `json:"warnings,omitempty"`
	DurationMs int64    `json:"duration_ms"`
}

var lockCmd = &cobra.Command{
	Use:   "lock [name]",
	Short: "Resolve dependencies and generate agent.lock",
	Long: `Read agent.yaml, resolve all dependencies, and generate agent.lock.

The environment to lock is determined by:
  1. If a name is given, use ~/.agentenv/envs/<name>/
  2. If the current directory has an agent.yaml, use it
  3. If an environment is active, use it`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLock,
}

func runLock(cmd *cobra.Command, args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	envDir, err := findEnvDir(name)
	if err != nil {
		return err
	}

	yamlPath := filepath.Join(envDir, "agent.yaml")
	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		return fmt.Errorf("agent.yaml not found at %s", yamlPath)
	}

	spec, err := readAgentYAML(yamlPath)
	if err != nil {
		return fmt.Errorf("reading agent.yaml: %w", err)
	}

	requests := buildRequests(spec)
	if len(requests) == 0 {
		lf := &types.Lockfile{
			Version:   1,
			Generated: time.Now().UTC().Format(time.RFC3339),
			Packages:  []types.LockedPackage{},
		}
		if err := writeLockfileToDisk(envDir, lf); err != nil {
			return err
		}
		if jsonOutput {
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode(lockSummary{
				Path:     filepath.Join(envDir, "agent.lock"),
				Packages: 0,
			})
		}
		fmt.Fprintf(cmd.OutOrStdout(), "No packages defined in %s — writing empty lockfile.\n", yamlPath)
		return nil
	}

	r := resolver.NewResolver()
	r.DepSource = depSource

	opts := resolver.ResolveOptions{
		MaxDepth:    3,
		MaxPackages: 50,
		Strict:      lockStrict,
	}

	isTTY := isTerminal(cmd.OutOrStdout())
	var spin *spinner
	if isTTY && !jsonOutput {
		spin = newSpinner(cmd.OutOrStdout())
		spin.start("Resolving dependencies...")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := r.Resolve(ctx, requests, opts)
	if spin != nil {
		spin.stop()
	}
	if err != nil {
		return fmt.Errorf("resolution failed: %w", err)
	}

	lf, err := lockfile.Generate(result.Packages)
	if err != nil {
		return fmt.Errorf("generating lockfile: %w", err)
	}

	if err := writeLockfileToDisk(envDir, lf); err != nil {
		return err
	}

	conflictCount := countConflicts(result.Warnings)
	summary := lockSummary{
		Path:       filepath.Join(envDir, "agent.lock"),
		Packages:   len(lf.Packages),
		Conflicts:  conflictCount,
		Warnings:   result.Warnings,
		DurationMs: result.Duration.Milliseconds(),
	}

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(summary)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ agent.lock updated (%d packages, %d conflicts)\n",
		len(lf.Packages), conflictCount)

	if len(result.Warnings) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nWarnings:")
		for _, w := range result.Warnings {
			fmt.Fprintf(cmd.OutOrStdout(), "  ⚠ %s\n", w)
		}
	}

	return nil
}

func findEnvDir(name string) (string, error) {
	root, err := agentenvRoot()
	if err != nil {
		return "", err
	}

	if name != "" {
		envDir := filepath.Join(root, "envs", name)
		if fi, err := os.Stat(envDir); err != nil || !fi.IsDir() {
			return "", fmt.Errorf("environment %q not found at %s", name, envDir)
		}
		return envDir, nil
	}

	cwd, err := os.Getwd()
	if err == nil {
		if _, err := os.Stat(filepath.Join(cwd, "agent.yaml")); err == nil {
			return cwd, nil
		}
	}

	activeLock := filepath.Join(root, "ACTIVE")
	if data, err := os.ReadFile(activeLock); err == nil {
		activeName := parseActiveName(string(data))
		if activeName != "" {
			envDir := filepath.Join(root, "envs", activeName)
			if fi, err := os.Stat(envDir); err == nil && fi.IsDir() {
				return envDir, nil
			}
		}
	}

	return "", fmt.Errorf("no environment found: specify a name, cd to env dir, or activate an environment first")
}

func readAgentYAML(path string) (*envfile.EnvironmentSpec, error) {
	return envfile.ReadFile(path)
}

func buildRequests(spec *envfile.EnvironmentSpec) []resolver.PackageRequest {
	groups := []struct {
		pkgType string
		m       map[string]envfile.PackageRef
	}{
		{"skill", spec.Skills},
		{"mcp", spec.MCPs},
		{"agent", spec.Agents},
		{"tool", spec.Tools},
		{"hook", spec.Hooks},
		{"prompt", spec.Prompts},
	}

	var total int
	for _, g := range groups {
		total += len(g.m)
	}
	requests := make([]resolver.PackageRequest, 0, total)

	for _, g := range groups {
		for name, pkg := range g.m {
			if pkg.Version == "" {
				pkg.Version = "*"
			}
			requests = append(requests, resolver.PackageRequest{
				Name:       name,
				Type:       g.pkgType,
				Source:     pkg.Source,
				Constraint: pkg.Version,
			})
		}
	}

	return requests
}

func depSource(name, pkgType string) string {
	switch types.PackageType(pkgType) {
	case types.PackageTypeMCP:
		return "npm:@agentenv/" + name
	default:
		return "github:agentenv/" + name
	}
}

func writeLockfileToDisk(envDir string, lf *types.Lockfile) error {
	data, err := parser.WriteLockfile(lf)
	if err != nil {
		return fmt.Errorf("serializing lockfile: %w", err)
	}

	lockPath := filepath.Join(envDir, "agent.lock")
	if err := os.WriteFile(lockPath, data, 0o644); err != nil {
		return fmt.Errorf("writing agent.lock: %w", err)
	}
	return nil
}

func countConflicts(warnings []string) int {
	count := 0
	for _, w := range warnings {
		if strings.Contains(w, "collision") {
			count++
		}
	}
	return count
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type spinner struct {
	w       io.Writer
	done    chan struct{}
	stopped atomic.Bool
}

func newSpinner(w io.Writer) *spinner {
	return &spinner{w: w, done: make(chan struct{})}
}

func (s *spinner) start(msg string) {
	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		frame := 0
		for {
			select {
			case <-s.done:
				return
			case <-tick.C:
				if s.stopped.Load() {
					return
				}
				fmt.Fprintf(s.w, "\r%s %s", spinnerFrames[frame%len(spinnerFrames)], msg)
				frame++
			}
		}
	}()
}

func (s *spinner) stop() {
	s.stopped.Store(true)
	close(s.done)
	fmt.Fprint(s.w, "\r\033[K")
}

func isTerminal(w interface{}) bool {
	if f, ok := w.(*os.File); ok {
		fi, err := f.Stat()
		if err != nil {
			return false
		}
		return (fi.Mode() & os.ModeCharDevice) != 0
	}
	return false
}

func init() {
	lockCmd.Flags().BoolVar(&lockUpdate, "update", false, "Update existing lockfile")
	lockCmd.Flags().BoolVar(&lockStrict, "strict", false, "Fail on non-deterministic or conflicting constraints")
	rootCmd.AddCommand(lockCmd)
}
