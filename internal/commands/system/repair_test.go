package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/workflow"
)

func TestRunRepair(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(t *testing.T, root string)
		expectRenames map[string]string
		expectMoves   map[string]string
		expectCreate  []string
		expectSchema  bool
		expectError   bool
	}{
		{
			name: "clean_directory_no_issues",
			setupFunc: func(t *testing.T, root string) {
				// Create proper structure with schema
				createProperStructure(t, root)
			},
			expectRenames: map[string]string{},
			expectMoves:   map[string]string{},
			expectCreate:  []string{},
			expectSchema:  false,
			expectError:   false,
		},
		{
			name: "old_planned_directory",
			setupFunc: func(t *testing.T, root string) {
				// Create old "planned/" directory
				mustMkdir(t, filepath.Join(root, "planned"))
			},
			expectRenames: map[string]string{"planned": "planning"},
			expectMoves:   map[string]string{},
			expectCreate:  []string{"ready", "active", "ritual", "dungeon/completed", "dungeon/archived", "dungeon/someday"},
			expectSchema:  true,
			expectError:   false,
		},
		{
			name: "old_completed_directory",
			setupFunc: func(t *testing.T, root string) {
				// Create old top-level "completed/" directory
				mustMkdir(t, filepath.Join(root, "completed"))
			},
			expectRenames: map[string]string{},
			expectMoves:   map[string]string{"completed": "dungeon/completed"},
			expectCreate:  []string{"planning", "ready", "active", "ritual", "dungeon/archived", "dungeon/someday"},
			expectSchema:  true,
			expectError:   false,
		},
		{
			name: "both_old_issues",
			setupFunc: func(t *testing.T, root string) {
				// Create both old patterns
				mustMkdir(t, filepath.Join(root, "planned"))
				mustMkdir(t, filepath.Join(root, "completed"))
			},
			expectRenames: map[string]string{"planned": "planning"},
			expectMoves:   map[string]string{"completed": "dungeon/completed"},
			expectCreate:  []string{"ready", "active", "ritual", "dungeon/archived", "dungeon/someday"},
			expectSchema:  true,
			expectError:   false,
		},
		{
			name: "missing_directories",
			setupFunc: func(t *testing.T, root string) {
				// Create only planning/ but missing others
				mustMkdir(t, filepath.Join(root, "planning"))
			},
			expectRenames: map[string]string{},
			expectMoves:   map[string]string{},
			expectCreate:  []string{"ready", "active", "ritual", "dungeon/completed", "dungeon/archived", "dungeon/someday"},
			expectSchema:  true,
			expectError:   false,
		},
		{
			name: "missing_schema_only",
			setupFunc: func(t *testing.T, root string) {
				// Create all directories but no schema
				mustMkdir(t, filepath.Join(root, "planning"))
				mustMkdir(t, filepath.Join(root, "ready"))
				mustMkdir(t, filepath.Join(root, "active"))
				mustMkdir(t, filepath.Join(root, "ritual"))
				mustMkdir(t, filepath.Join(root, "dungeon", "completed"))
				mustMkdir(t, filepath.Join(root, "dungeon", "archived"))
				mustMkdir(t, filepath.Join(root, "dungeon", "someday"))
			},
			expectRenames: map[string]string{},
			expectMoves:   map[string]string{},
			expectCreate:  []string{},
			expectSchema:  true,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for test
			tmpDir := t.TempDir()
			festivalsRoot := filepath.Join(tmpDir, "festivals")
			mustMkdir(t, festivalsRoot)

			// Setup test structure
			if tt.setupFunc != nil {
				tt.setupFunc(t, festivalsRoot)
			}

			// Change to festivals root for the test
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.Chdir(oldWd) }()
			if err := os.Chdir(festivalsRoot); err != nil {
				t.Fatal(err)
			}

			// Analyze repair
			ctx := context.Background()
			plan, err := analyzeRepair(ctx, festivalsRoot)
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify renames
			if len(plan.renameDirs) != len(tt.expectRenames) {
				t.Errorf("expected %d renames, got %d", len(tt.expectRenames), len(plan.renameDirs))
			}
			for old, new := range tt.expectRenames {
				if plan.renameDirs[old] != new {
					t.Errorf("expected rename %s → %s, got %s", old, new, plan.renameDirs[old])
				}
			}

			// Verify moves
			if len(plan.moveDirs) != len(tt.expectMoves) {
				t.Errorf("expected %d moves, got %d", len(tt.expectMoves), len(plan.moveDirs))
			}
			for src, dst := range tt.expectMoves {
				if plan.moveDirs[src] != dst {
					t.Errorf("expected move %s → %s, got %s", src, dst, plan.moveDirs[src])
				}
			}

			// Verify creates (order-independent check)
			if len(plan.createDirs) != len(tt.expectCreate) {
				t.Errorf("expected %d creates, got %d: %v", len(tt.expectCreate), len(plan.createDirs), plan.createDirs)
			} else {
				createMap := make(map[string]bool)
				for _, dir := range plan.createDirs {
					createMap[dir] = true
				}
				for _, expected := range tt.expectCreate {
					if !createMap[expected] {
						t.Errorf("expected to create %s but it wasn't in plan", expected)
					}
				}
			}

			// Verify schema creation
			if plan.createSchema != tt.expectSchema {
				t.Errorf("expected createSchema=%v, got %v", tt.expectSchema, plan.createSchema)
			}

			// Execute repair if there are changes
			if plan.hasIssues() {
				if err := executeRepair(ctx, festivalsRoot, plan); err != nil {
					t.Fatalf("executeRepair failed: %v", err)
				}

				// Verify all renames were applied
				for _, new := range tt.expectRenames {
					newPath := filepath.Join(festivalsRoot, new)
					if _, err := os.Stat(newPath); os.IsNotExist(err) {
						t.Errorf("rename target %s does not exist after repair", new)
					}
				}

				// Verify all moves were applied
				for _, dst := range tt.expectMoves {
					dstPath := filepath.Join(festivalsRoot, dst)
					if _, err := os.Stat(dstPath); os.IsNotExist(err) {
						t.Errorf("move destination %s does not exist after repair", dst)
					}
				}

				// Verify all creates were applied
				for _, dir := range tt.expectCreate {
					dirPath := filepath.Join(festivalsRoot, dir)
					if _, err := os.Stat(dirPath); os.IsNotExist(err) {
						t.Errorf("directory %s was not created", dir)
					}
				}

				// Verify schema was created if expected
				if tt.expectSchema {
					schemaPath := filepath.Join(festivalsRoot, workflow.SchemaFileName)
					if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
						t.Error("schema file was not created")
					}
				}
			}
		})
	}
}

