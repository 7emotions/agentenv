package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/7emotions/agentenv/pkg/types"
)

// OpenCodeAdapter implements AgentAdapter for OpenCode CLI.
// It manages skills under basePath/skills/, agents under basePath/agents/,
// and a manifest at basePath/.agentenv-manifest.json.
type OpenCodeAdapter struct {
	basePath string
	homePath string // home directory for backup path resolution; defaults to os.UserHomeDir()
}

// NewOpenCodeAdapter creates an OpenCodeAdapter targeting ~/.config/opencode/
// using the current user's home directory.
func NewOpenCodeAdapter() *OpenCodeAdapter {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return &OpenCodeAdapter{
		basePath: filepath.Join(home, ".config", "opencode"),
		homePath: home,
	}
}

// NewOpenCodeAdapterWithBase creates an OpenCodeAdapter with a custom
// base path. This is used for testing to avoid touching ~/.config/opencode/.
func NewOpenCodeAdapterWithBase(basePath string) *OpenCodeAdapter {
	return &OpenCodeAdapter{basePath: basePath}
}

// NewOpenCodeAdapterWithBaseAndHome creates an OpenCodeAdapter with custom
// base and home paths for testing.
func NewOpenCodeAdapterWithBaseAndHome(basePath, homePath string) *OpenCodeAdapter {
	return &OpenCodeAdapter{basePath: basePath, homePath: homePath}
}

// Name returns "opencode".
func (a *OpenCodeAdapter) Name() string { return "opencode" }

// DisplayName returns "OpenCode".
func (a *OpenCodeAdapter) DisplayName() string { return "OpenCode" }

// GetSkillBasePath returns the skills directory path (basePath/skills/).
func (a *OpenCodeAdapter) GetSkillBasePath() string {
	return a.skillsDir()
}

// GetAgentBasePath returns the agents directory path (basePath/agents/).
func (a *OpenCodeAdapter) GetAgentBasePath() string {
	return a.agentsDir()
}

func (a *OpenCodeAdapter) skillsDir() string {
	return filepath.Join(a.basePath, "skills")
}

func (a *OpenCodeAdapter) agentsDir() string {
	return filepath.Join(a.basePath, "agents")
}

func (a *OpenCodeAdapter) manifestPath() string {
	return filepath.Join(a.basePath, ".agentenv-manifest.json")
}

func (a *OpenCodeAdapter) configPath() string {
	return filepath.Join(a.basePath, "opencode.jsonc")
}

func (a *OpenCodeAdapter) homeDir() string {
	if a.homePath != "" {
		return a.homePath
	}
	home, _ := os.UserHomeDir()
	return home
}

func (a *OpenCodeAdapter) backupDir() string {
	return filepath.Join(a.homeDir(), ".agentenv", "backups", "opencode")
}

// ReadManifest reads the agentenv manifest from disk. If the file does
// not exist, it returns an empty Manifest (Version: 1) without error.
func (a *OpenCodeAdapter) ReadManifest(_ context.Context) (*Manifest, error) {
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
func (a *OpenCodeAdapter) WriteManifest(_ context.Context, manifest *Manifest) error {
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

func findSkillFile(dir string) ([]byte, error) {
	var mdPath string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			mdPath = path
			return io.EOF
		}
		return nil
	})
	if mdPath == "" {
		return nil, fmt.Errorf("no .md file found in %q", dir)
	}
	return os.ReadFile(mdPath)
}

// InstallSkill copies SKILL.md from sourcePath into
// ~/.config/opencode/skills/<name>/SKILL.md and records it in the manifest.
//
// If a skill directory with the same name already exists and is not tracked
// in the manifest, it returns an error to avoid overwriting user-managed skills.
// If the existing skill is managed by agentenv, the old directory is replaced.
func (a *OpenCodeAdapter) InstallSkill(ctx context.Context, name, sourcePath string) error {
	fi, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat skill source %q: %w", sourcePath, err)
	}

	var data []byte
	if fi.IsDir() {
		data, err = findSkillFile(sourcePath)
		if err != nil {
			return fmt.Errorf("cannot find skill file in %q: %w", sourcePath, err)
		}
	} else {
		data, err = os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("read skill source %q: %w", sourcePath, err)
		}
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

