package adapter

// === Research Summary: OpenAI Codex CLI ===
//
// OpenAI Codex is a local CLI + IDE extension with:
//   - Config:  ~/.codex/config.toml (TOML format)
//   - MCP:     [mcp_servers] table in config.toml, using either flat fields
//              (command/args/url) or a nested transport = {type, command, args, url}
//              inline table for both stdio and streamable-http transports.
//   - Auth:    ~/.codex/auth.json
//   - Project: .codex/config.toml (trusted projects only)
//   - No local skills directory — skills are managed via config.toml profiles.
//   - No local agents directory  — agents are configured via config.toml profiles.
//   - CLI:     codex mcp add/remove/list, codex features enable/disable
//
// This adapter targets the user-level config at ~/.codex/config.toml.
// Skills and agents methods return descriptive errors because Codex does not
// support file-based local skill/agent management. MCP methods are fully
// implemented with bidirectional format conversion.
//
// References:
//   - https://developers.openai.com/codex/config-basic
//   - https://developers.openai.com/codex/config-sample
//   - https://openai-codex.mintlify.app/configuration/mcp-servers
//   - https://github.com/openai/codex/pull/12718 (local mcp.json support)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/7emotions/agentenv/pkg/types"
)

// CodexAdapter implements AgentAdapter for OpenAI Codex CLI.
//
// It manages MCP servers under ~/.codex/config.toml [mcp_servers] and a
// manifest at ~/.codex/.agentenv-manifest.json. Skills and agents are NOT
// supported because Codex does not expose file-system-based skill/agent
// directories — those features are configured via config.toml profiles.
type CodexAdapter struct {
	basePath string
	homePath string // home directory for backup path resolution; defaults to os.UserHomeDir()
}

// NewCodexAdapter creates a CodexAdapter targeting ~/.codex/ using the
// current user's home directory.
func NewCodexAdapter() *CodexAdapter {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return &CodexAdapter{
		basePath: filepath.Join(home, ".codex"),
		homePath: home,
	}
}

// NewCodexAdapterWithBase creates a CodexAdapter with a custom base path.
// This is used for testing to avoid touching ~/.codex/.
func NewCodexAdapterWithBase(basePath string) *CodexAdapter {
	return &CodexAdapter{basePath: basePath}
}

// NewCodexAdapterWithBaseAndHome creates a CodexAdapter with custom base
// and home paths for testing isolation.
func NewCodexAdapterWithBaseAndHome(basePath, homePath string) *CodexAdapter {
	return &CodexAdapter{basePath: basePath, homePath: homePath}
}

// Name returns "codex".
func (a *CodexAdapter) Name() string { return "codex" }

// DisplayName returns "Codex".
func (a *CodexAdapter) DisplayName() string { return "Codex" }

// GetSkillBasePath returns an empty string — Codex does not support a local
// skills directory.
func (a *CodexAdapter) GetSkillBasePath() string { return "" }

// GetAgentBasePath returns an empty string — Codex does not support a local
// agents directory.
func (a *CodexAdapter) GetAgentBasePath() string { return "" }

func (a *CodexAdapter) configPath() string {
	return filepath.Join(a.basePath, "config.toml")
}

func (a *CodexAdapter) manifestPath() string {
	return filepath.Join(a.basePath, ".agentenv-manifest.json")
}

func (a *CodexAdapter) homeDir() string {
	if a.homePath != "" {
		return a.homePath
	}
	home, _ := os.UserHomeDir()
	return home
}

func (a *CodexAdapter) backupDir() string {
	return filepath.Join(a.homeDir(), ".agentenv", "backups", "codex")
}

// ---- Manifest ----

