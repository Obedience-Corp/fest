// Package commit provides the fest commit command for git integration.
package commit

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/camp/pkg/commitkit"
	"github.com/Obedience-Corp/fest/internal/activity"
	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/guidance/selection"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/pathutil"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

var (
	message          string
	taskRef          string
	festivalFlag     string
	noTag            bool
	jsonOut          bool
	autoStage        bool
	autoWrite        bool
	noRoot           bool
	commitLarge      bool
	commitNested     bool
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

When a festival or sequence has a linked project, up to two commits are created
even when this command is run from inside the festival:
  - Project commit: stages all project changes (skipped when the project is clean)
  - Campaign root commit: stages only festival directory, .campaign/fest/,
    festivals/.festival/.state/, and the submodule pointer

The sequence's fest_working_dir is preferred over the festival navigation link
and legacy fest.yaml project_path. A festival with no linked project creates one
campaign-root commit containing only festival-scoped files (not git add -A).

Use --no-root to skip the campaign root commit.

Reference format: [FE-{id}]
  - FE: Festival component identifier
  - {id}: Task ref (FEST-123456) or festival ID (e.g., CS0001)

Optional position segments: [FE-{id}-PH-{phase}-SQ-{sequence}]
  PH- and SQ- are additive: they record the phase and sequence the commit
  belongs to, and appear only when that position is unambiguous. The current
  directory decides it; otherwise the single in-progress sequence does.
  Parallel sequences leave both segments off, and SQ- is never emitted
  without PH-.

Detection priority:
  1. Explicit --task flag value
  2. Task fest_ref from current directory (if inside festival task)
  3. Explicit --festival flag (path, name, or ID)
  4. Festival ID from fest.yaml metadata

Examples:
  fest commit -m "Implement feature"
  # In linked project or sequence → [FE-CS0001] Implement feature
  # In festival task              → [FE-FEST-123456] Implement feature

  fest commit --task FEST-234567 -m "Related work"
  # → [FE-FEST-234567] Related work

  fest commit --festival OA0001 -m "Work from unlinked dir"
  # → [FE-OA0001] Work from unlinked dir

  fest commit -m "Update scaffold"
  # Inside 001_IMPLEMENT/02_camp_pilot → [FE-CC0008-PH-001-SQ-02] Update scaffold

  fest commit --no-tag -m "No reference"
  # → No reference

  fest commit --stage=false -m "Only commit staged"
  # Skip auto-staging, commit only what's already staged

  fest commit --auto-write
  # Run the configured campaign commit-message hook from the target repo`,
		RunE: runCommit,
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "commit message (required unless --auto-write)")
	cmd.Flags().StringVar(&taskRef, "task", "", "task reference ID to use (e.g., FEST-123456)")
	cmd.Flags().StringVar(&festivalFlag, "festival", "", "festival path, name, or ID (overrides auto-detection)")
	cmd.Flags().BoolVar(&noTag, "no-tag", false, "don't prepend task reference")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output result as JSON")
	cmd.Flags().BoolVar(&autoStage, "stage", true, "auto-stage all changes before commit")
	cmd.Flags().BoolVar(&autoWrite, "auto-write", false, "run configured commit message writer")
	cmd.Flags().BoolVar(&commitLarge, "commit-large", false, "commit over-threshold files instead of keeping them out of git")
	cmd.Flags().BoolVar(&commitNested, "commit-nested", false, "commit undeclared nested git repositories as gitlinks instead of keeping them out of git")
	cmd.Flags().BoolVar(&noRoot, "no-root", false, "skip campaign root commit (project commit only)")
	cmd.Flags().BoolVar(&syncSubmoduleRef, "sync-submodule-ref", false, "deprecated: campaign root commit is now automatic")
	_ = cmd.Flags().MarkDeprecated("sync-submodule-ref", "campaign root commit is now automatic; use --no-root to skip")

	return cmd
}

