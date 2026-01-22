package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/github"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

type syncOptions struct {
	source  string
	branch  string
	force   bool
	timeout int
	retry   int
	dryRun  bool
}

// NewSyncCommand creates the sync command
func NewSyncCommand() *cobra.Command {
	opts := &syncOptions{}

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "System: Download latest fest templates from GitHub",
		Long: `Download the latest fest methodology templates from GitHub to ~/.config/fest/

This is a SYSTEM command that maintains fest itself, not your festival content.
It fetches the complete .festival/ template structure from the configured
repository and stores it locally for use with 'fest init' and 'fest system update'.

Run this periodically to get the latest methodology templates and documentation.`,
		Example: `  fest system sync                          # Use defaults from config
  fest system sync --source github.com/user/repo  # Sync from specific repo
  fest system sync --force                       # Overwrite existing cache`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(cmd.Context(), cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.source, "source", "", "GitHub repository URL")
	cmd.Flags().StringVar(&opts.branch, "branch", "", "Git branch to sync from (default: from config or 'main')")
	cmd.Flags().BoolVar(&opts.force, "force", false, "overwrite existing files without checking")
	cmd.Flags().IntVar(&opts.timeout, "timeout", 30, "timeout in seconds")
	cmd.Flags().IntVar(&opts.retry, "retry", 3, "number of retry attempts")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "show what would be downloaded")

	return cmd
}

func runSync(ctx context.Context, _ *cobra.Command, opts *syncOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Determine target directory
	targetDir := filepath.Join(config.ConfigDir(), "festivals")

	// Create UI handler
	display := ui.New(shared.IsNoColor(), shared.IsVerbose())

	// Load configuration
	cfg, err := config.Load(ctx, shared.GetConfigFile())
	if err != nil && opts.source == "" {
		return errors.Wrap(err, "no config found and no --source specified - use --source flag")
	}

	// Determine repository URL
	repoURL := opts.source
	if repoURL == "" && cfg != nil {
		repoURL = cfg.Repository.URL
	}
	if repoURL == "" {
		repoURL = config.DefaultRepositoryURL
	}

	// Determine branch: flag > config > default
	branch := opts.branch
	if branch == "" && cfg != nil && cfg.Repository.Branch != "" {
		branch = cfg.Repository.Branch
	}
	if branch == "" {
		branch = "main"
	}
	opts.branch = branch

	// Parse repository to get owner and repo
	owner, repo, err := parseRepoURLForSync(repoURL)
	if err != nil {
		return errors.Validation("invalid repository URL").WithField("url", repoURL)
	}

	// Determine repository path: config > default
	repoPath := config.DefaultRepoPath
	if cfg != nil && cfg.Repository.Path != "" {
		repoPath = cfg.Repository.Path
	}

	// Create downloader
	downloader := github.NewDownloader(repoURL, opts.branch, repoPath)
	downloader.SetTimeout(opts.timeout)
	downloader.SetRetry(opts.retry)

	// Check if directory exists and we're not forcing
	useGit := github.IsGitAvailable()
	var remoteSHA string // Used to store SHA after git sync

	if fileExists(targetDir) && !opts.force {
		display.Info("Checking for updates from %s...", repoURL)

		if useGit {
			// Git-based update check (works with private repos)
			hasUpdates, sha, err := downloader.CheckForUpdatesWithGit(targetDir)
			if err != nil {
				display.Warning("Failed to check for updates: %v", err)
				display.Info("Use --force to re-download")
				return nil
			}
			remoteSHA = sha

			if !hasUpdates {
				display.Success("Fest templates are up to date!")
				return nil
			}
			display.Info("Updates available (new commit: %s)", sha[:8])
		} else {
			// HTTP-based update check (public repos only)
			hasUpdates, changes, err := downloader.CheckForUpdates(owner, repo, targetDir)
			if err != nil {
				display.Warning("Failed to check for updates: %v", err)
				display.Info("Use --force to re-download")
				return nil
			}

			if !hasUpdates {
				display.Success("Fest templates are up to date!")
				return nil
			}

			// Show changes
			display.Info("\nUpdates available (%d changes):", len(changes))
			for _, change := range changes {
				display.Info("  %s", change)
			}
		}

		// Prompt user
		if !display.Confirm("\nDownload updates?") {
			display.Info("Sync cancelled")
			return nil
		}
	}

	if opts.dryRun {
		display.Info("DRY RUN: Would sync from %s (branch: %s)", repoURL, opts.branch)
		return nil
	}

	display.Info("Syncing from %s (branch: %s)...", repoURL, opts.branch)

	// Try git-based download first (works with SSH keys for private repos)
	// Fall back to HTTP API if git is not available
	var downloadErr error
	if useGit {
		// Get remote SHA if we don't have it yet (e.g., --force or first sync)
		if remoteSHA == "" {
			sha, err := downloader.GetRemoteSHA()
			if err != nil {
				display.Warning("Failed to get remote SHA: %v", err)
			} else {
				remoteSHA = sha
			}
		}

		display.Info("Using git clone (supports private repos with SSH keys)...")
		progressBar := display.NewProgressBar("Cloning", 3)
		downloadErr = downloader.DownloadWithGit(targetDir, func(current, total int64, status string) {
			progressBar.Update(current, total, status)
		})
		progressBar.Finish()

		if downloadErr != nil {
			display.Warning("Git clone failed: %v", downloadErr)
			display.Info("Falling back to HTTP API...")
		} else if remoteSHA != "" {
			// Store the SHA for future update checks
			if err := github.WriteLastSyncSHA(targetDir, remoteSHA); err != nil {
				display.Warning("Failed to save sync marker: %v", err)
			}
		}
	}

	// Fall back to HTTP API download if git failed or not available
	if downloadErr != nil || !useGit {
		progressBar := display.NewProgressBar("Downloading", -1)
		downloadErr = downloader.Download(targetDir, func(current, total int64, file string) {
			progressBar.Update(current, total, file)
		})
		progressBar.Finish()
	}

	if downloadErr != nil {
		return errors.IO("downloading templates", downloadErr).WithField("url", repoURL)
	}

	// Delete orphaned files (files that exist locally but not in remote)
	// Note: Only works with HTTP method, git clone replaces everything
	if opts.force && !useGit {
		display.Info("Removing orphaned files...")
		deleted, err := downloader.DeleteOrphaned(owner, repo, targetDir)
		if err != nil {
			display.Warning("Failed to clean up orphaned files: %v", err)
		} else if len(deleted) > 0 {
			display.Info("Removed %d orphaned files", len(deleted))
			if shared.IsVerbose() {
				for _, f := range deleted {
					display.Info("  - %s", f)
				}
			}
		}
	}

	// Update config with sync time
	if cfg != nil {
		cfg.LastSync = timeNow()
		if err := config.Save(ctx, cfg); err != nil {
			display.Warning("Failed to update config: %v", err)
		}
	}

	display.Success("Successfully synced fest templates to %s", targetDir)
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func timeNow() string {
	return time.Now().Format(time.RFC3339)
}

func parseRepoURLForSync(url string) (owner, repo string, err error) {
	// Remove protocol
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Remove github.com
	url = strings.TrimPrefix(url, "github.com/")

	// Split owner and repo
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return "", "", errors.Validation("invalid repository URL format")
	}

	owner = parts[0]
	repo = strings.TrimSuffix(parts[1], ".git")

	return owner, repo, nil
}
