package status

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/camp/pkg/commitkit"
	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/commands/tui"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/registry"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/ui/theme"
	"github.com/Obedience-Corp/fest/internal/workflow"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// selectFestivalForStatus opens an interactive selector for choosing a festival.
func selectFestivalForStatus(ctx context.Context, cwd, newStatus string) (*show.FestivalInfo, error) {
	// Find festivals directory
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil {
		return nil, errors.Wrap(err, "finding festivals directory")
	}
	if festivalsDir == "" {
		return nil, errors.NotFound("festivals directory").
			WithField("hint", "navigate to a workspace with festivals/ directory")
	}

	// Configure selector with appropriate title
	config := tui.DefaultSelectorConfig()
	config.Title = "Select Festival to Change Status"
	config.Description = fmt.Sprintf("Choose festival to set to '%s'", newStatus)

	// Create and run selector
	selector := tui.NewFestivalSelector(festivalsDir, config)
	result, err := selector.Run(ctx)
	if err != nil {
		return nil, err
	}

	if result.Cancelled {
		return nil, nil // User cancelled - not an error
	}

	return result.Selected, nil
}

// applyStatusToFestival applies a status change to a festival.
func applyStatusToFestival(ctx context.Context, display *ui.UI, festival *show.FestivalInfo, newStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// Resolve user-facing aliases to filesystem paths (e.g., "completed" -> "dungeon/completed")
	newStatus = id.ResolveStatusPath(newStatus)

	// Validate status for festivals using internal schema
	if !isValidFestivalStatus(newStatus) {
		validOptions := getValidFestivalStatuses()
		return errors.Validation("invalid status").
			WithField("status", newStatus).
			WithField("valid_options", strings.Join(validOptions, ", "))
	}

	// Apply the status change
	return handleFestivalStatusChange(ctx, display, festival, newStatus, opts)
}

// updateGoalFrontmatter reads a goal file, updates its fest_status in frontmatter, and writes it back.
func UpdateGoalFrontmatter(ctx context.Context, goalPath string, newStatus frontmatter.Status) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	content, err := os.ReadFile(goalPath)
	if err != nil {
		return errors.IO("reading goal file", err)
	}

	fm, remaining, err := frontmatter.Parse(content)
	if err != nil {
		return errors.Wrap(err, "parsing frontmatter")
	}
	if fm == nil {
		return errors.Validation("goal file has no frontmatter").WithField("path", goalPath)
	}

	fm.Status = newStatus
	fm.Updated = time.Now()

	updated, err := frontmatter.Inject(remaining, fm)
	if err != nil {
		return errors.Wrap(err, "injecting updated frontmatter")
	}

	if err := os.WriteFile(goalPath, updated, 0o644); err != nil {
		return errors.IO("writing updated goal file", err)
	}
	return nil
}

// readGoalStatus reads the current status from a goal file's frontmatter.
func readGoalStatus(ctx context.Context, goalPath string) (frontmatter.Status, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	content, err := os.ReadFile(goalPath)
	if err != nil {
		return "", errors.IO("reading goal file", err)
	}

	fm, _, err := frontmatter.Parse(content)
	if err != nil {
		return "", errors.Wrap(err, "parsing frontmatter")
	}
	if fm == nil {
		return "", errors.Validation("goal file has no frontmatter").WithField("path", goalPath)
	}

	return fm.Status, nil
}

// handleFestivalStatusChange handles changing a festival's status by moving its directory.
func handleFestivalStatusChange(ctx context.Context, display *ui.UI, festival *show.FestivalInfo, newStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// NOTE: No contract changes needed here. The watcher contract declares
	// status *directories* (e.g., festivals/planning/, festivals/active/),
	// not individual festivals within them. Moving a festival between
	// directories triggers the daemon's directory watcher automatically --
	// the daemon sees the create/delete events in the watched directories
	// and discovers or removes festivals accordingly. The contract only
	// changes when NEW directories are added (fest init), not when
	// festivals move between existing ones.

	// Resolve user-facing aliases to filesystem paths (e.g., "completed" -> "dungeon/completed")
	newStatus = id.ResolveStatusPath(newStatus)

	// When target is "dungeon" (bare), prompt for substatus selection
	if newStatus == "dungeon" {
		resolved, err := resolveDungeonSubstatus(ctx)
		if err != nil {
			return err
		}
		if resolved == "" {
			display.Info("Selection cancelled.")
			return nil
		}
		newStatus = resolved
	}

	if festival.Status == newStatus {
		return emitAlreadyAtStatus(display, opts, newStatus)
	}

	// Validate transition and confirm unless forced
	if !opts.force {
		standard, validOpts, err := isStandardTransition(ctx, festival.Status, newStatus)
		if err != nil {
			return err
		}
		if !standard {
			if !confirmNonStandardTransition(display, festival, newStatus, validOpts) {
				display.Info("Operation cancelled.")
				return nil
			}
		} else {
			if !confirmFestivalMove(display, festival, newStatus) {
				display.Info("Operation cancelled.")
				return nil
			}
		}
	}

	// Execute the move
	return executeFestivalMove(ctx, festival, newStatus, opts)
}

