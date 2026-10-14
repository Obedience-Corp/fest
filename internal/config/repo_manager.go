package config

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/dirlink"
	"github.com/Obedience-Corp/fest/internal/errors"
)

// RepoManager handles config repo operations
type RepoManager struct {
	manifest     *ConfigRepoManifest
	saveManifest func(context.Context, *ConfigRepoManifest) error
}

// NewRepoManager creates a new repo manager
func NewRepoManager(ctx context.Context) (*RepoManager, error) {
	manifest, err := LoadRepoManifest(ctx)
	if err != nil {
		return nil, err
	}
	return &RepoManager{manifest: manifest, saveManifest: SaveRepoManifest}, nil
}

// Manifest returns the current manifest
func (rm *RepoManager) Manifest() *ConfigRepoManifest {
	return rm.manifest
}

// Add adds a new config repo (git clone or symlink local path)
func (rm *RepoManager) Add(ctx context.Context, name, source string) (*ConfigRepo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Check if name already exists
	if existing := rm.manifest.GetRepo(name); existing != nil {
		return nil, errors.Validation("config repo already exists").WithField("name", name)
	}

	// Ensure config-repos directory exists
	reposDir := ConfigReposPath()
	if err := os.MkdirAll(reposDir, dirPermissions); err != nil {
		return nil, errors.IO("creating config-repos directory", err).WithField("path", reposDir)
	}

	localPath := RepoLocalPath(name)
	isGitRepo := IsGitURL(source)

	if isGitRepo {
		// Git clone with context
		cmd := exec.CommandContext(ctx, "git", "clone", source, localPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, errors.IO("cloning repo", err).WithField("source", source)
		}
	} else {
		// Local path - create symlink
		absSource, err := filepath.Abs(source)
		if err != nil {
			return nil, errors.IO("resolving source path", err).WithField("source", source)
		}

		// Verify source exists
		info, err := os.Stat(absSource)
		if err != nil {
			return nil, errors.NotFound("source path").WithField("path", absSource)
		}
		if !info.IsDir() {
			return nil, errors.Validation("source path is not a directory").WithField("path", absSource)
		}

		if err := dirlink.Create(localPath, absSource); err != nil {
			return nil, errors.IO("creating config repo link", err).WithField("source", absSource).WithField("target", localPath)
		}
	}

	repo := ConfigRepo{
		Name:      name,
		Source:    source,
		LocalPath: localPath,
		IsGitRepo: isGitRepo,
		LastSync:  time.Now().UTC(),
	}

	rm.manifest.AddRepo(repo)
	if err := SaveRepoManifest(ctx, rm.manifest); err != nil {
		rm.manifest.RemoveRepo(name)
		if !repo.IsGitRepo {
			if cleanupErr := dirlink.Remove(localPath); cleanupErr != nil {
				return nil, stderrors.Join(err, errors.IO("cleaning up unregistered config repo link", cleanupErr).WithField("path", localPath))
			}
		}
		return nil, err
	}

	return &repo, nil
}

// Sync syncs a config repo (git pull for git repos, no-op for local)
func (rm *RepoManager) Sync(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	repo := rm.manifest.GetRepo(name)
	if repo == nil {
		return errors.NotFound("config repo").WithField("name", name)
	}

	if !repo.IsGitRepo {
		// Local symlink - nothing to sync
		repo.LastSync = time.Now().UTC()
		rm.manifest.AddRepo(*repo)
		return SaveRepoManifest(ctx, rm.manifest)
	}

	// Git pull with context
	cmd := exec.CommandContext(ctx, "git", "-C", repo.LocalPath, "pull")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.IO("syncing repo", err).WithField("name", name)
	}

	repo.LastSync = time.Now().UTC()
	rm.manifest.AddRepo(*repo)
	return SaveRepoManifest(ctx, rm.manifest)
}

