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
	"gopkg.in/yaml.v3"
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

	// Find the festivals root
	festivalsRoot, err := tpl.FindFestivalsRoot(cwd)
	if err != nil {
		return errors.Wrap(err, "finding festivals root").
			WithHint("run this command from within a festivals/ directory")
	}

	// Analyze current structure
	plan, err := analyzeRepair(ctx, festivalsRoot)
	if err != nil {
		return errors.Wrap(err, "analyzing festival directory")
	}

	// Display what we found
	displayRepairPlan(plan)

	// If nothing to fix, we're done
	if !plan.hasIssues() {
		fmt.Println()
		fmt.Println(ui.Success("No issues detected - directory structure is valid"))
		return nil
	}

	if opts.dryRun {
		fmt.Printf("\n%s\n", ui.Dim("Dry run - no changes made"))
		return nil
	}

	// Confirm unless forced
	if !opts.force {
		fmt.Printf("\nProceed with repair? [y/N] ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println(ui.Dim("Repair cancelled"))
			return nil
		}
	}

	// Execute repairs
	if err := executeRepair(ctx, festivalsRoot, plan); err != nil {
		return errors.Wrap(err, "executing repair")
	}

	fmt.Println()
	fmt.Println(ui.Success("Repair complete"))
	fmt.Printf("  %s %s\n", ui.Label("Location"), ui.Dim(festivalsRoot))

	return nil
}

// repairPlan describes what issues were found and what will be fixed.
type repairPlan struct {
	festivalsRoot string
	renameDirs    map[string]string // old → new name renames
	moveDirs      map[string]string // src → dst directory moves
	createDirs    []string          // directories to create
	createSchema  bool              // whether to create .workflow.yaml
}

// hasIssues returns true if there are any issues to fix.
func (p *repairPlan) hasIssues() bool {
	return len(p.renameDirs) > 0 || len(p.moveDirs) > 0 || len(p.createDirs) > 0 || p.createSchema
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

	// Check if schema exists
	svc := workflow.NewService(festivalsRoot)
	if !svc.HasSchema() {
		plan.createSchema = true
	}

	// Check for old "planned/" directory (should be "planning/")
	plannedPath := filepath.Join(festivalsRoot, "planned")
	planningPath := filepath.Join(festivalsRoot, "planning")
	if info, err := os.Stat(plannedPath); err == nil && info.IsDir() {
		// Only rename if planning/ doesn't already exist
		if _, err := os.Stat(planningPath); os.IsNotExist(err) {
			plan.renameDirs["planned"] = "planning"
		} else {
			// Both exist - this is a conflict we can't auto-resolve
			fmt.Println(ui.Warning("Both planned/ and planning/ exist - manual intervention required"))
		}
	}

	// Check if completed/ exists at top level (should be dungeon/completed/)
	completedPath := filepath.Join(festivalsRoot, "completed")
	if info, err := os.Stat(completedPath); err == nil && info.IsDir() {
		dungeonCompletedPath := filepath.Join(festivalsRoot, "dungeon", "completed")
		if _, err := os.Stat(dungeonCompletedPath); os.IsNotExist(err) {
			plan.moveDirs["completed"] = "dungeon/completed"
		}
	}

	// Check which directories need creation (using FestivalSchema as reference)
	schema := workflow.FestivalSchema()
	for _, dirPath := range schema.AllDirectories() {
		// Skip if this directory will be created by a rename (it's a rename target)
		skipCreate := false
		for _, newName := range plan.renameDirs {
			if newName == dirPath {
				skipCreate = true
				break
			}
		}
		if skipCreate {
			continue
		}

		// Skip if this directory will be created by a move
		for _, dst := range plan.moveDirs {
			if dst == dirPath {
				skipCreate = true
				break
			}
		}
		if skipCreate {
			continue
		}

		fullPath := filepath.Join(festivalsRoot, dirPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			plan.createDirs = append(plan.createDirs, dirPath)
		}
	}

	return plan, nil
}

