package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/7emotions/agentenv/pkg/types"
)

// CursorAdapter implements AgentAdapter for Cursor IDE / Cursor Agent CLI.
//
// Cursor is an AI-first code editor built on top of VS Code. It stores its
// agent-level configuration under ~/.cursor/ (global) and .cursor/ (per-project).
//
// Config layout (global, ~/.cursor/):
//
//	~/.cursor/
//	├── mcp.json              — MCP servers (same "mcpServers" format as Claude Code)
//	├── cli-config.json       — CLI permissions and settings
//	├── permissions.json       — permissions config
//	├── skills/               — agent skills (one directory per skill, each with SKILL.md)
//	├── agents/               — agent definitions (.md files)
//	├── rules/                — agent rules (.mdc files)
//	└── commands/             — custom slash commands
//
// Project-level (.cursor/):
//
//	.cursor/
//	├── mcp.json              — project-scoped MCP servers
//	├── rules/                — project rules
//	├── agents/               — project agent definitions
//	├── commands/             — project commands
//	└── settings.json          — workspace settings
//
// The CursorAdapter manages the GLOBAL scope (~/.cursor/), analogous to how
// ClaudeCodeAdapter manages ~/.claude/ and OpenCodeAdapter manages ~/.config/opencode/.
//
// Cursor's MCP format is identical to Claude Code's: both use the "mcpServers"
// top-level key with command/args/env/url fields. The conversion functions from
// mcp.go (claudeMCPConfig, claudeToUniversal, universalToClaude) are reused.
type CursorAdapter struct {
	basePath string
	homePath string // home directory for backup path resolution; defaults to os.UserHomeDir()
}

// NewCursorAdapter creates a CursorAdapter targeting ~/.cursor/
// using the current user's home directory.
func NewCursorAdapter() *CursorAdapter {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return &CursorAdapter{
		basePath: filepath.Join(home, ".cursor"),
		homePath: home,
	}
}

// NewCursorAdapterWithBase creates a CursorAdapter with a custom
// base path. This is used for testing to avoid touching ~/.cursor/.
func NewCursorAdapterWithBase(basePath string) *CursorAdapter {
	return &CursorAdapter{basePath: basePath}
}

// NewCursorAdapterWithBaseAndHome creates a CursorAdapter with custom
// base and home paths for testing isolation.
func NewCursorAdapterWithBaseAndHome(basePath, homePath string) *CursorAdapter {
	return &CursorAdapter{basePath: basePath, homePath: homePath}
}

// Name returns "cursor".
func (a *CursorAdapter) Name() string { return "cursor" }

// DisplayName returns "Cursor".
func (a *CursorAdapter) DisplayName() string { return "Cursor" }

// GetSkillBasePath returns the skills directory path (~/.cursor/skills/).
func (a *CursorAdapter) GetSkillBasePath() string {
	return a.skillsDir()
}

// GetAgentBasePath returns the agents directory path (~/.cursor/agents/).
func (a *CursorAdapter) GetAgentBasePath() string {
	return a.agentsDir()
}

func (a *CursorAdapter) skillsDir() string {
	return filepath.Join(a.basePath, "skills")
}

func (a *CursorAdapter) agentsDir() string {
	return filepath.Join(a.basePath, "agents")
}

func (a *CursorAdapter) manifestPath() string {
	return filepath.Join(a.basePath, ".agentenv-manifest.json")
}

func (a *CursorAdapter) mcpConfigPath() string {
	return filepath.Join(a.basePath, "mcp.json")
}

func (a *CursorAdapter) homeDir() string {
	if a.homePath != "" {
		return a.homePath
	}
	home, _ := os.UserHomeDir()
	return home
}

func (a *CursorAdapter) backupDir() string {
	return filepath.Join(a.homeDir(), ".agentenv", "backups", "cursor")
}

// ---- Manifest ----