// SyncAll syncs all config repos
func (rm *RepoManager) SyncAll(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var lastErr error
	for _, repo := range rm.manifest.Repos {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := rm.Sync(ctx, repo.Name); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Use sets a config repo as the active one (creates symlink)
func (rm *RepoManager) Use(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	repo := rm.manifest.GetRepo(name)
	if repo == nil {
		return errors.NotFound("config repo").WithField("name", name)
	}

	// Verify local path exists
	if _, err := os.Stat(repo.LocalPath); err != nil {
		return errors.NotFound("repo local path").WithField("path", repo.LocalPath)
	}

	activePath := ActiveConfigPath()
	previousActive := rm.manifest.Active

	err := dirlink.Replace(activePath, repo.LocalPath, func() error {
		rm.manifest.Active = name
		return SaveRepoManifest(ctx, rm.manifest)
	})
	if err != nil {
		rm.manifest.Active = previousActive
		return errors.IO("activating config repo link", err).
			WithField("name", name).
			WithField("source", repo.LocalPath).
			WithField("target", activePath)
	}
	return nil
}

// Remove removes a config repo
func (rm *RepoManager) Remove(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	repo := rm.manifest.GetRepo(name)
	if repo == nil {
		return errors.NotFound("config repo").WithField("name", name)
	}

	// Keep both paths recoverable until the manifest update is durable. In
	// particular, an active link must not disappear while repos.json still names
	// it as active after a failed save.
	var activeStage *stagedRepoPath
	var err error
	if rm.manifest.Active == name {
		activeStage, err = stageRepoPath(ActiveConfigPath(), true)
		if err != nil {
			return errors.IO("staging active config link", err)
		}
	}

	repoStage, err := stageRepoPath(repo.LocalPath, !repo.IsGitRepo)
	if err != nil {
		return stderrors.Join(errors.IO("staging config repo", err), activeStage.restore())
	}

	updated := *rm.manifest
	updated.Repos = append([]ConfigRepo(nil), rm.manifest.Repos...)
	updated.RemoveRepo(name)
	if updated.Active == name {
		updated.Active = ""
	}
	if err := rm.saveManifest(ctx, &updated); err != nil {
		return stderrors.Join(err, repoStage.restore(), activeStage.restore())
	}
	*rm.manifest = updated

	// The manifest no longer references these paths. A cleanup failure leaves
	// only a named staging directory to inspect, never an untracked deletion of
	// the external source behind a local link.
	return stderrors.Join(repoStage.discard(), activeStage.discard())
}

type stagedRepoPath struct {
	original string
	backup   string
	stageDir string
	link     bool
}

func stageRepoPath(path string, link bool) (*stagedRepoPath, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", path, err)
	}
	if link {
		kind, err := dirlink.Kind(path)
		if err != nil {
			return nil, err
		}
		if kind == dirlink.KindNone {
			return nil, fmt.Errorf("%w: %s", dirlink.ErrNotLink, path)
		}
	} else if !info.IsDir() {
		return nil, fmt.Errorf("cloned config repo is not a directory: %s", path)
	}

	stageDir, err := os.MkdirTemp(filepath.Dir(path), ".fest-remove-*")
	if err != nil {
		return nil, fmt.Errorf("create removal staging directory: %w", err)
	}
	backup := filepath.Join(stageDir, filepath.Base(path))
	if err := os.Rename(path, backup); err != nil {
		_ = os.Remove(stageDir)
		return nil, fmt.Errorf("stage %s: %w", path, err)
	}
	return &stagedRepoPath{original: path, backup: backup, stageDir: stageDir, link: link}, nil
}

func (s *stagedRepoPath) restore() error {
	if s == nil {
		return nil
	}
	if err := os.Rename(s.backup, s.original); err != nil {
		return fmt.Errorf("restore %s from %s: %w", s.original, s.backup, err)
	}
	if err := os.Remove(s.stageDir); err != nil {
		return fmt.Errorf("remove staging directory %s: %w", s.stageDir, err)
	}
	return nil
}

func (s *stagedRepoPath) discard() error {
	if s == nil {
		return nil
	}
	var err error
	if s.link {
		err = dirlink.Remove(s.backup)
	} else {
		err = os.RemoveAll(s.backup)
	}
	if err != nil {
		return fmt.Errorf("remove staged config repo path %s: %w", s.backup, err)
	}
	if err := os.Remove(s.stageDir); err != nil {
		return fmt.Errorf("remove staging directory %s: %w", s.stageDir, err)
	}
	return nil
}

// List returns all configured repos
func (rm *RepoManager) List() []ConfigRepo {
	return rm.manifest.Repos
}

// GetActive returns the currently active repo
func (rm *RepoManager) GetActive() *ConfigRepo {
	return rm.manifest.GetActiveRepo()
}

// GetActiveName returns the name of the active repo
func (rm *RepoManager) GetActiveName() string {
	return rm.manifest.Active
}

// ActiveUserPath returns the path to user/ directory in active config
// Returns empty string if no active config
func ActiveUserPath() string {
	activePath := ActiveConfigPath()
	if _, err := os.Stat(activePath); err != nil {
		return ""
	}
	userPath := filepath.Join(activePath, "user")
	if _, err := os.Stat(userPath); err != nil {
		return ""
	}
	return userPath
}

// ActiveFestivalsPath returns the path to festivals/.festival/ in active config
// Returns empty string if no active config
func ActiveFestivalsPath() string {
	activePath := ActiveConfigPath()
	if _, err := os.Stat(activePath); err != nil {
		return ""
	}
	festPath := filepath.Join(activePath, "festivals", ".festival")
	if _, err := os.Stat(festPath); err != nil {
		return ""
	}
	return festPath
}
