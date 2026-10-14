package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
)

const (
	// ConfigReposDir is the directory name for config repos
	ConfigReposDir = "config-repos"
	// ActiveConfigLink is the symlink name for active config
	ActiveConfigLink = "active"
	// RepoManifestFile is the manifest filename
	RepoManifestFile = "repos.json"
)

// ConfigRepo represents a user's configuration repository
type ConfigRepo struct {
	Name      string    `json:"name"`
	Source    string    `json:"source"`     // git URL or local path
	LocalPath string    `json:"local_path"` // ~/.obey/fest/config-repos/<name>
	IsGitRepo bool      `json:"is_git_repo"`
	LastSync  time.Time `json:"last_sync,omitempty"`
}

// ConfigRepoManifest stores the list of configured repos
type ConfigRepoManifest struct {
	Version int          `json:"version"`
	Active  string       `json:"active,omitempty"` // name of active repo
	Repos   []ConfigRepo `json:"repos"`
}

// ConfigReposPath returns the path to config repos directory
func ConfigReposPath() string {
	return filepath.Join(ConfigDir(), ConfigReposDir)
}

// ActiveConfigPath returns the path to the active config symlink
func ActiveConfigPath() string {
	return filepath.Join(ConfigDir(), ActiveConfigLink)
}

// RepoManifestPath returns the path to the repos manifest file
func RepoManifestPath() string {
	return filepath.Join(ConfigDir(), RepoManifestFile)
}

// LoadRepoManifest loads the repo manifest from disk
func LoadRepoManifest(ctx context.Context) (*ConfigRepoManifest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	manifestPath := RepoManifestPath()

	// Return empty manifest if file doesn't exist
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return &ConfigRepoManifest{
			Version: 1,
			Repos:   []ConfigRepo{},
		}, nil
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, errors.IO("reading repo manifest", err).WithField("path", manifestPath)
	}

	var manifest ConfigRepoManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, errors.Parse("parsing repo manifest", err).WithField("path", manifestPath)
	}

	return &manifest, nil
}

// SaveRepoManifest saves the repo manifest to disk
func SaveRepoManifest(ctx context.Context, manifest *ConfigRepoManifest) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	configDir := ConfigDir()
	if err := os.MkdirAll(configDir, dirPermissions); err != nil {
		return errors.IO("creating config directory", err).WithField("path", configDir)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshaling repo manifest")
	}

	manifestPath := RepoManifestPath()
	tmp, err := os.CreateTemp(configDir, ".repos-*.json.tmp")
	if err != nil {
		return errors.IO("creating repo manifest temporary file", err).WithField("path", manifestPath)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()
	if err := tmp.Chmod(filePermissions); err != nil {
		return errors.IO("setting repo manifest permissions", err).WithField("path", tmpPath)
	}
	if _, err := tmp.Write(data); err != nil {
		return errors.IO("writing repo manifest", err).WithField("path", tmpPath)
	}
	if err := tmp.Sync(); err != nil {
		return errors.IO("syncing repo manifest", err).WithField("path", tmpPath)
	}
	if err := tmp.Close(); err != nil {
		return errors.IO("closing repo manifest", err).WithField("path", tmpPath)
	}
	if err := os.Rename(tmpPath, manifestPath); err != nil {
		return errors.IO("replacing repo manifest", err).WithField("path", manifestPath)
	}

	return nil
}

// GetRepo returns a repo by name from the manifest
func (m *ConfigRepoManifest) GetRepo(name string) *ConfigRepo {
	for i := range m.Repos {
		if m.Repos[i].Name == name {
			return &m.Repos[i]
		}
	}
	return nil
}

// AddRepo adds a repo to the manifest (or updates if exists)
func (m *ConfigRepoManifest) AddRepo(repo ConfigRepo) {
	for i := range m.Repos {
		if m.Repos[i].Name == repo.Name {
			m.Repos[i] = repo
			return
		}
	}
	m.Repos = append(m.Repos, repo)
}

// RemoveRepo removes a repo from the manifest by name
func (m *ConfigRepoManifest) RemoveRepo(name string) bool {
	for i := range m.Repos {
		if m.Repos[i].Name == name {
			m.Repos = append(m.Repos[:i], m.Repos[i+1:]...)
			return true
		}
	}
	return false
}

// GetActiveRepo returns the currently active repo, or nil if none
func (m *ConfigRepoManifest) GetActiveRepo() *ConfigRepo {
	if m.Active == "" {
		return nil
	}
	return m.GetRepo(m.Active)
}

// RepoLocalPath returns the local path for a repo by name
func RepoLocalPath(name string) string {
	return filepath.Join(ConfigReposPath(), name)
}

// IsGitURL checks if a source string looks like a git URL
func IsGitURL(source string) bool {
	for _, prefix := range []string{"https://", "http://", "ssh://", "git://", "file://"} {
		if strings.HasPrefix(strings.ToLower(source), prefix) {
			return true
		}
	}
	// Git's SCP-style form is host:path or user@host:path. A Windows drive
	// path has its colon at index one and must be treated as a local source.
	colon := strings.IndexByte(source, ':')
	return colon > 1 && !strings.ContainsAny(source[:colon], `/\`)
}
