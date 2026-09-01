// Package promote implements the fest promote command for lifecycle transitions.
package promote

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/commands/resolver"
	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/commands/status"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

// validTransitions defines the allowed lifecycle promotions.
var validTransitions = map[string]string{
	"planning": "ready",
	"ready":    "active",
	"active":   "completed",
}

type promoteOptions struct {
	force    bool
	json     bool
	noCommit bool
	dungeon  string
}

// NewPromoteCommand creates the fest promote command.
func NewPromoteCommand() *cobra.Command {
	opts := &promoteOptions{}
	cmd := &cobra.Command{
		Use:   "promote [festival]",
		Short: "Promote a festival to the next lifecycle status",
		Long: `Promote moves a festival through the lifecycle: planning → ready → active → completed.

Each transition validates readiness:
  planning → ready:    Festival goal must be defined
  ready → active:      Festival is ready to begin execution
  active → completed:  All tasks must be completed

By default, promotes the festival you are currently inside. From elsewhere in a
camp, pass a festival name or run promote interactively to pick one:
  fest promote my-feature       Promote a festival by name (tab completion)
  fest promote                  Pick a festival from a fuzzy picker (in a terminal)

Use --dungeon to send a festival directly to a dungeon status:
  fest promote --dungeon someday     Shelve for later
  fest promote --dungeon archived    Archive the festival
  fest promote --dungeon completed   Mark as completed (skips task validation)`,
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completePromoteTarget,
		RunE: func(cmd *cobra.Command, args []string) error {
			selector := ""
			if len(args) > 0 {
				selector = args[0]
			}
			return runPromote(cmd.Context(), opts, selector)
		},
	}

	cmd.Flags().BoolVar(&opts.force, "force", false, "skip readiness validation")
	cmd.Flags().BoolVar(&opts.json, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&opts.noCommit, "no-commit", false, "skip auto-commit after promotion (rejected when agent.require_auto_commit is enabled)")
	cmd.Flags().StringVar(&opts.dungeon, "dungeon", "", "send to dungeon status (completed, archived, someday)")

	cmd.AddCommand(NewPromoteCompletionsCommand())

	return cmd
}

// promotePickerOptions limits completion to promotable festivals, ordered
// active → ready → planning (then recency) to match fest go navigation.
// The interactive picker uses resolver.DefaultPickerOptions so it also
// narrows to the status directory the user is browsing, matching fest watch.
func promotePickerOptions() shared.FestivalPickerOptions {
	return shared.FestivalPickerOptions{
		PreferredStatuses:        []string{"active", "ready", "planning"},
		OrderByStatusThenRecency: true,
	}
}

