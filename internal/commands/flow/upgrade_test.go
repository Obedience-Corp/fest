package flow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/workflow"
)

// setupFestivalsDir creates a test festivals directory with .festival/ metadata.
func setupFestivalsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	festivalsDir := filepath.Join(dir, "festivals")

	// Create festivals structure with .festival metadata
	os.MkdirAll(filepath.Join(festivalsDir, ".festival"), 0755)
	os.MkdirAll(filepath.Join(festivalsDir, "active"), 0755)
	os.MkdirAll(filepath.Join(festivalsDir, "planning"), 0755)

	return festivalsDir
}

func TestAnalyzeMigration_FreshDirectory(t *testing.T) {
	festivalsDir := setupFestivalsDir(t)

	plan, err := analyzeMigration(context.Background(), festivalsDir)
	if err != nil {
		t.Fatalf("analyzeMigration error: %v", err)
	}

	// Should have no moves (no completed/ at top level)
	if len(plan.moveDirs) != 0 {
		t.Errorf("expected no moves, got %d", len(plan.moveDirs))
	}

	// Should need to create some dirs
	if len(plan.createDirs) == 0 {
		t.Error("expected directories to create")
	}

	// Should want to create schema
	if !plan.createSchema {
		t.Error("expected createSchema to be true")
	}
}

func TestAnalyzeMigration_WithCompleted(t *testing.T) {
	festivalsDir := setupFestivalsDir(t)

	// Create completed/ at top level with content
	os.MkdirAll(filepath.Join(festivalsDir, "completed", "test-festival"), 0755)
	os.WriteFile(filepath.Join(festivalsDir, "completed", "test-festival", "FESTIVAL_GOAL.md"), []byte("goal"), 0644)

	plan, err := analyzeMigration(context.Background(), festivalsDir)
	if err != nil {
		t.Fatalf("analyzeMigration error: %v", err)
	}

	// Should plan to move completed/ → dungeon/completed/
	if dst, ok := plan.moveDirs["completed"]; !ok {
		t.Error("expected move for completed/")
	} else if dst != "dungeon/completed" {
		t.Errorf("expected move to dungeon/completed, got %s", dst)
	}

	// dungeon/completed should NOT appear in createDirs since it's a move target
	for _, dir := range plan.createDirs {
		if dir == "dungeon/completed" {
			t.Error("dungeon/completed should not be in createDirs when it's a move target")
		}
	}
}

func TestAnalyzeMigration_WithExistingDungeon(t *testing.T) {
	festivalsDir := setupFestivalsDir(t)

	// Create existing dungeon structure
	os.MkdirAll(filepath.Join(festivalsDir, "dungeon", "archived"), 0755)
	os.MkdirAll(filepath.Join(festivalsDir, "dungeon", "someday"), 0755)

	plan, err := analyzeMigration(context.Background(), festivalsDir)
	if err != nil {
		t.Fatalf("analyzeMigration error: %v", err)
	}

	// archived and someday already exist, so they shouldn't be in createDirs
	for _, dir := range plan.createDirs {
		if dir == "dungeon/archived" {
			t.Error("dungeon/archived should not be in createDirs - already exists")
		}
		if dir == "dungeon/someday" {
			t.Error("dungeon/someday should not be in createDirs - already exists")
		}
	}
}

func TestAnalyzeMigration_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := analyzeMigration(ctx, "/fake/path")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestExecuteMigration_FreshDirectory(t *testing.T) {
	festivalsDir := setupFestivalsDir(t)

	plan, err := analyzeMigration(context.Background(), festivalsDir)
	if err != nil {
		t.Fatalf("analyzeMigration error: %v", err)
	}

	err = executeMigration(context.Background(), festivalsDir, plan)
	if err != nil {
		t.Fatalf("executeMigration error: %v", err)
	}

	// Verify schema file was created
	schemaPath := filepath.Join(festivalsDir, workflow.SchemaFileName)
	if _, err := os.Stat(schemaPath); err != nil {
		t.Errorf("schema file not created: %v", err)
	}

	// Verify FestivalSchema directories exist
	schema := workflow.FestivalSchema()
	for _, dir := range schema.AllDirectories() {
		fullPath := filepath.Join(festivalsDir, dir)
		if _, err := os.Stat(fullPath); err != nil {
			t.Errorf("directory %q not created: %v", dir, err)
		}
	}
}