// CommitResult represents the result of a commit operation
type CommitResult struct {
	Success      bool   `json:"success"`
	Hash         string `json:"hash,omitempty"`
	Message      string `json:"message"`
	TaskRef      string `json:"task_ref,omitempty"`
	Phase        string `json:"phase,omitempty"`
	Sequence     string `json:"sequence,omitempty"`
	CampaignTag  string `json:"campaign_tag,omitempty"`
	CampaignHash string `json:"campaign_hash,omitempty"`
	Synced       bool   `json:"synced,omitempty"` // deprecated: kept for JSON compat
	Error        string `json:"error,omitempty"`
}

func runCommit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	result := &CommitResult{}

	if autoWrite && message != "" {
		result.Success = false
		result.Error = "--auto-write cannot be used with --message"
		return outputResult(result)
	}
	if !autoWrite && message == "" {
		result.Success = false
		result.Error = "required flag(s) \"message\" not set"
		return outputResult(result)
	}

	// Resolve workspace for campaign integration (nil-safe: ok if absent).
	ws, _ := scope.WorkspaceFrom(ctx)

	festivalPath, hasFestival := scope.FestivalFrom(ctx)
	if festivalFlag != "" {
		var resolveErr error
		festivalPath, resolveErr = resolveFestivalFlagPath(ctx, ws, festivalFlag)
		if resolveErr != nil {
			result.Success = false
			result.Error = resolveErr.Error()
			return outputResult(result)
		}
		hasFestival = true
		ctx = scope.WithFestival(ctx, festivalPath)
		cmd.SetContext(ctx)
	}

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

	primaryRepoPath, err := currentGitRoot(ctx)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return outputResult(result)
	}

	// A sequence's working directory and a festival's project link are explicit
	// commit targets. Resolve them while the caller is still in the festival so
	// the command can preserve festival context without requiring a manual cd.
	projectCommit := false
	var submoduleRelPath string
	if ws != nil && ws.Type == scope.WorkspaceTypeCampaign {
		primaryRepoPath, projectCommit, submoduleRelPath, err = resolveCommitTarget(
			ctx, ws.Root, primaryRepoPath, festivalPath, hasFestival,
		)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			return outputResult(result)
		}
	}

	// Guard outcomes and refusals go to the command's stderr so --json stdout
	// stays a pure document.
	report := cmd.ErrOrStderr()

	// Auto-stage changes if enabled (default: true).
	// When at campaign root (not in submodule), stage only festival-scoped paths.
	if autoStage {
		if projectCommit {
			// In a project: stage all changes in the project repo.
			if err := stageAllChanges(ctx, primaryRepoPath, report); err != nil {
				result.Success = false
				result.Error = err.Error()
				return outputResult(result)
			}
		} else if ws != nil && ws.Type == scope.WorkspaceTypeCampaign && hasFestival {
			// At campaign root: stage only festival-scoped paths (with outcome
			// reporting so large/bulk exclusions are not silent).
			paths, err := festivalScopedPaths(ctx, ws.Root, festivalPath, "")
			if err != nil {
				result.Success = false
				result.Error = err.Error()
				return outputResult(result)
			}
			// An empty list means git can match no festival-scoped path, so
			// there is nothing to stage. Passing it on would fail the commit
			// with commitkit's ErrNoFilesSpecified over paths that hold no
			// user data; the commit proceeds on whatever is already staged.
			if len(paths) > 0 {
				if stageErr := stageFiles(ctx, ws.Root, report, paths...); stageErr != nil {
					result.Success = false
					result.Error = stageErr.Error()
					return outputResult(result)
				}
			}
		} else {
			// Fallback: stage all (non-campaign workspace)
			if err := stageAllChanges(ctx, primaryRepoPath, report); err != nil {
				result.Success = false
				result.Error = err.Error()
				return outputResult(result)
			}
		}
	}

	rawMsg := message
	if autoWrite {
		if ws == nil || ws.Type != scope.WorkspaceTypeCampaign {
			result.Success = false
			result.Error = "auto-write requires campaign context"
			return outputResult(result)
		}
		fmt.Fprintln(os.Stderr, ui.Info("Writing commit message..."))
		rawMsg, err = commitkit.AutoWriteCommitMessage(ctx, ws.Root, primaryRepoPath)
		if err != nil {
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
			}
		}
	}

	pos := commitPosition(ctx, ref, festivalPath, hasFestival)

	// Build commit message with consolidated tag.
	festMessage := festCommitMessage(ref, pos, rawMsg)

	// A clean linked project is valid when the festival itself changed (for
	// example, close-out evidence). Skip only that project commit and continue
	// to the scoped campaign-root commit below. All other modes retain the
	// ordinary no-changes error from commitWithCampaignSupport.
	primaryCommitted := true
	if projectCommit {
		hasProjectChanges, hasErr := commitkit.HasStagedChanges(ctx, primaryRepoPath)
		if hasErr != nil {
			result.Error = errors.Wrap(hasErr, "checking staged project changes").Error()
			return outputResult(result)
		}
		primaryCommitted = hasProjectChanges
	}

	if primaryCommitted {
		if err := commitWithCampaignSupport(ctx, ws, primaryRepoPath, ref, pos, rawMsg, festMessage, result); err != nil {
			result.Error = err.Error()
			return outputResult(result)
		}
	} else {
		result.Message, result.CampaignTag = campaignCommitMessage(ctx, ws, ref, pos, rawMsg, festMessage)
	}

	result.TaskRef = ref
	result.Phase = pos.Phase
	result.Sequence = pos.Sequence

	// Campaign root commit: after a project commit, stage festival-scoped files
	// and any tracked submodule pointer separately. Without a project target,
	// the primary campaign-root commit above already handled festival files.
	if noRoot && projectCommit && !primaryCommitted {
		result.Error = "no changes to commit"
		return outputResult(result)
	}
	if !noRoot && projectCommit && ws != nil && ws.Type == scope.WorkspaceTypeCampaign && hasFestival {
		rootHash, rootErr := commitFestivalAtRoot(ctx, ws, festivalPath, submoduleRelPath, result.CampaignTag, ref, pos, rawMsg, report)
		if rootErr != nil {
			if !primaryCommitted {
				result.Error = rootErr.Error()
				return outputResult(result)
			}
			_, _ = fmt.Fprintf(report, "%s %s\n", ui.Warning("campaign root commit skipped:"), rootErr.Error())
		} else if rootHash != "" {
			result.CampaignHash = rootHash
			if !primaryCommitted {
				result.Hash = rootHash
				result.Message = festivalRootCommitMessage(result.CampaignTag, ref, pos, rawMsg)
			}
		} else if !primaryCommitted {
			result.Error = "no changes to commit"
			return outputResult(result)
		}
	}
	if !primaryCommitted && result.Hash == "" {
		result.Error = "no changes to commit"
		return outputResult(result)
	}

	result.Success = true

	// Activity log: commit.made at festival level only.
	if festivalPath != "" {
		actEmitter := activity.NewFromFestival(ctx, festivalPath, func(error) {})
		data := map[string]any{
			"git_sha": result.Hash,
			"message": message,
			"scope":   "project",
		}
		if result.CampaignHash != "" {
			data["campaign_hash"] = result.CampaignHash
		}
		if projectCommit {
			data["scope"] = "project"
		} else {
			data["scope"] = "campaign-root"
		}
		actEmitter.Emit(ctx, "commit.made", activity.Scope{
			Phase:    result.Phase,
			Sequence: result.Sequence,
			Task:     result.TaskRef,
		}, "fest commit -m \""+message+"\"", activity.WithData(data))
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
			fmt.Printf("%s %s\n", ui.Label("Message"), result.Message)
			if result.TaskRef != "" {
				fmt.Printf("%s %s\n", ui.Label("Task"), ui.Value(result.TaskRef, ui.TaskColor))
			}
			if summary := positionSummary(position{Phase: result.Phase, Sequence: result.Sequence}); summary != "" {
				fmt.Printf("%s %s\n", ui.Label("Position"), ui.Value(summary))
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

func currentGitRoot(ctx context.Context) (string, error) {
	return gitRootAt(ctx, "")
}

func gitRootAt(ctx context.Context, dir string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", errors.Wrap(err, "context cancelled")
	}
	args := []string{"rev-parse", "--show-toplevel"}
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "finding git root")
	}
	return strings.TrimSpace(string(out)), nil
}