// ReadManifest reads the agentenv manifest from disk. If the file does
// not exist, it returns an empty Manifest (Version: 1) without error.
func (a *CursorAdapter) ReadManifest(_ context.Context) (*Manifest, error) {
	data, err := os.ReadFile(a.manifestPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Version: 1}, nil
		}
		return nil, fmt.Errorf("read manifest file: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

// WriteManifest writes the agentenv manifest to disk as indented JSON.
func (a *CursorAdapter) WriteManifest(_ context.Context, manifest *Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(a.manifestPath(), data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

// ---- Skills ----

// InstallSkill copies SKILL.md from sourcePath into
// ~/.cursor/skills/<name>/SKILL.md and records it in the manifest.
//
// If a skill directory with the same name already exists and is not tracked
// in the manifest, it returns an error to avoid overwriting user-managed skills.
// If the existing skill is managed by agentenv, the old directory is replaced.
func (a *CursorAdapter) InstallSkill(ctx context.Context, name, sourcePath string) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read skill source %q: %w", sourcePath, err)
	}

	skillsDir := a.skillsDir()
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}

	skillDir := filepath.Join(skillsDir, name)

	if _, err := os.Stat(skillDir); err == nil {
		manifest, err := a.ReadManifest(ctx)
		if err != nil {
			return fmt.Errorf("skill %q already exists and manifest cannot be read: %w", name, err)
		}
		if _, managed := manifest.Skills[name]; !managed {
			return fmt.Errorf("skill %q already exists and is not managed by agentenv", name)
		}
		if err := os.RemoveAll(skillDir); err != nil {
			return fmt.Errorf("remove existing skill dir: %w", err)
		}
	}

	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return fmt.Errorf("create skill dir: %w", err)
	}

	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, data, 0o644); err != nil {
		return fmt.Errorf("write SKILL.md: %w", err)
	}

	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		manifest = &Manifest{
			Version: 1,
			Skills:  make(map[string]ManifestItem),
		}
	}
	if manifest.Skills == nil {
		manifest.Skills = make(map[string]ManifestItem)
	}
	manifest.Skills[name] = ManifestItem{
		Name:        name,
		SourcePath:  sourcePath,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}
	return a.WriteManifest(ctx, manifest)
}

// RemoveSkill removes ~/.cursor/skills/<name>/ if it is managed by
// agentenv. If the skill is not in the manifest, it returns nil without
// modifying anything (preserving non-agentenv skills).
func (a *CursorAdapter) RemoveSkill(ctx context.Context, name string) error {
	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	if _, managed := manifest.Skills[name]; !managed {
		return nil
	}

	skillDir := filepath.Join(a.skillsDir(), name)
	if err := os.RemoveAll(skillDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove skill dir: %w", err)
	}

	delete(manifest.Skills, name)
	return a.WriteManifest(ctx, manifest)
}

// ListSkills returns the names of skills currently installed and managed by
// agentenv for Cursor, based on the manifest.
func (a *CursorAdapter) ListSkills(ctx context.Context) ([]string, error) {
	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	names := make([]string, 0, len(manifest.Skills))
	for name := range manifest.Skills {
		names = append(names, name)
	}
	return names, nil
}

// ---- Agents ----

// InstallAgent copies the agent definition file at sourcePath to
// ~/.cursor/agents/<name>.md and records it in the manifest.
func (a *CursorAdapter) InstallAgent(ctx context.Context, name string, sourcePath string) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read agent source %q: %w", sourcePath, err)
	}

	agentsDir := a.agentsDir()
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return fmt.Errorf("create agents dir: %w", err)
	}

	agentPath := filepath.Join(agentsDir, name+".md")
	if err := os.WriteFile(agentPath, data, 0o644); err != nil {
		return fmt.Errorf("write agent %q: %w", name, err)
	}

	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	if manifest.Agents == nil {
		manifest.Agents = make(map[string]ManifestItem)
	}
	manifest.Agents[name] = ManifestItem{
		Name:        name,
		SourcePath:  sourcePath,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}
	return a.WriteManifest(ctx, manifest)
}

// RemoveAgent removes ~/.cursor/agents/<name>.md if the agent is managed by
// agentenv. If the agent is not in the manifest, it returns nil without
// modifying anything (preserving non-agentenv agents).
func (a *CursorAdapter) RemoveAgent(ctx context.Context, name string) error {
	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	if _, managed := manifest.Agents[name]; !managed {
		return nil
	}

	agentPath := filepath.Join(a.agentsDir(), name+".md")
	if err := os.Remove(agentPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove agent %q: %w", name, err)
	}

	delete(manifest.Agents, name)
	return a.WriteManifest(ctx, manifest)
}

// ListAgents returns the names of agents currently installed and managed by
// agentenv for Cursor, based on the manifest.
func (a *CursorAdapter) ListAgents(ctx context.Context) ([]string, error) {
	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	names := make([]string, 0, len(manifest.Agents))
	for name := range manifest.Agents {
		names = append(names, name)
	}
	return names, nil
}

// ---- MCP ----

// ReadMCPConfig reads ~/.cursor/mcp.json and returns it in universal format.
// Cursor uses the same "mcpServers" format as Claude Code, so the shared
// conversion functions from mcp.go are reused.
func (a *CursorAdapter) ReadMCPConfig(_ context.Context) (*types.UniversalMCPConfig, error) {
	data, err := os.ReadFile(a.mcpConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &types.UniversalMCPConfig{
				Servers: make(map[string]types.UniversalMCPServer),
			}, nil
		}
		return nil, fmt.Errorf("read mcp.json: %w", err)
	}
	if len(data) == 0 {
		return &types.UniversalMCPConfig{
			Servers: make(map[string]types.UniversalMCPServer),
		}, nil
	}
	var cc claudeMCPConfig
	if err := json.Unmarshal(data, &cc); err != nil {
		return nil, fmt.Errorf("parse mcp.json: %w", err)
	}
	return claudeToUniversal(&cc), nil
}

