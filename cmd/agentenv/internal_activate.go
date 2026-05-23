package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	agentenvError "github.com/7emotions/agentenv/pkg/errors"
	"github.com/7emotions/agentenv/pkg/adapter"
	"github.com/7emotions/agentenv/pkg/envfile"
	"github.com/7emotions/agentenv/pkg/lockfile"
	"github.com/7emotions/agentenv/pkg/store"
	"github.com/7emotions/agentenv/pkg/types"
	"github.com/spf13/cobra"
)

// getAdapter is overridable by tests for custom adapter resolution.
var getAdapter = func(framework string) (adapter.AgentAdapter, error) {
	return adapter.Get(framework)
}

// action records a completed install step for rollback.
type action struct {
	kind string // "skill", "mcp", "agent"
	name string
}

// activeOutput is the JSON structure returned to the shell function.
type activeOutput struct {
	Active   string            `json:"active"`
	Env      map[string]string `json:"env"`
	Prompt   string            `json:"prompt"`
	Warnings []string          `json:"warnings,omitempty"`
}

// agentenvRoot returns the ~/.agentenv/ directory path.
func agentenvRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", agentenvError.SystemError("Cannot determine home directory",
			"Check that $HOME is set and accessible.").WithCause(err)
	}
	return filepath.Join(home, ".agentenv"), nil
}

// readState reads state.json from an environment directory.
func readState(envDir string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filepath.Join(envDir, "state.json"))
	if err != nil {
		return nil, agentenvError.SystemError(
			fmt.Sprintf("Cannot read state.json from %s", envDir),
			"Check that the environment exists and is not corrupted.").WithCause(err)
	}
	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, agentenvError.SystemError(
			fmt.Sprintf("Cannot parse state.json in %s", envDir),
			"The state file may be corrupted. Try recreating the environment.").WithCause(err)
	}
	return state, nil
}

// agentFrameworkFromState extracts the agent_framework field from state.
func agentFrameworkFromState(state map[string]interface{}) string {
	if f, ok := state["agent_framework"].(string); ok && f != "" {
		return f
	}
	return "claude-code"
}

// extractTarGz extracts a tar.gz archive into destDir.
// It strips the top-level directory commonly present in GitHub tarballs.
func extractTarGz(data []byte, destDir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return agentenvError.SystemError("Cannot read compressed package data",
			"The package may be corrupted. Try running 'agentenv lock' again.").WithCause(err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return agentenvError.SystemError("Cannot read package archive",
				"The package may be corrupted. Try running 'agentenv lock' again.").WithCause(err)
		}

		// Strip top-level directory (GitHub tarballs have a prefix dir)
		name := header.Name
		if idx := strings.IndexByte(name, '/'); idx >= 0 {
			name = name[idx+1:]
		}
		if name == "" {
			continue
		}

		target := filepath.Join(destDir, name)

		// Prevent directory traversal
		cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(target), cleanDest) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return agentenvError.SystemError(
					fmt.Sprintf("Cannot create directory %s", target),
					"Check file permissions.").WithCause(err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return agentenvError.SystemError(
					fmt.Sprintf("Cannot create parent directory for %s", target),
					"Check file permissions.").WithCause(err)
			}
			f, err := os.Create(target)
			if err != nil {
				return agentenvError.SystemError(
					fmt.Sprintf("Cannot create file %s", target),
					"Check file permissions and disk space.").WithCause(err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return agentenvError.SystemError(
					fmt.Sprintf("Cannot write file %s", target),
					"Check disk space.").WithCause(err)
			}
			f.Close()
		}
	}
	return nil
}

