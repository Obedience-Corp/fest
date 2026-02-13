package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workflow"
	"github.com/spf13/cobra"
)

type repairOptions struct {
	dryRun bool
	force  bool
}

// NewRepairCommand creates the system repair subcommand.
func NewRepairCommand() *cobra.Command {
	opts := &repairOptions{}
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Fix festival directory layout issues",
		Long: `Repair the festivals/ directory by detecting and fixing common issues.

This command analyzes your festival directory structure and fixes:
  - Renames planned/ → planning/ (old naming convention)
  - Moves completed/ → dungeon/completed/ (old layout)
  - Creates missing directories (ready/, ritual/, dungeon/ subdirs)
  - Creates .workflow.yaml if missing
  - Moves orphan festivals from dungeon/ root → dungeon/archived/
  - Converts legacy progress.yaml → progress_events.jsonl

The repair command is safe to run multiple times - it only makes changes
when issues are detected.`,
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepair(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "preview changes without executing")
	cmd.Flags().BoolVar(&opts.force, "force", false, "skip confirmation prompt")

	return cmd
}

func runRepair(ctx context.Context, opts *repairOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	festivalsRoot, err := tpl.FindFestivalsRoot(cwd)
	if err != nil {
		return errors.Wrap(err, "finding festivals root").
			WithHint("run this command from within a festivals/ directory")
	}

	plan, err := analyzeRepair(ctx, festivalsRoot)
	if err != nil {
		return errors.Wrap(err, "analyzing festival directory")
	}

	displayRepairPlan(plan)

	if !plan.hasIssues() {
		fmt.Println()
		fmt.Println(ui.Success("No issues detected - directory structure is valid"))
		return nil
	}

	if opts.dryRun {
		fmt.Printf("\n%s\n", ui.Dim("Dry run - no changes made"))
		return nil
	}

	if !opts.force && !confirmRepair() {
		return nil
	}

	if err := executeRepair(ctx, festivalsRoot, plan); err != nil {
		return errors.Wrap(err, "executing repair")
	}

	fmt.Println()
	fmt.Println(ui.Success("Repair complete"))
	fmt.Printf("  %s %s\n", ui.Label("Location"), ui.Dim(festivalsRoot))
	return nil
}

// confirmRepair prompts the user for confirmation and returns true if accepted.
func confirmRepair() bool {
	fmt.Printf("\nProceed with repair? [y/N] ")
	var response string
	fmt.Scanln(&response)
	if response != "y" && response != "Y" {
		fmt.Println(ui.Dim("Repair cancelled"))
		return false
	}
	return true
}

// repairPlan describes what issues were found and what will be fixed.
type repairPlan struct {
	festivalsRoot   string
	renameDirs      map[string]string // old → new name renames
	moveDirs        map[string]string // src → dst directory moves
	createDirs      []string          // directories to create
	createSchema    bool              // whether to create .workflow.yaml
	dungeonOrphans  []string          // flat items in dungeon/ → dungeon/archived/
	unknownChildren []string          // non-schema dungeon children (flag only)
	progressMigrate []string          // festival paths with legacy progress.yaml
}

// hasIssues returns true if there are any issues to fix.
func (p *repairPlan) hasIssues() bool {
	return len(p.renameDirs) > 0 || len(p.moveDirs) > 0 || len(p.createDirs) > 0 ||
		p.createSchema || len(p.dungeonOrphans) > 0 || len(p.progressMigrate) > 0
}

// analyzeRepair examines the current structure and builds a repair plan.
func analyzeRepair(ctx context.Context, festivalsRoot string) (*repairPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	plan := &repairPlan{
		festivalsRoot: festivalsRoot,
		renameDirs:    make(map[string]string),
		moveDirs:      make(map[string]string),
	}

	if err := analyzeRenames(festivalsRoot, plan); err != nil {
		return nil, err
	}
	analyzeMoves(festivalsRoot, plan)
	analyzeMissingDirs(festivalsRoot, plan)

	if err := analyzeDungeonOrphans(ctx, festivalsRoot, plan); err != nil {
		return nil, errors.Wrap(err, "analyzing dungeon contents")
	}
	if err := analyzeLegacyProgress(ctx, festivalsRoot, plan); err != nil {
		return nil, errors.Wrap(err, "analyzing legacy progress files")
	}

	return plan, nil
}

// analyzeRenames checks for old directory naming conventions that need renaming.
func analyzeRenames(festivalsRoot string, plan *repairPlan) error {
	svc := workflow.NewService(festivalsRoot)
	if !svc.HasSchema() {
		plan.createSchema = true
	}

	plannedPath := filepath.Join(festivalsRoot, "planned")
	planningPath := filepath.Join(festivalsRoot, "planning")
	info, err := os.Stat(plannedPath)
	if err != nil || !info.IsDir() {
		return nil
	}
	if _, err := os.Stat(planningPath); os.IsNotExist(err) {
		plan.renameDirs["planned"] = "planning"
	} else {
		return fmt.Errorf("both planned/ and planning/ exist — manual intervention required")
	}
	return nil
}

