package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/7emotions/agentenv/pkg/types"
)

// OpenCodeAdapter implements AgentAdapter for OpenCode CLI.
// It manages skills under basePath/skills/, agents under basePath/agents/,
// and a manifest at basePath/.agentenv-manifest.json.
type OpenCodeAdapter struct {
	basePath string
}

// NewOpenCodeAdapter creates an OpenCodeAdapter targeting ~/.config/opencode/
// using the current user's home directory.
func NewOpenCodeAdapter() *OpenCodeAdapter {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return &OpenCodeAdapter{basePath: filepath.Join(home, ".config", "opencode")}
}

// NewOpenCodeAdapterWithBase creates an OpenCodeAdapter with a custom
// base path. This is used for testing to avoid touching ~/.config/opencode/.
func NewOpenCodeAdapterWithBase(basePath string) *OpenCodeAdapter {
	return &OpenCodeAdapter{basePath: basePath}
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

// ---- Stubs for methods not yet implemented ----

func (a *OpenCodeAdapter) InstallSkill(ctx context.Context, name string, sourcePath string) error {
	return fmt.Errorf("not implemented")
}

func (a *OpenCodeAdapter) RemoveSkill(ctx context.Context, name string) error {
	return fmt.Errorf("not implemented")
}

func (a *OpenCodeAdapter) ListSkills(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *OpenCodeAdapter) InstallAgent(ctx context.Context, name string, sourcePath string) error {
	return fmt.Errorf("not implemented")
}

func (a *OpenCodeAdapter) RemoveAgent(ctx context.Context, name string) error {
	return fmt.Errorf("not implemented")
}

func (a *OpenCodeAdapter) ListAgents(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *OpenCodeAdapter) InstallMCP(ctx context.Context, name string, server types.UniversalMCPServer) error {
	return fmt.Errorf("not implemented")
}

func (a *OpenCodeAdapter) RemoveMCP(ctx context.Context, name string) error {
	return fmt.Errorf("not implemented")
}

func (a *OpenCodeAdapter) ReadMCPConfig(ctx context.Context) (*types.UniversalMCPConfig, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *OpenCodeAdapter) WriteMCPConfig(ctx context.Context, config *types.UniversalMCPConfig) error {
	return fmt.Errorf("not implemented")
}

func (a *OpenCodeAdapter) Backup(ctx context.Context) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (a *OpenCodeAdapter) Restore(ctx context.Context, backupID string) error {
	return fmt.Errorf("not implemented")
}

// Ensure OpenCodeAdapter implements AgentAdapter.
var _ AgentAdapter = (*OpenCodeAdapter)(nil)