// buildMCPServerFromConfig converts a raw config map from agent.yaml
// into a UniversalMCPServer struct.
func buildMCPServerFromConfig(cfg map[string]interface{}) types.UniversalMCPServer {
	server := types.UniversalMCPServer{}
	if cfg == nil {
		return server
	}
	if cmd, ok := cfg["command"].(string); ok {
		server.Command = cmd
	}
	if argsRaw, ok := cfg["args"].([]interface{}); ok {
		for _, a := range argsRaw {
			if s, ok := a.(string); ok {
				server.Args = append(server.Args, s)
			}
		}
	}
	if envRaw, ok := cfg["env"].(map[string]interface{}); ok {
		env := make(map[string]string, len(envRaw))
		for k, v := range envRaw {
			if s, ok := v.(string); ok {
				env[k] = s
			}
		}
		server.Env = env
	}
	if url, ok := cfg["url"].(string); ok {
		server.URL = url
	}
	if headersRaw, ok := cfg["headers"].(map[string]interface{}); ok {
		headers := make(map[string]string, len(headersRaw))
		for k, v := range headersRaw {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
		server.Headers = headers
	}
	if desc, ok := cfg["description"].(string); ok {
		server.Description = desc
	}
	if server.Command != "" {
		server.Transport = "stdio"
	} else if server.URL != "" {
		server.Transport = "sse"
	}
	return server
}

func findPackageConfig(spec *envfile.EnvironmentSpec, pkgType types.PackageType, name string) map[string]interface{} {
	if spec == nil {
		return nil
	}
	var ref *envfile.PackageRef
	switch pkgType {
	case types.PackageTypeSkill:
		if spec.Skills != nil {
			if r, ok := spec.Skills[name]; ok {
				ref = &r
			}
		}
	case types.PackageTypeMCP:
		if spec.MCPs != nil {
			if r, ok := spec.MCPs[name]; ok {
				ref = &r
			}
		}
	case types.PackageTypeAgent:
		if spec.Agents != nil {
			if r, ok := spec.Agents[name]; ok {
				ref = &r
			}
		}
	case types.PackageTypeTool:
		if spec.Tools != nil {
			if r, ok := spec.Tools[name]; ok {
				ref = &r
			}
		}
	case types.PackageTypeHook:
		if spec.Hooks != nil {
			if r, ok := spec.Hooks[name]; ok {
				ref = &r
			}
		}
	case types.PackageTypePrompt:
		if spec.Prompts != nil {
			if r, ok := spec.Prompts[name]; ok {
				ref = &r
			}
		}
	}
	if ref != nil {
		return ref.Config
	}
	return nil
}

// rollback undoes all completed actions in reverse order and restores from backup.
// Errors from individual undo steps are collected and returned as a slice.
// Non-critical errors do not halt the rollback process.
func rollback(ctx context.Context, a adapter.AgentAdapter, actions []action, backupID string) []string {
	var rollbackErrors []string
	for i := len(actions) - 1; i >= 0; i-- {
		switch actions[i].kind {
		case "skill":
			if err := a.RemoveSkill(ctx, actions[i].name); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Sprintf("rollback skill %s: %v", actions[i].name, err))
			}
		case "mcp":
			if err := a.RemoveMCP(ctx, actions[i].name); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Sprintf("rollback mcp %s: %v", actions[i].name, err))
			}
		case "agent":
			if err := a.RemoveAgent(ctx, actions[i].name); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Sprintf("rollback agent %s: %v", actions[i].name, err))
			}
		}
	}
	if backupID != "" {
		if err := a.Restore(ctx, backupID); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Sprintf("rollback restore: %v", err))
		}
	}
	return rollbackErrors
}