// displayRepairPlan shows the repair plan to the user.
func displayRepairPlan(plan *repairPlan) {
	fmt.Println(ui.H2("Festival Directory Repair"))
	fmt.Printf("  %s %s\n\n", ui.Label("Location"), ui.Dim(plan.festivalsRoot))

	if len(plan.renameDirs) > 0 {
		fmt.Println(ui.Label("  Renames:"))
		for old, new := range plan.renameDirs {
			fmt.Printf("    %s → %s\n", ui.Value(old+"/", ui.WarningColor), ui.Value(new+"/", ui.SuccessColor))
		}
		fmt.Println()
	}

	if len(plan.moveDirs) > 0 {
		fmt.Println(ui.Label("  Moves:"))
		for src, dst := range plan.moveDirs {
			fmt.Printf("    %s → %s\n", ui.Value(src+"/", ui.WarningColor), ui.Value(dst+"/", ui.SuccessColor))
		}
		fmt.Println()
	}

	if len(plan.createDirs) > 0 {
		fmt.Println(ui.Label("  Create:"))
		for _, dir := range plan.createDirs {
			fmt.Printf("    %s\n", ui.Value(dir+"/", ui.SuccessColor))
		}
		fmt.Println()
	}

	if plan.createSchema {
		fmt.Printf("  %s %s\n", ui.Label("Schema"), ui.Value(".workflow.yaml", ui.SuccessColor))
	}
}

// executeRepair performs the actual repair operations.
func executeRepair(ctx context.Context, festivalsRoot string, plan *repairPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// 1. Execute directory renames first (planned/ → planning/)
	for old, new := range plan.renameDirs {
		if err := ctx.Err(); err != nil {
			return err
		}
		oldPath := filepath.Join(festivalsRoot, old)
		newPath := filepath.Join(festivalsRoot, new)

		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("renaming %s to %s: %w", old, new, err)
		}
		fmt.Printf("  %s %s → %s\n", ui.Success("Renamed"), old, new)
	}

	// 2. Create dungeon directory first (parent for moves)
	dungeonPath := filepath.Join(festivalsRoot, "dungeon")
	if err := os.MkdirAll(dungeonPath, 0755); err != nil {
		return fmt.Errorf("creating dungeon directory: %w", err)
	}

	// 3. Execute directory moves (completed/ → dungeon/completed/)
	for src, dst := range plan.moveDirs {
		if err := ctx.Err(); err != nil {
			return err
		}
		srcPath := filepath.Join(festivalsRoot, src)
		dstPath := filepath.Join(festivalsRoot, dst)

		if err := os.Rename(srcPath, dstPath); err != nil {
			return fmt.Errorf("moving %s to %s: %w", src, dst, err)
		}
		fmt.Printf("  %s %s → %s\n", ui.Success("Moved"), src, dst)
	}

	// 4. Create missing directories
	for _, dir := range plan.createDirs {
		if err := ctx.Err(); err != nil {
			return err
		}
		fullPath := filepath.Join(festivalsRoot, dir)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", dir, err)
			}
			fmt.Printf("  %s %s/\n", ui.Success("Created"), dir)
		}
	}

	// 5. Write workflow schema if needed
	if plan.createSchema {
		if err := ctx.Err(); err != nil {
			return err
		}
		schema := workflow.FestivalSchema()
		data, err := yaml.Marshal(schema)
		if err != nil {
			return fmt.Errorf("marshaling schema: %w", err)
		}
		schemaPath := filepath.Join(festivalsRoot, workflow.SchemaFileName)
		if err := os.WriteFile(schemaPath, data, 0644); err != nil {
			return fmt.Errorf("writing schema file: %w", err)
		}
		fmt.Printf("  %s %s\n", ui.Success("Created"), ".workflow.yaml")
	}

	return nil
}
