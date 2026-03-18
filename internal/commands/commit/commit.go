// Package commit provides the fest commit command for git integration.
package commit

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/camp/pkg/commitkit"
	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

// Commit reference format prefix: OBEY-FE (Obey Festival component)
const commitRefPrefix = "OBEY-FE"

var (
	message          string
	taskRef          string
	festivalFlag     string
	noTag            bool
	jsonOut          bool
	autoStage        bool
	noRoot           bool
	syncSubmoduleRef bool // deprecated: kept for backward compat
)

// NewCommitCommand creates the fest commit command
func NewCommitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Create git commit with task reference",
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Long: `Create a git commit with the festival/task ID embedded in the message.

Requires festival context: either run from inside a festival directory,
a linked project directory (see 'fest link'), or use --festival to specify one.

The fest commit command wraps git commit and automatically:
  1. Stages changes and prepends the festival reference to the commit message
  2. Creates a campaign root commit for festival-scoped files (task docs, progress, state)

When run from a linked project directory, two commits are created:
  - Project commit: stages all project changes
  - Campaign root commit: stages only festival directory, .campaign/fest/,
    festivals/.festival/.state/, and the submodule pointer

When run from a festival directory, one commit is created at the campaign root
with only festival-scoped files staged (not git add -A).

Use --no-root to skip the campaign root commit.

Reference format: [OBEY-FE-{id}]
  - OBEY: Obey workflow tool prefix
  - FE: Festival component identifier
  - {id}: Task ref (FEST-xxxxxx) or festival ID (e.g., CS0001)

Detection priority:
  1. Explicit --task flag value
  2. Task fest_ref from current directory (if inside festival task)
  3. Festival ID from fest.yaml metadata
  4. Explicit --festival flag (name or ID)

Examples:
  fest commit -m "Implement feature"
  # In linked project → [OBEY-FE-CS0001] Implement feature
  # In festival task  → [OBEY-FE-FEST-a3b2c1] Implement feature

  fest commit --task FEST-b4c5d6 -m "Related work"
  # → [OBEY-FE-FEST-b4c5d6] Related work

  fest commit --festival OA0001 -m "Work from unlinked dir"
  # → [OBEY-FE-OA0001] Work from unlinked dir

  fest commit --no-tag -m "No reference"
  # → No reference

  fest commit --stage=false -m "Only commit staged"
  # Skip auto-staging, commit only what's already staged`,
		RunE: runCommit,
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "commit message")
	cmd.Flags().StringVar(&taskRef, "task", "", "task reference ID to use (e.g., FEST-a3b2c1)")
	cmd.Flags().StringVar(&festivalFlag, "festival", "", "festival name or ID (overrides auto-detection)")
	cmd.Flags().BoolVar(&noTag, "no-tag", false, "don't prepend task reference")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output result as JSON")
	cmd.Flags().BoolVar(&autoStage, "stage", true, "auto-stage all changes before commit")
	cmd.Flags().BoolVar(&noRoot, "no-root", false, "skip campaign root commit (project commit only)")
	cmd.Flags().BoolVar(&syncSubmoduleRef, "sync-submodule-ref", false, "deprecated: campaign root commit is now automatic")
	_ = cmd.Flags().MarkDeprecated("sync-submodule-ref", "campaign root commit is now automatic; use --no-root to skip")

	_ = cmd.MarkFlagRequired("message")

	return cmd
}

// CommitResult represents the result of a commit operation
type CommitResult struct {
	Success      bool   `json:"success"`
	Hash         string `json:"hash,omitempty"`
	Message      string `json:"message"`
	TaskRef      string `json:"task_ref,omitempty"`
	CampaignTag  string `json:"campaign_tag,omitempty"`
	CampaignHash string `json:"campaign_hash,omitempty"`
	Synced       bool   `json:"synced,omitempty"` // deprecated: kept for JSON compat
	Error        string `json:"error,omitempty"`
}