func TestRunRepair_DungeonOrphans(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	createProperStructure(t, festivalsRoot)

	// Create orphan festivals directly in dungeon/ (not in any child dir)
	// These have fest.yaml to identify them as festivals
	orphan1 := filepath.Join(festivalsRoot, "dungeon", "api-integration-tests")
	orphan2 := filepath.Join(festivalsRoot, "dungeon", "fix-over-engineered-mess")
	mustMkdir(t, orphan1)
	mustMkdir(t, orphan2)
	mustWriteFile(t, filepath.Join(orphan1, "fest.yaml"), "name: API Integration Tests\n")
	mustWriteFile(t, filepath.Join(orphan2, "fest.yaml"), "name: Fix Over-Engineered Mess\n")

	ctx := context.Background()
	plan, err := analyzeRepair(ctx, festivalsRoot)
	if err != nil {
		t.Fatalf("analyzeRepair: %v", err)
	}

	// Should detect both orphans
	if len(plan.dungeonOrphans) != 2 {
		t.Fatalf("expected 2 dungeon orphans, got %d: %v", len(plan.dungeonOrphans), plan.dungeonOrphans)
	}

	// Verify hasIssues is true
	if !plan.hasIssues() {
		t.Error("expected hasIssues=true with dungeon orphans")
	}

	// Execute repair
	if err := executeRepair(ctx, festivalsRoot, plan); err != nil {
		t.Fatalf("executeRepair: %v", err)
	}

	// Verify orphans moved to dungeon/archived/
	for _, name := range []string{"api-integration-tests", "fix-over-engineered-mess"} {
		oldPath := filepath.Join(festivalsRoot, "dungeon", name)
		newPath := filepath.Join(festivalsRoot, "dungeon", "archived", name)
		if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
			t.Errorf("orphan %s still exists at old location", name)
		}
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			t.Errorf("orphan %s not found at new location dungeon/archived/%s", name, name)
		}
	}
}

