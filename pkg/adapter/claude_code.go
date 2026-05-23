package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ClaudeCodeAdapter implements AgentAdapter for Anthropic's Claude Code
// CLI (claude.ai). It manages skills under ~/.claude/skills/ and maintains
// a manifest at ~/.claude/.agentenv-manifest.json.
type ClaudeCodeAdapter struct {
	basePath string
}

// NewClaudeCodeAdapter creates a ClaudeCodeAdapter targeting ~/.claude/
// using the current user's home directory.
func NewClaudeCodeAdapter() *ClaudeCodeAdapter {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return &ClaudeCodeAdapter{basePath: filepath.Join(home, ".claude")}
}

// NewClaudeCodeAdapterWithBase creates a ClaudeCodeAdapter with a custom
// base path. This is used for testing to avoid touching the real ~/.claude/.
func NewClaudeCodeAdapterWithBase(basePath string) *ClaudeCodeAdapter {
	return &ClaudeCodeAdapter{basePath: basePath}
}

// Name returns "claude-code".
func (a *ClaudeCodeAdapter) Name() string { return "claude-code" }

// DisplayName returns "Claude Code".
func (a *ClaudeCodeAdapter) DisplayName() string { return "Claude Code" }

// GetSkillBasePath returns the skills directory path (~/.claude/skills/).
func (a *ClaudeCodeAdapter) GetSkillBasePath() string {
	return a.skillsDir()
}

func (a *ClaudeCodeAdapter) skillsDir() string {
	return filepath.Join(a.basePath, "skills")
}

func (a *ClaudeCodeAdapter) manifestPath() string {
	return filepath.Join(a.basePath, ".agentenv-manifest.json")
}

// InstallSkill creates a symlink from sourcePath into ~/.claude/skills/<name>/
// and records it in the manifest.
//
// If a skill with the same name already exists and is not tracked in the
// manifest, it returns an error to avoid overwriting user-managed skills.
// If the existing skill is managed by agentenv, the old symlink is replaced.
func (a *ClaudeCodeAdapter) InstallSkill(ctx context.Context, name, sourcePath string) error {
	skillsDir := a.skillsDir()
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}

	linkPath := filepath.Join(skillsDir, name)

	if _, err := os.Lstat(linkPath); err == nil {
		manifest, err := a.ReadManifest(ctx)
		if err != nil {
			return fmt.Errorf("skill %q already exists and manifest cannot be read: %w", name, err)
		}
		if _, managed := manifest.Skills[name]; !managed {
			return fmt.Errorf("skill %q already exists and is not managed by agentenv", name)
		}
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("remove existing symlink: %w", err)
		}
	}

	if err := os.Symlink(sourcePath, linkPath); err != nil {
		return fmt.Errorf("create symlink: %w", err)
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

// RemoveSkill removes the symlink at ~/.claude/skills/<name>/ if it is
// managed by agentenv. If the skill is not in the manifest, it returns
// nil without modifying anything (preserving non-agentenv skills).
func (a *ClaudeCodeAdapter) RemoveSkill(ctx context.Context, name string) error {
	manifest, err := a.ReadManifest(ctx)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	if _, managed := manifest.Skills[name]; !managed {
		return nil
	}

	linkPath := filepath.Join(a.skillsDir(), name)
	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove symlink: %w", err)
	}

	delete(manifest.Skills, name)
	return a.WriteManifest(ctx, manifest)
}

// ListSkills returns the names of skills currently installed and managed
// by agentenv, based on the manifest.
func (a *ClaudeCodeAdapter) ListSkills(ctx context.Context) ([]string, error) {
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

// ReadManifest reads the agentenv manifest from disk. If the file does
// not exist, it returns an empty Manifest (Version: 1) without error.
func (a *ClaudeCodeAdapter) ReadManifest(_ context.Context) (*Manifest, error) {
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
func (a *ClaudeCodeAdapter) WriteManifest(_ context.Context, manifest *Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(a.manifestPath(), data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

// agentsDir returns the agents directory path (~/.claude/agents/).
func (a *ClaudeCodeAdapter) agentsDir() string {
	return filepath.Join(a.basePath, "agents")
}

// InstallAgent copies the agent definition file at sourcePath to
// ~/.claude/agents/<name>.md and records it in the manifest.
func (a *ClaudeCodeAdapter) InstallAgent(ctx context.Context, name string, sourcePath string) error {
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

// RemoveAgent removes ~/.claude/agents/<name>.md if the agent is managed by
// agentenv. If the agent is not in the manifest, it returns nil without
// modifying anything (preserving non-agentenv agents).
func (a *ClaudeCodeAdapter) RemoveAgent(ctx context.Context, name string) error {
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
// agentenv, based on the manifest.
func (a *ClaudeCodeAdapter) ListAgents(ctx context.Context) ([]string, error) {
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

// GetAgentBasePath returns the agents directory path (~/.claude/agents/).
func (a *ClaudeCodeAdapter) GetAgentBasePath() string {
	return a.agentsDir()
}

// Ensure ClaudeCodeAdapter implements AgentAdapter.
var _ AgentAdapter = (*ClaudeCodeAdapter)(nil)