// analyzeMoves checks for top-level directories that belong under dungeon/.
func analyzeMoves(festivalsRoot string, plan *repairPlan) {
	completedPath := filepath.Join(festivalsRoot, "completed")
	info, err := os.Stat(completedPath)
	if err != nil || !info.IsDir() {
		return
	}
	dungeonCompletedPath := filepath.Join(festivalsRoot, "dungeon", "completed")
	if _, err := os.Stat(dungeonCompletedPath); os.IsNotExist(err) {
		plan.moveDirs["completed"] = "dungeon/completed"
	}
}

// analyzeMissingDirs checks which schema directories need creation.
func analyzeMissingDirs(festivalsRoot string, plan *repairPlan) {
	schema := workflow.FestivalSchema()
	for _, dirPath := range schema.AllDirectories() {
		if isPlannedTarget(dirPath, plan) {
			continue
		}
		fullPath := filepath.Join(festivalsRoot, dirPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			plan.createDirs = append(plan.createDirs, dirPath)
		}
	}
}

// isPlannedTarget returns true if the directory will be created by a rename or move.
func isPlannedTarget(dirPath string, plan *repairPlan) bool {
	for _, newName := range plan.renameDirs {
		if newName == dirPath {
			return true
		}
	}
	for _, dst := range plan.moveDirs {
		if dst == dirPath {
			return true
		}
	}
	return false
}

// displayRepairPlan shows the repair plan to the user.
func displayRepairPlan(plan *repairPlan) {
	fmt.Println(ui.H2("Festival Directory Repair"))
	fmt.Printf("  %s %s\n\n", ui.Label("Location"), ui.Dim(plan.festivalsRoot))

	displayRenames(plan)
	displayMoves(plan)
	displayCreates(plan)
	displaySchema(plan)
	displayDungeonOrphans(plan)
	displayUnknownChildren(plan)
	displayProgressMigration(plan)
}

func displayRenames(plan *repairPlan) {
	if len(plan.renameDirs) == 0 {
		return
	}
	fmt.Println(ui.Label("  Renames:"))
	for old, newName := range plan.renameDirs {
		fmt.Printf("    %s → %s\n", ui.Value(old+"/", ui.WarningColor), ui.Value(newName+"/", ui.SuccessColor))
	}
	fmt.Println()
}

func displayMoves(plan *repairPlan) {
	if len(plan.moveDirs) == 0 {
		return
	}
	fmt.Println(ui.Label("  Moves:"))
	for src, dst := range plan.moveDirs {
		fmt.Printf("    %s → %s\n", ui.Value(src+"/", ui.WarningColor), ui.Value(dst+"/", ui.SuccessColor))
	}
	fmt.Println()
}

func displayCreates(plan *repairPlan) {
	if len(plan.createDirs) == 0 {
		return
	}
	fmt.Println(ui.Label("  Create:"))
	for _, dir := range plan.createDirs {
		fmt.Printf("    %s\n", ui.Value(dir+"/", ui.SuccessColor))
	}
	fmt.Println()
}

func displaySchema(plan *repairPlan) {
	if !plan.createSchema {
		return
	}
	fmt.Printf("  %s %s\n\n", ui.Label("Schema"), ui.Value(".workflow.yaml", ui.SuccessColor))
}

func displayDungeonOrphans(plan *repairPlan) {
	if len(plan.dungeonOrphans) == 0 {
		return
	}
	fmt.Printf("  %s\n", ui.Label("Dungeon Orphans (→ dungeon/archived/):"))
	for _, name := range plan.dungeonOrphans {
		fmt.Printf("    %s\n", ui.Value(name+"/", ui.WarningColor))
	}
	fmt.Println()
}

func displayUnknownChildren(plan *repairPlan) {
	if len(plan.unknownChildren) == 0 {
		return
	}
	fmt.Printf("  %s\n", ui.Warning("Unknown Dungeon Children (manual action needed):"))
	for _, name := range plan.unknownChildren {
		entryPath := filepath.Join(plan.festivalsRoot, "dungeon", name)
		count := countSubdirs(entryPath)
		fmt.Printf("    %s  (contains %d items)\n", ui.Value(name+"/", ui.WarningColor), count)
	}
	fmt.Println()
}

func displayProgressMigration(plan *repairPlan) {
	if len(plan.progressMigrate) == 0 {
		return
	}
	fmt.Printf("  %s\n", ui.Label("Progress Migration (progress.yaml → progress_events.jsonl):"))
	fmt.Printf("    %d festivals with legacy progress format\n", len(plan.progressMigrate))
	fmt.Println()
}
