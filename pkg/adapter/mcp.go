package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agentenv/agentenv/pkg/types"
)

type claudeMCPConfig struct {
	MCPServers map[string]claudeMCPServer `json:"mcpServers"`
}

type claudeMCPServer struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (a *ClaudeCodeAdapter) mcpConfigPath() string {
	return filepath.Join(a.basePath, ".mcp.json")
}

func (a *ClaudeCodeAdapter) homeDir() string {
	return filepath.Dir(a.basePath)
}

func (a *ClaudeCodeAdapter) backupDir() string {
	return filepath.Join(a.homeDir(), ".agentenv", "backups", "claude-code")
}

func claudeToUniversal(cc *claudeMCPConfig) *types.UniversalMCPConfig {
	servers := make(map[string]types.UniversalMCPServer, len(cc.MCPServers))
	for name, s := range cc.MCPServers {
		us := types.UniversalMCPServer{
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
			URL:     s.URL,
			Headers: s.Headers,
		}
		if s.Command != "" {
			us.Transport = "stdio"
		} else if s.URL != "" {
			us.Transport = "sse"
		}
		servers[name] = us
	}
	return &types.UniversalMCPConfig{Servers: servers}
}

func universalToClaude(uc *types.UniversalMCPConfig) *claudeMCPConfig {
	servers := make(map[string]claudeMCPServer, len(uc.Servers))
	for name, us := range uc.Servers {
		servers[name] = claudeMCPServer{
			Command: us.Command,
			Args:    us.Args,
			Env:     us.Env,
			URL:     us.URL,
			Headers: us.Headers,
		}
	}
	return &claudeMCPConfig{MCPServers: servers}
}

// ReadMCPConfig reads the Claude Code .mcp.json and returns it in universal format.
func (a *ClaudeCodeAdapter) ReadMCPConfig(_ context.Context) (*types.UniversalMCPConfig, error) {
	data, err := os.ReadFile(a.mcpConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &types.UniversalMCPConfig{
				Servers: make(map[string]types.UniversalMCPServer),
			}, nil
		}
		return nil, fmt.Errorf("read .mcp.json: %w", err)
	}
	if len(data) == 0 {
		return &types.UniversalMCPConfig{
			Servers: make(map[string]types.UniversalMCPServer),
		}, nil
	}
	var cc claudeMCPConfig
	if err := json.Unmarshal(data, &cc); err != nil {
		return nil, fmt.Errorf("parse .mcp.json: %w", err)
	}
	return claudeToUniversal(&cc), nil
}

// WriteMCPConfig converts a universal MCP config to Claude Code format and writes it.
func (a *ClaudeCodeAdapter) WriteMCPConfig(_ context.Context, config *types.UniversalMCPConfig) error {
	cc := universalToClaude(config)
	data, err := json.MarshalIndent(cc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .mcp.json: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(a.mcpConfigPath()), 0o755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}
	if err := os.WriteFile(a.mcpConfigPath(), data, 0o644); err != nil {
		return fmt.Errorf("write .mcp.json: %w", err)
	}
	return nil
}

// InstallMCP adds an MCP server to the Claude Code config and tracks it in the manifest.
func (a *ClaudeCodeAdapter) InstallMCP(ctx context.Context, name string, config types.UniversalMCPServer) error {
	uc, err := a.ReadMCPConfig(ctx)
	if err != nil {
		return fmt.Errorf("read MCP config: %w", err)
	}

	if _, err := a.Backup(ctx); err != nil {
		return fmt.Errorf("backup before install: %w", err)
	}

	uc.Servers[name] = config

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

// RemoveMCP removes an MCP server from the Claude Code config and manifest.
func (a *ClaudeCodeAdapter) RemoveMCP(ctx context.Context, name string) error {
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

// Backup copies the .mcp.json and manifest to the backup directory.
func (a *ClaudeCodeAdapter) Backup(_ context.Context) (string, error) {
	timestamp := time.Now().UTC().Format("20060102T150405")
	dir := a.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	mcpPath := a.mcpConfigPath()
	mcpData, err := os.ReadFile(mcpPath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read .mcp.json for backup: %w", err)
	}
	mcpBackupPath := filepath.Join(dir, timestamp+".json")
	if err := os.WriteFile(mcpBackupPath, mcpData, 0o644); err != nil {
		return "", fmt.Errorf("write mcp backup: %w", err)
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

// Restore restores .mcp.json and manifest from a backup.
func (a *ClaudeCodeAdapter) Restore(_ context.Context, backupID string) error {
	dir := a.backupDir()

	mcpBackupPath := filepath.Join(dir, backupID+".json")
	mcpData, err := os.ReadFile(mcpBackupPath)
	if err != nil {
		if os.IsNotExist(err) {
			os.Remove(a.mcpConfigPath())
		} else {
			return fmt.Errorf("read mcp backup: %w", err)
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(a.mcpConfigPath()), 0o755); err != nil {
			return fmt.Errorf("create .claude dir: %w", err)
		}
		if err := os.WriteFile(a.mcpConfigPath(), mcpData, 0o644); err != nil {
			return fmt.Errorf("restore .mcp.json: %w", err)
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