func TestExecuteMigration_MovesCompleted(t *testing.T) {
	festivalsDir := setupFestivalsDir(t)

	// Create completed/ with a festival
	os.MkdirAll(filepath.Join(festivalsDir, "completed", "test-festival"), 0755)
	os.WriteFile(
		filepath.Join(festivalsDir, "completed", "test-festival", "FESTIVAL_GOAL.md"),
		[]byte("Test goal"),
		0644,
	)

	plan, err := analyzeMigration(context.Background(), festivalsDir)
	if err != nil {
		t.Fatalf("analyzeMigration error: %v", err)
	}

	err = executeMigration(context.Background(), festivalsDir, plan)
	if err != nil {
		t.Fatalf("executeMigration error: %v", err)
	}

	// completed/ should no longer exist at top level
	if _, err := os.Stat(filepath.Join(festivalsDir, "completed")); err == nil {
		t.Error("completed/ should not exist at top level after migration")
	}

	// Content should be at dungeon/completed/
	goalPath := filepath.Join(festivalsDir, "dungeon", "completed", "test-festival", "FESTIVAL_GOAL.md")
	if _, err := os.Stat(goalPath); err != nil {
		t.Errorf("festival content not found at dungeon/completed/: %v", err)
	}

	// Verify content is preserved
	data, err := os.ReadFile(goalPath)
	if err != nil {
		t.Fatalf("reading moved file: %v", err)
	}
	if string(data) != "Test goal" {
		t.Errorf("content changed during migration: got %q, want %q", string(data), "Test goal")
	}
}

func TestExecuteMigration_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	plan := &migrationPlan{
		festivalsRoot: "/fake",
		moveDirs:      make(map[string]string),
	}

	err := executeMigration(ctx, "/fake", plan)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestExecuteMigration_PreservesExistingDungeon(t *testing.T) {
	festivalsDir := setupFestivalsDir(t)

	// Create existing dungeon content
	os.MkdirAll(filepath.Join(festivalsDir, "dungeon", "archived", "old-festival"), 0755)
	os.WriteFile(
		filepath.Join(festivalsDir, "dungeon", "archived", "old-festival", "data.txt"),
		[]byte("preserved"),
		0644,
	)

	plan, err := analyzeMigration(context.Background(), festivalsDir)
	if err != nil {
		t.Fatalf("analyzeMigration error: %v", err)
	}

	err = executeMigration(context.Background(), festivalsDir, plan)
	if err != nil {
		t.Fatalf("executeMigration error: %v", err)
	}

	// Existing dungeon content should be preserved
	data, err := os.ReadFile(filepath.Join(festivalsDir, "dungeon", "archived", "old-festival", "data.txt"))
	if err != nil {
		t.Fatalf("existing dungeon content lost: %v", err)
	}
	if string(data) != "preserved" {
		t.Errorf("existing dungeon content changed: got %q", string(data))
	}
}

func TestDryRunDoesNotModify(t *testing.T) {
	festivalsDir := setupFestivalsDir(t)

	// Create completed/ with content
	os.MkdirAll(filepath.Join(festivalsDir, "completed", "test-festival"), 0755)

	// Take snapshot of directory state
	entriesBefore, _ := os.ReadDir(festivalsDir)

	// Analyze (this is what dry-run uses)
	plan, err := analyzeMigration(context.Background(), festivalsDir)
	if err != nil {
		t.Fatalf("analyzeMigration error: %v", err)
	}

	// Verify the plan has changes
	if len(plan.moveDirs) == 0 && len(plan.createDirs) == 0 {
		t.Fatal("expected non-empty plan")
	}

	// Verify directory state is unchanged (analysis doesn't modify)
	entriesAfter, _ := os.ReadDir(festivalsDir)
	if len(entriesBefore) != len(entriesAfter) {
		t.Error("analyzeMigration modified directory state")
	}

	// Schema should not exist
	if _, err := os.Stat(filepath.Join(festivalsDir, workflow.SchemaFileName)); err == nil {
		t.Error("schema file should not exist after analysis-only")
	}
}

func TestAlreadyMigrated(t *testing.T) {
	festivalsDir := setupFestivalsDir(t)

	// Initially no schema
	svc := workflow.NewService(festivalsDir)
	if svc.HasSchema() {
		t.Fatal("schema should not exist initially")
	}

	// Write schema manually
	os.WriteFile(
		filepath.Join(festivalsDir, workflow.SchemaFileName),
		[]byte("version: 1\ntype: status-workflow\ndirectories:\n  active:\n    description: test\n"),
		0644,
	)

	// Now HasSchema should return true
	svc2 := workflow.NewService(festivalsDir)
	if !svc2.HasSchema() {
		t.Fatal("expected schema to exist after writing")
	}
}