// resolveCommitTarget selects the repository that owns implementation changes.
// A caller already inside a project stays there. A caller inside a festival is
// redirected using the nearest sequence's fest_working_dir, then the festival
// navigation link, then the legacy fest.yaml project_path fallback.
func resolveCommitTarget(
	ctx context.Context,
	campaignRoot string,
	currentRepo string,
	festivalPath string,
	hasFestival bool,
) (repoPath string, project bool, submoduleRelPath string, err error) {
	if filepath.Clean(currentRepo) != filepath.Clean(campaignRoot) {
		return currentRepo, true, gitlinkRelPath(ctx, campaignRoot, currentRepo), nil
	}
	if !hasFestival || festivalPath == "" {
		return currentRepo, false, "", nil
	}

	target, declared, err := festivalProjectTarget(campaignRoot, festivalPath)
	if err != nil {
		return "", false, "", err
	}
	if !declared {
		return currentRepo, false, "", nil
	}

	info, statErr := os.Stat(target)
	if statErr != nil {
		return "", false, "", errors.Wrap(statErr, "finding linked project").WithField("path", target)
	}
	if !info.IsDir() {
		return "", false, "", errors.Validation("linked project is not a directory").WithField("path", target)
	}

	targetRepo, rootErr := gitRootAt(ctx, target)
	if rootErr != nil {
		return "", false, "", errors.Wrap(rootErr, "resolving linked project repository").WithField("path", target)
	}
	if filepath.Clean(targetRepo) == filepath.Clean(campaignRoot) {
		return currentRepo, false, "", nil
	}

	return targetRepo, true, gitlinkRelPath(ctx, campaignRoot, targetRepo), nil
}