// WriteMCPConfig converts a universal MCP config to Cursor format and writes
// it to ~/.cursor/mcp.json. Since Cursor uses the same "mcpServers" format as
// Claude Code, the shared conversion functions from mcp.go are reused.
func (a *CursorAdapter) WriteMCPConfig(_ context.Context, config *types.UniversalMCPConfig) error {
	cc := universalToClaude(config)
	data, err := json.MarshalIndent(cc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mcp.json: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(a.mcpConfigPath()), 0o755); err != nil {
		return fmt.Errorf("create .cursor dir: %w", err)
	}
	if err := os.WriteFile(a.mcpConfigPath(), data, 0o644); err != nil {
		return fmt.Errorf("write mcp.json: %w", err)
	}
	return nil
}

// InstallMCP adds (or updates) an MCP server in ~/.cursor/mcp.json and
// records it in the agentenv manifest. A backup is created before
// modification.
func (a *CursorAdapter) InstallMCP(ctx context.Context, name string, server types.UniversalMCPServer) error {
	uc, err := a.ReadMCPConfig(ctx)
	if err != nil {
		return fmt.Errorf("read MCP config: %w", err)
	}

	if _, err := a.Backup(ctx); err != nil {
		return fmt.Errorf("backup before install: %w", err)
	}

	uc.Servers[name] = server

	if err := a.WriteMCPConfig(ctx, uc); err != nil {
		return fmt.Errorf("write MCP config: %w", err)
	}

	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	if manifest.MCPServers == nil {
		manifest.MCPServers = make(map[string]ManifestItem)
	}
	manifest.MCPServers[name] = ManifestItem{
		Name:        name,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}
	return a.WriteManifest(ctx, manifest)
}

// RemoveMCP removes an MCP server from ~/.cursor/mcp.json and the
// agentenv manifest. A backup is created before modification.
func (a *CursorAdapter) RemoveMCP(ctx context.Context, name string) error {
	uc, err := a.ReadMCPConfig(ctx)
	if err != nil {
		return fmt.Errorf("read MCP config: %w", err)
	}

	if _, err := a.Backup(ctx); err != nil {
		return fmt.Errorf("backup before remove: %w", err)
	}

	delete(uc.Servers, name)

	if err := a.WriteMCPConfig(ctx, uc); err != nil {
		return fmt.Errorf("write MCP config: %w", err)
	}

	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	if manifest.MCPServers != nil {
		delete(manifest.MCPServers, name)
	}
	return a.WriteManifest(ctx, manifest)
}

// ---- Backup / Restore ----

// Backup copies ~/.cursor/mcp.json and the agentenv manifest to
// ~/.agentenv/backups/cursor/ with a timestamp-based filename.
// Returns the backup identifier (timestamp string).
func (a *CursorAdapter) Backup(_ context.Context) (string, error) {
	timestamp := time.Now().UTC().Format("20060102T150405")
	dir := a.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	configData, err := os.ReadFile(a.mcpConfigPath())
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read mcp.json for backup: %w", err)
	}
	configBackupPath := filepath.Join(dir, timestamp+".json")
	if err := os.WriteFile(configBackupPath, configData, 0o644); err != nil {
		return "", fmt.Errorf("write config backup: %w", err)
	}

	manifestData, err := os.ReadFile(a.manifestPath())
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read manifest for backup: %w", err)
	}
	manifestBackupPath := filepath.Join(dir, timestamp+"-manifest.json")
	if err := os.WriteFile(manifestBackupPath, manifestData, 0o644); err != nil {
		return "", fmt.Errorf("write manifest backup: %w", err)
	}

	return timestamp, nil
}

// Restore restores ~/.cursor/mcp.json and the manifest from a backup.
// Missing backup files are treated as non-fatal (the target files are removed).
func (a *CursorAdapter) Restore(_ context.Context, backupID string) error {
	dir := a.backupDir()

	configBackupPath := filepath.Join(dir, backupID+".json")
	configData, err := os.ReadFile(configBackupPath)
	if err != nil {
		if os.IsNotExist(err) {
			os.Remove(a.mcpConfigPath())
		} else {
			return fmt.Errorf("read config backup: %w", err)
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(a.mcpConfigPath()), 0o755); err != nil {
			return fmt.Errorf("create .cursor dir: %w", err)
		}
		if err := os.WriteFile(a.mcpConfigPath(), configData, 0o644); err != nil {
			return fmt.Errorf("restore mcp.json: %w", err)
		}
	}

	manifestBackupPath := filepath.Join(dir, backupID+"-manifest.json")
	manifestData, err := os.ReadFile(manifestBackupPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read manifest backup: %w", err)
		}
		os.Remove(a.manifestPath())
	} else {
		if err := os.WriteFile(a.manifestPath(), manifestData, 0o644); err != nil {
			return fmt.Errorf("restore manifest: %w", err)
		}
	}

	return nil
}

// Ensure CursorAdapter implements AgentAdapter.
var _ AgentAdapter = (*CursorAdapter)(nil)