// resolveDungeonSubstatus prompts the user to select a dungeon child directory.
func resolveDungeonSubstatus(ctx context.Context) (string, error) {
	// Use internal FestivalSchema to get dungeon children
	schema := workflow.FestivalSchema()
	var dungeonChildren []string
	if dir, ok := schema.GetDirectory("dungeon"); ok && dir.Nested && len(dir.Children) > 0 {
		for childName := range dir.Children {
			dungeonChildren = append(dungeonChildren, "dungeon/"+childName)
		}
	}

	// Fallback to centralized status directories if schema has no dungeon children
	if len(dungeonChildren) == 0 {
		for _, s := range id.StatusDirectories {
			if strings.HasPrefix(s, "dungeon/") {
				dungeonChildren = append(dungeonChildren, s)
			}
		}
	}

	options := theme.ToOptions(dungeonChildren)
	var selected string
	cancelled, err := theme.QuickSelect(ctx, "Select dungeon substatus", options, &selected)
	if err != nil {
		return "", errors.Wrap(err, "dungeon substatus selection failed")
	}
	if cancelled {
		return "", nil
	}
	return selected, nil
}

// isStandardTransition checks if a transition is defined in the festival workflow schema.
// Returns true if the transition is standard, along with the valid options for the current status.
func isStandardTransition(ctx context.Context, fromStatus, toStatus string) (standard bool, validOpts []string, err error) {
	if err := ctx.Err(); err != nil {
		return false, nil, errors.Wrap(err, "context cancelled")
	}
	schema := workflow.FestivalSchema()
	if schema.IsValidTransition(fromStatus, toStatus) {
		return true, nil, nil
	}
	return false, transitionOptsForStatus(schema, fromStatus), nil
}

// confirmNonStandardTransition warns about a non-standard transition and asks for confirmation.
func confirmNonStandardTransition(display *ui.UI, festival *show.FestivalInfo, newStatus string, validOpts []string) bool {
	display.Warning("Non-standard transition: %s to %s is not defined in the workflow schema.",
		festival.Status, newStatus)
	if len(validOpts) > 0 {
		fmt.Printf("  Standard transitions from %s:\n", ui.GetStateStyle(festival.Status).Render(festival.Status))
		for _, opt := range validOpts {
			fmt.Printf("    -> %s\n", ui.GetStateStyle(opt).Render(opt))
		}
		fmt.Println()
	}
	prompt := fmt.Sprintf("Proceed with non-standard move of %s to %s?",
		ui.Value(festival.Name, ui.FestivalColor),
		ui.GetStateStyle(newStatus).Render(newStatus))
	return display.Confirm("%s", prompt)
}

// transitionOptsForStatus returns the valid transition targets for a given status.
func transitionOptsForStatus(schema *workflow.Schema, status string) []string {
	dir, ok := schema.GetDirectory(status)
	if !ok || len(dir.TransitionOpts) == 0 {
		return schema.AllDirectories()
	}
	return dir.TransitionOpts
}

// emitAlreadyAtStatus outputs a message when festival is already at the requested status.
func emitAlreadyAtStatus(display *ui.UI, opts *statusOptions, status string) error {
	if opts.json {
		result := map[string]any{
			"success": true,
			"message": "already at requested status",
			"status":  status,
		}
		if err := shared.EncodeJSON(os.Stdout, result); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
	} else {
		fmt.Printf("%s %s\n", ui.Info("Already at status"), ui.GetStateStyle(status).Render(status))
	}
	return nil
}

// confirmFestivalMove prompts the user to confirm a festival move operation.
func confirmFestivalMove(display *ui.UI, festival *show.FestivalInfo, newStatus string) bool {
	prompt := fmt.Sprintf("Move festival %s from %s to %s?",
		ui.Value(festival.Name, ui.FestivalColor),
		ui.GetStateStyle(festival.Status).Render(festival.Status),
		ui.GetStateStyle(newStatus).Render(newStatus))
	return display.Confirm("%s", prompt)
}

