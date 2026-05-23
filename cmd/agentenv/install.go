package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/7emotions/agentenv/pkg/lockfile"
	"github.com/7emotions/agentenv/pkg/parser"
	"github.com/7emotions/agentenv/pkg/resolver"
	"github.com/7emotions/agentenv/pkg/source"
	"github.com/7emotions/agentenv/pkg/store"
	"github.com/7emotions/agentenv/pkg/types"
	"github.com/spf13/cobra"
)

var installFile string

type installSummary struct {
	Path       string   `json:"path"`
	Packages   int      `json:"packages"`
	Fetched    int      `json:"fetched"`
	Cached     int      `json:"cached"`
	Errors     []string `json:"errors,omitempty"`
	DurationMs int64    `json:"duration_ms"`
}

var installCmd = &cobra.Command{
	Use:   "install [name]",
	Short: "Fetch packages into the store and activate if needed",
	Long: `Read agent.lock (or agent.yaml if no lock) and fetch all packages into the
local store. If the environment is currently active, trigger re-activation.

Flags:
  --file   Use an alternate agent.yaml file`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInstall,
}

func runInstall(cmd *cobra.Command, args []string) error {
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

	st, err := store.NewStore(filepath.Join(root, "store"))
	if err != nil {
		return fmt.Errorf("initialize store: %w", err)
	}
	st.SetEnvBasePath(filepath.Join(root, "envs"))

	packages, err := resolveInstallPackages(envDir)
	if err != nil {
		return err
	}

	if len(packages) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No packages to install.\n")
		return nil
	}

	isTTY := isTerminal(cmd.OutOrStdout())
	var fetched, cached int
	var errors []string

	for _, pkg := range packages {
		slug := store.Slugify(pkg.Source)
		pkgType := string(pkg.Type)

		if st.Exists(pkgType, slug, pkg.Resolved) {
			cached++
			continue
		}

		prefix := "Fetching"
		if isTTY {
			prefix = "⠋ Fetching"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s@%s...\n", prefix, pkg.Name, pkg.Resolved)

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

	summary := installSummary{
		Path:       envDir,
		Packages:   len(packages),
		Fetched:    fetched,
		Cached:     cached,
		Errors:     errors,
		DurationMs: time.Since(start).Milliseconds(),
	}

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(summary)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n✓ Install complete: %d fetched, %d cached, %d errors\n",
		fetched, cached, len(errors))

	for _, e := range errors {
		fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s\n", e)
	}

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

func resolveInstallPackages(envDir string) ([]types.LockedPackage, error) {
	lockPath := filepath.Join(envDir, "agent.lock")
	if data, err := os.ReadFile(lockPath); err == nil {
		lf, err := lockfile.Read(data)
		if err != nil {
			return nil, fmt.Errorf("parsing agent.lock: %w", err)
		}
		return lf.Packages, nil
	}

	yamlPath := filepath.Join(envDir, "agent.yaml")
	if installFile != "" {
		yamlPath = installFile
	}

	spec, err := readAgentYAML(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("reading agent.yaml: %w", err)
	}

	requests := buildRequests(spec)
	if len(requests) == 0 {
		return nil, nil
	}

	r := resolver.NewResolver()
	r.DepSource = depSource

	result, err := r.Resolve(context.Background(), requests, resolver.ResolveOptions{
		MaxDepth:    3,
		MaxPackages: 50,
	})
	if err != nil {
		return nil, fmt.Errorf("resolution failed: %w", err)
	}

	lf, err := parser.WriteLockfile(&types.Lockfile{
		Version:   1,
		Generated: time.Now().UTC().Format(time.RFC3339),
		Packages:  convertToLocked(result.Packages),
	})
	if err != nil {
		return nil, fmt.Errorf("serializing lock: %w", err)
	}

	parsed, err := lockfile.Read(lf)
	if err != nil {
		return nil, fmt.Errorf("parsing generated lock: %w", err)
	}
	return parsed.Packages, nil
}

func convertToLocked(packages []resolver.ResolvedPackage) []types.LockedPackage {
	locked := make([]types.LockedPackage, len(packages))
	for i, pkg := range packages {
		deps := make([]types.LockedDep, len(pkg.Dependencies))
		for j, dep := range pkg.Dependencies {
			deps[j] = types.LockedDep{
				Name:     dep.Name,
				Version:  dep.Version,
				Resolved: dep.Resolved,
			}
		}
		locked[i] = types.LockedPackage{
			Name:         pkg.Name,
			Type:         types.PackageType(pkg.Type),
			Source:       pkg.Source,
			Version:      pkg.Version,
			Resolved:     pkg.Resolved,
			SHA256:       pkg.SHA256,
			Dependencies: deps,
		}
	}
	return locked
}

func init() {
	installCmd.Flags().StringVar(&installFile, "file", "", "Use an alternate agent.yaml file")
	rootCmd.AddCommand(installCmd)
}
