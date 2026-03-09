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
	"github.com/Obedience-Corp/fest/internal/version"
	"github.com/spf13/cobra"
)

type syncOptions struct {
	source  string
	branch  string
	tag     string
	channel string
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
		Annotations: map[string]string{
			"agent_allowed": "false",
			"agent_reason":  "Downloads from GitHub, requires network and human judgment",
		},
		Long: `Download the latest fest methodology templates from GitHub to ~/.obey/fest/

This is a SYSTEM command that maintains fest itself, not your festival content.
It fetches the complete .festival/ template structure from the configured
repository and stores it locally for use with 'fest init' and 'fest system update'.

Run this periodically to get the latest methodology templates and documentation.`,
		Example: `  fest system sync                              # Use defaults (channel-based)
  fest system sync --channel stable               # Sync latest stable tag
  fest system sync --tag v0.2.0                   # Sync exact tag
  fest system sync --branch main                  # Sync from branch
  fest system sync --source github.com/user/repo  # Sync from specific repo
  fest system sync --force                        # Overwrite existing cache`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(cmd.Context(), cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.source, "source", "", "GitHub repository URL")
	cmd.Flags().StringVar(&opts.branch, "branch", "", "Git branch to sync from (default: from config or 'main')")
	cmd.Flags().StringVar(&opts.tag, "tag", "", "Exact git tag to sync from (e.g., v0.2.0)")
	cmd.Flags().StringVar(&opts.channel, "channel", "", "Release channel: stable or dev")
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

	// Resolve ref intent: CLI flags → config → release-profile default
	var repoConfig config.Repository
	if cfg != nil {
		repoConfig = cfg.Repository
	}
	intent := config.ResolveRefIntent(opts.branch, opts.tag, opts.channel, repoConfig)

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

	// Resolve channel intent to a concrete tag before creating downloader
	var resolvedRefType, resolvedRefName string
	var downloader *github.Downloader
	if intent.Mode == config.SyncModeChannel {
		channel := intent.Value
		if channel == "" {
			channel = version.DefaultChannel()
		}
		tag, tagErr := github.ResolveLatestTag(repoURL, channel)
		if tagErr != nil {
			return errors.Wrapf(tagErr, "resolving latest %s tag", channel)
		}
		display.Info("Resolved %s channel to tag %s", channel, tag)
		resolvedRefType = "tag"
		resolvedRefName = tag
		downloader = github.NewDownloaderWithRef(repoURL, "tag", tag, repoPath)
	} else if intent.Mode == config.SyncModeTag {
		resolvedRefType = "tag"
		resolvedRefName = intent.Value
		downloader = github.NewDownloaderWithRef(repoURL, "tag", intent.Value, repoPath)
	} else {
		resolvedRefType = "branch"
		resolvedRefName = intent.Value
		downloader = github.NewDownloaderWithRef(repoURL, "branch", intent.Value, repoPath)
	}
	downloader.SetTimeout(opts.timeout)
	downloader.SetRetry(opts.retry)

	// Check if directory exists and we're not forcing
	useGit := github.IsGitAvailable()
	var remoteSHA string               // Used to store SHA after git sync
	var newSyncState *github.SyncState // Used when templates actually changed

	if fileExists(targetDir) && !opts.force {
		display.Info("Checking for updates from %s...", repoURL)

		if useGit {
			// Stage 1: Quick check using commit SHA (works with private repos)
			hasCommitChanges, sha, err := downloader.CheckForUpdatesWithGit(targetDir)
			if err != nil {
				display.Warning("Failed to check for updates: %v", err)
				display.Info("Use --force to re-download")
				return nil
			}
			remoteSHA = sha

			if !hasCommitChanges {
				display.Success("Fest templates are up to date!")
				return nil
			}

			// Stage 2: Full check - commit SHA changed, but templates may be the same
			display.Info("New commits detected (commit: %s), checking templates...", sha[:8])
			hasTemplateChanges, newState, err := downloader.CheckForTemplateUpdates(targetDir, sha)
			if err != nil {
				display.Warning("Failed to check template changes: %v", err)
				display.Info("Proceeding with full sync...")
				// Continue with full sync since we couldn't determine if templates changed
			} else if !hasTemplateChanges {
				// Templates haven't changed - just update the commit SHA
				display.Success("Fest templates are up to date (no template changes)!")
				if newState != nil {
					if err := github.WriteSyncState(targetDir, newState); err != nil {
						display.Warning("Failed to save sync state: %v", err)
					}
				}
				return nil
			} else {
				newSyncState = newState
				display.Info("Template updates available")
			}
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
		display.Info("DRY RUN: Would sync from %s (%s: %s)", repoURL, resolvedRefType, resolvedRefName)
		return nil
	}

	display.Info("Syncing from %s (%s: %s)...", repoURL, resolvedRefType, resolvedRefName)

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
		} else {
			// Store the sync state for future update checks
			if newSyncState != nil {
				// We already computed the content hash during update check; attach ref info
				newSyncState.RefType = resolvedRefType
				newSyncState.RefName = resolvedRefName
				if err := github.WriteSyncState(targetDir, newSyncState); err != nil {
					display.Warning("Failed to save sync state: %v", err)
				}
			} else if remoteSHA != "" {
				// Compute content hash for new/forced syncs
				contentHash, hashErr := github.ComputeContentHash(targetDir)
				if hashErr != nil {
					display.Warning("Failed to compute content hash: %v", hashErr)
					// Fall back to just commit SHA, preserving ref info
					fallbackState := &github.SyncState{
						CommitSHA: remoteSHA,
						RefType:   resolvedRefType,
						RefName:   resolvedRefName,
					}
					if err := github.WriteSyncState(targetDir, fallbackState); err != nil {
						display.Warning("Failed to save sync marker: %v", err)
					}
				} else {
					state := &github.SyncState{
						CommitSHA:   remoteSHA,
						ContentHash: contentHash,
						RefType:     resolvedRefType,
						RefName:     resolvedRefName,
					}
					if err := github.WriteSyncState(targetDir, state); err != nil {
						display.Warning("Failed to save sync state: %v", err)
					}
				}
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
	// Note: Git method handles this in DownloadWithGit; HTTP method needs explicit cleanup
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