// ReadManifest reads the agentenv manifest from disk. If the file does not
// exist, it returns an empty Manifest (Version: 1) without error.
func (a *CodexAdapter) ReadManifest(_ context.Context) (*Manifest, error) {
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
func (a *CodexAdapter) WriteManifest(_ context.Context, manifest *Manifest) error {
	if err := os.MkdirAll(a.basePath, 0o755); err != nil {
		return fmt.Errorf("create codex dir: %w", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(a.manifestPath(), data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

// ---- Skills (unsupported) ----

// InstallSkill always returns an error because Codex does not support
// local file-based skill management. Skills are configured via config.toml
// profiles under the [profiles] table.
func (a *CodexAdapter) InstallSkill(_ context.Context, _ string, _ string) error {
	return fmt.Errorf("Codex does not support local file-based skill management; skills are configured via config.toml profiles")
}

// RemoveSkill always returns an error because Codex does not support
// local file-based skill management.
func (a *CodexAdapter) RemoveSkill(_ context.Context, _ string) error {
	return fmt.Errorf("Codex does not support local file-based skill management; skills are configured via config.toml profiles")
}

// ListSkills always returns an empty slice — Codex does not support
// local file-based skill management.
func (a *CodexAdapter) ListSkills(_ context.Context) ([]string, error) {
	return []string{}, nil
}

// ---- Agents (unsupported) ----

// InstallAgent always returns an error because Codex does not support
// local file-based agent definitions. Agents are configured via config.toml
// profiles under the [profiles] table.
func (a *CodexAdapter) InstallAgent(_ context.Context, _ string, _ string) error {
	return fmt.Errorf("Codex does not support local file-based agent definitions; agents are configured via config.toml profiles")
}

// RemoveAgent always returns an error because Codex does not support
// local file-based agent definitions.
func (a *CodexAdapter) RemoveAgent(_ context.Context, _ string) error {
	return fmt.Errorf("Codex does not support local file-based agent definitions; agents are configured via config.toml profiles")
}

// ListAgents always returns an empty slice — Codex does not support
// local file-based agent definitions.
func (a *CodexAdapter) ListAgents(_ context.Context) ([]string, error) {
	return []string{}, nil
}

// ---- MCP types ----

// codexMCPServerTOML represents a single MCP server entry under
// [mcp_servers.<name>] in config.toml. It supports both the flat format
// (command/args/url at top level) and the transport format (nested
// transport = {type, command, args, url} inline table).
type codexMCPServerTOML struct {
	Command     string            `toml:"command"`
	Args        []string          `toml:"args"`
	URL         string            `toml:"url"`
	Env         map[string]string `toml:"env"`
	Enabled     *bool             `toml:"enabled"`
	BearerToken string            `toml:"bearer_token_env_var"`
	HTTPHeaders map[string]string `toml:"http_headers"`
	Description string            `toml:"description"`

	// Transport format (newer): transport = { type = "stdio", command = "...", args = [...] }
	Transport *codexTransportTOML `toml:"transport"`
}

// codexTransportTOML represents the nested [mcp_servers.<name>.transport]
// inline table for the newer MCP config format.
type codexTransportTOML struct {
	Type    string   `toml:"type"`
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
	URL     string   `toml:"url"`
}

// isEnabled returns true if the server is enabled (or not explicitly disabled).
// Servers without an explicit enabled field are considered enabled by default.
func (s *codexMCPServerTOML) isEnabled() bool {
	if s.Enabled == nil {
		return true
	}
	return *s.Enabled
}

// ---- Format conversion ----

// codexToUniversal converts Codex's [mcp_servers] format to the canonical
// UniversalMCPConfig. For each server it prefers the transport table (newer
// format) and falls back to top-level command/url fields (older format).
// Servers with enabled=false are skipped.
func codexToUniversal(servers map[string]codexMCPServerTOML) *types.UniversalMCPConfig {
	result := make(map[string]types.UniversalMCPServer, len(servers))
	for name, s := range servers {
		if !s.isEnabled() {
			continue
		}
		us := types.UniversalMCPServer{
			Description: s.Description,
		}

		// Determine transport: prefer transport table, fall back to flat fields.
		if s.Transport != nil {
			switch s.Transport.Type {
			case "stdlib", "": // empty type with command → stdio
				us.Transport = "stdio"
				us.Command = s.Transport.Command
				us.Args = s.Transport.Args
			case "streamable_http":
				us.Transport = "sse"
				us.URL = s.Transport.URL
			default:
				// Unknown type, try to infer from fields.
				if s.Transport.Command != "" {
					us.Transport = "stdio"
					us.Command = s.Transport.Command
					us.Args = s.Transport.Args
				} else if s.Transport.URL != "" {
					us.Transport = "sse"
					us.URL = s.Transport.URL
				}
			}
		} else if s.Command != "" {
			us.Transport = "stdio"
			us.Command = s.Command
			us.Args = s.Args
		} else if s.URL != "" {
			us.Transport = "sse"
			us.URL = s.URL
		}

		// Collect env vars from both formats.
		if s.Env != nil {
			us.Env = s.Env
		}
		// For the transport format, env is at the server level.
		if s.HTTPHeaders != nil {
			us.Headers = s.HTTPHeaders
		}

		result[name] = us
	}
	return &types.UniversalMCPConfig{Servers: result}
}

// universalToCodex converts a UniversalMCPConfig to Codex's [mcp_servers]
// format using the modern transport table representation.
func universalToCodex(uc *types.UniversalMCPConfig) map[string]codexMCPServerTOML {
	servers := make(map[string]codexMCPServerTOML, len(uc.Servers))
	for name, us := range uc.Servers {
		enabled := true
		s := codexMCPServerTOML{
			Enabled:     &enabled,
			Description: us.Description,
			Env:         us.Env,
			HTTPHeaders: us.Headers,
		}

		switch us.Transport {
		case "sse", "http":
			s.Transport = &codexTransportTOML{
				Type: "streamable_http",
				URL:  us.URL,
			}
		default: // "stdio" or empty
			s.Transport = &codexTransportTOML{
				Type:    "stdio",
				Command: us.Command,
				Args:    us.Args,
			}
		}

		servers[name] = s
	}
	return servers
}

// ---- MCP methods ----

// ReadMCPConfig reads ~/.codex/config.toml, extracts the [mcp_servers]
// table, and converts each server to the universal format. Returns an empty
// config when the file does not exist or has no [mcp_servers] section.
func (a *CodexAdapter) ReadMCPConfig(_ context.Context) (*types.UniversalMCPConfig, error) {
	empty := &types.UniversalMCPConfig{
		Servers: make(map[string]types.UniversalMCPServer),
	}

	data, err := os.ReadFile(a.configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return empty, nil
		}
		return nil, fmt.Errorf("read config.toml: %w", err)
	}

	var cfg struct {
		MCPServers map[string]codexMCPServerTOML `toml:"mcp_servers"`
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		// If the file exists but can't be parsed as TOML with our struct,
		// return empty rather than failing — the file might have a schema
		// we don't fully model.
		return empty, nil
	}

	if len(cfg.MCPServers) == 0 {
		return empty, nil
	}

	return codexToUniversal(cfg.MCPServers), nil
}

// WriteMCPConfig converts the universal MCP config to the Codex TOML format
// and writes it into ~/.codex/config.toml. Existing top-level keys outside
// [mcp_servers] are preserved by decoding first into a generic map, updating
// only the mcp_servers key, and re-encoding.
//
// NOTE: TOML comments and formatting are NOT preserved on write — the file
// is re-serialized by the BurntSushi/toml encoder.
func (a *CodexAdapter) WriteMCPConfig(_ context.Context, config *types.UniversalMCPConfig) error {
	// Read existing config as generic map to preserve non-MCP keys.
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(a.configPath()); err == nil {
		// Use a second decode into a raw map. The BurntSushi/toml decoder
		// populates map[string]interface{} with nested maps/slices/primitives.
		_ = toml.Unmarshal(data, &existing)
	}

	// Build the new mcp_servers section as a map of maps for TOML encoding.
	newServers := universalToCodex(config)
	serversMap := make(map[string]interface{}, len(newServers))
	for name, s := range newServers {
		entry := map[string]interface{}{}
		if s.Enabled != nil {
			entry["enabled"] = *s.Enabled
		}
		entry["description"] = s.Description
		if s.Env != nil {
			entry["env"] = s.Env
		}
		if s.HTTPHeaders != nil {
			entry["http_headers"] = s.HTTPHeaders
		}
		if s.Transport != nil {
			entry["transport"] = map[string]interface{}{
				"type":    s.Transport.Type,
				"command": s.Transport.Command,
				"args":    s.Transport.Args,
				"url":     s.Transport.URL,
			}
		}
		serversMap[name] = entry
	}
	existing["mcp_servers"] = serversMap

	if err := os.MkdirAll(a.basePath, 0o755); err != nil {
		return fmt.Errorf("create codex dir: %w", err)
	}

	f, err := os.Create(a.configPath())
	if err != nil {
		return fmt.Errorf("create config.toml: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(existing); err != nil {
		return fmt.Errorf("encode config.toml: %w", err)
	}

	return nil
}

// InstallMCP adds (or updates) an MCP server in ~/.codex/config.toml and
// records it in the agentenv manifest. A backup is created before
// modification.
func (a *CodexAdapter) InstallMCP(ctx context.Context, name string, server types.UniversalMCPServer) error {
	uc, err := a.ReadMCPConfig(ctx)
	if err != nil {
		return fmt.Errorf("read MCP config: %w", err)
	}

	if _, err := a.Backup(ctx); err != nil {
		return fmt.Errorf("backup before install: %w", err)
	}

	if uc.Servers == nil {
		uc.Servers = make(map[string]types.UniversalMCPServer)
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

// RemoveMCP removes an MCP server from ~/.codex/config.toml and the
// agentenv manifest. A backup is created before modification.
func (a *CodexAdapter) RemoveMCP(ctx context.Context, name string) error {
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

// Backup saves the Codex config.toml and agentenv manifest to
// ~/.agentenv/backups/codex/. Returns the backup identifier (UTC timestamp).
func (a *CodexAdapter) Backup(_ context.Context) (string, error) {
	timestamp := time.Now().UTC().Format("20060102T150405")
	dir := a.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	// Backup config.toml (may not exist on first run).
	configData, err := os.ReadFile(a.configPath())
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read config.toml for backup: %w", err)
	}
	configBackupPath := filepath.Join(dir, timestamp+".toml")
	if err := os.WriteFile(configBackupPath, configData, 0o644); err != nil {
		return "", fmt.Errorf("write config backup: %w", err)
	}

	// Backup manifest (may not exist on first run).
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

// Restore restores Codex config.toml and the agentenv manifest from a
// backup. Missing backup files are treated as non-fatal (the corresponding
// target file is removed if no backup exists).
func (a *CodexAdapter) Restore(_ context.Context, backupID string) error {
	dir := a.backupDir()

	// Restore config.toml.
	configBackupPath := filepath.Join(dir, backupID+".toml")
	configData, err := os.ReadFile(configBackupPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// No backup → remove the current config if it exists.
			os.Remove(a.configPath())
		} else {
			return fmt.Errorf("read config backup: %w", err)
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(a.configPath()), 0o755); err != nil {
			return fmt.Errorf("create codex config dir: %w", err)
		}
		if err := os.WriteFile(a.configPath(), configData, 0o644); err != nil {
			return fmt.Errorf("restore config.toml: %w", err)
		}
	}

	// Restore manifest.
	manifestBackupPath := filepath.Join(dir, backupID+"-manifest.json")
	manifestData, err := os.ReadFile(manifestBackupPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
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

// Ensure CodexAdapter implements AgentAdapter.
var _ AgentAdapter = (*CodexAdapter)(nil)
