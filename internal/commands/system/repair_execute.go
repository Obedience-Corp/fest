package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workflow"
	"gopkg.in/yaml.v3"
)

// executeRepair performs the actual repair operations.
func executeRepair(ctx context.Context, festivalsRoot string, plan *repairPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := executeRenames(ctx, festivalsRoot, plan); err != nil {
		return err
	}
	if err := executeMoves(ctx, festivalsRoot, plan); err != nil {
		return err
	}
	if err := executeCreateDirs(ctx, festivalsRoot, plan); err != nil {
		return err
	}
	if err := executeCreateSchema(ctx, festivalsRoot, plan); err != nil {
		return err
	}
	if err := executeProgressMigration(ctx, festivalsRoot, plan); err != nil {
		return err
	}
	return executeOrphanMoves(ctx, festivalsRoot, plan)
}

// executeRenames applies directory renames (e.g. planned/ → planning/).
func executeRenames(ctx context.Context, festivalsRoot string, plan *repairPlan) error {
	for old, newName := range plan.renameDirs {
		if err := ctx.Err(); err != nil {
			return err
		}
		oldPath := filepath.Join(festivalsRoot, old)
		newPath := filepath.Join(festivalsRoot, newName)
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("renaming %s to %s: %w", old, newName, err)
		}
		fmt.Printf("  %s %s → %s\n", ui.Success("Renamed"), old, newName)
	}
	return nil
}

// executeMoves applies directory moves (e.g. completed/ → dungeon/completed/).
func executeMoves(ctx context.Context, festivalsRoot string, plan *repairPlan) error {
	// Ensure dungeon/ exists as parent for moves
	dungeonPath := filepath.Join(festivalsRoot, "dungeon")
	if err := os.MkdirAll(dungeonPath, 0755); err != nil {
		return fmt.Errorf("creating dungeon directory: %w", err)
	}

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
	return nil
}

// executeCreateDirs creates missing schema directories.
func executeCreateDirs(ctx context.Context, festivalsRoot string, plan *repairPlan) error {
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
	return nil
}

// executeCreateSchema writes the .workflow.yaml schema file.
func executeCreateSchema(ctx context.Context, festivalsRoot string, plan *repairPlan) error {
	if !plan.createSchema {
		return nil
	}
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

// executeProgressMigration converts legacy progress.yaml files to JSONL.
// Runs before orphan moves so paths are still valid.
func executeProgressMigration(ctx context.Context, festivalsRoot string, plan *repairPlan) error {
	for _, festivalPath := range plan.progressMigrate {
		if err := ctx.Err(); err != nil {
			return err
		}
		store := progress.NewStore(festivalPath)
		if err := store.Load(ctx); err != nil {
			rel, _ := filepath.Rel(festivalsRoot, festivalPath)
			fmt.Printf("  %s progress migration for %s: %v\n", ui.Warning("Warning"), rel, err)
			continue
		}
		rel, _ := filepath.Rel(festivalsRoot, festivalPath)
		fmt.Printf("  %s progress for %s\n", ui.Success("Migrated"), rel)
	}
	return nil
}

// executeOrphanMoves moves dungeon orphans into dungeon/archived/.
func executeOrphanMoves(ctx context.Context, festivalsRoot string, plan *repairPlan) error {
	if len(plan.dungeonOrphans) == 0 {
		return nil
	}

	archivedPath := filepath.Join(festivalsRoot, "dungeon", "archived")
	if err := os.MkdirAll(archivedPath, 0755); err != nil {
		return fmt.Errorf("creating dungeon/archived: %w", err)
	}

	for _, name := range plan.dungeonOrphans {
		if err := ctx.Err(); err != nil {
			return err
		}
		src := filepath.Join(festivalsRoot, "dungeon", name)
		dst := filepath.Join(archivedPath, name)
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("moving dungeon orphan %s: %w", name, err)
		}
		fmt.Printf("  %s dungeon/%s → dungeon/archived/%s\n", ui.Success("Moved"), name, name)
	}
	return nil
}