// festivalProjectTarget returns the explicit implementation directory for the
// caller's current sequence or festival. The bool distinguishes "not linked"
// from an invalid declared link, which must fail loudly rather than committing
// the wrong repository.
func festivalProjectTarget(campaignRoot, festivalPath string) (string, bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false, errors.Wrap(err, "getting working directory")
	}

	if rel, relErr := filepath.Rel(festivalPath, cwd); relErr == nil && rel != "." && !pathEscapesRoot(rel) {
		parts := strings.Split(rel, string(os.PathSeparator))
		if len(parts) >= 2 {
			sequencePath := filepath.Join(festivalPath, parts[0], parts[1])
			if workingDir := selection.ExtractWorkingDir(sequencePath); workingDir != "" {
				_, absolute, resolveErr := pathutil.ResolveProjectPathValue(workingDir, campaignRoot)
				if resolveErr != nil {
					return "", true, errors.Wrap(resolveErr, "resolving fest_working_dir")
				}
				return absolute, true, nil
			}
		}
	}

	if nav, navErr := navigation.LoadNavigation(); navErr == nil {
		if linked := nav.GetLinkedProject(filepath.Base(festivalPath)); linked != "" {
			return linked, true, nil
		}
	}

	cfg, cfgErr := config.LoadFestivalConfig(festivalPath, campaignRoot)
	if cfgErr != nil {
		return "", false, errors.Wrap(cfgErr, "loading festival config")
	}
	if cfg.ProjectPath == "" {
		return "", false, nil
	}
	_, absolute, resolveErr := pathutil.ResolveProjectPathValue(cfg.ProjectPath, campaignRoot)
	if resolveErr != nil {
		return "", true, errors.Wrap(resolveErr, "resolving project_path")
	}
	return absolute, true, nil
}

func pathEscapesRoot(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// gitlinkRelPath returns the campaign-relative path only when the target is a
// tracked submodule. Worktrees and external repositories still receive the
// project commit, but are not offered to the campaign-root staging path list.
func gitlinkRelPath(ctx context.Context, campaignRoot, repoPath string) string {
	rel, err := filepath.Rel(campaignRoot, repoPath)
	if err != nil || pathEscapesRoot(rel) || rel == "." {
		return ""
	}
	out, err := exec.CommandContext(ctx, "git", "-C", campaignRoot, "ls-files", "--stage", "--", rel).Output()
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(out)), "160000 ") {
		return ""
	}
	return rel
}