// executeFestivalMove performs the actual directory move for a festival status change.
func executeFestivalMove(ctx context.Context, festival *show.FestivalInfo, newStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	// Calculate new path
	festivalsRoot := festivalsRootFromPath(festival.Path, festival.Status)

	var newPath string
	if strings.HasPrefix(newStatus, "dungeon/") {
		// All dungeon statuses use date-based directories
		dateDir := CalculateDateDir(time.Now())
		dungeonStatusDir := filepath.Join(festivalsRoot, newStatus)
		var err error
		newPath, err = MoveToDateDirectory(festival.Path, dungeonStatusDir, dateDir)
		if err != nil {
			return errors.Wrap(err, "moving festival to date directory")
		}
	} else {
		newPath = filepath.Join(festivalsRoot, newStatus, festival.Name)

		// Check if destination exists
		if _, err := os.Stat(newPath); err == nil {
			return errors.Validation("destination already exists").WithField("path", newPath)
		}

		// Create destination directory if needed
		destDir := filepath.Join(festivalsRoot, newStatus)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return errors.IO("creating destination directory", err)
		}

		// Move the directory
		if err := os.Rename(festival.Path, newPath); err != nil {
			return errors.IO("moving festival directory", err)
		}
	}

	// Detect if CWD is inside the festival being moved
	cwd, cwdErr := os.Getwd()
	var cdHint string
	if cwdErr == nil {
		rel, relErr := filepath.Rel(festival.Path, cwd)
		if relErr == nil && !strings.HasPrefix(rel, "..") {
			// CWD is inside this festival — compute equivalent path in new location
			cdHint = filepath.Join(newPath, rel)
		}
	}

	// Update FESTIVAL_GOAL.md frontmatter with the new status
	festivalGoalPath := filepath.Join(newPath, "FESTIVAL_GOAL.md")
	if _, err := os.Stat(festivalGoalPath); err == nil {
		if fmErr := UpdateGoalFrontmatter(ctx, festivalGoalPath, frontmatter.Status(newStatus)); fmErr != nil {
			// Log but don't fail — the directory move already succeeded
			fmt.Printf("%s %s\n", ui.Dim("Warning: could not update FESTIVAL_GOAL.md frontmatter:"), ui.Dim(fmErr.Error()))
		}
	}

	// Update fest.yaml metadata with the new status
	var festivalID string
	if festCfg, cfgErr := config.LoadFestivalConfig(newPath, ""); cfgErr == nil {
		festivalID = festCfg.Metadata.ID
		festCfg.Metadata.AddStatusChange(newStatus, newPath, "")
		if saveErr := config.SaveFestivalConfig(newPath, "", festCfg); saveErr != nil {
			fmt.Printf("%s %s\n", ui.Dim("Warning: could not update fest.yaml status:"), ui.Dim(saveErr.Error()))
		}
	}

	// Update navigation links after successful move
	linkAction := UpdateNavigationAfterMove(festival.Name, newStatus, newPath)

	// Auto-commit the status change unless --no-commit was specified
	var commitHash string
	if !opts.noCommit {
		// Build the list of paths touched by the status change
		changedPaths := []string{festival.Path, newPath}
		if linkAction != "" {
			if navPath, navErr := navigation.NavigationPath(); navErr == nil {
				changedPaths = append(changedPaths, navPath)
			}
		}
		hash, err := AutoCommitStatusChange(ctx, festival.Name, festivalID, festival.Status, newStatus, changedPaths)
		if err != nil {
			fmt.Printf("%s %s\n", ui.Dim("Warning: auto-commit failed:"), ui.Dim(err.Error()))
		} else if hash != "" {
			commitHash = hash
		}
	}

	return emitFestivalMoveSuccess(opts, festival, newStatus, newPath, linkAction, commitHash, cdHint)
}

// updateNavigationAfterMove updates the navigation link after a festival move.
// On completion, the link is removed (project freed). On other transitions, the
// festival path is updated so the link stays accurate.
// Returns a human-readable description of the action taken, or empty string.
func UpdateNavigationAfterMove(festivalName, newStatus, newPath string) string {
	nav, err := navigation.LoadNavigation()
	if err != nil {
		// Navigation not available (no campaign context, etc.) - skip silently
		return ""
	}

	link, linked := nav.GetLink(festivalName)
	if !linked {
		return ""
	}

	// Terminal statuses (anything under dungeon/) should unlink the project
	resolvedStatus := id.ResolveStatusPath(newStatus)
	if strings.HasPrefix(resolvedStatus, "dungeon/") {
		nav.RemoveLink(festivalName)
		if saveErr := nav.Save(); saveErr != nil {
			return fmt.Sprintf("warning: could not unlink project: %v", saveErr)
		}
		return fmt.Sprintf("unlinked project %s", link.Path)
	}

	// Non-terminal transition: update the festival path in the link
	nav.SetLinkWithPath(festivalName, link.Path, newPath)
	if saveErr := nav.Save(); saveErr != nil {
		return fmt.Sprintf("warning: could not update link path: %v", saveErr)
	}
	return "link path updated"
}