// completePromoteTarget provides shell completion for the festival selector,
// ordered by status (active → ready → planning) rather than alphabetically.
func completePromoteTarget(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	candidates, err := shared.ListFestivalPickCandidates(cmd.Context(), cwd, promotePickerOptions())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return shared.OrderedSelectorNames(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// NewPromoteCompletionsCommand creates the hidden subcommand that emits colorized,
// status-ordered festival completions for the 'fest promote' shell widget, giving
// promote the same tab-completion experience as fgo.
func NewPromoteCompletionsCommand() *cobra.Command {
	var color bool
	cmd := &cobra.Command{
		Use:    "completions",
		Short:  "Output promote completion words for shell integration",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPromoteCompletions(cmd.Context(), color)
		},
	}
	cmd.Flags().BoolVar(&color, "color", false, "output value\\tcolorized_display for zsh compadd")
	return cmd
}

func runPromoteCompletions(ctx context.Context, color bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	candidates, err := shared.ListFestivalPickCandidates(ctx, cwd, promotePickerOptions())
	if err != nil {
		return nil
	}
	if color {
		for _, line := range shared.ColorSelectorCompletions(candidates, "") {
			fmt.Println(line)
		}
		return nil
	}
	for _, name := range shared.OrderedSelectorNames(candidates, "") {
		fmt.Println(name)
	}
	return nil
}

var errPromoteCancelled = errors.New("promotion cancelled")

// resolveFestivalForPromote resolves the festival to promote using the shared
// TargetResolver (cwd-scoped picker, matching fest watch). The returned bool
// reports whether the festival was chosen from the interactive picker. When
// allowPicker is false (e.g. --json mode), the picker tier is skipped so
// non-interactive callers get a structured NotFound instead of a TUI launch.
func resolveFestivalForPromote(ctx context.Context, festivalsDir, cwd, selector string, allowPicker bool) (*show.FestivalInfo, bool, error) {
	r := promoteTargetResolver(cwd, festivalsDir, allowPicker)
	festival, source, err := r.Resolve(ctx, selector)
	if err != nil {
		if resolver.IsPickerCancelled(err) {
			return nil, false, errPromoteCancelled
		}
		return nil, false, err
	}
	return festival, source == resolver.ResolveSourcePicker, nil
}

// promoteTargetResolver is the shared TargetResolver used by fest promote.
// PickerOptions is DefaultPickerOptions so a user browsing festivals/ready
// sees ready festivals first, matching fest watch.
func promoteTargetResolver(cwd, festivalsDir string, allowPicker bool) resolver.TargetResolver {
	r := resolver.DefaultTargetResolver(resolver.TargetResolverOptions{
		PickerOptions: resolver.DefaultPickerOptions,
	})
	// Inject the caller-provided cwd and festivals directory so the resolver
	// does not re-derive them from os.Getwd (the scope may have resolved a
	// different workspace root than the process cwd).
	r.Getwd = func() (string, error) { return cwd, nil }
	if festivalsDir != "" {
		r.FindFestivals = func(string) (string, error) { return festivalsDir, nil }
	}
	if !allowPicker {
		r.CanPickFestival = func() bool { return false }
	}
	return r
}

// confirmPromotion asks the operator to confirm a picker-selected promotion.
func confirmPromotion(name, from, to string) bool {
	fmt.Printf("Promote %s: %s → %s? [y/N] ",
		ui.Value(name, ui.FestivalColor),
		ui.GetStateStyle(from).Render(from),
		ui.GetStateStyle(to).Render(to))

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

func runPromote(ctx context.Context, opts *promoteOptions, selector string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	festivalsDir := ""
	if ws, ok := scope.WorkspaceFrom(ctx); ok && ws != nil {
		festivalsDir = ws.FestivalsPath
	}

	festival, fromPicker, err := resolveFestivalForPromote(ctx, festivalsDir, cwd, selector, !opts.json)
	if err != nil {
		if err == errPromoteCancelled {
			fmt.Println(ui.Dim("Promotion cancelled"))
			return nil
		}
		if opts.json {
			if encErr := shared.EncodeJSON(os.Stdout, map[string]any{
				"success": false,
				"error":   err.Error(),
			}); encErr != nil {
				return encErr
			}
			return errors.ErrAlreadyPrinted
		}
		return err
	}

	_, err = promoteCore(ctx, festival, fromPicker, opts)
	return err
}

// PromoteResolved promotes an already-resolved festival through the fest promote
// flow, asking for confirmation, and returns the post-move path (empty if nothing moved).
func PromoteResolved(ctx context.Context, festival *show.FestivalInfo) (string, error) {
	var err error
	ctx, err = ensureWorkspaceContext(ctx, festival.Path, workspace.FindWorkspace)
	if err != nil {
		return "", errors.Wrap(err, "resolving workspace for promotion")
	}
	return promoteCore(ctx, festival, true, &promoteOptions{})
}

type workspaceResolver func(context.Context, string) (workspace.WorkspaceInfo, error)

// ensureWorkspaceContext supplies the workspace metadata that lifecycle
// auto-commit requires when promotion is embedded in a global-scope command
// such as fest watch. Direct fest promote calls already carry this context.
func ensureWorkspaceContext(ctx context.Context, festivalPath string, resolve workspaceResolver) (context.Context, error) {
	if ws, ok := scope.WorkspaceFrom(ctx); ok && ws != nil {
		return ctx, nil
	}

	resolved, err := resolve(ctx, festivalPath)
	if err != nil {
		return ctx, err
	}

	workspaceType := scope.WorkspaceTypeStandalone
	if resolved.Type == workspace.WorkspaceTypeCampaign {
		workspaceType = scope.WorkspaceTypeCampaign
	}

	return scope.WithWorkspace(ctx, &scope.WorkspaceInfo{
		Root:          resolved.Root,
		FestivalsPath: resolved.FestivalsPath,
		Type:          workspaceType,
	}), nil
}

func promoteCore(ctx context.Context, festival *show.FestivalInfo, confirm bool, opts *promoteOptions) (string, error) {
	currentStatus := festival.Status

	// Resolve auto-commit policy from trusted configuration before honoring
	// the --no-commit flag. Agents must not bypass required auto-commit.
	if opts.noCommit {
		festivalsRoot, _ := tpl.FindFestivalsRoot(festival.Path)
		agentCfg, err := config.LoadEffectiveAgentConfig(festivalsRoot, festival.Path)
		if err != nil {
			return "", errors.Wrap(err, "loading auto-commit policy")
		}
		shouldCommit, rejected := config.EffectiveAutoCommit(agentCfg, opts.noCommit)
		if rejected {
			if opts.json {
				if encErr := shared.EncodeJSON(os.Stdout, map[string]any{
					"success": false,
					"error":   config.AutoCommitRequiredMessage,
					"hint":    config.AutoCommitRequiredHint,
				}); encErr != nil {
					return "", encErr
				}
				return "", errors.ErrAlreadyPrinted
			}
			return "", errors.Validation(config.AutoCommitRequiredMessage).
				WithHint(config.AutoCommitRequiredHint)
		}
		opts.noCommit = !shouldCommit
	}

	var nextStatus string

	if opts.dungeon != "" {
		// Dungeon override: resolve the dungeon status
		resolved := id.ResolveStatusPath(opts.dungeon)
		if !strings.HasPrefix(resolved, "dungeon/") {
			if opts.json {
				if encErr := shared.EncodeJSON(os.Stdout, map[string]any{
					"success": false,
					"error":   fmt.Sprintf("invalid dungeon status %q", opts.dungeon),
					"value":   opts.dungeon,
					"hint":    "valid values: completed, archived, someday",
				}); encErr != nil {
					return "", encErr
				}
				return "", errors.ErrAlreadyPrinted
			}
			return "", errors.Validation("invalid dungeon status").
				WithField("value", opts.dungeon).
				WithHint("valid values: completed, archived, someday")
		}
		nextStatus = resolved
	} else {
		// Standard lifecycle promotion
		var ok bool
		nextStatus, ok = validTransitions[currentStatus]
		if !ok {
			if opts.json {
				if encErr := shared.EncodeJSON(os.Stdout, map[string]any{
					"success": false,
					"error":   fmt.Sprintf("cannot promote festival with status %q", currentStatus),
					"status":  currentStatus,
				}); encErr != nil {
					return "", encErr
				}
				return "", errors.ErrAlreadyPrinted
			}
			return "", errors.Validation("cannot promote festival").
				WithField("status", currentStatus).
				WithHint("only planning, ready, and active festivals can be promoted")
		}

		// Validate readiness unless forced (skip for dungeon moves)
		if !opts.force {
			if err := validateReadiness(ctx, festival, currentStatus, nextStatus); err != nil {
				if opts.json {
					if encErr := shared.EncodeJSON(os.Stdout, map[string]any{
						"success": false,
						"error":   err.Error(),
						"from":    currentStatus,
						"to":      nextStatus,
						"hint":    "use --force to skip validation",
					}); encErr != nil {
						return "", encErr
					}
					return "", errors.ErrAlreadyPrinted
				}
				fmt.Printf("%s %s\n", ui.Warning("Promotion blocked"), ui.Dim(err.Error()))
				fmt.Printf("\n  %s\n", ui.Dim("Use --force to skip validation"))
				return "", nil
			}
		}
	}

	// Chain dependency gate: block ready → active if hard upstream deps are incomplete.
	if nextStatus == "active" && !opts.force && opts.dungeon == "" {
		if blocked, blockMsg := checkChainDependencies(ctx, festival); blocked {
			if opts.json {
				if encErr := shared.EncodeJSON(os.Stdout, map[string]any{
					"success": false,
					"error":   "chain dependencies not met",
					"details": blockMsg,
					"hint":    "use --force to skip chain dependency check",
				}); encErr != nil {
					return "", encErr
				}
				return "", errors.ErrAlreadyPrinted
			}
			fmt.Printf("%s %s\n", ui.Warning("Chain dependency gate:"), blockMsg)
			fmt.Printf("\n  %s\n", ui.Dim("Use --force to skip chain dependency check"))
			return "", nil
		}
	}

	if confirm && !confirmPromotion(festival.Name, currentStatus, nextStatus) {
		fmt.Println(ui.Dim("Promotion cancelled"))
		return "", nil
	}

	// Execute the move using existing atomic status change
	newPath, err := status.AtomicStatusChange(ctx, festival.Path, currentStatus, nextStatus)
	if err != nil {
		return "", errors.Wrap(err, "promoting festival")
	}

	// Update FESTIVAL_GOAL.md frontmatter with the new status
	festivalGoalPath := filepath.Join(newPath, "FESTIVAL_GOAL.md")
	if _, statErr := os.Stat(festivalGoalPath); statErr == nil {
		if fmErr := status.UpdateGoalFrontmatter(ctx, festivalGoalPath, frontmatter.Status(nextStatus)); fmErr != nil {
			fmt.Printf("%s %s\n", ui.Dim("Warning: could not update FESTIVAL_GOAL.md frontmatter:"), ui.Dim(fmErr.Error()))
		}
	}

	// Update fest.yaml metadata with the new status
	var festivalID string
	workspaceRoot := ""
	if ws, wsErr := workspace.FindWorkspace(ctx, newPath); wsErr == nil {
		workspaceRoot = ws.Root
	}
	if festCfg, cfgErr := config.LoadFestivalConfig(newPath, workspaceRoot); cfgErr == nil {
		festivalID = festCfg.Metadata.ID
		festCfg.Metadata.AddStatusChange(nextStatus, newPath, "")
		if saveErr := config.SaveFestivalConfig(newPath, workspaceRoot, festCfg); saveErr != nil {
			fmt.Printf("%s %s\n", ui.Dim("Warning: could not update fest.yaml status:"), ui.Dim(saveErr.Error()))
		}
	}

	// Update navigation links after successful move
	linkAction := status.UpdateNavigationAfterMove(ctx, festival.Name, nextStatus, newPath)

	// Auto-commit the status change unless --no-commit was specified
	var commitHash string
	if !opts.noCommit {
		// Build the list of paths touched by the promotion
		changedPaths := []string{festival.Path, newPath}
		if linkAction != "" {
			if navPath, navErr := navigation.NavigationPath(); navErr == nil {
				changedPaths = append(changedPaths, navPath)
			}
		}
		hash, commitErr := status.AutoCommitStatusChange(ctx, festival.Name, festivalID, currentStatus, nextStatus, changedPaths)
		if commitErr != nil {
			fmt.Printf("%s %s\n", ui.Dim("Warning: auto-commit failed:"), ui.Dim(commitErr.Error()))
		} else if hash != "" {
			commitHash = hash
		}
	}

	// Output result
	if opts.json {
		result := map[string]any{
			"success":  true,
			"festival": festival.Name,
			"from":     currentStatus,
			"to":       nextStatus,
			"new_path": newPath,
		}
		if commitHash != "" {
			result["commit"] = commitHash
		}
		return newPath, shared.EncodeJSON(os.Stdout, result)
	}

	fmt.Println(ui.H2("Festival Promoted"))
	fmt.Printf("%s %s\n", ui.Label("Festival"), ui.Value(festival.Name, ui.FestivalColor))
	fmt.Printf("%s %s → %s\n",
		ui.Label("Status"),
		ui.GetStateStyle(currentStatus).Render(currentStatus),
		ui.GetStateStyle(nextStatus).Render(nextStatus))
	fmt.Printf("%s %s\n", ui.Label("New path"), ui.Dim(newPath))
	if commitHash != "" {
		fmt.Printf("%s %s\n", ui.Label("Commit"), ui.Dim(commitHash))
	}

	return newPath, nil
}

// validateReadiness checks if a festival is ready for the next lifecycle status.
func validateReadiness(ctx context.Context, festival *show.FestivalInfo, from, to string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch to {
	case "ready":
		return validatePlannedToReady(festival)
	case "active":
		return nil // ready → active has no additional requirements
	case "completed":
		return validateActiveToCompleted(ctx, festival)
	default:
		return nil
	}
}

// validatePlannedToReady checks that a festival is ready to move to the ready status.
func validatePlannedToReady(festival *show.FestivalInfo) error {
	// Festival goal must exist
	goalPath := filepath.Join(festival.Path, "FESTIVAL_GOAL.md")
	if _, err := os.Stat(goalPath); err != nil {
		return fmt.Errorf("FESTIVAL_GOAL.md is missing")
	}
	return nil
}

// validateActiveToCompleted checks that a festival is ready to be completed.
func validateActiveToCompleted(ctx context.Context, festival *show.FestivalInfo) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	mgr, mgrErr := progress.NewManager(ctx, festival.Path)
	if mgrErr != nil {
		return fmt.Errorf("could not initialize progress manager: %w", mgrErr)
	}
	festProgress, err := mgr.GetFestivalProgress(ctx, festival.Path)
	if err != nil {
		return fmt.Errorf("could not calculate progress: %w", err)
	}

	if festProgress.Overall.Total == 0 {
		return fmt.Errorf("festival has no tasks")
	}

	if festProgress.Overall.Completed < festProgress.Overall.Total {
		return fmt.Errorf("%d of %d tasks incomplete",
			festProgress.Overall.Total-festProgress.Overall.Completed,
			festProgress.Overall.Total)
	}

	return nil
}

// checkChainDependencies checks if a festival's hard upstream chain dependencies
// are all completed. Returns (blocked, message). If the festival is not part of
// any chain, returns (false, "").
func checkChainDependencies(ctx context.Context, festival *show.FestivalInfo) (bool, string) {
	// Load festival ID from fest.yaml.
	festCfg, cfgErr := config.LoadFestivalConfig(festival.Path, "")
	if cfgErr != nil || !festCfg.Metadata.HasMetadata() {
		return false, "" // Can't determine ID — skip silently.
	}
	festivalID := festCfg.Metadata.ID

	// Anchor the festivals root on the festival being promoted, not process cwd:
	// selector and picker resolution can promote from outside festivals/.
	root, err := tpl.FindFestivalsRoot(festival.Path)
	if err != nil {
		return false, ""
	}

	// Find chain containing this festival.
	c, ref, err := chainpkg.FindForFestival(ctx, festivalID, root)
	if err != nil || c == nil {
		return false, "" // Not in any chain — skip silently.
	}

	// Build search dirs for status resolution.
	searchDirs := make([]string, len(id.StatusDirectories))
	for i, d := range id.StatusDirectories {
		searchDirs[i] = workspace.JoinStatus(root, d)
	}

	// Best-effort: a missing chain member must not blank the upstream we gate on.
	resolved, _ := chainpkg.ResolveAvailable(ctx, c, searchDirs)
	statuses := make(map[string]chainpkg.FestivalStatus, len(c.Festivals))
	for _, node := range c.Festivals {
		rf, ok := resolved[node.Ref]
		if !ok || rf.Path == "" {
			statuses[node.Ref] = chainpkg.FestivalPlanning
			continue
		}
		cfg, cfgErr := config.LoadFestivalConfig(rf.Path, "")
		if cfgErr != nil {
			statuses[node.Ref] = chainpkg.FestivalPlanning
			continue
		}
		statuses[node.Ref] = mapFestivalStatus(cfg.Metadata.CurrentStatus())
	}

	// Check hard upstream deps.
	var incomplete []string
	for _, e := range c.Edges {
		if e.To == ref && e.Type == chainpkg.EdgeHard {
			if statuses[e.From] != chainpkg.FestivalCompleted {
				upstream := c.FestivalByRef(e.From)
				if upstream != nil {
					incomplete = append(incomplete, fmt.Sprintf("%s (%s)", upstream.Name, upstream.ID))
				}
			}
		}
	}

	if len(incomplete) == 0 {
		return false, ""
	}

	return true, fmt.Sprintf("%d hard upstream dependencies incomplete: %s",
		len(incomplete), strings.Join(incomplete, ", "))
}

// mapFestivalStatus converts a string status to a chain FestivalStatus.
func mapFestivalStatus(s string) chainpkg.FestivalStatus {
	switch s {
	case "planning":
		return chainpkg.FestivalPlanning
	case "ready":
		return chainpkg.FestivalReady
	case "active":
		return chainpkg.FestivalActive
	case "completed", "dungeon/completed":
		return chainpkg.FestivalCompleted
	default:
		return chainpkg.FestivalPlanning
	}
}