// stageAllChanges stages everything in the repository at repoPath through
// camp's staging guard: lock retry with stale-lock cleanup, size and bulk
// protection, and a typed refusal fest renders in its own voice. The repo is
// passed explicitly because the guard resolves its thresholds from it; the
// old form ran `git add -A` in whatever cwd fest happened to hold.
// report receives exclusion/unavailable lines (typically cmd.ErrOrStderr()).
//
// The --commit-large override rides through StageAllWithOptions to the same
// guard decision camp's own flag reaches, so a refusal here can truthfully
// offer `fest commit --commit-large` as the retry.
func stageAllChanges(ctx context.Context, repoPath string, report io.Writer) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	outcome, err := commitkit.StageAllWithOptions(ctx, repoPath,
		commitkit.StageOptions{CommitLarge: commitLarge, CommitNested: commitNested})
	if err != nil {
		var blocked *commitkit.GuardBlockedError
		if stderrors.As(err, &blocked) {
			return errors.New(guardRefusalMessage(blocked))
		}
		return errors.Wrap(err, "staging changes")
	}
	reportStageOutcome(report, outcome)
	return nothingLeftAfterExclusions(ctx, repoPath, outcome)
}

// stageFiles stages the given paths through commitkit with outcome reporting
// and fest-rendered GuardBlockedError text — the same contract as stageAllChanges,
// for festival-scoped and campaign-root path lists.
//
// The options form carries --commit-large so fest's flag reaches the guard on
// the same terms the stage-all branch gets. It does not reach a guard today:
// camp only guards sweep forms, reading an explicit path list as the user's own
// intent, so a synthesized list like this one is staged unchecked and the
// refusal branch below cannot fire. The wiring and the retry it names are the
// right ones the moment camp can guard a list fest built rather than the user.
func stageFiles(ctx context.Context, repoPath string, report io.Writer, files ...string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}
	outcome, err := commitkit.StageFilesWithOptions(ctx, repoPath,
		commitkit.StageOptions{CommitLarge: commitLarge, CommitNested: commitNested}, files...)
	if err != nil {
		var blocked *commitkit.GuardBlockedError
		if stderrors.As(err, &blocked) {
			return errors.New(guardRefusalMessage(blocked))
		}
		return errors.Wrap(err, "staging files")
	}
	reportStageOutcome(report, outcome)
	return nothingLeftAfterExclusions(ctx, repoPath, outcome)
}

// nothingLeftAfterExclusions turns "guard excluded everything" into a clear
// error instead of letting commitkit.Commit return a bare ErrNoChanges after
// the user already saw exclusion lines on stderr.
func nothingLeftAfterExclusions(ctx context.Context, repoPath string, outcome *commitkit.StageOutcome) error {
	if outcome == nil || (len(outcome.Excluded) == 0 && len(outcome.NestedRepos) == 0) {
		return nil
	}
	has, err := commitkit.HasStagedChanges(ctx, repoPath)
	if err != nil {
		return errors.Wrap(err, "checking staged changes after guard exclusions")
	}
	if !has {
		return errors.New(allExcludedMessage(outcome))
	}
	return nil
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
	festivalPath, err := resolveFestivalFlagPath(ctx, ws, flag)
	if err != nil {
		return "", err
	}
	return loadFestivalID(festivalPath, ws.Root)
}