// emitFestivalMoveSuccess outputs success message after moving a festival.
func emitFestivalMoveSuccess(opts *statusOptions, festival *show.FestivalInfo, newStatus, newPath, linkAction, commitHash, cdHint string) error {
	if opts.json {
		result := map[string]any{
			"success":    true,
			"festival":   festival.Name,
			"old_status": festival.Status,
			"new_status": newStatus,
			"old_path":   festival.Path,
			"new_path":   newPath,
		}
		if linkAction != "" {
			result["link_action"] = linkAction
		}
		if commitHash != "" {
			result["commit"] = commitHash
		}
		if cdHint != "" {
			result["cd_hint"] = cdHint
		}
		if err := shared.EncodeJSON(os.Stdout, result); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
	} else {
		fmt.Println(ui.Success("✓ Festival status updated"))
		fmt.Printf("%s %s\n", ui.Label("Festival"), ui.Value(festival.Name, ui.FestivalColor))
		fmt.Printf("%s %s\n", ui.Label("From"), ui.GetStateStyle(festival.Status).Render(festival.Status))
		fmt.Printf("%s %s\n", ui.Label("To"), ui.GetStateStyle(newStatus).Render(newStatus))
		fmt.Printf("%s %s\n", ui.Label("Path"), ui.Dim(newPath))
		if linkAction != "" {
			fmt.Printf("%s %s\n", ui.Label("Link"), ui.Dim(linkAction))
		}
		if commitHash != "" {
			fmt.Printf("%s %s\n", ui.Label("Commit"), ui.Value(commitHash))
		}
		if cdHint != "" {
			fmt.Println()
			fmt.Println(ui.Warning("Your current directory was inside the moved festival."))
			fmt.Printf("%s cd %s\n", ui.Label("Navigate"), cdHint)
		}
	}
	return nil
}

// autoCommitStatusChange stages and commits the filesystem changes from a festival
// status move. It follows the non-blocking pattern: failures produce warnings,
// never blocking the status change itself.
//
// Uses commitkit for lock-aware git operations with automatic stale lock cleanup.
func AutoCommitStatusChange(ctx context.Context, festivalName, festivalID, oldStatus, newStatus string, changedPaths []string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	ws, ok := scope.WorkspaceFrom(ctx)
	if !ok || ws == nil {
		return "", errors.New("workspace not available in context")
	}

	if ws.FestivalsPath != "" {
		eventsPath := registry.GetEventsPath(ws.FestivalsPath)
		if _, err := os.Stat(eventsPath); err == nil {
			changedPaths = statusChangeCommitPaths(changedPaths, eventsPath)
		}
	}

	// Stage only the paths affected by the status change (lock-aware with retry).
	if err := commitkit.StageFiles(ctx, ws.Root, changedPaths...); err != nil {
		return "", errors.Wrap(err, "stage")
	}

	// Check if anything is actually staged.
	hasChanges, err := commitkit.HasStagedChanges(ctx, ws.Root)
	if err != nil {
		return "", errors.Wrap(err, "check staged")
	}
	if !hasChanges {
		return "", nil
	}

	// Build commit message.
	var message string
	if festivalID != "" {
		message = fmt.Sprintf("chore(fest): status change: %s (%s) %s -> %s", festivalName, festivalID, oldStatus, newStatus)
	} else {
		message = fmt.Sprintf("chore(fest): status change: %s %s -> %s", festivalName, oldStatus, newStatus)
	}

	// Prepend campaign tag if in a campaign workspace.
	if ws.Type == scope.WorkspaceTypeCampaign {
		cid, err := commitkit.DetectCampaign(ctx)
		if err == nil && cid != "" {
			name, _ := commitkit.DetectCampaignName(ctx)
			message = commitkit.PrependContextTagsFullNamed(name, cid, "", "", "", message)
		}
	}

	// Commit (lock-aware with retry).
	if err := commitkit.Commit(ctx, ws.Root, commitkit.CommitOptions{Message: message}); err != nil {
		return "", errors.Wrap(err, "commit")
	}

	// Get short hash for display.
	hash, err := commitkit.ShortHash(ctx, ws.Root)
	if err != nil {
		// Commit succeeded but we couldn't get the hash — not critical.
		return "committed", nil
	}

	return hash, nil
}

func statusChangeCommitPaths(changedPaths []string, eventsPath string) []string {
	paths := make([]string, 0, len(changedPaths)+1)
	seen := make(map[string]struct{}, len(changedPaths)+1)

	appendPath := func(path string) {
		if path == "" {
			return
		}
		key := filepath.Clean(path)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		paths = append(paths, path)
	}

	for _, path := range changedPaths {
		appendPath(path)
	}
	appendPath(eventsPath)

	return paths
}