func TestRunRepair_DungeonOrphans_PhaseDetection(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	createProperStructure(t, festivalsRoot)

	// Create orphan detected by numbered phase dirs (no fest.yaml)
	orphan := filepath.Join(festivalsRoot, "dungeon", "my-festival")
	mustMkdir(t, filepath.Join(orphan, "001_PLANNING"))
	mustMkdir(t, filepath.Join(orphan, "002_IMPLEMENTATION"))

	ctx := context.Background()
	plan, err := analyzeRepair(ctx, festivalsRoot)
	if err != nil {
		t.Fatalf("analyzeRepair: %v", err)
	}

	if len(plan.dungeonOrphans) != 1 {
		t.Fatalf("expected 1 dungeon orphan, got %d: %v", len(plan.dungeonOrphans), plan.dungeonOrphans)
	}
	if plan.dungeonOrphans[0] != "my-festival" {
		t.Errorf("expected orphan 'my-festival', got %q", plan.dungeonOrphans[0])
	}
}

func TestRunRepair_DungeonOrphans_FestDirDetection(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	createProperStructure(t, festivalsRoot)

	// Create orphan detected by .fest/ directory
	orphan := filepath.Join(festivalsRoot, "dungeon", "some-fest")
	mustMkdir(t, filepath.Join(orphan, ".fest"))

	ctx := context.Background()
	plan, err := analyzeRepair(ctx, festivalsRoot)
	if err != nil {
		t.Fatalf("analyzeRepair: %v", err)
	}

	if len(plan.dungeonOrphans) != 1 {
		t.Fatalf("expected 1 dungeon orphan, got %d", len(plan.dungeonOrphans))
	}
	if plan.dungeonOrphans[0] != "some-fest" {
		t.Errorf("expected orphan 'some-fest', got %q", plan.dungeonOrphans[0])
	}
}

func TestRunRepair_UnknownDungeonChildren(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	createProperStructure(t, festivalsRoot)

	// Create unknown category dir "future/" containing festival subdirs
	futureDir := filepath.Join(festivalsRoot, "dungeon", "future")
	mustMkdir(t, futureDir)
	// Put regular subdirs (not festivals) inside to make it a category dir
	mustMkdir(t, filepath.Join(futureDir, "some-idea"))
	mustMkdir(t, filepath.Join(futureDir, "another-idea"))

	ctx := context.Background()
	plan, err := analyzeRepair(ctx, festivalsRoot)
	if err != nil {
		t.Fatalf("analyzeRepair: %v", err)
	}

	// Should be flagged as unknown child, not an orphan
	if len(plan.unknownChildren) != 1 {
		t.Fatalf("expected 1 unknown child, got %d: %v", len(plan.unknownChildren), plan.unknownChildren)
	}
	if plan.unknownChildren[0] != "future" {
		t.Errorf("expected unknown child 'future', got %q", plan.unknownChildren[0])
	}
	if len(plan.dungeonOrphans) != 0 {
		t.Errorf("expected 0 dungeon orphans, got %d", len(plan.dungeonOrphans))
	}

	// Unknown children should NOT cause hasIssues to be true by themselves
	// (they're informational only)
	plan2 := &repairPlan{unknownChildren: []string{"future"}}
	if plan2.hasIssues() {
		t.Error("unknownChildren alone should not trigger hasIssues")
	}
}

