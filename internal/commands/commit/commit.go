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

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/obediencecorp/camp/pkg/commitkit"
	"github.com/spf13/cobra"
)

// Commit reference format prefix: OBEY-FE (Obey Festival component)
const commitRefPrefix = "OBEY-FE"

var (
	message      string
	taskRef      string
	festivalFlag string
	noTag        bool
	jsonOut      bool
	autoStage    bool
)

// NewCommitCommand creates the fest commit command
func NewCommitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Create git commit with task reference",
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		Long: `Create a git commit with the festival/task ID embedded in the message.

Works from any directory within a campaign workspace. The festival reference
is auto-detected when possible, or can be specified with --festival.

The fest commit command wraps git commit and automatically:
  1. Stages all changes (git add -A) unless --stage=false
  2. Prepends the festival reference to the commit message

Reference format: [OBEY-FE-{id}]
  - OBEY: Obey workflow tool prefix
  - FE: Festival component identifier
  - {id}: Task ref (FEST-xxxxxx) or festival ID (e.g., CS0001)

Detection priority:
  1. Explicit --task flag value
  2. Task fest_ref from current directory (if inside festival task)
  3. Festival ID from scope context (fest.yaml metadata)
  4. Explicit --festival flag (name or ID)
  5. Navigation link reverse-lookup (linked project detection)

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

	cmd.MarkFlagRequired("message")

	return cmd
}

// CommitResult represents the result of a commit operation
type CommitResult struct {
	Success     bool   `json:"success"`
	Hash        string `json:"hash,omitempty"`
	Message     string `json:"message"`
	TaskRef     string `json:"task_ref,omitempty"`
	CampaignTag string `json:"campaign_tag,omitempty"`
	Synced      bool   `json:"synced,omitempty"`
	Error       string `json:"error,omitempty"`
}

func runCommit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	result := &CommitResult{}

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

	// Auto-stage changes if enabled (default: true)
	if autoStage {
		if err := stageAllChanges(ctx); err != nil {
			result.Success = false
			result.Error = err.Error()
			return outputResult(result)
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
			} else {
				// Strategy 4: Navigation link reverse-lookup
				if fid, err := detectFestivalIDFromNavigation(ctx); err == nil && fid != "" {
					ref = formatCommitRef(fid)
				}
			}
		}
	}

	// Build fest-tagged commit message.
	festMessage := message
	if ref != "" {
		festMessage = fmt.Sprintf("[%s] %s", ref, message)
	}

	// Resolve workspace for campaign integration (nil-safe: ok if absent).
	ws, _ := scope.WorkspaceFrom(ctx)

	// Execute commit with optional campaign tag prepend and submodule sync.
	if err := commitWithCampaignSupport(ctx, ws, festMessage, result); err != nil {
		result.Success = false
		result.Error = err.Error()
		return outputResult(result)
	}

	result.Success = true
	result.TaskRef = ref

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
			if result.Synced {
				fmt.Printf("%s %s\n", ui.Label("Synced"), ui.Value("campaign root updated"))
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

// detectFestivalIDFromNavigation finds the festival linked to the current
// working directory (or a parent) via navigation links. This handles the case
// where CWD is inside a linked project but deeper than the exact link target.
func detectFestivalIDFromNavigation(ctx context.Context) (string, error) {
	ws, ok := scope.WorkspaceFrom(ctx)
	if !ok || ws == nil {
		return "", errors.NotFound("workspace")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "getting working directory")
	}

	nav, err := navigation.LoadNavigation()
	if err != nil {
		return "", errors.Wrap(err, "loading navigation")
	}

	festivalName := nav.FindFestivalForPath(cwd)
	if festivalName == "" {
		return "", errors.NotFound("navigation link for current directory")
	}

	// Search for the festival by name in status directories.
	for _, status := range id.StatusDirectories {
		festivalPath := filepath.Join(ws.FestivalsPath, status, festivalName)
		if info, err := os.Stat(festivalPath); err == nil && info.IsDir() {
			return loadFestivalID(festivalPath, ws.Root)
		}
	}

	return "", errors.NotFound(fmt.Sprintf("festival directory for %q", festivalName))
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
// integration. If the workspace is a campaign, it prepends [OBEY-CAMPAIGN-{id}]
// to the message and syncs the submodule ref after committing. Campaign
// detection or sync failures degrade gracefully — the commit still proceeds.
func commitWithCampaignSupport(ctx context.Context, ws *scope.WorkspaceInfo, festMessage string, result *CommitResult) error {
	commitMessage := festMessage

	var campaignID string
	if ws != nil && ws.Type == scope.WorkspaceTypeCampaign {
		cid, err := commitkit.DetectCampaign(ctx)
		if err == nil && cid != "" {
			campaignID = cid
			tag := commitkit.FormatCampaignTag(campaignID)
			commitMessage = tag + " " + festMessage
			result.CampaignTag = tag
		}
	}

	hash, err := executeGitCommit(ctx, commitMessage)
	if err != nil {
		return err
	}
	result.Hash = hash
	result.Message = commitMessage

	if campaignID != "" && ws != nil {
		relPath, relErr := resolveProjectRelPath(ws.Root)
		if relErr == nil {
			syncErr := commitkit.SyncSubmoduleRef(ctx, ws.Root, relPath, campaignID)
			if syncErr == nil {
				result.Synced = true
			}
		}
	}

	return nil
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
