package flow

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

type upgradeOptions struct {
	dryRun bool
	force  bool
}

// newUpgradeCommand creates the flow upgrade subcommand.
func newUpgradeCommand() *cobra.Command {
	opts := &upgradeOptions{}
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Migrate festivals/ to the extended workflow structure",
		Long: `Upgrade the festivals/ directory to use the full workflow structure.

This creates the extended directory layout:
  planning/   - Festivals being designed
  ready/      - Festivals ready for execution
  active/     - Currently executing festivals
  dungeon/    - Terminal states
    completed/  - Successfully finished
    archived/   - Preserved but inactive
    someday/    - Deferred, may revisit

If completed/ exists at the top level, it is moved to dungeon/completed/.
A .workflow.yaml schema file is created to track the workflow configuration.`,
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "preview changes without executing")
	cmd.Flags().BoolVar(&opts.force, "force", false, "skip confirmation prompt")

	return cmd
}

func runUpgrade(ctx context.Context, opts *upgradeOptions) error {
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

	// Check if workflow already exists
	svc := workflow.NewService(festivalsRoot)
	if svc.HasSchema() {
		fmt.Println(ui.Success("Workflow already initialized"))
		fmt.Printf("  %s %s\n", ui.Label("Location"), ui.Dim(festivalsRoot))
		return nil
	}

	// Analyze current structure
	plan, err := analyzeMigration(ctx, festivalsRoot)
	if err != nil {
		return errors.Wrap(err, "analyzing migration")
	}

	// Display plan
	displayPlan(plan)

	if opts.dryRun {
		fmt.Printf("\n%s\n", ui.Dim("Dry run - no changes made"))
		return nil
	}

	// Confirm unless forced
	if !opts.force {
		fmt.Printf("\nProceed with upgrade? [y/N] ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println(ui.Dim("Upgrade cancelled"))
			return nil
		}
	}

	// Execute migration
	if err := executeMigration(ctx, festivalsRoot, plan); err != nil {
		return errors.Wrap(err, "executing migration")
	}

	fmt.Println()
	fmt.Println(ui.Success("Workflow upgrade complete"))
	fmt.Printf("  %s %s\n", ui.Label("Location"), ui.Dim(festivalsRoot))

	return nil
}

// migrationPlan describes what the upgrade will do.
type migrationPlan struct {
	festivalsRoot string
	createDirs    []string          // Directories to create
	moveDirs      map[string]string // src → dst directory moves
	createSchema  bool              // Whether to create .workflow.yaml
}

// analyzeMigration examines the current structure and builds a migration plan.
func analyzeMigration(ctx context.Context, festivalsRoot string) (*migrationPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	plan := &migrationPlan{
		festivalsRoot: festivalsRoot,
		moveDirs:      make(map[string]string),
		createSchema:  true,
	}

	// Check if completed/ exists at top level (needs to move to dungeon/completed/)
	completedPath := filepath.Join(festivalsRoot, "completed")
	if info, err := os.Stat(completedPath); err == nil && info.IsDir() {
		dungeonCompletedPath := filepath.Join(festivalsRoot, "dungeon", "completed")
		if _, err := os.Stat(dungeonCompletedPath); os.IsNotExist(err) {
			plan.moveDirs["completed"] = "dungeon/completed"
		}
	}

	// Check which directories need creation (skip dirs that will be created by moves)
	schema := workflow.FestivalSchema()
	for _, dirPath := range schema.AllDirectories() {
		// Skip if this directory will be created by a move
		if _, isMoveDst := plan.moveDirs[dirPath]; isMoveDst {
			continue
		}
		// Skip if a move creates this path
		skipCreate := false
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

// displayPlan shows the migration plan to the user.
func displayPlan(plan *migrationPlan) {
	fmt.Println(ui.H2("Workflow Upgrade Plan"))
	fmt.Printf("  %s %s\n\n", ui.Label("Location"), ui.Dim(plan.festivalsRoot))

	if len(plan.moveDirs) > 0 {
		fmt.Println(ui.Label("  Moves:"))
		for src, dst := range plan.moveDirs {
			fmt.Printf("    %s → %s\n", ui.Value(src, ui.WarningColor), ui.Value(dst, ui.SuccessColor))
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

// executeMigration performs the actual migration.
func executeMigration(ctx context.Context, festivalsRoot string, plan *migrationPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// 1. Create dungeon directory first (parent for moves)
	dungeonPath := filepath.Join(festivalsRoot, "dungeon")
	if err := os.MkdirAll(dungeonPath, 0755); err != nil {
		return fmt.Errorf("creating dungeon directory: %w", err)
	}

	// 2. Execute directory moves (completed/ → dungeon/completed/)
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

	// 3. Create missing directories
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

	// 4. Write workflow schema directly (don't use Init - it checks for parent
	// flows and uses DefaultSchema instead of FestivalSchema)
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

	return nil
}