// activateEnv performs the full transactional environment activation.
func activateEnv(ctx context.Context, name string, cmd *cobra.Command) error {
	root, err := agentenvRoot()
	if err != nil {
		return err
	}

	envDir := filepath.Join(root, "envs", name)

	// 1. Check env exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return agentenvError.UserError(
			fmt.Sprintf("Environment %q not found", name),
			fmt.Sprintf("Create it first: agentenv create %s", name))
	}

	// 2. Read state.json for agent framework
	state, err := readState(envDir)
	if err != nil {
		return err
	}
	framework := agentFrameworkFromState(state)

	// 3. Read lockfile
	lockfilePath := filepath.Join(envDir, "agent.lock")
	yamlPath := filepath.Join(envDir, "agent.yaml")

	var packages []types.LockedPackage
	if data, err := os.ReadFile(lockfilePath); err == nil {
		lf, err := lockfile.Read(data)
		if err != nil {
			return agentenvError.SystemError(
				fmt.Sprintf("Cannot read lockfile for %s", name),
				"The lockfile may be corrupted. Run 'agentenv lock' again.").WithCause(err)
		}
		packages = lf.Packages
	} else if !os.IsNotExist(err) {
		return agentenvError.SystemError(
			fmt.Sprintf("Cannot check lockfile for %s", name),
			"Check file permissions.").WithCause(err)
	}

	// 4. Read agent.yaml for package configs
	var spec *envfile.EnvironmentSpec
	if data, err := os.ReadFile(yamlPath); err == nil {
		s, err := envfile.Read(data)
		if err == nil {
			spec = s
		}
		// Partial spec is OK — proceed with nil spec (no configs)
	}

	// 5. Check concurrent activation — auto-deactivate current if different
	activeLock := filepath.Join(root, "ACTIVE")
	if data, err := os.ReadFile(activeLock); err == nil {
		currentActive := strings.TrimSpace(string(data))
		if currentActive != "" && currentActive != name {
			if err := deactivateEnv(ctx, currentActive, root); err != nil {
				return agentenvError.SystemError(
					fmt.Sprintf("Cannot deactivate current environment %q", currentActive),
					"Try deactivating manually: agentenv deactivate").WithCause(err)
			}
		}
	}

	// 6. Get adapter
	a, err := getAdapter(framework)
	if err != nil {
		return agentenvError.SystemError(
			fmt.Sprintf("No adapter available for agent framework %q", framework),
			"Supported frameworks: claude-code. Check the framework in state.json.").WithCause(err)
	}

	// 7. Backup current state before any modifications
	backupID, err := a.Backup(ctx)
	if err != nil {
		return agentenvError.SystemError("Cannot create backup before activation",
			"Check file permissions in ~/.claude/.").WithCause(err)
	}

	// 8. Create store handle
	storeRoot := filepath.Join(root, "store")
	st, err := store.NewStore(storeRoot)
	if err != nil {
		return agentenvError.SystemError("Cannot initialize package store",
			"Check that ~/.agentenv/store/ is accessible.").WithCause(err)
	}
	st.SetEnvBasePath(filepath.Join(root, "envs"))

	// 9. Install packages — transactional with rollback
	var actions []action
	var warnings []string
	var failures []string

	for _, pkg := range packages {
		slug := store.Slugify(pkg.Source)

		// Check offline availability
		if !st.Exists(string(pkg.Type), slug, pkg.Resolved) {
			failures = append(failures,
				fmt.Sprintf("Package %q (%s@%s) not in cache. Run 'agentenv lock' online first.",
					pkg.Name, pkg.Source, pkg.Resolved))
			continue
		}

		// Create a store reference link for GC tracking
		if err := st.Link(envDir, string(pkg.Type), pkg.Name, slug, pkg.Resolved); err != nil {
			warnings = append(warnings, fmt.Sprintf("Cannot link %s: %v", pkg.Name, err))
		}

		switch pkg.Type {
		case types.PackageTypeSkill:
			if err := installSkillFromStore(ctx, a, st, pkg, slug, envDir); err != nil {
				failures = append(failures, err.Error())
				continue
			}
			actions = append(actions, action{kind: "skill", name: pkg.Name})

		case types.PackageTypeMCP:
			if err := installMCPFromStore(ctx, a, spec, pkg); err != nil {
				failures = append(failures, err.Error())
				continue
			}
			actions = append(actions, action{kind: "mcp", name: pkg.Name})

		case types.PackageTypeAgent:
			if err := installAgentFromStore(ctx, a, st, pkg, slug, envDir); err != nil {
				failures = append(failures, err.Error())
				continue
			}
			actions = append(actions, action{kind: "agent", name: pkg.Name})

		default:
			warnings = append(warnings,
				fmt.Sprintf("Package type %q not yet supported for activation: %s", pkg.Type, pkg.Name))
		}
	}

	// 10. If all packages failed and there were packages to install, rollback
	if len(actions) == 0 && len(packages) > 0 {
		rollback(ctx, a, actions, backupID)
		return agentenvError.UserError(
			"Activation failed: no packages could be installed",
			strings.Join(failures, "\n  "))
	}

	// If there were partial failures, report warnings
	if len(failures) > 0 {
		warnings = append(warnings,
			fmt.Sprintf("Some packages failed to install:\n  %s", strings.Join(failures, "\n  ")))
	}

	// 11. Write ACTIVE lock
	if err := os.WriteFile(activeLock, []byte(name), 0o644); err != nil {
		// Rollback on lock write failure
		rollback(ctx, a, actions, backupID)
		return agentenvError.SystemError(
			fmt.Sprintf("Cannot write active lock for %s", name),
			"Check file permissions in ~/.agentenv/.").WithCause(err)
	}

	// 12. Output JSON for shell function
	output := activeOutput{
		Active:   name,
		Env:      map[string]string{"AGENTENV_ACTIVE": name},
		Prompt:   fmt.Sprintf("(agentenv:%s)", name),
		Warnings: warnings,
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	if err := enc.Encode(output); err != nil {
		return agentenvError.SystemError("Cannot encode activation output",
			"This is a bug. Please report it.").WithCause(err)
	}

	return nil
}