func runCommit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	result := &CommitResult{}

	// Resolve workspace for campaign integration (nil-safe: ok if absent).
	ws, _ := scope.WorkspaceFrom(ctx)

	// Determine if we're in a project submodule vs campaign root.
	var inSubmodule bool
	var submoduleRelPath string
	if ws != nil && ws.Type == scope.WorkspaceTypeCampaign {
		inSubmodule, submoduleRelPath = isInSubmodule(ws.Root)
	}

	festivalPath, hasFestival := scope.FestivalFrom(ctx)

	// Check if we're in a git repository
	inRepo, err := isGitRepo(ctx)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return outputResult(result)
	}
	if !inRepo {
		result.Success = false
		result.Error = "not in a git repository"
		return outputResult(result)
	}

	// Auto-stage changes if enabled (default: true).
	// When at campaign root (not in submodule), stage only festival-scoped paths.
	if autoStage {
		if inSubmodule {
			// In submodule: stage all changes in the project repo
			if err := stageAllChanges(ctx); err != nil {
				result.Success = false
				result.Error = err.Error()
				return outputResult(result)
			}
		} else if ws != nil && ws.Type == scope.WorkspaceTypeCampaign && hasFestival {
			// At campaign root: stage only festival-scoped paths
			paths, err := festivalScopedPaths(ws.Root, festivalPath, "")
			if err != nil {
				result.Success = false
				result.Error = err.Error()
				return outputResult(result)
			}
			if err := commitkit.StageFiles(ctx, ws.Root, paths...); err != nil {
				result.Success = false
				result.Error = errors.Wrap(err, "staging festival files").Error()
				return outputResult(result)
			}
		} else {
			// Fallback: stage all (non-campaign workspace)
			if err := stageAllChanges(ctx); err != nil {
				result.Success = false
				result.Error = err.Error()
				return outputResult(result)
			}
		}
	}

	// Get task reference via cascading detection strategies.
	var ref string
	if !noTag {
		if taskRef != "" {
			// Validate provided ref
			if !id.Validate(taskRef) {
				result.Success = false
				result.Error = fmt.Sprintf("invalid task reference format: %s (expected FEST-xxxxxx)", taskRef)
				return outputResult(result)
			}
			ref = formatCommitRef(taskRef)
		} else {
			// Strategy 1: Task ref from CWD within festival directory
			if r, err := detectCurrentTaskRef(ctx); err == nil && r != "" {
				ref = formatCommitRef(r)
			} else if fid, err := detectFestivalID(ctx); err == nil && fid != "" {
				// Strategy 2: Festival ID from scope context (fest.yaml)
				ref = formatCommitRef(fid)
			} else if festivalFlag != "" {
				// Strategy 3: Explicit --festival flag
				if fid, err := detectFestivalIDFromFlag(ctx, festivalFlag); err == nil && fid != "" {
					ref = formatCommitRef(fid)
				}
			}
		}
	}

	// Build commit message with consolidated tag.
	festMessage := message
	if ref != "" {
		festMessage = fmt.Sprintf("[%s] %s", ref, message)
	}

	// Execute commit with campaign tag support.
	if err := commitWithCampaignSupport(ctx, ws, ref, message, festMessage, result); err != nil {
		result.Success = false
		result.Error = err.Error()
		return outputResult(result)
	}

	result.Success = true
	result.TaskRef = ref

	// Campaign root commit: stage festival-scoped files and commit separately.
	// Only when we're in a submodule (the campaign root commit is a second commit).
	// When at the campaign root, the primary commit above already handled festival files.
	if !noRoot && inSubmodule && ws != nil && ws.Type == scope.WorkspaceTypeCampaign && hasFestival {
		rootHash, rootErr := commitFestivalAtRoot(ctx, ws, festivalPath, submoduleRelPath, result.CampaignTag, ref, message)
		if rootErr != nil {
			fmt.Fprintf(os.Stderr, "%s %s\n", ui.Warning("campaign root commit skipped:"), rootErr.Error())
		} else if rootHash != "" {
			result.CampaignHash = rootHash
		}
	}

	return outputResult(result)
}