// resolveFestivalFlagPath resolves every form accepted by --festival. Paths
// are validated directly; names and IDs are searched across lifecycle status
// directories. Keeping this resolution at command entry lets the same context
// drive tag detection, project targeting, and the campaign-root commit.
func resolveFestivalFlagPath(ctx context.Context, ws *scope.WorkspaceInfo, flag string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", errors.Wrap(err, "context cancelled")
	}
	if ws == nil {
		return "", errors.NotFound("workspace")
	}

	if filepath.IsAbs(flag) {
		return validateFestivalFlagPath(flag, ws.Root)
	}

	// A relative path can be relative to the caller or the campaign root. Only
	// treat it as a path when it exists or contains an explicit path separator;
	// a bare value remains eligible for the name/ID search below.
	cwd, cwdErr := os.Getwd()
	if cwdErr == nil {
		for _, candidate := range []string{filepath.Join(cwd, flag), filepath.Join(ws.Root, flag)} {
			if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
				return validateFestivalFlagPath(candidate, ws.Root)
			}
		}
	}
	if strings.ContainsRune(flag, os.PathSeparator) {
		return "", errors.NotFound(fmt.Sprintf("festival path %q", flag))
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
				return festivalPath, nil
			}

			// Match by ID suffix (e.g., directory ends with "-OA0001")
			if strings.HasSuffix(entry.Name(), "-"+flag) {
				return festivalPath, nil
			}

			// Match by loading config and comparing metadata.id
			if fid, err := loadFestivalID(festivalPath, ws.Root); err == nil && fid == flag {
				return festivalPath, nil
			}
		}
	}

	return "", errors.NotFound(fmt.Sprintf("festival matching flag %q", flag))
}

func validateFestivalFlagPath(path, campaignRoot string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", errors.Wrap(err, "resolving festival path")
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", errors.Wrap(err, "finding festival path").WithField("path", absPath)
	}
	if !info.IsDir() {
		return "", errors.Validation("festival path is not a directory").WithField("path", absPath)
	}
	if _, err := loadFestivalID(absPath, campaignRoot); err != nil {
		return "", errors.Wrap(err, "validating festival path").WithField("path", absPath)
	}
	return absPath, nil
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
// festival refs into a single name-style tag:
// [{campaign-name}:{cid}-FE-{fid}-PH-{phase}-SQ-{sequence}].
// Campaign detection or sync failures degrade gracefully — the commit still proceeds.
func commitWithCampaignSupport(ctx context.Context, ws *scope.WorkspaceInfo, repoPath, festRef string, pos position, rawMsg, festMessage string, result *CommitResult) error {
	commitMessage, campaignTag := campaignCommitMessage(ctx, ws, festRef, pos, rawMsg, festMessage)
	result.CampaignTag = campaignTag

	// commitkit.Commit captures git's output internally, which is the --json
	// contract: nothing but the encoded document may reach stdout. The same
	// pair already commits the campaign root in commitCampaignRoot.
	if err := commitkit.Commit(ctx, repoPath, commitkit.CommitOptions{Message: commitMessage}); err != nil {
		if stderrors.Is(err, commitkit.ErrNoChanges) {
			return errors.New("no changes to commit")
		}
		return errors.Wrap(err, "committing")
	}
	hash, err := commitkit.ShortHash(ctx, repoPath)
	if err != nil {
		return errors.Wrap(err, "getting commit hash")
	}
	result.Hash = hash
	result.Message = commitMessage

	return nil
}

// festivalScopedPaths returns the campaign-root-relative paths that should be
// staged for a festival commit at the campaign root:
//   - The festival directory itself (task docs, .fest/progress_events.jsonl)
//   - .campaign/fest/ (navigation state)
//   - festivals/.festival/.state/ (global festival event log)
//   - The submodule pointer path (if submoduleRelPath is non-empty)
//
// Only paths git can match are returned. The two fest-owned state directories
// appear the first time fest navigates in a campaign, so a campaign fresh from
// `camp init` has neither, and a single unmatched pathspec fails the whole
// `git add` — which made the first `fest commit` in a new campaign impossible.
func festivalScopedPaths(ctx context.Context, campaignRoot, festivalPath, submoduleRelPath string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	festivalRel, err := filepath.Rel(campaignRoot, festivalPath)
	if err != nil {
		return nil, errors.Wrap(err, "computing festival relative path")
	}

	candidates := []string{
		festivalRel,
		filepath.Join(".campaign", "fest"),
		filepath.Join("festivals", ".festival", ".state"),
	}

	if submoduleRelPath != "" {
		candidates = append(candidates, submoduleRelPath)
	}

	return matchablePaths(ctx, campaignRoot, candidates), nil
}