func TestRunRepair_ProgressMigration(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	createProperStructure(t, festivalsRoot)

	// Create festivals with legacy progress.yaml in various locations
	fest1 := filepath.Join(festivalsRoot, "active", "my-festival")
	fest2 := filepath.Join(festivalsRoot, "dungeon", "completed", "old-festival")
	fest3 := filepath.Join(festivalsRoot, "planning", "new-festival")

	for _, festPath := range []string{fest1, fest2, fest3} {
		mustMkdir(t, filepath.Join(festPath, ".fest"))
		progressYAML := `festival: test
updated_at: 2025-01-01T00:00:00Z
tasks:
  001_PLANNING/01_task:
    task_id: 001_PLANNING/01_task
    status: completed
    progress: 100
`
		mustWriteFile(t, filepath.Join(festPath, ".fest", "progress.yaml"), progressYAML)
	}

	// Also create one festival that already has JSONL (should NOT be flagged)
	fest4 := filepath.Join(festivalsRoot, "active", "already-migrated")
	mustMkdir(t, filepath.Join(fest4, ".fest"))
	mustWriteFile(t, filepath.Join(fest4, ".fest", "progress.yaml"), "festival: test\n")
	mustWriteFile(t, filepath.Join(fest4, ".fest", "progress_events.jsonl"), "")

	ctx := context.Background()
	plan, err := analyzeRepair(ctx, festivalsRoot)
	if err != nil {
		t.Fatalf("analyzeRepair: %v", err)
	}

	// Should detect exactly 3 festivals needing migration
	if len(plan.progressMigrate) != 3 {
		t.Fatalf("expected 3 progress migrations, got %d: %v", len(plan.progressMigrate), plan.progressMigrate)
	}

	// Verify hasIssues is true
	if !plan.hasIssues() {
		t.Error("expected hasIssues=true with progress migrations")
	}

	// Execute repair
	if err := executeRepair(ctx, festivalsRoot, plan); err != nil {
		t.Fatalf("executeRepair: %v", err)
	}

	// Verify JSONL files were created and YAML files removed
	for _, festPath := range []string{fest1, fest2, fest3} {
		jsonlPath := filepath.Join(festPath, ".fest", "progress_events.jsonl")
		yamlPath := filepath.Join(festPath, ".fest", "progress.yaml")
		if _, err := os.Stat(jsonlPath); os.IsNotExist(err) {
			t.Errorf("progress_events.jsonl not created at %s", jsonlPath)
		}
		if _, err := os.Stat(yamlPath); !os.IsNotExist(err) {
			t.Errorf("progress.yaml still exists at %s (should be deleted after migration)", yamlPath)
		}
	}
}

func TestRunRepair_CombinedIssues(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	mustMkdir(t, festivalsRoot)

	// Old naming: planned/ instead of planning/
	mustMkdir(t, filepath.Join(festivalsRoot, "planned"))

	// Old layout: top-level completed/
	mustMkdir(t, filepath.Join(festivalsRoot, "completed"))

	// Dungeon with orphan festival and unknown child
	mustMkdir(t, filepath.Join(festivalsRoot, "dungeon"))
	orphan := filepath.Join(festivalsRoot, "dungeon", "orphan-fest")
	mustMkdir(t, orphan)
	mustWriteFile(t, filepath.Join(orphan, "fest.yaml"), "name: orphan\n")

	unknownDir := filepath.Join(festivalsRoot, "dungeon", "future")
	mustMkdir(t, filepath.Join(unknownDir, "sub-item"))

	// Legacy progress in the orphan
	mustMkdir(t, filepath.Join(orphan, ".fest"))
	mustWriteFile(t, filepath.Join(orphan, ".fest", "progress.yaml"), `festival: orphan
updated_at: 2025-01-01T00:00:00Z
tasks: {}
`)

	ctx := context.Background()
	plan, err := analyzeRepair(ctx, festivalsRoot)
	if err != nil {
		t.Fatalf("analyzeRepair: %v", err)
	}

	// Verify all issue types detected
	if len(plan.renameDirs) != 1 {
		t.Errorf("expected 1 rename, got %d", len(plan.renameDirs))
	}
	if len(plan.moveDirs) != 1 {
		t.Errorf("expected 1 move, got %d", len(plan.moveDirs))
	}
	if len(plan.dungeonOrphans) != 1 {
		t.Errorf("expected 1 dungeon orphan, got %d: %v", len(plan.dungeonOrphans), plan.dungeonOrphans)
	}
	if len(plan.unknownChildren) != 1 {
		t.Errorf("expected 1 unknown child, got %d: %v", len(plan.unknownChildren), plan.unknownChildren)
	}
	if len(plan.progressMigrate) != 1 {
		t.Errorf("expected 1 progress migration, got %d: %v", len(plan.progressMigrate), plan.progressMigrate)
	}
	if !plan.createSchema {
		t.Error("expected createSchema=true")
	}

	// Execute all repairs
	if err := executeRepair(ctx, festivalsRoot, plan); err != nil {
		t.Fatalf("executeRepair: %v", err)
	}

	// Verify planned/ renamed to planning/
	if _, err := os.Stat(filepath.Join(festivalsRoot, "planning")); os.IsNotExist(err) {
		t.Error("planning/ not created")
	}

	// Verify completed/ moved to dungeon/completed/
	if _, err := os.Stat(filepath.Join(festivalsRoot, "dungeon", "completed")); os.IsNotExist(err) {
		t.Error("dungeon/completed/ not created")
	}

	// Verify orphan moved to dungeon/archived/
	archivedOrphan := filepath.Join(festivalsRoot, "dungeon", "archived", "orphan-fest")
	if _, err := os.Stat(archivedOrphan); os.IsNotExist(err) {
		t.Error("orphan not moved to dungeon/archived/")
	}

	// Verify progress migrated (JSONL should exist at new location)
	jsonlPath := filepath.Join(archivedOrphan, ".fest", "progress_events.jsonl")
	if _, err := os.Stat(jsonlPath); os.IsNotExist(err) {
		t.Error("progress_events.jsonl not created for orphan festival")
	}

	// Verify schema created
	if _, err := os.Stat(filepath.Join(festivalsRoot, workflow.SchemaFileName)); os.IsNotExist(err) {
		t.Error(".workflow.yaml not created")
	}
}