func outputResult(result *CommitResult) error {
	if jsonOut {
		if err := shared.EncodeJSON(os.Stdout, result); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
	} else {
		if result.Success {
			fmt.Println(ui.H1("Commit"))
			fmt.Printf("%s %s\n", ui.Label("Hash"), ui.Value(result.Hash))
			fmt.Printf("%s %s\n", ui.Label("Message"), highlightTaskRefs(result.Message))
			if result.TaskRef != "" {
				fmt.Printf("%s %s\n", ui.Label("Task"), ui.Value(result.TaskRef, ui.TaskColor))
			}
			if result.CampaignTag != "" {
				fmt.Printf("%s %s\n", ui.Label("Campaign"), ui.Value(result.CampaignTag))
			}
			if result.CampaignHash != "" {
				fmt.Printf("%s %s\n", ui.Label("Root Commit"), ui.Value(result.CampaignHash))
			}
		} else {
			return errors.New(result.Error)
		}
	}
	return nil
}

func isGitRepo(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, errors.Wrap(err, "context cancelled")
	}
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return false, errors.Wrap(ctx.Err(), "context cancelled")
		}
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, errors.Wrap(err, "checking git repository")
	}
	return true, nil
}

func executeGitCommit(ctx context.Context, message string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", errors.Wrap(err, "context cancelled")
	}
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", errors.Wrap(err, "git commit failed")
	}

	// Get the commit hash
	hashCmd := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	var out bytes.Buffer
	hashCmd.Stdout = &out
	if err := hashCmd.Run(); err != nil {
		return "", errors.Wrap(err, "failed to get commit hash")
	}

	return strings.TrimSpace(out.String()), nil
}

// stageAllChanges runs git add -A to stage all changes
func stageAllChanges(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	cmd := exec.CommandContext(ctx, "git", "add", "-A")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "git add failed")
	}
	return nil
}

// formatCommitRef formats an ID with the standard commit reference prefix.
// Format: [OBEY-FE-{id}] where FE = Festival component
func formatCommitRef(id string) string {
	return fmt.Sprintf("%s-%s", commitRefPrefix, id)
}

// detectFestivalID retrieves the festival ID from fest.yaml metadata
func detectFestivalID(ctx context.Context) (string, error) {
	festivalPath, ok := scope.FestivalFrom(ctx)
	if !ok || festivalPath == "" {
		return "", errors.NotFound("festival")
	}

	cfg, err := config.LoadFestivalConfig(festivalPath, "")
	if err != nil {
		return "", errors.Wrap(err, "loading festival config")
	}

	if cfg.Metadata.ID == "" {
		return "", errors.NotFound("festival ID in metadata")
	}

	return cfg.Metadata.ID, nil
}

// detectFestivalIDFromFlag resolves a festival ID from the --festival flag value.
// It accepts either a festival ID (e.g., "OA0001") or a festival directory name
// (e.g., "obey-alignment-OA0001") and searches the workspace's festivals directory.
func detectFestivalIDFromFlag(ctx context.Context, flag string) (string, error) {
	ws, ok := scope.WorkspaceFrom(ctx)
	if !ok || ws == nil {
		return "", errors.NotFound("workspace")
	}

	// Search status directories for a matching festival.
	for _, status := range id.StatusDirectories {
		statusDir := filepath.Join(ws.FestivalsPath, status)
		entries, err := os.ReadDir(statusDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			festivalPath := filepath.Join(statusDir, entry.Name())

			// Match by directory name (e.g., "obey-alignment-OA0001")
			if entry.Name() == flag {
				return loadFestivalID(festivalPath, ws.Root)
			}

			// Match by ID suffix (e.g., directory ends with "-OA0001")
			if strings.HasSuffix(entry.Name(), "-"+flag) {
				return loadFestivalID(festivalPath, ws.Root)
			}

			// Match by loading config and comparing metadata.id
			if fid, err := loadFestivalID(festivalPath, ws.Root); err == nil && fid == flag {
				return fid, nil
			}
		}
	}

	return "", errors.NotFound(fmt.Sprintf("festival matching flag %q", flag))
}

