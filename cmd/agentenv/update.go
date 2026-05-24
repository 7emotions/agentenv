package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/7emotions/agentenv/pkg/lockfile"
	"github.com/7emotions/agentenv/pkg/resolver"
	"github.com/7emotions/agentenv/pkg/source"
	"github.com/7emotions/agentenv/pkg/store"
	"github.com/7emotions/agentenv/pkg/types"
	"github.com/spf13/cobra"
)

type updateSummary struct {
	Path       string   `json:"path"`
	Packages   int      `json:"packages"`
	Fetched    int      `json:"fetched"`
	Cached     int      `json:"cached"`
	Conflicts  int      `json:"conflicts"`
	Errors     []string `json:"errors,omitempty"`
	DurationMs int64    `json:"duration_ms"`
}

var updateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Update all packages to latest compatible versions",
	Long: `Re-resolve all dependencies and re-install every package for an environment.

Updates all packages to their latest compatible versions within the
constraints specified in agent.yaml. The full flow is:
  1. Read agent.yaml
  2. Re-resolve dependencies via PubGrub (picks latest compatible versions)
  3. Write updated agent.lock
  4. Fetch packages into the local store
  5. Re-activate if the environment is currently active

The environment to update is determined by:
  1. If a name is given, use ~/.agentenv/envs/<name>/
  2. If the current directory has an agent.yaml, use it
  3. If an environment is active, use it`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	start := time.Now()
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	envDir, err := findEnvDir(name)
	if err != nil {
		return err
	}

	root, err := agentenvRoot()
	if err != nil {
		return err
	}

	// 1. Read agent.yaml
	yamlPath := filepath.Join(envDir, "agent.yaml")
	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		return fmt.Errorf("agent.yaml not found at %s", yamlPath)
	}

	spec, err := readAgentYAML(yamlPath)
	if err != nil {
		return fmt.Errorf("reading agent.yaml: %w", err)
	}

	requests := buildRequests(spec)

	// 2. Re-resolve dependencies
	var lf *types.Lockfile
	if len(requests) == 0 {
		lf = &types.Lockfile{
			Version:   1,
			Generated: time.Now().UTC().Format(time.RFC3339),
			Packages:  []types.LockedPackage{},
		}
	} else {
		r := resolver.NewPubGrubResolver(resolver.WithDepSource(depSource))

		opts := resolver.ResolveOptions{
			MaxDepth:    3,
			MaxPackages: 50,
			Strict:      false,
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

		lf, err = lockfile.Generate(result.Packages)
		if err != nil {
			return fmt.Errorf("generating lockfile: %w", err)
		}
	}

	// 3. Write updated lockfile
	if err := writeLockfileToDisk(envDir, lf); err != nil {
		return err
	}

	// 4. Fetch packages into store
	st, err := store.NewStore(filepath.Join(root, "store"))
	if err != nil {
		return fmt.Errorf("initialize store: %w", err)
	}
	st.SetEnvBasePath(filepath.Join(root, "envs"))

	var fetched, cached int
	var errors []string

	for _, pkg := range lf.Packages {
		slug := store.Slugify(pkg.Source)
		pkgType := string(pkg.Type)

		if st.Exists(pkgType, slug, pkg.Resolved) {
			cached++
			continue
		}

		srcURL, err := types.ParseSourceURL(pkg.Source)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: invalid source %q: %v", pkg.Name, pkg.Source, err))
			continue
		}

		handler, err := source.GetHandler(srcURL.Scheme)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", pkg.Name, err))
			continue
		}

		data, _, err := handler.Fetch(srcURL, pkg.Resolved)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: fetch failed: %v", pkg.Name, err))
			continue
		}

		if _, err := st.Put(pkgType, slug, pkg.Resolved, data); err != nil {
			errors = append(errors, fmt.Sprintf("%s: store failed: %v", pkg.Name, err))
			continue
		}
		fetched++
	}

	summary := updateSummary{
		Path:       filepath.Join(envDir, "agent.lock"),
		Packages:   len(lf.Packages),
		Fetched:    fetched,
		Cached:     cached,
		DurationMs: time.Since(start).Milliseconds(),
		Errors:     errors,
	}

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(summary)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Updated %d packages (%d fetched, %d cached)\n",
		len(lf.Packages), fetched, cached)

	for _, e := range errors {
		fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s\n", e)
	}

	// 5. Re-activate if the environment is currently active
	activeLock := filepath.Join(root, "ACTIVE")
	if data, err := os.ReadFile(activeLock); err == nil {
		activeName := parseActiveName(string(data))
		if activeName != "" {
			envName := filepath.Base(envDir)
			if activeName == envName {
				fmt.Fprintf(cmd.OutOrStdout(), "Re-activating environment %q...\n", activeName)
				if err := activateEnv(context.Background(), activeName, internalActivateCmd); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  ⚠ activation failed: %v\n", err)
				}
			}
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
