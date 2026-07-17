package system

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/workflow"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// phasePattern matches numbered phase directories like "001_PLANNING", "002_IMPLEMENTATION".
var phasePattern = regexp.MustCompile(`^\d{3}_`)

// looksLikeFestival returns true if a directory appears to be a festival
// (has fest.yaml, .fest/, or numbered phase directories inside).
func looksLikeFestival(dirPath string) bool {
	if _, err := os.Stat(filepath.Join(dirPath, "fest.yaml")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dirPath, ".fest")); err == nil {
		return true
	}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() && phasePattern.MatchString(entry.Name()) {
			return true
		}
	}
	return false
}

// analyzeDungeonOrphans scans dungeon/ for items that aren't valid schema children.
// Items that look like festivals are orphans (moved to archived/).
// Items that don't look like festivals are unknown children (flagged for manual review).
func analyzeDungeonOrphans(ctx context.Context, festivalsRoot string, plan *repairPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	schema := workflow.FestivalSchema()
	dungeonDir, ok := schema.Directories["dungeon"]
	if !ok {
		return nil
	}

	dungeonPath := workspace.JoinDungeon(festivalsRoot)
	entries, err := os.ReadDir(dungeonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	validChildren := make(map[string]bool, len(dungeonDir.Children))
	for name := range dungeonDir.Children {
		validChildren[name] = true
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || validChildren[entry.Name()] {
			continue
		}
		entryPath := filepath.Join(dungeonPath, entry.Name())
		if looksLikeFestival(entryPath) {
			plan.dungeonOrphans = append(plan.dungeonOrphans, entry.Name())
		} else {
			plan.unknownChildren = append(plan.unknownChildren, entry.Name())
		}
	}

	return nil
}

// analyzeLegacyProgress walks all status directories looking for festivals
// with .fest/progress.yaml that haven't been converted to progress_events.jsonl.
func analyzeLegacyProgress(ctx context.Context, festivalsRoot string, plan *repairPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	seen := make(map[string]bool)

	// Scan orphan festivals (will be moved later, but paths are still valid now)
	for _, orphan := range plan.dungeonOrphans {
		p := workspace.JoinDungeon(festivalsRoot, orphan)
		if hasLegacyProgress(p) {
			plan.progressMigrate = append(plan.progressMigrate, p)
			seen[p] = true
		}
	}

	// Scan unknown children's contents
	for _, unknown := range plan.unknownChildren {
		unknownPath := workspace.JoinDungeon(festivalsRoot, unknown)
		scanFestivalsInDir(ctx, unknownPath, plan, seen)
	}

	// Scan all schema status directories
	schema := workflow.FestivalSchema()
	for name, dir := range schema.Directories {
		if err := ctx.Err(); err != nil {
			return err
		}
		if dir.Nested && len(dir.Children) > 0 {
			for child := range dir.Children {
				scanFestivalsInDir(ctx, workspace.JoinStatus(festivalsRoot, name+"/"+child), plan, seen)
			}
		} else {
			scanFestivalsInDir(ctx, workspace.JoinStatus(festivalsRoot, name), plan, seen)
		}
	}

	return nil
}

// scanFestivalsInDir scans a directory for festival subdirs with legacy progress files.
func scanFestivalsInDir(ctx context.Context, dirPath string, plan *repairPlan, seen map[string]bool) {
	if ctx.Err() != nil {
		return
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		festPath := filepath.Join(dirPath, entry.Name())
		if seen[festPath] {
			continue
		}
		if hasLegacyProgress(festPath) {
			plan.progressMigrate = append(plan.progressMigrate, festPath)
			seen[festPath] = true
		}
	}
}

// hasLegacyProgress returns true if a festival has progress.yaml but no progress_events.jsonl.
func hasLegacyProgress(festivalPath string) bool {
	legacyPath := filepath.Join(festivalPath, progress.ProgressDir, progress.ProgressFileName)
	eventsPath := filepath.Join(festivalPath, progress.ProgressDir, progress.ProgressEventsFile)

	if _, err := os.Stat(legacyPath); err != nil {
		return false
	}
	if _, err := os.Stat(eventsPath); err == nil {
		return false // already has JSONL
	}
	return true
}

// countSubdirs counts non-hidden subdirectories in a path.
func countSubdirs(dirPath string) int {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			count++
		}
	}
	return count
}