// loadFestivalID loads fest.yaml from the given festival directory and returns
// the metadata.id value.
func loadFestivalID(festivalPath, campaignRoot string) (string, error) {
	cfg, err := config.LoadFestivalConfig(festivalPath, campaignRoot)
	if err != nil {
		return "", errors.Wrap(err, "loading festival config")
	}
	if cfg.Metadata.ID == "" {
		return "", errors.NotFound("festival ID in metadata")
	}
	return cfg.Metadata.ID, nil
}

// commitWithCampaignSupport executes the git commit with optional campaign
// integration. If the workspace is a campaign, it consolidates campaign and
// festival refs into a single tag: [OBEY-CAMPAIGN-{cid}-FE-{fid}].
// Campaign detection or sync failures degrade gracefully — the commit still proceeds.
func commitWithCampaignSupport(ctx context.Context, ws *scope.WorkspaceInfo, festRef, rawMsg, festMessage string, result *CommitResult) error {
	commitMessage := festMessage

	var campaignID string
	if ws != nil && ws.Type == scope.WorkspaceTypeCampaign {
		cid, err := commitkit.DetectCampaign(ctx)
		if err == nil && cid != "" {
			campaignID = cid
			if festRef != "" {
				// Consolidate: [OBEY-CAMPAIGN-{cid}-FE-{fid}] msg
				// instead of [OBEY-CAMPAIGN-{cid}] [OBEY-FE-{fid}] msg
				shortID := campaignID
				if len(shortID) > 8 {
					shortID = shortID[:8]
				}
				// festRef is "OBEY-FE-{id}" — strip the "OBEY-" prefix to avoid
				// redundancy in the consolidated tag.
				componentRef := strings.TrimPrefix(festRef, "OBEY-")
				tag := fmt.Sprintf("[OBEY-CAMPAIGN-%s-%s]", shortID, componentRef)
				commitMessage = tag + " " + rawMsg
				result.CampaignTag = tag
			} else {
				tag := commitkit.FormatCampaignTag(campaignID)
				commitMessage = tag + " " + festMessage
				result.CampaignTag = tag
			}
		}
	}

	hash, err := executeGitCommit(ctx, commitMessage)
	if err != nil {
		return err
	}
	result.Hash = hash
	result.Message = commitMessage

	return nil
}

// isInSubmodule returns true and the submodule-relative path if the current
// working directory is inside a git submodule of the campaign root.
func isInSubmodule(campaignRoot string) (bool, string) {
	relPath, err := resolveProjectRelPath(campaignRoot)
	if err != nil {
		return false, ""
	}
	return true, relPath
}

// festivalScopedPaths returns the campaign-root-relative paths that should be
// staged for a festival commit at the campaign root:
//   - The festival directory itself (task docs, .fest/progress_events.jsonl)
//   - .campaign/fest/ (navigation state)
//   - festivals/.festival/.state/ (global festival event log)
//   - The submodule pointer path (if submoduleRelPath is non-empty)
func festivalScopedPaths(campaignRoot, festivalPath, submoduleRelPath string) ([]string, error) {
	festivalRel, err := filepath.Rel(campaignRoot, festivalPath)
	if err != nil {
		return nil, errors.Wrap(err, "computing festival relative path")
	}

	paths := []string{
		festivalRel,
		filepath.Join(".campaign", "fest"),
		filepath.Join("festivals", ".festival", ".state"),
	}

	if submoduleRelPath != "" {
		paths = append(paths, submoduleRelPath)
	}

	return paths, nil
}