func TestLooksLikeFestival(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, dir string)
		expected bool
	}{
		{
			name: "has_fest_yaml",
			setup: func(t *testing.T, dir string) {
				mustWriteFile(t, filepath.Join(dir, "fest.yaml"), "name: test\n")
			},
			expected: true,
		},
		{
			name: "has_fest_dir",
			setup: func(t *testing.T, dir string) {
				mustMkdir(t, filepath.Join(dir, ".fest"))
			},
			expected: true,
		},
		{
			name: "has_numbered_phases",
			setup: func(t *testing.T, dir string) {
				mustMkdir(t, filepath.Join(dir, "001_PLANNING"))
			},
			expected: true,
		},
		{
			name: "empty_directory",
			setup: func(t *testing.T, dir string) {
				// nothing
			},
			expected: false,
		},
		{
			name: "has_regular_subdirs",
			setup: func(t *testing.T, dir string) {
				mustMkdir(t, filepath.Join(dir, "sub-item"))
				mustMkdir(t, filepath.Join(dir, "another"))
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "test-dir")
			mustMkdir(t, dir)
			tt.setup(t, dir)
			got := looksLikeFestival(dir)
			if got != tt.expected {
				t.Errorf("looksLikeFestival = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRunRepair_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	mustMkdir(t, festivalsRoot)

	// Create old structure
	mustMkdir(t, filepath.Join(festivalsRoot, "planned"))

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Analyze should fail with context error
	_, err := analyzeRepair(ctx, festivalsRoot)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestRunRepair_ExecuteContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	festivalsRoot := filepath.Join(tmpDir, "festivals")
	mustMkdir(t, festivalsRoot)

	// Create repair plan
	plan := &repairPlan{
		festivalsRoot: festivalsRoot,
		createDirs:    []string{"planning"},
	}

	// Create context with timeout that will expire
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for context to expire
	<-ctx.Done()

	// Execute should fail with context error
	err := executeRepair(ctx, festivalsRoot, plan)
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded error, got %v", err)
	}
}

// Helper functions

func createProperStructure(t *testing.T, root string) {
	t.Helper()

	// Create FestivalSchema structure
	schema := workflow.FestivalSchema()
	for _, dir := range schema.AllDirectories() {
		mustMkdir(t, filepath.Join(root, dir))
	}

	// Create schema file
	data := []byte(`version: 1
type: status-workflow
name: Festival Workflow
description: Status workflow for festival lifecycle management
default_status: planning
track_history: true
history_file: .workflow-history.jsonl
directories:
  planning:
    description: Festivals being designed and documented
    order: 1
  ready:
    description: Planned and ready for execution
    order: 2
  active:
    description: Festivals currently being executed
    order: 3
  ritual:
    description: Repeatable festival templates
    order: 4
  dungeon:
    description: Terminal festival states
    order: 5
    nested: true
    children:
      completed:
        description: Successfully finished festivals
        order: 1
      archived:
        description: Preserved but no longer active
        order: 2
      someday:
        description: Backlog — nice-to-have work
        order: 3
`)
	schemaPath := filepath.Join(root, workflow.SchemaFileName)
	if err := os.WriteFile(schemaPath, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}