// RemoveSkill removes ~/.config/opencode/skills/<name>/ if it is managed by
// agentenv. If the skill is not in the manifest, it returns nil without
// modifying anything (preserving non-agentenv skills).
func (a *OpenCodeAdapter) RemoveSkill(ctx context.Context, name string) error {
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
// agentenv for OpenCode, based on the manifest.
func (a *OpenCodeAdapter) ListSkills(ctx context.Context) ([]string, error) {
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
// ~/.config/opencode/agents/<name>.md and records it in the manifest.
func (a *OpenCodeAdapter) InstallAgent(ctx context.Context, name string, sourcePath string) error {
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

// RemoveAgent removes ~/.config/opencode/agents/<name>.md if the agent is
// managed by agentenv. If the agent is not in the manifest, it returns nil
// without modifying anything (preserving non-agentenv agents).
func (a *OpenCodeAdapter) RemoveAgent(ctx context.Context, name string) error {
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
// agentenv for OpenCode, based on the manifest.
func (a *OpenCodeAdapter) ListAgents(ctx context.Context) ([]string, error) {
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

// ---- MCP types and helpers ----

// opencodeMCPServer represents a single MCP server entry inside opencode.jsonc.
// OpenCode embeds MCP config under the "mcp" key, using a format that differs
// from Claude Code's flat .mcp.json schema.
type opencodeMCPServer struct {
	Type        string            `json:"type"`                  // "local" or "remote"
	Command     []string          `json:"command,omitempty"`     // full command array (binary + args)
	URL         string            `json:"url,omitempty"`         // remote endpoint URL
	Enabled     *bool             `json:"enabled,omitempty"`     // whether the server is active
	Environment map[string]string `json:"environment,omitempty"` // env vars (OpenCode uses "environment", not "env")
	Timeout     int               `json:"timeout,omitempty"`     // startup timeout in ms
	Headers     map[string]string `json:"headers,omitempty"`     // HTTP headers for remote servers
	Description string            `json:"description,omitempty"`
}

// opencodeJSONC is the top-level structure of opencode.jsonc.
// It contains an "mcp" key holding the MCP server map. Only the "mcp"
// key is modeled here — other keys are preserved by the Write path.
type opencodeJSONC struct {
	MCP map[string]opencodeMCPServer `json:"mcp"`
}

// ---- Format conversion ----

// opencodeToUniversal converts OpenCode's nested "mcp" format to the
// canonical UniversalMCPConfig. Local servers map to stdio transport
// (command + args split from the array); remote servers map to sse.
func opencodeToUniversal(oc *opencodeJSONC) *types.UniversalMCPConfig {
	servers := make(map[string]types.UniversalMCPServer, len(oc.MCP))
	for name, s := range oc.MCP {
		us := types.UniversalMCPServer{
			Headers:     s.Headers,
			Description: s.Description,
		}
		switch s.Type {
		case "remote":
			us.Transport = "sse"
			us.URL = s.URL
		default: // "local" or empty → stdio
			us.Transport = "stdio"
			if len(s.Command) > 0 {
				us.Command = s.Command[0]
				us.Args = s.Command[1:]
			}
		}
		if s.Environment != nil {
			us.Env = s.Environment
		}
		servers[name] = us
	}
	return &types.UniversalMCPConfig{Servers: servers}
}

// universalToOpenCode converts a UniversalMCPConfig to OpenCode's
// nested "mcp" format. stdio servers become type "local" with the
// command and args merged into a single array; sse/http become "remote".
func universalToOpenCode(uc *types.UniversalMCPConfig) *opencodeJSONC {
	servers := make(map[string]opencodeMCPServer, len(uc.Servers))
	for name, us := range uc.Servers {
		s := opencodeMCPServer{
			Description: us.Description,
			Headers:     us.Headers,
		}
		if us.Transport == "sse" || us.Transport == "http" {
			s.Type = "remote"
			s.URL = us.URL
		} else {
			s.Type = "local"
			if us.Command != "" {
				s.Command = append([]string{us.Command}, us.Args...)
			}
		}
		if us.Env != nil {
			s.Environment = us.Env
		}
		// Default to enabled when writing so the server is active.
		enabled := true
		s.Enabled = &enabled
		servers[name] = s
	}
	return &opencodeJSONC{MCP: servers}
}

// ---- MCP methods ----

// ReadMCPConfig reads opencode.jsonc, extracts the "mcp" key, and
// converts each server to the universal format. Returns an empty config
// when the file does not exist or has no "mcp" key.
func (a *OpenCodeAdapter) ReadMCPConfig(_ context.Context) (*types.UniversalMCPConfig, error) {
	var oc opencodeJSONC
	if err := ReadAndUnmarshalJSONC(a.configPath(), &oc); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &types.UniversalMCPConfig{
				Servers: make(map[string]types.UniversalMCPServer),
			}, nil
		}
		return nil, fmt.Errorf("read opencode.jsonc: %w", err)
	}

	if len(oc.MCP) == 0 {
		return &types.UniversalMCPConfig{
			Servers: make(map[string]types.UniversalMCPServer),
		}, nil
	}
	return opencodeToUniversal(&oc), nil
}

// WriteMCPConfig converts the universal MCP config to OpenCode format
// and writes it into opencode.jsonc. Existing top-level keys outside "mcp"
// are preserved by reading the file first and only updating the "mcp" key.
//
// NOTE: JSONC comments are NOT preserved on write — the file is written
// as standard indented JSON.
func (a *OpenCodeAdapter) WriteMCPConfig(_ context.Context, config *types.UniversalMCPConfig) error {
	existing := make(map[string]json.RawMessage)
	if data, err := os.ReadFile(a.configPath()); err == nil {
		if uerr := json.Unmarshal(data, &existing); uerr != nil {
			existing = make(map[string]json.RawMessage)
		}
	}

	oc := universalToOpenCode(config)
	mcpData, err := json.Marshal(oc.MCP)
	if err != nil {
		return fmt.Errorf("marshal MCP servers: %w", err)
	}
	existing["mcp"] = mcpData

	result, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal opencode.jsonc: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(a.configPath()), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(a.configPath(), append(result, '\n'), 0o644); err != nil {
		return fmt.Errorf("write opencode.jsonc: %w", err)
	}
	return nil
}

// InstallMCP adds (or updates) an MCP server in opencode.jsonc and
// records it in the agentenv manifest. A backup is created before
// modification.
func (a *OpenCodeAdapter) InstallMCP(ctx context.Context, name string, server types.UniversalMCPServer) error {
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

// RemoveMCP removes an MCP server from opencode.jsonc and the
// agentenv manifest. A backup is created before modification.
func (a *OpenCodeAdapter) RemoveMCP(ctx context.Context, name string) error {
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

func (a *OpenCodeAdapter) Backup(_ context.Context) (string, error) {
	timestamp := time.Now().UTC().Format("20060102T150405")
	dir := a.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	configData, err := os.ReadFile(a.configPath())
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read opencode.jsonc for backup: %w", err)
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

func (a *OpenCodeAdapter) Restore(_ context.Context, backupID string) error {
	dir := a.backupDir()

	configBackupPath := filepath.Join(dir, backupID+".json")
	configData, err := os.ReadFile(configBackupPath)
	if err != nil {
		if os.IsNotExist(err) {
			os.Remove(a.configPath())
		} else {
			return fmt.Errorf("read config backup: %w", err)
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(a.configPath()), 0o755); err != nil {
			return fmt.Errorf("create opencode config dir: %w", err)
		}
		if err := os.WriteFile(a.configPath(), configData, 0o644); err != nil {
			return fmt.Errorf("restore opencode.jsonc: %w", err)
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

// Ensure OpenCodeAdapter implements AgentAdapter.
var _ AgentAdapter = (*OpenCodeAdapter)(nil)