// commitCampaignRoot stages festival-scoped paths at the campaign root and
// commits them. Returns the short hash of the campaign commit, or empty string
// if there were no changes to commit. Errors from staging/committing are
// returned; "no changes" is a silent skip.
func commitCampaignRoot(ctx context.Context, campaignRoot string, paths []string, commitMessage string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", errors.Wrap(err, "context cancelled")
	}

	if err := commitkit.StageFiles(ctx, campaignRoot, paths...); err != nil {
		return "", errors.Wrap(err, "staging festival files at campaign root")
	}

	hasChanges, err := commitkit.HasStagedChanges(ctx, campaignRoot)
	if err != nil {
		return "", errors.Wrap(err, "checking staged changes at campaign root")
	}
	if !hasChanges {
		return "", nil
	}

	if err := commitkit.Commit(ctx, campaignRoot, commitkit.CommitOptions{
		Message: commitMessage,
	}); err != nil {
		return "", errors.Wrap(err, "committing festival files at campaign root")
	}

	hash, err := commitkit.ShortHash(ctx, campaignRoot)
	if err != nil {
		return "", errors.Wrap(err, "getting campaign root commit hash")
	}

	return hash, nil
}

// commitFestivalAtRoot orchestrates the campaign root commit for festival-scoped
// files. It computes the scoped paths, builds a tagged message with "fest:" prefix,
// and delegates to commitCampaignRoot.
func commitFestivalAtRoot(ctx context.Context, ws *scope.WorkspaceInfo, festivalPath, submoduleRelPath, campaignTag, festRef, rawMsg string) (string, error) {
	paths, err := festivalScopedPaths(ws.Root, festivalPath, submoduleRelPath)
	if err != nil {
		return "", err
	}

	// Build the campaign root commit message with "fest:" prefix.
	rootMsg := "fest: " + rawMsg

	// Apply the same campaign tag used for the project commit.
	if campaignTag != "" {
		rootMsg = campaignTag + " " + rootMsg
	} else if festRef != "" {
		rootMsg = fmt.Sprintf("[%s] %s", festRef, rootMsg)
	}

	return commitCampaignRoot(ctx, ws.Root, paths, rootMsg)
}

// resolveProjectRelPath returns the current git repository's path relative to
// campaignRoot. Used to identify which submodule pointer to update.
func resolveProjectRelPath(campaignRoot string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "getting working directory")
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "finding git root")
	}
	gitRoot := strings.TrimSpace(string(out))

	if gitRoot == campaignRoot {
		return "", errors.New("current directory is the campaign root, not a submodule")
	}

	relPath, err := filepath.Rel(campaignRoot, gitRoot)
	if err != nil {
		return "", errors.Wrap(err, "computing relative project path")
	}

	if strings.HasPrefix(relPath, "..") {
		return "", errors.New("project is outside the campaign root")
	}

	return relPath, nil
}

func detectCurrentTaskRef(ctx context.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Get festival path from scope context (resolved by PersistentPreRunE)
	festivalPath, ok := scope.FestivalFrom(ctx)
	if !ok || festivalPath == "" {
		return "", errors.NotFound("festival")
	}

	// Check if we're in a task directory
	relPath, err := filepath.Rel(festivalPath, cwd)
	if err != nil {
		return "", err
	}

	// If the path starts with ".." we're outside the festival directory
	// This means we can't detect a task ref - let caller fall back to festival ID
	if strings.HasPrefix(relPath, "..") {
		return "", errors.NotFound("task reference - outside festival directory")
	}

	// Walk up looking for a task file
	parts := strings.Split(relPath, string(os.PathSeparator))
	if len(parts) >= 3 {
		// We might be in sequence/task level
		seqDir := filepath.Join(festivalPath, parts[0], parts[1])
		entries, err := os.ReadDir(seqDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
					continue
				}
				if strings.HasPrefix(strings.ToUpper(entry.Name()), "SEQUENCE_") {
					continue
				}

				// Try to parse frontmatter from this task
				taskPath := filepath.Join(seqDir, entry.Name())
				content, err := os.ReadFile(taskPath)
				if err != nil {
					continue
				}

				fm, _, err := frontmatter.Parse(content)
				if err != nil || fm == nil {
					continue
				}

				if fm.Ref != "" {
					return fm.Ref, nil
				}
			}
		}
	}

	return "", errors.NotFound("task reference")
}