// installSkillFromStore extracts a skill package from the store and installs it.
func installSkillFromStore(ctx context.Context, a adapter.AgentAdapter, st *store.Store,
	pkg types.LockedPackage, slug, envDir string) error {
	data, err := st.Get(string(pkg.Type), slug, pkg.Resolved)
	if err != nil {
		return agentenvError.SystemError(
			fmt.Sprintf("Cannot read skill %s from store", pkg.Name),
			"The package may be corrupted. Run 'agentenv lock' again.").WithCause(err)
	}

	extractDir := filepath.Join(envDir, "installed", string(pkg.Type), pkg.Name)
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return agentenvError.SystemError(
			fmt.Sprintf("Cannot create extract directory for skill %s", pkg.Name),
			"Check file permissions and disk space.").WithCause(err)
	}
	if err := extractTarGz(data, extractDir); err != nil {
		return err
	}

	if err := a.InstallSkill(ctx, pkg.Name, extractDir); err != nil {
		return agentenvError.SystemError(
			fmt.Sprintf("Cannot install skill %s", pkg.Name),
			"The skill may be incompatible. Check the skill format.").WithCause(err)
	}
	return nil
}

// installMCPFromStore installs an MCP package using config from agent.yaml.
func installMCPFromStore(ctx context.Context, a adapter.AgentAdapter,
	spec *envfile.EnvironmentSpec, pkg types.LockedPackage) error {
	if spec == nil {
		return agentenvError.UserError(
			fmt.Sprintf("Cannot install MCP %s: agent.yaml is required for MCP configuration", pkg.Name),
			"Add MCP configuration to agent.yaml under the 'mcps' section.")
	}
	cfg := findPackageConfig(spec, types.PackageTypeMCP, pkg.Name)
	server := buildMCPServerFromConfig(cfg)
	if server.Command == "" && server.URL == "" {
		return agentenvError.UserError(
			fmt.Sprintf("Cannot install MCP %s: no command or URL in MCP config", pkg.Name),
			"Add a 'command' or 'url' field to the MCP config in agent.yaml.")
	}

	if err := a.InstallMCP(ctx, pkg.Name, server); err != nil {
		return agentenvError.SystemError(
			fmt.Sprintf("Cannot install MCP %s", pkg.Name),
			"Check the MCP configuration in agent.yaml.").WithCause(err)
	}
	return nil
}

// installAgentFromStore extracts an agent package and installs it.
func installAgentFromStore(ctx context.Context, a adapter.AgentAdapter,
	st *store.Store, pkg types.LockedPackage, slug, envDir string) error {
	data, err := st.Get(string(pkg.Type), slug, pkg.Resolved)
	if err != nil {
		return agentenvError.SystemError(
			fmt.Sprintf("Cannot read agent %s from store", pkg.Name),
			"The package may be corrupted. Run 'agentenv lock' again.").WithCause(err)
	}

	extractDir := filepath.Join(envDir, "installed", string(pkg.Type), pkg.Name)
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return agentenvError.SystemError(
			fmt.Sprintf("Cannot create extract directory for agent %s", pkg.Name),
			"Check file permissions and disk space.").WithCause(err)
	}
	if err := extractTarGz(data, extractDir); err != nil {
		return err
	}

	// Look for .md files in the extracted directory
	var mdPath string
		filepath.WalkDir(extractDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			mdPath = path
			return io.EOF // stop walking
		}
		return nil
	})

	if mdPath == "" {
		return agentenvError.UserError(
			fmt.Sprintf("Cannot install agent %s: no .md file found in package", pkg.Name),
			"The package may not contain a valid agent definition.")
	}

	if err := a.InstallAgent(ctx, pkg.Name, mdPath); err != nil {
		return agentenvError.SystemError(
			fmt.Sprintf("Cannot install agent %s", pkg.Name),
			"The agent file may be incompatible. Check the agent format.").WithCause(err)
	}
	return nil
}