// matchablePaths keeps the pathspecs `git add` can resolve in the repository at
// repoRoot, preserving the given order. A path present in the working tree
// qualifies; a path that is gone but still tracked qualifies too, because
// staging it is what records the deletion. Anything else is dropped: git
// refuses the entire `git add` over one unmatched pathspec, so an absent
// path would take the paths that do exist down with it.
func matchablePaths(ctx context.Context, repoRoot string, candidates []string) []string {
	matchable := make([]string, 0, len(candidates))
	for _, path := range candidates {
		if _, err := os.Lstat(filepath.Join(repoRoot, path)); err == nil {
			matchable = append(matchable, path)
			continue
		}
		if trackedInGit(ctx, repoRoot, path) {
			matchable = append(matchable, path)
		}
	}
	return matchable
}

// trackedInGit reports whether the index of the repository at repoRoot holds
// any entry under path. A tracked path missing from the working tree is the
// deletion case, which must still reach `git add`.
func trackedInGit(ctx context.Context, repoRoot, path string) bool {
	out, err := exec.CommandContext(ctx, "git", "-C", repoRoot, "ls-files", "--", path).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// commitCampaignRoot stages festival-scoped paths at the campaign root and
// commits only those paths. Unrelated staged campaign-root content is left
// staged. Returns the short hash of the campaign commit, or empty string if
// there were no festival-scoped changes to commit. Errors from staging or
// committing are returned; "no changes" is a silent skip (including when the
// guard excluded every path — already reported on report).
func commitCampaignRoot(ctx context.Context, campaignRoot string, paths []string, commitMessage string, report io.Writer) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", errors.Wrap(err, "context cancelled")
	}

	// No matchable festival-scoped path is the same silent skip as no changes:
	// there is nothing for the root commit to carry, and commitkit would
	// reject the empty list with ErrNoFilesSpecified rather than stage it.
	if len(paths) == 0 {
		return "", nil
	}

	outcome, err := commitkit.StageFilesWithOptions(ctx, campaignRoot,
		commitkit.StageOptions{CommitLarge: commitLarge, CommitNested: commitNested}, paths...)
	if err != nil {
		var blocked *commitkit.GuardBlockedError
		if stderrors.As(err, &blocked) {
			return "", errors.New(guardRefusalMessage(blocked))
		}
		return "", errors.Wrap(err, "staging festival files at campaign root")
	}
	reportStageOutcome(report, outcome)

	hasChanges, err := hasStagedPathChanges(ctx, campaignRoot, paths)
	if err != nil {
		return "", errors.Wrap(err, "checking staged changes at campaign root")
	}
	if !hasChanges {
		return "", nil
	}

	// Drain first so a queued camp bookkeeping commit cannot land in the
	// middle of this one. Then commit the staged festival paths from a temp
	// index so unrelated staged campaign files stay staged, and later
	// worktree writes (including during DrainJobs) cannot replace the staged
	// snapshot. Drain errors are discarded for the same reason commitkit's
	// own stage/commit entrypoints discard them: the festival commit is not
	// the place to fail a camp queue fault.
	_ = commitkit.DrainJobs(ctx, campaignRoot)
	if err := commitOnlyPaths(ctx, campaignRoot, commitMessage, paths); err != nil {
		return "", err
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
func commitFestivalAtRoot(ctx context.Context, ws *scope.WorkspaceInfo, festivalPath, submoduleRelPath, campaignTag, festRef string, pos position, rawMsg string, report io.Writer) (string, error) {
	paths, err := festivalScopedPaths(ctx, ws.Root, festivalPath, submoduleRelPath)
	if err != nil {
		return "", err
	}

	return commitCampaignRoot(ctx, ws.Root, paths, festivalRootCommitMessage(campaignTag, festRef, pos, rawMsg), report)
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
