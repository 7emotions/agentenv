// Package adapter provides agent-specific adapters for managing skills,
// MCP servers, and agents through a common interface.
package adapter

import (
	"context"

	"github.com/7emotions/agentenv/pkg/types"
)

// AgentAdapter defines the interface for agent-specific operations
// such as skill management and manifest tracking.
type AgentAdapter interface {
	// Name returns the unique identifier for this agent (e.g., "claude-code").
	Name() string

	// DisplayName returns a human-readable name for this agent.
	DisplayName() string

	// Skills

	// InstallSkill creates a symlink from sourcePath into the agent's
	// skills directory under the given name and records it in the manifest.
	InstallSkill(ctx context.Context, name string, sourcePath string) error

	// RemoveSkill removes a symlink from the agent's skills directory
	// if it is managed by agentenv, and updates the manifest.
	RemoveSkill(ctx context.Context, name string) error

	// ListSkills returns the names of skills currently installed
	// and managed by agentenv for this agent.
	ListSkills(ctx context.Context) ([]string, error)

	// GetSkillBasePath returns the base directory where skills are
	// installed for this agent.
	GetSkillBasePath() string

	// Manifest

	// ReadManifest reads the agentenv manifest for this agent from disk.
	// If the manifest does not exist, it returns an empty Manifest.
	ReadManifest(ctx context.Context) (*Manifest, error)

	// WriteManifest writes the agentenv manifest for this agent to disk.
	WriteManifest(ctx context.Context, manifest *Manifest) error

	// Agents

	// InstallAgent copies an agent definition file to the agent's agents
	// directory under the given name and records it in the manifest.
	InstallAgent(ctx context.Context, name string, sourcePath string) error

	// RemoveAgent removes an agent definition from the agent's agents
	// directory if it is managed by agentenv, and updates the manifest.
	RemoveAgent(ctx context.Context, name string) error

	// ListAgents returns the names of agent definitions currently installed
	// and managed by agentenv for this agent.
	ListAgents(ctx context.Context) ([]string, error)

	// GetAgentBasePath returns the base directory where agent definitions
	// are installed for this agent.
	GetAgentBasePath() string

	// MCP

	// InstallMCP adds or updates an MCP server entry in the agent's
	// MCP config and records it in the manifest.
	InstallMCP(ctx context.Context, name string, server types.UniversalMCPServer) error

	// RemoveMCP removes an MCP server entry from the agent's MCP config
	// and updates the manifest.
	RemoveMCP(ctx context.Context, name string) error

	// ReadMCPConfig reads the agent's MCP configuration and returns it
	// in universal format. Returns an empty config if the file does not exist.
	ReadMCPConfig(ctx context.Context) (*types.UniversalMCPConfig, error)

	// WriteMCPConfig converts the universal MCP configuration to the
	// agent-specific format and writes it to the agent's MCP config file.
	WriteMCPConfig(ctx context.Context, config *types.UniversalMCPConfig) error

	// Backup saves the agent's MCP config and manifest to agentenv's
	// backup directory. Returns the backup identifier (timestamp string).
	Backup(ctx context.Context) (string, error)

	// Restore restores the agent's MCP config and manifest from a backup.
	Restore(ctx context.Context, backupID string) error
}

// Manifest tracks agentenv-managed resources (skills, MCP servers, agents)
// for a single agent instance.
type Manifest struct {
	Version    int                     `json:"version"`
	EnvName    string                  `json:"env_name"`
	Skills     map[string]ManifestItem `json:"skills,omitempty"`
	MCPServers map[string]ManifestItem `json:"mcp_servers,omitempty"`
	Agents     map[string]ManifestItem `json:"agents,omitempty"`
}

// ManifestItem describes a single managed resource entry in the manifest.
type ManifestItem struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	SourcePath  string `json:"source_path"`
	InstalledAt string `json:"installed_at"`
}