// deactivateEnv deactivates the named environment without requiring a cobra Command.
func deactivateEnv(ctx context.Context, name string, root string) error {
	envDir := filepath.Join(root, "envs", name)

	state, err := readState(envDir)
	if err != nil {
		// Can't determine framework — try claude-code as default
		state = map[string]interface{}{"agent_framework": "claude-code"}
	}
	framework := agentFrameworkFromState(state)

	a, err := getAdapter(framework)
	if err != nil {
		return agentenvError.SystemError(
			fmt.Sprintf("No adapter available for %q", framework),
			"Supported frameworks: claude-code.").WithCause(err)
	}

	// Read manifest and remove all managed items
	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		return agentenvError.SystemError(
			fmt.Sprintf("Cannot read manifest for %s", name),
			"The manifest may be corrupted.").WithCause(err)
	}

	var removalErrors []string

	// Remove skills
	for skillName := range manifest.Skills {
		if err := a.RemoveSkill(ctx, skillName); err != nil {
			removalErrors = append(removalErrors, fmt.Sprintf("remove skill %s: %v", skillName, err))
		}
	}

	// Remove MCP servers
	for mcpName := range manifest.MCPServers {
		if err := a.RemoveMCP(ctx, mcpName); err != nil {
			removalErrors = append(removalErrors, fmt.Sprintf("remove mcp %s: %v", mcpName, err))
		}
	}

	// Remove agents
	for agentName := range manifest.Agents {
		if err := a.RemoveAgent(ctx, agentName); err != nil {
			removalErrors = append(removalErrors, fmt.Sprintf("remove agent %s: %v", agentName, err))
		}
	}

	// Restore pre-activation MCP config from the first backup (before any agentenv installs)
	// We restore only .mcp.json, NOT the manifest (the Remove calls already cleaned the manifest).
	backupDir := filepath.Join(root, "backups", framework)
	if entries, err := os.ReadDir(backupDir); err == nil && len(entries) > 0 {
		var earliest string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(name, ".json") && !strings.Contains(name, "-manifest.") {
				ts := strings.TrimSuffix(name, ".json")
				if earliest == "" || ts < earliest {
					earliest = ts
				}
			}
		}
		if earliest != "" {
			mcpBackup := filepath.Join(backupDir, earliest+".json")
			if data, err := os.ReadFile(mcpBackup); err == nil {
				mcpDir := filepath.Dir(a.GetSkillBasePath())
				mcpPath := filepath.Join(mcpDir, ".mcp.json")
				_ = os.WriteFile(mcpPath, data, 0o644)
			}
		}
	}

	// Remove ACTIVE lock
	activeLock := filepath.Join(root, "ACTIVE")
	if err := os.Remove(activeLock); err != nil && !os.IsNotExist(err) {
		removalErrors = append(removalErrors, fmt.Sprintf("remove ACTIVE lock: %v", err))
	}

	if len(removalErrors) > 0 {
		return agentenvError.SystemError(
			"Deactivation completed with errors",
			"Some packages could not be removed. Try manual cleanup.")
	}

	return nil
}

var internalActivateCmd = &cobra.Command{
	Use:    "_internal_activate <name>",
	Short:  "INTERNAL: activate an environment (called by shell function)",
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return activateEnv(cmd.Context(), args[0], cmd)
	},
}

func init() {
	rootCmd.AddCommand(internalActivateCmd)
}
